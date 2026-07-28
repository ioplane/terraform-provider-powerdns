package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ ephemeral.EphemeralResource              = (*cryptoKeyMaterialEphemeral)(nil)
	_ ephemeral.EphemeralResourceWithConfigure = (*cryptoKeyMaterialEphemeral)(nil)

	_ ephemeral.EphemeralResource              = (*tsigKeySecretEphemeral)(nil)
	_ ephemeral.EphemeralResourceWithConfigure = (*tsigKeySecretEphemeral)(nil)
)

// Ephemeral resources are how key material can be used without being kept.
//
// The managed resources deliberately cannot return a secret: `powerdns_zone_cryptokey`
// reads the collection endpoint and `powerdns_tsigkey` never stores what a write
// returned. That closes the leak and leaves a real need unmet — an operator who
// has to hand a DNSSEC private key to a signing appliance, or a TSIG secret to
// a secondary server's configuration.
//
// An ephemeral resource meets it without reopening the leak. Terraform fetches
// the value during an operation, passes it to whatever consumes it, and
// discards it: nothing is written to state and nothing to the plan file. The
// value is fetched again next time rather than remembered.
//
// The trade is that an ephemeral value cannot be referenced by a managed
// resource's ordinary attribute — only by another ephemeral or write-only one.
// That restriction is the feature: it is what stops the value being persisted
// by something downstream.

// cryptoKeyMaterialEphemeral reads a DNSSEC private key without storing it.
type cryptoKeyMaterialEphemeral struct {
	clients *Clients
}

// NewCryptoKeyMaterialEphemeral returns the ephemeral resource factory.
func NewCryptoKeyMaterialEphemeral() ephemeral.EphemeralResource {
	return &cryptoKeyMaterialEphemeral{}
}

type cryptoKeyMaterialModel struct {
	Zone       types.String `tfsdk:"zone"`
	KeyID      types.Int64  `tfsdk:"key_id"`
	PrivateKey types.String `tfsdk:"private_key"`
	DNSKey     types.String `tfsdk:"dnskey"`
}

func (e *cryptoKeyMaterialEphemeral) Metadata(
	_ context.Context,
	req ephemeral.MetadataRequest,
	resp *ephemeral.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_cryptokey_material"
}

func (e *cryptoKeyMaterialEphemeral) Schema(
	_ context.Context,
	_ ephemeral.SchemaRequest,
	resp *ephemeral.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a DNSSEC private key **without storing it**.\n\n" +
			"Terraform fetches the value during an operation and discards it: nothing " +
			"reaches the state file or the plan file, and it is fetched again on the " +
			"next run rather than remembered.\n\n" +
			"`powerdns_zone_cryptokey` deliberately cannot return this. Use this when " +
			"the key has to be handed to something else — a signing appliance, or a " +
			"secret manager — and note that an ephemeral value can only be consumed by " +
			"another ephemeral or write-only attribute, which is what stops it being " +
			"persisted downstream.",

		Attributes: map[string]schema.Attribute{
			"zone": schema.StringAttribute{
				MarkdownDescription: "The zone the key belongs to.",
				Required:            true,
			},
			"key_id": schema.Int64Attribute{
				MarkdownDescription: "The key's numeric id, as reported by " +
					"`powerdns_zone_cryptokey.key_id`.",
				Required: true,
			},
			"private_key": schema.StringAttribute{
				MarkdownDescription: "The private key, in PowerDNS's own format.",
				Computed:            true,
				Sensitive:           true,
			},
			"dnskey": schema.StringAttribute{
				MarkdownDescription: "The public DNSKEY record, for convenience.",
				Computed:            true,
			},
		},
	}
}

func (e *cryptoKeyMaterialEphemeral) Configure(
	_ context.Context,
	req ephemeral.ConfigureRequest,
	resp *ephemeral.ConfigureResponse,
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
	e.clients = clients
}

func (e *cryptoKeyMaterialEphemeral) Open(
	ctx context.Context,
	req ephemeral.OpenRequest,
	resp *ephemeral.OpenResponse,
) {
	var config cryptoKeyMaterialModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := e.clients.RequireAuth("the powerdns_cryptokey_material ephemeral resource")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneID := canonicalName(config.Zone.ValueString())
	keyID := int(config.KeyID.ValueInt64())

	// GetCryptoKey, deliberately: this is the one place in the provider that
	// asks for key material, and the only place where doing so is safe,
	// because an ephemeral result is never persisted.
	key, err := client.GetCryptoKey(ctx, zoneID, keyID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading the DNSSEC key material",
			"Key "+strconv.Itoa(keyID)+" in zone "+zoneID+".\n\n"+err.Error(),
		)
		return
	}

	if key.PrivateKey == "" {
		resp.Diagnostics.AddError(
			"PowerDNS returned no key material",
			"GET /zones/"+zoneID+"/cryptokeys/"+strconv.Itoa(keyID)+" answered without "+
				"a private key. That endpoint has always included one; if it no longer "+
				"does, this ephemeral resource has nothing to offer and the managed "+
				"resource should be used instead.",
		)
		return
	}

	config.PrivateKey = types.StringValue(key.PrivateKey)
	config.DNSKey = types.StringValue(key.DNSKey)

	resp.Diagnostics.Append(resp.Result.Set(ctx, &config)...)
}

// tsigKeySecretEphemeral reads a TSIG secret without storing it.
type tsigKeySecretEphemeral struct {
	clients *Clients
}

// NewTSIGKeySecretEphemeral returns the ephemeral resource factory.
func NewTSIGKeySecretEphemeral() ephemeral.EphemeralResource {
	return &tsigKeySecretEphemeral{}
}

type tsigKeySecretModel struct {
	Name      types.String `tfsdk:"name"`
	Algorithm types.String `tfsdk:"algorithm"`
	Secret    types.String `tfsdk:"secret"`
}

func (e *tsigKeySecretEphemeral) Metadata(
	_ context.Context,
	req ephemeral.MetadataRequest,
	resp *ephemeral.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_tsigkey_secret"
}

func (e *tsigKeySecretEphemeral) Schema(
	_ context.Context,
	_ ephemeral.SchemaRequest,
	resp *ephemeral.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a TSIG secret **without storing it**.\n\n" +
			"The usual reason is configuring a secondary server, which needs the same " +
			"secret the primary authenticates with. `powerdns_tsigkey` cannot return " +
			"it; this can, because an ephemeral value reaches neither the state file " +
			"nor the plan file.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "The key name, with or without a trailing dot.",
				Required:            true,
			},
			"algorithm": schema.StringAttribute{
				MarkdownDescription: "The key's HMAC algorithm.",
				Computed:            true,
			},
			"secret": schema.StringAttribute{
				MarkdownDescription: "The base64 shared secret.",
				Computed:            true,
				Sensitive:           true,
			},
		},
	}
}

func (e *tsigKeySecretEphemeral) Configure(
	_ context.Context,
	req ephemeral.ConfigureRequest,
	resp *ephemeral.ConfigureResponse,
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
	e.clients = clients
}

func (e *tsigKeySecretEphemeral) Open(
	ctx context.Context,
	req ephemeral.OpenRequest,
	resp *ephemeral.OpenResponse,
) {
	var config tsigKeySecretModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := e.clients.RequireAuth("the powerdns_tsigkey_secret ephemeral resource")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	keyID := canonicalName(config.Name.ValueString())

	key, err := client.GetTSIGKey(ctx, keyID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading the TSIG secret",
			keyID+".\n\n"+err.Error(),
		)
		return
	}

	if strings.TrimSpace(key.Key) == "" {
		resp.Diagnostics.AddError(
			"PowerDNS returned no TSIG secret",
			"GET /tsigkeys/"+keyID+" answered with an empty key. The collection "+
				"endpoint blanks it by design, but the single-key read has always "+
				"carried it.",
		)
		return
	}

	config.Algorithm = types.StringValue(key.Algorithm)
	config.Secret = types.StringValue(key.Key)

	resp.Diagnostics.Append(resp.Result.Set(ctx, &config)...)
}
