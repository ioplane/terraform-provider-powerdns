package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	_ resource.Resource                = (*tsigKeyResource)(nil)
	_ resource.ResourceWithIdentity    = (*tsigKeyResource)(nil)
	_ resource.ResourceWithConfigure   = (*tsigKeyResource)(nil)
	_ resource.ResourceWithImportState = (*tsigKeyResource)(nil)
)

// tsigKeyResource manages a TSIG key.
//
// # The secret
//
// PowerDNS returns the shared secret from `POST /tsigkeys`, from
// `PUT /tsigkeys/{id}` and from `GET /tsigkeys/{id}`, and blanks it in
// `GET /tsigkeys`. This resource reconciles against the collection and never
// stores what a write returned, exactly as the DNSSEC key resource does.
//
// Importing a secret is offered through a **write-only** attribute. Terraform
// sends the value to the provider and stores it in neither state nor the plan
// file, which is the only shape in which a secret can be configured without
// ending up on disk. It requires Terraform 1.11 or later.
//
// Leaving it unset asks PowerDNS to generate one, in which case the secret
// cannot be read back through this resource at all. That is the same trade as
// the DNSSEC keys: a value nothing exposes is a value nothing can leak.
type tsigKeyResource struct {
	clients *Clients
}

// NewTSIGKeyResource returns the resource factory.
func NewTSIGKeyResource() resource.Resource {
	return &tsigKeyResource{}
}

type tsigKeyModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Algorithm types.String `tfsdk:"algorithm"`
	Secret    types.String `tfsdk:"secret_wo"`
}

func (r *tsigKeyResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_tsigkey"
}

func (r *tsigKeyResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A TSIG key, for authenticating zone transfers and " +
			"dynamic updates.\n\n" +
			"**The secret never reaches state.** Leave `secret_wo` unset and PowerDNS " +
			"generates one, which then cannot be read back through this resource. Set " +
			"it to import a secret you already hold: it is a write-only attribute, so " +
			"Terraform sends it to the provider and stores it in neither the state file " +
			"nor the plan file. Write-only attributes need Terraform 1.11 or later.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The canonical key name, which is the name with a " +
					"trailing dot: a key created as `transfer` has id `transfer.`.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The key name.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					planmodify.SemanticString(
						"compared against the canonical id, which carries a trailing dot",
						normalise.TSIGKeyID),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"algorithm": schema.StringAttribute{
				MarkdownDescription: "An HMAC algorithm, such as `hmac-sha256`.\n\n" +
					"Changing it **replaces the key**, and that is not a design choice. " +
					"`PUT /tsigkeys/{id}` calls `setTSIGKey(name, algorithm, key)` and " +
					"deletes the previous entry only when the *name* changed " +
					"(`ws-auth.cc:1932` at tag `auth-5.1.3`). Changing only the " +
					"algorithm therefore leaves the old key in place and adds a second " +
					"one under the same id — verified against 5.1.3, where three PUTs " +
					"produced three entries. Replacing is the only way to end up with " +
					"one key.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(
						"hmac-md5", "hmac-sha1", "hmac-sha224",
						"hmac-sha256", "hmac-sha384", "hmac-sha512",
					),
				},
			},
			"secret_wo": schema.StringAttribute{
				MarkdownDescription: "A base64 secret to import, instead of having " +
					"PowerDNS generate one. Write-only: Terraform sends it to the " +
					"provider and stores it nowhere.\n\n" +
					"Because it is stored nowhere, a change to it cannot be detected " +
					"from state. Rotate a secret by changing the key name, or by " +
					"tainting the resource.",
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
			},
		},
	}
}

func (r *tsigKeyResource) Configure(
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

func (r *tsigKeyResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan tsigKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A write-only attribute is absent from the plan by construction; its value
	// arrives on the config, and only during an apply.
	var config tsigKeyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_tsigkey")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	key := auth.TSIGKey{
		Name:      plan.Name.ValueString(),
		Algorithm: plan.Algorithm.ValueString(),
		// Empty asks PowerDNS to generate one.
		Key: config.Secret.ValueString(),
	}

	created, err := client.CreateTSIGKey(ctx, key)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating the TSIG key",
			key.Name+".\n\n"+err.Error(),
		)
		return
	}

	// created.Key holds the secret. It is read here and nowhere else, and the
	// model has no field that could carry it into state.
	plan.ID = types.StringValue(created.ID)
	plan.Secret = types.StringNull()

	resp.Diagnostics.Append(setTSIGKeyIdentity(ctx, resp.Identity, created.ID)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tsigKeyResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state tsigKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_tsigkey")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The collection, never GetTSIGKey: the latter returns the secret.
	keys, err := client.ListTSIGKeys(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing the TSIG keys", err.Error())
		return
	}

	keyID := state.ID.ValueString()
	for _, key := range keys {
		if key.ID != keyID {
			continue
		}
		if key.Key != "" {
			// The collection is documented to blank it. If that changes, this
			// resource is no longer safe.
			resp.Diagnostics.AddError(
				"PowerDNS returned a TSIG secret from the collection endpoint",
				"GET /tsigkeys included a secret, which it has never done before. "+
					"The provider refuses to continue rather than risk writing it to "+
					"state. Please report this.",
			)
			return
		}

		state.Algorithm = types.StringValue(key.Algorithm)
		// secret_wo is write-only and has no value in state, ever.
		state.Secret = types.StringNull()
		resp.Diagnostics.Append(setTSIGKeyIdentity(ctx, resp.Identity, key.ID)...)
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	resp.State.RemoveResource(ctx)
}

// Update exists because the framework requires it, and does nothing.
//
// Every attribute this resource carries forces replacement: name and algorithm
// because PowerDNS's PUT adds a second key rather than changing the one that
// is there, and secret_wo because a write-only value cannot be compared
// against state to know whether it changed. So there is no in-place change to
// make.
func (r *tsigKeyResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan tsigKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Secret = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tsigKeyResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state tsigKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_tsigkey")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := client.DeleteTSIGKey(ctx, state.ID.ValueString())
	if err != nil && !errors.Is(err, transport.ErrNotFound) {
		resp.Diagnostics.AddError(
			"Error deleting the TSIG key",
			state.ID.ValueString()+".\n\n"+err.Error(),
		)
	}
}

// ImportState takes the key name, with or without its trailing dot.
func (r *tsigKeyResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	id := canonicalName(req.ID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}
