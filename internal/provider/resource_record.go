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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-powerdns/internal/api/auth"
	"github.com/ioplane/terraform-provider-powerdns/internal/api/transport"
	"github.com/ioplane/terraform-provider-powerdns/internal/provider/normalise"
	"github.com/ioplane/terraform-provider-powerdns/internal/provider/planmodify"
)

var (
	_ resource.Resource                = (*recordResource)(nil)
	_ resource.ResourceWithIdentity    = (*recordResource)(nil)
	_ resource.ResourceWithConfigure   = (*recordResource)(nil)
	_ resource.ResourceWithImportState = (*recordResource)(nil)
)

// recordResource manages one RRSet.
//
// # Why this is an RRSet and not a record
//
// PowerDNS has no per-record identity. The addressable unit is the RRSet —
// every record sharing an owner name and a type — and a PATCH replaces the set
// wholesale. There is no way to add one A record to a name without sending
// every A record that name should end up with.
//
// So a resource modelling a single record would have to read the set, splice
// itself in, and write it back on every change. Two such resources touching
// the same name would then race: each would read a set without the other's
// record, and whichever applied second would delete the first. That failure is
// silent, survives a plan, and shows up as a record that vanishes.
//
// Modelling the RRSet directly makes the ownership explicit instead. One
// resource owns one name-and-type pair, and Terraform's own graph prevents two
// resources from claiming the same one.
type recordResource struct {
	clients *Clients
}

// NewRecordResource returns the resource factory.
func NewRecordResource() resource.Resource {
	return &recordResource{}
}

type recordModel struct {
	ID     types.String `tfsdk:"id"`
	Zone   types.String `tfsdk:"zone"`
	Name   types.String `tfsdk:"name"`
	Type   types.String `tfsdk:"type"`
	TTL    types.Int64  `tfsdk:"ttl"`
	Values types.List   `tfsdk:"values"`

	Disabled types.Bool   `tfsdk:"disabled"`
	Comment  types.String `tfsdk:"comment"`
}

func (r *recordResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_record"
}

func (r *recordResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An RRSet: every record sharing an owner name and a type.\n\n" +
			"PowerDNS has no per-record identity — a PATCH replaces the whole set — so " +
			"this resource owns a name-and-type pair rather than a single record. Two " +
			"resources managing the same pair would overwrite each other silently, " +
			"which is why `values` is a list rather than each record being its own " +
			"resource.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "`<zone>/<name>/<type>`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"zone": schema.StringAttribute{
				MarkdownDescription: "The zone holding this RRSet.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					planmodify.SemanticString(
						"compared as a DNS name", normalise.DNSName),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The owner name, fully qualified. PowerDNS lowercases " +
					"it, so `WWW.example.com.` and `www.example.com.` are the same name " +
					"and do not produce a diff.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					planmodify.SemanticString(
						"compared as a DNS name: case and a trailing dot do not matter",
						normalise.RecordName),
				},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The record type, such as `A`, `AAAA`, `MX` or `TXT`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ttl": schema.Int64Attribute{
				MarkdownDescription: "Time to live, in seconds. One TTL applies to the " +
					"whole set: DNS has no per-record TTL within an RRSet.",
				Required: true,
			},
			"values": schema.ListAttribute{
				MarkdownDescription: "The record values, one per record in the set. " +
					"Address values are compared numerically, so an IPv6 address written " +
					"uncompressed does not produce a permanent diff; every other type is " +
					"compared exactly, because a TXT record's quoting is significant.",
				Required:    true,
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				PlanModifiers: []planmodifier.List{
					recordValuesModifier{},
				},
			},
			"disabled": schema.BoolAttribute{
				MarkdownDescription: "Keep the set in the zone without serving it. " +
					"Survives a round trip, so a disabled record stays disabled rather " +
					"than being silently re-enabled on the next apply.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"comment": schema.StringAttribute{
				MarkdownDescription: "A comment attached to the set.",
				Optional:            true,
			},
		},
	}
}

func (r *recordResource) Configure(
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

func (r *recordResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan recordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(r.write(ctx, "creating", &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.identityFor(ctx, resp.Identity, plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *recordResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state recordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_record")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneID := canonicalName(state.Zone.ValueString())
	zone, err := client.GetZone(ctx, zoneID)
	if err != nil {
		// The zone going away takes its records with it.
		if errors.Is(err, transport.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading the zone holding this record",
			"Zone "+zoneID+".\n\n"+err.Error(),
		)
		return
	}

	// There is no endpoint for one RRSet: reading a record means reading the
	// zone and finding it. That is the API's shape, not a shortcut.
	found := findRRSet(zone.RRSets, state.Name.ValueString(), state.Type.ValueString())
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(applyRRSet(ctx, found, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.identityFor(ctx, resp.Identity, state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *recordResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan recordModel
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

func (r *recordResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state recordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_record")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// DELETE ignores the records, so the set is identified by name and type
	// alone.
	rrset := auth.RRSet{
		Name:       canonicalName(state.Name.ValueString()),
		Type:       state.Type.ValueString(),
		ChangeType: auth.ChangeTypeDelete,
	}

	zoneID := canonicalName(state.Zone.ValueString())
	err := client.PatchRRSets(ctx, zoneID, []auth.RRSet{rrset})
	if err != nil && !errors.Is(err, transport.ErrNotFound) {
		resp.Diagnostics.Append(recordDiagnostic("deleting", rrset, err))
	}
}

// ImportState takes `<zone>/<name>/<type>`.
func (r *recordResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	// An import block carries the identity instead of an id. Three attributes,
	// no parsing — which is the point of having an identity at all.
	if req.ID == "" {
		var zone, name, recordType types.String
		resp.Diagnostics.Append(
			req.Identity.GetAttribute(ctx, path.Root("zone_name"), &zone)...)
		resp.Diagnostics.Append(
			req.Identity.GetAttribute(ctx, path.Root("record_name"), &name)...)
		resp.Diagnostics.Append(
			req.Identity.GetAttribute(ctx, path.Root("record_type"), &recordType)...)
		if resp.Diagnostics.HasError() {
			return
		}

		zoneID, recordName := canonicalName(zone.ValueString()),
			canonicalName(name.ValueString())

		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"),
			recordID(zoneID, recordName, recordType.ValueString()))...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone"), zoneID)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), recordName)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("type"),
			recordType.ValueString())...)
		resp.Diagnostics.Append(setRecordIdentity(ctx, resp.Identity,
			zoneID, recordName, recordType.ValueString())...)
		return
	}

	parts := strings.Split(req.ID, "/")
	if len(parts) != recordIDParts {
		resp.Diagnostics.AddError(
			"Malformed import id",
			fmt.Sprintf("Expected `<zone>/<name>/<type>`, for example "+
				"`example.com./www.example.com./A`. Got %q.", req.ID),
		)
		return
	}

	zone, name, recordType := canonicalName(parts[0]), canonicalName(parts[1]), parts[2]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"),
		recordID(zone, name, recordType))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone"), zone)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("type"), recordType)...)
	resp.Diagnostics.Append(setRecordIdentity(ctx, resp.Identity, zone, name, recordType)...)
}

// write is Create and Update, which are the same request.
//
// A PATCH with changetype REPLACE writes the set as given whether or not it
// existed, so there is nothing for an update to do differently. Keeping them
// as one function means the two cannot drift apart — the failure that
// produces a resource which creates correctly and updates subtly wrong.
func (r *recordResource) write(
	ctx context.Context,
	action string,
	plan *recordModel,
) diag.Diagnostics {
	var diags diag.Diagnostics

	client, d := r.clients.RequireAuth("powerdns_record")
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}

	rrset, d := rrsetFromModel(ctx, *plan, auth.ChangeTypeReplace)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}

	zoneID := canonicalName(plan.Zone.ValueString())
	if err := client.PatchRRSets(ctx, zoneID, []auth.RRSet{rrset}); err != nil {
		diags.Append(recordDiagnostic(action, rrset, err))
		return diags
	}

	plan.ID = types.StringValue(recordID(zoneID, rrset.Name, rrset.Type))
	return diags
}

// identityFor is the RRSet's identity: zone, name and type as three
// attributes, not one parsed string.
func (r *recordResource) identityFor(
	ctx context.Context,
	identity *tfsdk.ResourceIdentity,
	model recordModel,
) diag.Diagnostics {
	return setRecordIdentity(ctx, identity,
		canonicalName(model.Zone.ValueString()),
		canonicalName(model.Name.ValueString()),
		model.Type.ValueString())
}
