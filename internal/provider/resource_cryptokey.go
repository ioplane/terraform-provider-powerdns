package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
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
	_ resource.Resource                = (*cryptoKeyResource)(nil)
	_ resource.ResourceWithIdentity    = (*cryptoKeyResource)(nil)
	_ resource.ResourceWithConfigure   = (*cryptoKeyResource)(nil)
	_ resource.ResourceWithImportState = (*cryptoKeyResource)(nil)
)

// cryptoKeyResource manages one DNSSEC key.
//
// # No private key reaches state
//
// PowerDNS returns key material from `POST /cryptokeys` and from
// `GET /cryptokeys/{id}`, and omits it from `GET /cryptokeys`. This resource
// reads the collection, never the single key, and never stores what the create
// response carried. `TestAccCryptoKey_NoPrivateKeyInState` reads the state file
// and fails if it finds any.
//
// The consequence is that a generated private key cannot be retrieved through
// this resource at all — which is the intended trade. Anything that genuinely
// needs it should use the ephemeral resource, where the value never becomes
// part of a plan or a state file.
type cryptoKeyResource struct {
	clients *Clients
}

// NewCryptoKeyResource returns the resource factory.
func NewCryptoKeyResource() resource.Resource {
	return &cryptoKeyResource{}
}

type cryptoKeyModel struct {
	ID   types.String `tfsdk:"id"`
	Zone types.String `tfsdk:"zone"`

	KeyID     types.Int64  `tfsdk:"key_id"`
	KeyType   types.String `tfsdk:"key_type"`
	Algorithm types.String `tfsdk:"algorithm"`
	Bits      types.Int64  `tfsdk:"bits"`

	Active    types.Bool `tfsdk:"active"`
	Published types.Bool `tfsdk:"published"`

	DNSKey types.String `tfsdk:"dnskey"`
	DS     types.List   `tfsdk:"ds"`
}

func (r *cryptoKeyResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_zone_cryptokey"
}

func (r *cryptoKeyResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A DNSSEC key for a zone.\n\n" +
			"**No private key reaches state.** PowerDNS returns key material when a key " +
			"is created and when one is read individually; this resource reconciles " +
			"against the collection endpoint, which omits it, and never stores what the " +
			"create response carried. A generated private key therefore cannot be read " +
			"back through this resource.\n\n" +
			"Setting `dnssec = true` on `powerdns_zone` makes PowerDNS generate a CSK " +
			"by itself. Manage keys here *or* let the zone do it, not both.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "`<zone>/<key_id>`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"zone": schema.StringAttribute{
				MarkdownDescription: "The zone this key signs.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					planmodify.SemanticString(
						"compared as a DNS name", normalise.DNSName),
				},
			},
			"key_id": schema.Int64Attribute{
				MarkdownDescription: "The key's numeric id. Assigned by the server from a " +
					"**global** counter, not one per zone, so it is meaningless without " +
					"the zone.",
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"key_type": schema.StringAttribute{
				MarkdownDescription: "`ksk`, `zsk` or `csk`.\n\n" +
					"PowerDNS stores the DNSKEY flags, not the type, and derives the " +
					"type from the flags together with how many keys the zone holds. A " +
					"zone's only key reads back as `csk` whatever it was created as, " +
					"and is renamed — not replaced — once a second key appears. `csk` " +
					"is therefore compared as compatible with both `ksk` and `zsk`, so " +
					"adding a key does not appear to change the one already there.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOfCaseInsensitive(
						auth.KeyTypeKSK, auth.KeyTypeZSK, auth.KeyTypeCSK),
				},
				PlanModifiers: []planmodifier.String{
					// The semantic comparison runs first, so a ksk/csk flip is
					// resolved before RequiresReplace sees the plan. Without
					// it, adding a ZSK would destroy and recreate the
					// key-signing key of a signed zone.
					planmodify.SemanticString(
						"`ksk` and `csk` are the same key: PowerDNS stores flags, not type",
						normalise.DNSSECKeyType),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"algorithm": schema.StringAttribute{
				MarkdownDescription: "The signing algorithm, such as `ECDSAP256SHA256`. " +
					"PowerDNS chooses a default when this is unset.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					// RequiresReplaceIfConfigured, not RequiresReplace: the
					// attribute is Optional and Computed, so leaving it unset
					// makes it unknown at plan time, and a plain
					// RequiresReplace reads that as a change and destroys the
					// key on every run.
					stringplanmodifier.RequiresReplaceIfConfigured(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"bits": schema.Int64Attribute{
				MarkdownDescription: "Key size. Meaningful for RSA; ignored for the " +
					"elliptic-curve algorithms, which have one size each.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplaceIfConfigured(),
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"active": schema.BoolAttribute{
				MarkdownDescription: "Whether the key signs. Changeable in place.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"published": schema.BoolAttribute{
				MarkdownDescription: "Whether the DNSKEY is served. A key can be active " +
					"but unpublished, which is how a rollover stages one.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"dnskey": schema.StringAttribute{
				MarkdownDescription: "The public DNSKEY record. Public by definition; " +
					"safe in state.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					// Without this every plan is non-empty: a Computed
					// attribute plans as "known after apply" unless it is told
					// the stored value still holds. The key material does not
					// change while the key exists.
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ds": schema.ListAttribute{
				MarkdownDescription: "DS records to publish in the parent zone. Present " +
					"for a key-signing key and empty for a ZSK, which has no delegation " +
					"signer.",
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *cryptoKeyResource) Configure(
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

func (r *cryptoKeyResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan cryptoKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_zone_cryptokey")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	key := auth.CryptoKey{
		KeyType:   strings.ToLower(plan.KeyType.ValueString()),
		Active:    plan.Active.ValueBool(),
		Algorithm: plan.Algorithm.ValueString(),
		Bits:      int(plan.Bits.ValueInt64()),
	}
	if !plan.Published.IsNull() && !plan.Published.IsUnknown() {
		key.Published = plan.Published.ValueBoolPointer()
	}

	zoneID := canonicalName(plan.Zone.ValueString())
	created, err := client.CreateCryptoKey(ctx, zoneID, key)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating the DNSSEC key",
			"Zone "+zoneID+".\n\n"+err.Error(),
		)
		return
	}

	// The create response carries the private key. Nothing here reads it, and
	// the re-read below goes to the collection, which omits it — so it never
	// leaves this function.
	plan.KeyID = types.Int64Value(int64(created.ID))
	plan.ID = types.StringValue(cryptoKeyID(zoneID, created.ID))

	resp.Diagnostics.Append(r.readInto(ctx, zoneID, created.ID, &plan, afterWrite)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(setCryptoKeyIdentity(ctx, resp.Identity,
		zoneID, int64(created.ID))...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *cryptoKeyResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state cryptoKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneID := canonicalName(state.Zone.ValueString())
	keyID := int(state.KeyID.ValueInt64())

	diags := r.readInto(ctx, zoneID, keyID, &state, afterRead)
	if isGone(diags) {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(setCryptoKeyIdentity(ctx, resp.Identity,
		zoneID, state.KeyID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *cryptoKeyResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan, state cryptoKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_zone_cryptokey")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only active and published are changeable; everything else forces
	// replacement, because PowerDNS cannot rewrite a key in place.
	zoneID := canonicalName(state.Zone.ValueString())
	keyID := int(state.KeyID.ValueInt64())

	published := plan.Published.ValueBool()
	err := client.SetCryptoKeyState(ctx, zoneID, keyID, plan.Active.ValueBool(), &published)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error changing the DNSSEC key state",
			fmt.Sprintf("Key %d in zone %s.\n\n%s", keyID, zoneID, err.Error()),
		)
		return
	}

	plan.KeyID = state.KeyID
	plan.ID = state.ID
	resp.Diagnostics.Append(r.readInto(ctx, zoneID, keyID, &plan, afterWrite)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *cryptoKeyResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state cryptoKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_zone_cryptokey")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneID := canonicalName(state.Zone.ValueString())
	keyID := int(state.KeyID.ValueInt64())

	err := client.DeleteCryptoKey(ctx, zoneID, keyID)
	if err != nil && !errors.Is(err, transport.ErrNotFound) {
		resp.Diagnostics.AddError(
			"Error deleting the DNSSEC key",
			fmt.Sprintf("Key %d in zone %s.\n\n%s", keyID, zoneID, err.Error()),
		)
	}
}

// ImportState takes `<zone>/<key_id>`.
func (r *cryptoKeyResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	zone, rawID, found := strings.Cut(req.ID, "/")
	if !found {
		resp.Diagnostics.AddError(
			"Malformed import id",
			fmt.Sprintf("Expected `<zone>/<key_id>`, for example `example.com./3`. "+
				"Got %q.", req.ID),
		)
		return
	}

	keyID, err := strconv.Atoi(rawID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Malformed import id",
			fmt.Sprintf("The key id must be a number; %q is not. PowerDNS assigns it "+
				"from a global counter, so it is visible in "+
				"`GET /zones/<zone>/cryptokeys`.", rawID),
		)
		return
	}

	zone = canonicalName(zone)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"),
		cryptoKeyID(zone, keyID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone"), zone)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("key_id"),
		int64(keyID))...)
}
