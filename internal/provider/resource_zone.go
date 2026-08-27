package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
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
	_ resource.Resource                = (*zoneResource)(nil)
	_ resource.ResourceWithIdentity    = (*zoneResource)(nil)
	_ resource.ResourceWithConfigure   = (*zoneResource)(nil)
	_ resource.ResourceWithImportState = (*zoneResource)(nil)
)

// zoneResource manages an Authoritative zone.
//
// Records are deliberately not part of this resource. PowerDNS has no
// per-record identity — the unit of change is the RRSet — and a zone resource
// that also owned records would fight every powerdns_record in the same
// configuration for ownership of the same rrsets endpoint.
//
// The one exception is creation: a zone must be created with its nameservers,
// because PowerDNS generates the SOA and NS records at that moment and will
// not do so later. That is why `nameservers` is create-only.
type zoneResource struct {
	clients *Clients
}

// NewZoneResource returns the resource factory.
func NewZoneResource() resource.Resource {
	return &zoneResource{}
}

// zoneModel mirrors the schema.
type zoneModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	Kind types.String `tfsdk:"kind"`

	Masters     types.List   `tfsdk:"masters"`
	Nameservers types.List   `tfsdk:"nameservers"`
	Account     types.String `tfsdk:"account"`
	Catalog     types.String `tfsdk:"catalog"`

	SOAEdit    types.String `tfsdk:"soa_edit"`
	SOAEditAPI types.String `tfsdk:"soa_edit_api"`
	APIRectify types.Bool   `tfsdk:"api_rectify"`

	DNSSEC      types.Bool   `tfsdk:"dnssec"`
	NSEC3Param  types.String `tfsdk:"nsec3param"`
	NSEC3Narrow types.Bool   `tfsdk:"nsec3narrow"`
	Presigned   types.Bool   `tfsdk:"presigned"`

	MasterTSIGKeyIDs types.List `tfsdk:"master_tsig_key_ids"`
	SlaveTSIGKeyIDs  types.List `tfsdk:"slave_tsig_key_ids"`

	Serial       types.Int64 `tfsdk:"serial"`
	EditedSerial types.Int64 `tfsdk:"edited_serial"`
}

func (r *zoneResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_zone"
}

func (r *zoneResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A zone on the PowerDNS Authoritative Server.\n\n" +
			"Records are managed by `powerdns_record`, not here: PowerDNS has no " +
			"per-record identity and the unit of change is the RRSet. The exception " +
			"is `nameservers`, which PowerDNS uses once at creation to generate the " +
			"SOA and NS records and cannot apply later.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The canonical zone name, which is what PowerDNS " +
					"uses as the primary key.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The zone name. A trailing dot is added if absent, " +
					"because PowerDNS stores the canonical form.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					planmodify.SemanticString(
						"compared as a DNS name: case and a trailing dot do not matter",
						normalise.DNSName),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "One of `Native`, `Master`, `Slave`, `Producer` or " +
					"`Consumer`. PowerDNS title-cases what it is given, so `native` and " +
					"`Native` are the same value and do not produce a diff.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOfCaseInsensitive(
						auth.KindNative, auth.KindMaster, auth.KindSlave,
						auth.KindProducer, auth.KindConsumer,
					),
				},
				PlanModifiers: []planmodifier.String{
					planmodify.SemanticString(
						"compared case-insensitively: the server title-cases it",
						normalise.ZoneKind),
				},
			},
			"masters": schema.ListAttribute{
				MarkdownDescription: "Primary servers for a `Slave` zone. Compared by " +
					"address value and ignoring order, so an IPv6 address written " +
					"uncompressed does not produce a permanent diff.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					planmodify.SemanticSet(
						"compared by address value, ignoring order",
						normalise.IPAddressKey),
				},
			},
			"nameservers": schema.ListAttribute{
				MarkdownDescription: "Nameservers used **only at creation**, to generate " +
					"the SOA and NS records. PowerDNS ignores them afterwards, so " +
					"changing this replaces the zone. Manage NS records with " +
					"`powerdns_record` instead.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listRequiresReplace(),
				},
			},
			"account": schema.StringAttribute{
				MarkdownDescription: "Free-form owner label, used by some deployments " +
					"for delegation.",
				Optional: true,
			},
			"catalog": schema.StringAttribute{
				MarkdownDescription: "The catalog zone this zone belongs to.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					planmodify.SemanticString(
						"compared as a DNS name", normalise.DNSName),
				},
			},
			"soa_edit": schema.StringAttribute{
				MarkdownDescription: "SOA-EDIT policy applied when serving the zone.",
				Optional:            true,
			},
			"soa_edit_api": schema.StringAttribute{
				MarkdownDescription: "SOA-EDIT-API policy applied when the API changes " +
					"the zone. **PowerDNS assigns `DEFAULT` at creation** whether or not " +
					"it was asked for, so this is computed: leaving it unset adopts what " +
					"the server chose rather than fighting it every plan.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"api_rectify": schema.BoolAttribute{
				MarkdownDescription: "Rectify the zone automatically after every API " +
					"change.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"dnssec": schema.BoolAttribute{
				MarkdownDescription: "Whether the zone is signed.\n\n" +
					"Computed rather than defaulted to `false`, because adding a " +
					"`powerdns_zone_cryptokey` turns it on server-side. A default would " +
					"make the zone plan to switch it back off on the next run, and the " +
					"two resources would fight for ever.\n\n" +
					"Set it explicitly to sign a zone with a server-generated CSK; leave " +
					"it unset and manage keys with `powerdns_zone_cryptokey`.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"nsec3param": schema.StringAttribute{
				MarkdownDescription: "NSEC3 parameters as `<algorithm> <flags> " +
					"<iterations> <salt>`, for example `1 0 0 ab`. Empty means NSEC " +
					"rather than NSEC3.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"nsec3narrow": schema.BoolAttribute{
				MarkdownDescription: "Narrow NSEC3: compute denial of existence on the " +
					"fly rather than storing it. Meaningful only with `nsec3param` set.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"presigned": schema.BoolAttribute{
				MarkdownDescription: "The zone arrives already signed, so PowerDNS " +
					"serves the signatures it was given rather than making its own.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"master_tsig_key_ids": schema.ListAttribute{
				MarkdownDescription: "TSIG keys used to sign outgoing AXFR requests, " +
					"for a `Slave` zone. Each is a key id — the canonical name with a " +
					"trailing dot, as `powerdns_tsigkey.id` reports it.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					planmodify.SemanticSet(
						"compared as canonical key names, ignoring order",
						normalise.TSIGKeyIDKey),
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"slave_tsig_key_ids": schema.ListAttribute{
				MarkdownDescription: "TSIG keys accepted on incoming AXFR requests, for " +
					"a zone this server is primary for.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					planmodify.SemanticSet(
						"compared as canonical key names, ignoring order",
						normalise.TSIGKeyIDKey),
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"serial": schema.Int64Attribute{
				MarkdownDescription: "The zone's SOA serial, as stored.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					// Without this a Computed attribute plans as "known after
					// apply" on every run, which makes the plan non-empty even
					// when nothing changed. The serial does move whenever the
					// zone is edited, and that is picked up on the next read.
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"edited_serial": schema.Int64Attribute{
				MarkdownDescription: "The serial a query would see, after SOA-EDIT is " +
					"applied. Differs from `serial` whenever `soa_edit` is set.",
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *zoneResource) Configure(
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

func (r *zoneResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan zoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_zone")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone := auth.Zone{
		Name:       canonicalName(plan.Name.ValueString()),
		Kind:       plan.Kind.ValueString(),
		Account:    plan.Account.ValueString(),
		Catalog:    plan.Catalog.ValueString(),
		SOAEdit:    plan.SOAEdit.ValueString(),
		SOAEditAPI: plan.SOAEditAPI.ValueString(),
	}
	if !plan.APIRectify.IsNull() {
		zone.APIRectify = plan.APIRectify.ValueBoolPointer()
	}
	if !plan.DNSSEC.IsNull() && !plan.DNSSEC.IsUnknown() {
		zone.DNSSEC = plan.DNSSEC.ValueBoolPointer()
	}
	applyDNSSECAttributes(plan, &zone)
	resp.Diagnostics.Append(applyTSIGKeyLists(ctx, plan, &zone)...)

	resp.Diagnostics.Append(elementsAs(ctx, plan.Masters, &zone.Masters)...)
	// Nameservers are not a Zone field: PowerDNS takes them as a create-only
	// argument alongside the zone, so they travel in the request body and
	// never appear in a read.
	var nameservers []string
	resp.Diagnostics.Append(elementsAs(ctx, plan.Nameservers, &nameservers)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := client.CreateZoneWithNameservers(ctx, zone, nameservers)
	if err != nil {
		resp.Diagnostics.Append(zoneDiagnostic("creating", zone.Name, err))
		return
	}

	resp.Diagnostics.Append(applyZone(ctx, created, &plan, afterWrite)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(setZoneIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *zoneResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state zoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_zone")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone, err := client.GetZone(ctx, state.ID.ValueString())
	if err != nil {
		// A zone deleted outside Terraform is not an error: removing it from
		// state is how the next plan offers to recreate it.
		if errors.Is(err, transport.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(zoneDiagnostic("reading", state.ID.ValueString(), err))
		return
	}

	resp.Diagnostics.Append(applyZone(ctx, zone, &state, afterRead)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Set on Read as well as Create: a resource created before this provider
	// grew identities has none until something writes one.
	resp.Diagnostics.Append(setZoneIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *zoneResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan, state zoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_zone")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneID := state.ID.ValueString()
	zone := auth.Zone{
		Kind:       plan.Kind.ValueString(),
		Account:    plan.Account.ValueString(),
		Catalog:    plan.Catalog.ValueString(),
		SOAEdit:    plan.SOAEdit.ValueString(),
		SOAEditAPI: plan.SOAEditAPI.ValueString(),
	}
	if !plan.APIRectify.IsNull() {
		zone.APIRectify = plan.APIRectify.ValueBoolPointer()
	}
	if !plan.DNSSEC.IsNull() && !plan.DNSSEC.IsUnknown() {
		zone.DNSSEC = plan.DNSSEC.ValueBoolPointer()
	}
	applyDNSSECAttributes(plan, &zone)
	resp.Diagnostics.Append(applyTSIGKeyLists(ctx, plan, &zone)...)
	resp.Diagnostics.Append(elementsAs(ctx, plan.Masters, &zone.Masters)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := client.UpdateZone(ctx, zoneID, zone); err != nil {
		resp.Diagnostics.Append(zoneDiagnostic("updating", zoneID, err))
		return
	}

	// The PUT answers 204, so the result of any normalisation is only visible
	// on a re-read. Setting state from the plan instead would store what was
	// asked for rather than what exists.
	updated, err := client.GetZone(ctx, zoneID)
	if err != nil {
		resp.Diagnostics.Append(zoneDiagnostic("re-reading after update", zoneID, err))
		return
	}

	resp.Diagnostics.Append(applyZone(ctx, updated, &plan, afterWrite)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *zoneResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state zoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_zone")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := client.DeleteZone(ctx, state.ID.ValueString())
	// Already gone is the outcome asked for.
	if err != nil && !errors.Is(err, transport.ErrNotFound) {
		resp.Diagnostics.Append(zoneDiagnostic("deleting", state.ID.ValueString(), err))
	}
}

// ImportState takes the canonical zone name.
func (r *zoneResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	// Either spelling: `terraform import` with a zone name, or an import block
	// carrying the identity. The helper picks whichever was used.
	id := req.ID
	if id == "" {
		var identity types.String
		resp.Diagnostics.Append(
			req.Identity.GetAttribute(ctx, path.Root("zone_name"), &identity)...)
		if resp.Diagnostics.HasError() {
			return
		}
		id = identity.ValueString()
	}
	id = canonicalName(id)

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), id)...)
	resp.Diagnostics.Append(setZoneIdentity(ctx, resp.Identity, id)...)
}
