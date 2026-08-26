package provider

import (
	"context"

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

	"github.com/ioplane/terraform-provider-powerdns/internal/api/rec"
	"github.com/ioplane/terraform-provider-powerdns/internal/provider/normalise"
	"github.com/ioplane/terraform-provider-powerdns/internal/provider/planmodify"
)

// The two ACL resources are singletons, and say so.
//
// Neither product has a collection of ACLs. The Recursor exposes two named
// netmask settings and dnsdist exactly one, so a resource per ACL would be a
// resource that can only exist once — and two of them in a configuration would
// silently overwrite each other.
//
// Making them singletons does not prevent that: Terraform cannot stop a second
// resource of the same type. What it does is make the ownership obvious in the
// schema, and Delete restores nothing rather than clearing an ACL a
// configuration never set. Removing the resource leaves the server as it is,
// which is the safe direction for something that decides who may query.

var (
	_ resource.Resource                = (*recursorACLResource)(nil)
	_ resource.ResourceWithConfigure   = (*recursorACLResource)(nil)
	_ resource.ResourceWithImportState = (*recursorACLResource)(nil)

	_ resource.Resource                = (*dnsdistACLResource)(nil)
	_ resource.ResourceWithConfigure   = (*dnsdistACLResource)(nil)
	_ resource.ResourceWithImportState = (*dnsdistACLResource)(nil)
)

// recursorACLResource manages one of the Recursor's two writable settings.
type recursorACLResource struct {
	clients *Clients
}

// NewRecursorACLResource returns the resource factory.
func NewRecursorACLResource() resource.Resource {
	return &recursorACLResource{}
}

type recursorACLModel struct {
	ID       types.String `tfsdk:"id"`
	Setting  types.String `tfsdk:"setting"`
	Netmasks types.List   `tfsdk:"netmasks"`
}

func (r *recursorACLResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_recursor_acl"
}

func (r *recursorACLResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "One of the Recursor's two writable netmask settings.\n\n" +
			"`ws-recursor.cc` registers `allow-from` and `allow-notify-from` as " +
			"separate handlers rather than one parameterised route, so every other " +
			"name answers 404 on read as well as write. This resource rejects any " +
			"other name before sending.\n\n" +
			"Needs `webservice.api_dir`. Deleting the resource leaves the setting as " +
			"it is: clearing an ACL that decides who may query is not a safe default.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The setting name.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					// Without this the id plans as "known after apply" on every
					// run and no plan is ever empty.
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"setting": schema.StringAttribute{
				MarkdownDescription: "`allow-from` or `allow-notify-from`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(rec.SettingAllowFrom, rec.SettingAllowNotifyFrom),
				},
			},
			"netmasks": schema.ListAttribute{
				MarkdownDescription: "The netmasks, compared as subnets and ignoring " +
					"order.",
				Required:    true,
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				PlanModifiers: []planmodifier.List{
					planmodify.SemanticSet(
						"compared as subnets, ignoring order", normalise.CIDRKey),
				},
			},
		},
	}
}

func (r *recursorACLResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	r.clients = configureResourceClients(req, resp)
}

func (r *recursorACLResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan recursorACLModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.write(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *recursorACLResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state recursorACLModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireRecursor("powerdns_recursor_acl")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	setting, err := client.GetSetting(ctx, state.Setting.ValueString())
	if err != nil {
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error reading the Recursor ACL", state.Setting.ValueString(), err))
		return
	}

	netmasks, d := stringListValue(ctx, setting.Value)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Netmasks = netmasks

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *recursorACLResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan recursorACLModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.write(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete leaves the setting alone.
//
// There is nothing to restore it to: the Recursor has no notion of an unset
// ACL, and writing an empty list would refuse every client. Removing the
// resource therefore removes Terraform's knowledge of the setting, not the
// setting.
func (r *recursorACLResource) Delete(
	_ context.Context,
	_ resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	resp.Diagnostics.AddWarning(
		"The Recursor ACL was left as it is",
		"Removing powerdns_recursor_acl stops Terraform managing the setting; it "+
			"does not clear it. There is no unset state for an ACL, and writing an "+
			"empty list would refuse every client.",
	)
}

// ImportState takes the setting name.
func (r *recursorACLResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("setting"), req.ID)...)
}

func (r *recursorACLResource) write(
	ctx context.Context,
	plan *recursorACLModel,
) diag.Diagnostics {
	var diags diag.Diagnostics

	client, d := r.clients.RequireRecursor("powerdns_recursor_acl")
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}

	var netmasks []string
	diags.Append(elementsAs(ctx, plan.Netmasks, &netmasks)...)
	if diags.HasError() {
		return diags
	}

	setting := plan.Setting.ValueString()
	if err := client.SetSetting(ctx, setting, netmasks); err != nil {
		diags.Append(capabilityDiagnostic("Error writing the Recursor ACL", setting, err))
		return diags
	}

	plan.ID = types.StringValue(setting)
	return diags
}

// dnsdistACLResource manages dnsdist's single writable setting.
type dnsdistACLResource struct {
	clients *Clients
}

// NewDNSDistACLResource returns the resource factory.
func NewDNSDistACLResource() resource.Resource {
	return &dnsdistACLResource{}
}

type dnsdistACLModel struct {
	ID       types.String `tfsdk:"id"`
	Netmasks types.List   `tfsdk:"netmasks"`
}

func (r *dnsdistACLResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_dnsdist_acl"
}

func (r *dnsdistACLResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "dnsdist's `allow-from` ACL — the only configuration " +
			"dnsdist's API can write.\n\n" +
			"Rules, pools, downstream servers and dynamic blocks are Lua or YAML and " +
			"have no HTTP path at all ([ADR 0006](https://github.com/ioplane/terraform-provider-powerdns/blob/main/docs/adr/0006-dnsdist-scope.md)). " +
			"This resource and a cache flush are the whole of what a provider can do.\n\n" +
			"Needs `setAPIWritable(true, dir)` in the Lua configuration; `apiConfigDir` " +
			"alone is not enough, and without it every `PUT` answers 405.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Fixed: dnsdist has one ACL.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"netmasks": schema.ListAttribute{
				MarkdownDescription: "The netmasks, compared as subnets and ignoring " +
					"order.",
				Required:    true,
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				PlanModifiers: []planmodifier.List{
					planmodify.SemanticSet(
						"compared as subnets, ignoring order", normalise.CIDRKey),
				},
			},
		},
	}
}

func (r *dnsdistACLResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	r.clients = configureResourceClients(req, resp)
}

func (r *dnsdistACLResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan dnsdistACLModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.write(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dnsdistACLResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state dnsdistACLModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireDNSDist("powerdns_dnsdist_acl")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	acl, err := client.GetACL(ctx)
	if err != nil {
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error reading the dnsdist ACL", "allow-from", err))
		return
	}

	netmasks, d := stringListValue(ctx, acl.Value)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Netmasks = netmasks

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dnsdistACLResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan dnsdistACLModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.write(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete leaves the ACL as it is, for the same reason as the Recursor's.
func (r *dnsdistACLResource) Delete(
	_ context.Context,
	_ resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	resp.Diagnostics.AddWarning(
		"The dnsdist ACL was left as it is",
		"Removing powerdns_dnsdist_acl stops Terraform managing allow-from; it does "+
			"not clear it. Writing an empty list would refuse every client.",
	)
}

// ImportState takes anything: there is one ACL.
func (r *dnsdistACLResource) ImportState(
	ctx context.Context,
	_ resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "allow-from")...)
}

func (r *dnsdistACLResource) write(
	ctx context.Context,
	plan *dnsdistACLModel,
) diag.Diagnostics {
	var diags diag.Diagnostics

	client, d := r.clients.RequireDNSDist("powerdns_dnsdist_acl")
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}

	var netmasks []string
	diags.Append(elementsAs(ctx, plan.Netmasks, &netmasks)...)
	if diags.HasError() {
		return diags
	}

	if err := client.SetACL(ctx, netmasks); err != nil {
		diags.Append(capabilityDiagnostic("Error writing the dnsdist ACL", "allow-from", err))
		return diags
	}

	plan.ID = types.StringValue("allow-from")
	return diags
}
