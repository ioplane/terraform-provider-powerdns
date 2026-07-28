package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	_ resource.Resource                = (*autoprimaryResource)(nil)
	_ resource.ResourceWithIdentity    = (*autoprimaryResource)(nil)
	_ resource.ResourceWithConfigure   = (*autoprimaryResource)(nil)
	_ resource.ResourceWithImportState = (*autoprimaryResource)(nil)
)

// autoprimaryResource permits a server to create zones here by sending NOTIFY.
//
// The pair of IP and nameserver is the key — PowerDNS assigns no id — so every
// attribute forces replacement and there is no update. Changing either field
// means deleting one entry and creating another, which is what the API does
// too.
type autoprimaryResource struct {
	clients *Clients
}

// NewAutoprimaryResource returns the resource factory.
func NewAutoprimaryResource() resource.Resource {
	return &autoprimaryResource{}
}

type autoprimaryModel struct {
	ID         types.String `tfsdk:"id"`
	IP         types.String `tfsdk:"ip"`
	Nameserver types.String `tfsdk:"nameserver"`
	Account    types.String `tfsdk:"account"`
}

func (r *autoprimaryResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_autoprimary"
}

func (r *autoprimaryResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A server permitted to create zones here by sending a " +
			"NOTIFY.\n\n" +
			"PowerDNS assigns no id: the pair of `ip` and `nameserver` is the key. " +
			"Every attribute therefore forces replacement, because changing either " +
			"means deleting one entry and creating another — which is what the API " +
			"does as well.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "`<ip>/<nameserver>`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ip": schema.StringAttribute{
				MarkdownDescription: "The primary's address. Compared by value, so an " +
					"IPv6 address written uncompressed does not produce a diff.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					planmodify.SemanticString(
						"compared as an address, not as a string", normalise.IPAddress),
				},
			},
			"nameserver": schema.StringAttribute{
				MarkdownDescription: "The primary's nameserver name.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					planmodify.SemanticString(
						"compared as a DNS name", normalise.DNSName),
				},
			},
			"account": schema.StringAttribute{
				MarkdownDescription: "Free-form owner label applied to zones this " +
					"primary creates.",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *autoprimaryResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	r.clients = configureResourceClients(req, resp)
}

func (r *autoprimaryResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan autoprimaryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_autoprimary")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	entry := auth.Autoprimary{
		IP:         plan.IP.ValueString(),
		Nameserver: plan.Nameserver.ValueString(),
		Account:    plan.Account.ValueString(),
	}

	if err := client.CreateAutoprimary(ctx, entry); err != nil {
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error creating the autoprimary", entry.IP+" / "+entry.Nameserver, err))
		return
	}

	plan.ID = types.StringValue(entry.IP + "/" + entry.Nameserver)
	resp.Diagnostics.Append(setAutoprimaryIdentity(ctx, resp.Identity,
		entry.IP, entry.Nameserver)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *autoprimaryResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state autoprimaryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_autoprimary")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There is no single-entry read: the collection is the only way to find
	// out whether a pair is still there.
	entries, err := client.ListAutoprimaries(ctx)
	if err != nil {
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error listing the autoprimaries", state.ID.ValueString(), err))
		return
	}

	wantIP := state.IP.ValueString()
	wantNS := state.Nameserver.ValueString()

	for _, entry := range entries {
		if !normalise.IPAddress(wantIP, entry.IP) ||
			!normalise.DNSName(wantNS, entry.Nameserver) {
			continue
		}
		state.Account = optionalString(entry.Account)
		resp.Diagnostics.Append(setAutoprimaryIdentity(ctx, resp.Identity,
			entry.IP, entry.Nameserver)...)
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	resp.State.RemoveResource(ctx)
}

// Update is unreachable: every attribute forces replacement.
func (r *autoprimaryResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan autoprimaryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *autoprimaryResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state autoprimaryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_autoprimary")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := client.DeleteAutoprimary(ctx,
		state.IP.ValueString(), state.Nameserver.ValueString())
	if err != nil && !errors.Is(err, transport.ErrNotFound) {
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error deleting the autoprimary", state.ID.ValueString(), err))
	}
}

// ImportState takes `<ip>/<nameserver>`.
func (r *autoprimaryResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	ip, nameserver, found := strings.Cut(req.ID, "/")
	if !found || ip == "" || nameserver == "" {
		resp.Diagnostics.AddError(
			"Malformed import id",
			fmt.Sprintf("Expected `<ip>/<nameserver>`, for example "+
				"`192.0.2.53/ns1.example.com.`. Got %q.", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("ip"), ip)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("nameserver"),
		nameserver)...)
}
