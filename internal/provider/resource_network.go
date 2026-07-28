package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-powerdns/internal/api/transport"
	"github.com/ioplane/terraform-provider-powerdns/internal/provider/normalise"
	"github.com/ioplane/terraform-provider-powerdns/internal/provider/planmodify"
)

var (
	_ resource.Resource                = (*networkResource)(nil)
	_ resource.ResourceWithConfigure   = (*networkResource)(nil)
	_ resource.ResourceWithImportState = (*networkResource)(nil)
)

// networkResource maps a client subnet to a view.
//
// LMDB only, like views. See viewZoneResource for why.
type networkResource struct {
	clients *Clients
}

// NewNetworkResource returns the resource factory.
func NewNetworkResource() resource.Resource {
	return &networkResource{}
}

type networkModel struct {
	ID      types.String `tfsdk:"id"`
	Network types.String `tfsdk:"network"`
	View    types.String `tfsdk:"view"`
}

func (r *networkResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_network"
}

func (r *networkResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Maps a client subnet to a view, so queries from that " +
			"subnet are answered from it.\n\n" +
			"**Requires the LMDB backend**, like `powerdns_view_zone`.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The subnet in CIDR form.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"network": schema.StringAttribute{
				MarkdownDescription: "The client subnet, in CIDR form. Compared by " +
					"value, so an IPv6 prefix written uncompressed does not produce a " +
					"permanent diff.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					planmodify.SemanticString(
						"compared as a subnet, not as a string", normalise.CIDR),
				},
			},
			"view": schema.StringAttribute{
				MarkdownDescription: "The view to answer this subnet from. Changing it " +
					"is applied in place.",
				Required: true,
			},
		},
	}
}

func (r *networkResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	r.clients = configureResourceClients(req, resp)
}

func (r *networkResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan networkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(r.write(ctx, "creating", &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *networkResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state networkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_network")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	network, err := client.GetNetwork(ctx, state.Network.ValueString())
	if err != nil {
		if errors.Is(err, transport.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error reading the network", state.Network.ValueString(), err))
		return
	}

	// An empty view means the subnet exists with no assignment, which is what
	// PowerDNS leaves behind when a mapping is removed.
	if network.View == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	state.View = types.StringValue(network.View)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *networkResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan networkModel
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

func (r *networkResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state networkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_network")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There is no delete: assigning the empty view is how a mapping is
	// removed. The subnet itself stays, unassigned.
	err := client.SetNetwork(ctx, state.Network.ValueString(), "")
	if err != nil && !errors.Is(err, transport.ErrNotFound) {
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error removing the network mapping", state.Network.ValueString(), err))
	}
}

// ImportState takes the CIDR.
func (r *networkResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("network"), req.ID)...)
}

// write is Create and Update: a PUT assigns the view either way.
func (r *networkResource) write(
	ctx context.Context,
	action string,
	plan *networkModel,
) diag.Diagnostics {
	var diags diag.Diagnostics

	client, d := r.clients.RequireAuth("powerdns_network")
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}

	cidr := plan.Network.ValueString()
	if err := client.SetNetwork(ctx, cidr, plan.View.ValueString()); err != nil {
		diags.Append(capabilityDiagnostic("Error "+action+" the network", cidr, err))
		return diags
	}

	plan.ID = types.StringValue(cidr)
	return diags
}

// capabilityDiagnostic reports a client error, saying plainly when the server
// cannot do the thing as configured.
//
// The transport has already worked out which of the four capability conditions
// applies and put the requirement in the message. This adds the object and
// separates "your server is not set up for this" from "something went wrong",
// because the first is not worth retrying.
func capabilityDiagnostic(summary, object string, err error) diag.Diagnostic {
	detail := object + ".\n\n" + err.Error()

	var apiErr *transport.APIError
	if errors.As(err, &apiErr) && apiErr.Capability != transport.CapabilityNone {
		return diag.NewErrorDiagnostic(
			summary+": the server cannot do this as configured", detail)
	}
	return diag.NewErrorDiagnostic(summary, detail)
}

// configureResourceClients is the Configure body every resource shares.
func configureResourceClients(
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) *Clients {
	if req.ProviderData == nil {
		return nil
	}

	clients, ok := req.ProviderData.(*Clients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("Expected *Clients, got %T. This is a bug in the provider.",
				req.ProviderData),
		)
		return nil
	}
	return clients
}
