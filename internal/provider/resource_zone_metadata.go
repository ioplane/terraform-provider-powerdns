package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-powerdns/internal/api/auth"
	"github.com/ioplane/terraform-provider-powerdns/internal/api/transport"
	"github.com/ioplane/terraform-provider-powerdns/internal/provider/normalise"
	"github.com/ioplane/terraform-provider-powerdns/internal/provider/planmodify"
)

var (
	_ resource.Resource                = (*zoneMetadataResource)(nil)
	_ resource.ResourceWithIdentity    = (*zoneMetadataResource)(nil)
	_ resource.ResourceWithConfigure   = (*zoneMetadataResource)(nil)
	_ resource.ResourceWithImportState = (*zoneMetadataResource)(nil)
)

// zoneMetadataResource manages one metadata kind on a zone.
//
// # Why one kind per resource
//
// A resource owning the whole metadata collection would have to delete
// anything it did not recognise, and PowerDNS puts `SOA-EDIT-API` there
// itself: every zone is created with it, unasked. Such a resource would try to
// remove it on the first apply, on every zone, for ever.
//
// Owning one kind sidesteps that entirely. What the provider does not manage,
// it does not touch — which is also the right answer for a server that may
// have metadata set by pdnsutil or by another tool.
type zoneMetadataResource struct {
	clients *Clients
}

// NewZoneMetadataResource returns the resource factory.
func NewZoneMetadataResource() resource.Resource {
	return &zoneMetadataResource{}
}

type zoneMetadataModel struct {
	ID     types.String `tfsdk:"id"`
	Zone   types.String `tfsdk:"zone"`
	Kind   types.String `tfsdk:"kind"`
	Values types.List   `tfsdk:"values"`
}

func (r *zoneMetadataResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_zone_metadata"
}

func (r *zoneMetadataResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "One metadata kind on a zone, such as `ALLOW-AXFR-FROM` " +
			"or `NOTIFY-DNSUPDATE`.\n\n" +
			"One kind per resource, deliberately. PowerDNS sets `SOA-EDIT-API` on every " +
			"zone it creates, so a resource owning the whole collection would try to " +
			"delete it on every apply. What this provider does not manage, it does not " +
			"touch.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "`<zone>/<kind>`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"zone": schema.StringAttribute{
				MarkdownDescription: "The zone this metadata belongs to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					planmodify.SemanticString(
						"compared as a DNS name", normalise.DNSName),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "The metadata kind. PowerDNS reserves the names it " +
					"documents and accepts an `X-` prefix for anything else.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"values": schema.ListAttribute{
				MarkdownDescription: "The values for this kind. Every kind is multi-valued " +
					"in the API, even the ones that only ever hold one item.",
				Required:    true,
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
		},
	}
}

func (r *zoneMetadataResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	if req.ProviderData == nil {
		return
	}

	clients, ok := req.ProviderData.(*Clients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("Expected *Clients, got %T. This is a bug in the provider.",
				req.ProviderData),
		)
		return
	}
	r.clients = clients
}

func (r *zoneMetadataResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan zoneMetadataModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(r.write(ctx, "creating", &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(setMetadataIdentity(ctx, resp.Identity,
		canonicalName(plan.Zone.ValueString()), plan.Kind.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *zoneMetadataResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state zoneMetadataModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_zone_metadata")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneID := canonicalName(state.Zone.ValueString())
	entry, err := client.GetMetadata(ctx, zoneID, state.Kind.ValueString())
	if err != nil {
		if errors.Is(err, transport.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading the zone metadata",
			state.Kind.ValueString()+" on "+zoneID+".\n\n"+err.Error(),
		)
		return
	}

	// An unset kind is 200 with an empty list, not 404: absence and emptiness
	// are the same state here. Verified against auth-5.1.3. So an empty read
	// is how a kind deleted outside Terraform presents, and removing the
	// resource is what offers to recreate it.
	if len(entry.Metadata) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	values, d := types.ListValueFrom(ctx, types.StringType, entry.Metadata)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Values = values

	resp.Diagnostics.Append(setMetadataIdentity(ctx, resp.Identity,
		zoneID, state.Kind.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *zoneMetadataResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan zoneMetadataModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(r.write(ctx, "updating", &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *zoneMetadataResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state zoneMetadataModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_zone_metadata")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneID := canonicalName(state.Zone.ValueString())
	err := client.DeleteMetadata(ctx, zoneID, state.Kind.ValueString())
	if err != nil && !errors.Is(err, transport.ErrNotFound) {
		resp.Diagnostics.AddError(
			"Error deleting the zone metadata",
			state.Kind.ValueString()+" on "+zoneID+".\n\n"+err.Error(),
		)
	}
}

// ImportState takes `<zone>/<kind>`.
func (r *zoneMetadataResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	zone, kind, found := strings.Cut(req.ID, "/")
	if !found || zone == "" || kind == "" {
		resp.Diagnostics.AddError(
			"Malformed import id",
			fmt.Sprintf("Expected `<zone>/<kind>`, for example "+
				"`example.com./ALLOW-AXFR-FROM`. Got %q.", req.ID),
		)
		return
	}

	zone = canonicalName(zone)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), zone+"/"+kind)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone"), zone)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("kind"), kind)...)
}

// write is Create and Update.
//
// PUT replaces a kind's values outright, so there is nothing an update does
// differently — the same reasoning as powerdns_record. The API also offers
// POST, which appends; this resource never uses it, because appending is not
// something a declarative configuration can express.
func (r *zoneMetadataResource) write(
	ctx context.Context,
	action string,
	plan *zoneMetadataModel,
) diag.Diagnostics {
	var diags diag.Diagnostics

	client, d := r.clients.RequireAuth("powerdns_zone_metadata")
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}

	kind := plan.Kind.ValueString()
	diags.Append(checkMetadataKind(kind)...)
	if diags.HasError() {
		return diags
	}

	var values []string
	diags.Append(elementsAs(ctx, plan.Values, &values)...)
	if diags.HasError() {
		return diags
	}

	zoneID := canonicalName(plan.Zone.ValueString())

	_, err := client.SetMetadata(ctx, zoneID, auth.Metadata{Kind: kind, Metadata: values})
	if err != nil {
		diags.AddError(
			"Error "+action+" the zone metadata",
			kind+" on "+zoneID+".\n\n"+err.Error(),
		)
		return diags
	}

	plan.ID = types.StringValue(zoneID + "/" + kind)
	return diags
}
