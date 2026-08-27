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

	"github.com/ioplane/terraform-provider-powerdns/internal/api/transport"
	"github.com/ioplane/terraform-provider-powerdns/internal/provider/normalise"
	"github.com/ioplane/terraform-provider-powerdns/internal/provider/planmodify"
)

var (
	_ resource.Resource                = (*viewZoneResource)(nil)
	_ resource.ResourceWithIdentity    = (*viewZoneResource)(nil)
	_ resource.ResourceWithConfigure   = (*viewZoneResource)(nil)
	_ resource.ResourceWithImportState = (*viewZoneResource)(nil)
)

// viewZoneResource places a zone in a view.
//
// # Why the resource is the membership, not the view
//
// A view has no object of its own. It exists because zones point at it and
// stops existing when the last one is removed, so there is nothing to create
// or delete directly — `POST /views/{view}` adds a zone and creates the view
// as a side effect.
//
// Modelling the membership matches that exactly. A `powerdns_view` resource
// would have to invent a lifecycle the API does not have, and would fight any
// second resource adding a zone to the same view.
//
// # LMDB only
//
// The relational backends do not implement views. A write answers 422 and the
// transport classifies it, so the diagnostic names the backend requirement
// rather than repeating a status code.
type viewZoneResource struct {
	clients *Clients
}

// NewViewZoneResource returns the resource factory.
func NewViewZoneResource() resource.Resource {
	return &viewZoneResource{}
}

type viewZoneModel struct {
	ID   types.String `tfsdk:"id"`
	View types.String `tfsdk:"view"`
	Zone types.String `tfsdk:"zone"`
}

func (r *viewZoneResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_view_zone"
}

func (r *viewZoneResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Places a zone in a view.\n\n" +
			"**Requires the LMDB backend.** The relational backends do not implement " +
			"views; a write answers `422` and this provider reports the requirement " +
			"rather than the status code.\n\n" +
			"The resource is the membership, not the view. A view has no object of its " +
			"own: it exists because zones point at it and disappears when the last one " +
			"is removed, so there is nothing to create or delete directly.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "`<view>/<zone>`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"view": schema.StringAttribute{
				MarkdownDescription: "The view name. Created on first use.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"zone": schema.StringAttribute{
				MarkdownDescription: "The zone to place in the view.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					planmodify.SemanticString(
						"compared as a DNS name", normalise.DNSName),
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *viewZoneResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	r.clients = configureResourceClients(req, resp)
}

func (r *viewZoneResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan viewZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_view_zone")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	view := plan.View.ValueString()
	zoneID := canonicalName(plan.Zone.ValueString())

	if err := client.AddZoneToView(ctx, view, zoneID); err != nil {
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error adding the zone to the view", view+" / "+zoneID, err))
		return
	}

	plan.ID = types.StringValue(view + "/" + zoneID)
	resp.Diagnostics.Append(setViewZoneIdentity(ctx, resp.Identity, view, zoneID)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *viewZoneResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state viewZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_view_zone")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zones, err := client.GetView(ctx, state.View.ValueString())
	if err != nil {
		// A view with no zones does not exist, so a 404 here means the
		// membership is gone — along with every other one in that view.
		if errors.Is(err, transport.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error reading the view", state.View.ValueString(), err))
		return
	}

	wanted := state.Zone.ValueString()
	for _, zone := range zones {
		if normalise.DNSName(wanted, zone) {
			resp.Diagnostics.Append(setViewZoneIdentity(ctx, resp.Identity,
				state.View.ValueString(), canonicalName(wanted))...)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

// Update is unreachable: both attributes force replacement.
//
// The API has no operation that would move a zone between views, so there is
// nothing an update could call even if the schema allowed one.
func (r *viewZoneResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan viewZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *viewZoneResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state viewZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := r.clients.RequireAuth("powerdns_view_zone")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	view := state.View.ValueString()
	zoneID := canonicalName(state.Zone.ValueString())

	// Removing the last zone removes the view, which is the API's behaviour
	// rather than something to compensate for.
	err := client.RemoveZoneFromView(ctx, view, zoneID)
	if err != nil && !errors.Is(err, transport.ErrNotFound) {
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error removing the zone from the view", view+" / "+zoneID, err))
	}
}

// ImportState takes `<view>/<zone>`.
func (r *viewZoneResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	view, zone, found := strings.Cut(req.ID, "/")
	if !found || view == "" || zone == "" {
		resp.Diagnostics.AddError(
			"Malformed import id",
			fmt.Sprintf("Expected `<view>/<zone>`, for example "+
				"`trusted/example.com.`. Got %q.", req.ID),
		)
		return
	}

	zone = canonicalName(zone)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"),
		view+"/"+zone)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("view"), view)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone"), zone)...)
}
