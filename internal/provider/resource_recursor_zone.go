package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-powerdns/internal/api/rec"
	"github.com/ioplane/terraform-provider-powerdns/internal/api/transport"
	"github.com/ioplane/terraform-provider-powerdns/internal/provider/normalise"
	"github.com/ioplane/terraform-provider-powerdns/internal/provider/planmodify"
)

var (
	_ resource.Resource                = (*recursorZoneResource)(nil)
	_ resource.ResourceWithIdentity    = (*recursorZoneResource)(nil)
	_ resource.ResourceWithConfigure   = (*recursorZoneResource)(nil)
	_ resource.ResourceWithImportState = (*recursorZoneResource)(nil)
)

// recursorZoneResource manages a Recursor zone.
//
// A Recursor zone shares a name with an Authoritative one and nothing else.
// There is no DNSSEC, no metadata, no SOA: it is either a forward instruction
// or a small authoritative island the Recursor serves itself.
//
// Every write needs `webservice.api_dir`. Without it the Recursor is read-only
// and answers 422 naming the setting, which the transport classifies so the
// diagnostic says what to configure.
type recursorZoneResource struct {
	clients *Clients
}

// NewRecursorZoneResource returns the resource factory.
func NewRecursorZoneResource() resource.Resource {
	return &recursorZoneResource{}
}

type recursorZoneModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	Kind types.String `tfsdk:"kind"`

	Servers          types.List `tfsdk:"servers"`
	RecursionDesired types.Bool `tfsdk:"recursion_desired"`
	NotifyAllowed    types.Bool `tfsdk:"notify_allowed"`
}

func (r *recursorZoneResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_recursor_zone"
}

func (r *recursorZoneResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A zone on the PowerDNS Recursor.\n\n" +
			"Shares a name with `powerdns_zone` and nothing else: a Recursor zone is " +
			"either a forward instruction or a small authoritative island, with no " +
			"DNSSEC, metadata or SOA.\n\n" +
			"**Every write needs `webservice.api_dir`.** Without it the Recursor is " +
			"read-only and this provider reports the setting rather than the status " +
			"code.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The canonical zone name.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The zone name.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					planmodify.SemanticString(
						"compared as a DNS name", normalise.DNSName),
				},
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "`Forwarded` to ask `servers`, or `Native` to " +
					"answer from records held locally.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOfCaseInsensitive(rec.KindNative, rec.KindForwarded),
				},
				PlanModifiers: []planmodifier.String{
					planmodify.SemanticString(
						"compared case-insensitively", normalise.ZoneKind),
				},
			},
			"servers": schema.ListAttribute{
				MarkdownDescription: "Upstreams for a `Forwarded` zone, as `host` or " +
					"`host:port`.\n\n" +
					"The Recursor appends `:53` to an address given without one, so " +
					"`192.0.2.53` and `192.0.2.53:53` are the same upstream and do not " +
					"produce a diff. A different port is a real change.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					planmodify.SemanticSet(
						"compared as upstreams, defaulting the port to 53 and ignoring order",
						normalise.UpstreamServer),
				},
			},
			"recursion_desired": schema.BoolAttribute{
				MarkdownDescription: "Set the RD bit on forwarded queries. Wanted when " +
					"forwarding to a resolver, not when forwarding to an authoritative " +
					"server — getting it wrong produces answers that look like a broken " +
					"upstream.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"notify_allowed": schema.BoolAttribute{
				MarkdownDescription: "Accept NOTIFY for this zone.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
}

func (r *recursorZoneResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	r.clients = configureResourceClients(req, resp)
}

func (r *recursorZoneResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan recursorZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireRecursor("powerdns_recursor_zone")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone, diags := recursorZoneFromModel(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := client.CreateZone(ctx, zone)
	if err != nil {
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error creating the Recursor zone", zone.Name, err))
		return
	}

	plan.ID = types.StringValue(created.ID)
	resp.Diagnostics.Append(setZoneIdentity(ctx, resp.Identity, created.ID)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *recursorZoneResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state recursorZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireRecursor("powerdns_recursor_zone")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone, err := client.GetZone(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, transport.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error reading the Recursor zone", state.ID.ValueString(), err))
		return
	}

	state.Kind = types.StringValue(zone.Kind)
	state.RecursionDesired = types.BoolValue(derefBool(zone.RecursionDesired))
	state.NotifyAllowed = types.BoolValue(derefBool(zone.NotifyAllowed))

	servers, d := stringListValue(ctx, zone.Servers)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Servers = servers

	resp.Diagnostics.Append(setZoneIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *recursorZoneResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan, state recursorZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireRecursor("powerdns_recursor_zone")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone, diags := recursorZoneFromModel(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A PUT replaces the zone outright rather than merging, so everything the
	// resource owns has to be in the body.
	zoneID := state.ID.ValueString()
	if err := client.UpdateZone(ctx, zoneID, zone); err != nil {
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error updating the Recursor zone", zoneID, err))
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *recursorZoneResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state recursorZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireRecursor("powerdns_recursor_zone")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := client.DeleteZone(ctx, state.ID.ValueString())
	if err != nil && !errors.Is(err, transport.ErrNotFound) {
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error deleting the Recursor zone", state.ID.ValueString(), err))
	}
}

// ImportState takes the canonical zone name.
func (r *recursorZoneResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	id := canonicalName(req.ID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), id)...)
}

// recursorZoneFromModel turns the model into a request body.
//
// Records are absent deliberately. A Native Recursor zone can hold them, but
// mixing "this zone forwards" and "this zone serves these records" into one
// resource makes a schema where half the attributes are meaningless for either
// kind. Records for a Native Recursor zone are a later resource if anybody
// wants them.
func recursorZoneFromModel(
	ctx context.Context,
	model recursorZoneModel,
) (rec.Zone, diag.Diagnostics) {
	var diags diag.Diagnostics

	zone := rec.Zone{
		Name: canonicalName(model.Name.ValueString()),
		Kind: model.Kind.ValueString(),
	}

	if !model.RecursionDesired.IsNull() && !model.RecursionDesired.IsUnknown() {
		zone.RecursionDesired = model.RecursionDesired.ValueBoolPointer()
	}
	if !model.NotifyAllowed.IsNull() && !model.NotifyAllowed.IsUnknown() {
		zone.NotifyAllowed = model.NotifyAllowed.ValueBoolPointer()
	}

	diags.Append(elementsAs(ctx, model.Servers, &zone.Servers)...)
	return zone, diags
}
