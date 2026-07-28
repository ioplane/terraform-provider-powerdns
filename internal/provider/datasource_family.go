package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Data sources for the Recursor and dnsdist.
//
// Both products are mostly read-only over HTTP — the Recursor writes two
// settings and its zones, dnsdist writes one setting and a cache flush — so
// most of what they expose is only useful as a data source. That is the
// asymmetry these cover: statistics and configuration a configuration wants to
// read and has no business managing.

var (
	_ datasource.DataSource              = (*recursorZoneDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*recursorZoneDataSource)(nil)

	_ datasource.DataSource              = (*dnsdistServerDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*dnsdistServerDataSource)(nil)
)

// recursorZoneDataSource reads a Recursor zone.
type recursorZoneDataSource struct {
	clients *Clients
}

// NewRecursorZoneDataSource returns the data source factory.
func NewRecursorZoneDataSource() datasource.DataSource {
	return &recursorZoneDataSource{}
}

type recursorZoneDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Kind             types.String `tfsdk:"kind"`
	Servers          types.List   `tfsdk:"servers"`
	RecursionDesired types.Bool   `tfsdk:"recursion_desired"`
	NotifyAllowed    types.Bool   `tfsdk:"notify_allowed"`
}

func (d *recursorZoneDataSource) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_recursor_zone"
}

func (d *recursorZoneDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a zone the Recursor holds.\n\n" +
			"Useful for a zone somebody else configured — an upstream forwarder set " +
			"by another team, whose target this configuration needs to know.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The canonical zone name.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The zone to read.",
				Required:            true,
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "`Native` or `Forwarded`.",
				Computed:            true,
			},
			"servers": schema.ListAttribute{
				MarkdownDescription: "Upstreams, for a `Forwarded` zone. The Recursor " +
					"reports these with the port it defaulted, so an address given as " +
					"`192.0.2.53` reads back as `192.0.2.53:53`.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"recursion_desired": schema.BoolAttribute{
				MarkdownDescription: "Whether forwarded queries carry the RD bit.",
				Computed:            true,
			},
			"notify_allowed": schema.BoolAttribute{
				MarkdownDescription: "Whether NOTIFY is accepted for this zone.",
				Computed:            true,
			},
		},
	}
}

func (d *recursorZoneDataSource) Configure(
	_ context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	d.clients = configureClients(req, resp)
}

func (d *recursorZoneDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	var config recursorZoneDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := d.clients.RequireRecursor("the powerdns_recursor_zone data source")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneID := canonicalName(config.Name.ValueString())
	zone, err := client.GetZone(ctx, zoneID)
	if err != nil {
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error reading the Recursor zone", zoneID, err))
		return
	}

	config.ID = types.StringValue(zone.ID)
	config.Kind = types.StringValue(zone.Kind)
	config.RecursionDesired = types.BoolValue(derefBool(zone.RecursionDesired))
	config.NotifyAllowed = types.BoolValue(derefBool(zone.NotifyAllowed))

	servers, listDiags := types.ListValueFrom(ctx, types.StringType, zone.Servers)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.Servers = servers

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// dnsdistServerDataSource reads dnsdist's summary object.
//
// One data source rather than several, because dnsdist answers
// `/servers/localhost` with everything: the ACL, the downstreams, the pools.
// Splitting it would mean several requests for one response.
type dnsdistServerDataSource struct {
	clients *Clients
}

// NewDNSDistServerDataSource returns the data source factory.
func NewDNSDistServerDataSource() datasource.DataSource {
	return &dnsdistServerDataSource{}
}

type dnsdistServerDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Version     types.String `tfsdk:"version"`
	ACL         types.String `tfsdk:"acl"`
	Downstreams types.List   `tfsdk:"downstreams"`
}

// dnsdistDownstreamModel is one backend behind dnsdist.
type dnsdistDownstreamModel struct {
	Name    types.String `tfsdk:"name"`
	Address types.String `tfsdk:"address"`
	State   types.String `tfsdk:"state"`
	Pools   types.List   `tfsdk:"pools"`
	Queries types.Int64  `tfsdk:"queries"`
}

func (d *dnsdistServerDataSource) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_dnsdist_server"
}

func (d *dnsdistServerDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads dnsdist's state: its version, ACL and downstream " +
			"servers.\n\n" +
			"Read-only, and not by omission. Downstreams, pools, rules and dynamic " +
			"blocks are configured in Lua or YAML and have no HTTP write path at all " +
			"— `powerdns_dnsdist_acl` and a cache flush are the whole of what dnsdist's " +
			"API permits. This is how a configuration observes what Lua decided.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Fixed: dnsdist reports one server.",
				Computed:            true,
			},
			"version": schema.StringAttribute{
				MarkdownDescription: "The running dnsdist version.",
				Computed:            true,
			},
			"acl": schema.StringAttribute{
				MarkdownDescription: "The ACL as dnsdist renders it in the summary — a " +
					"string, unlike the list `powerdns_dnsdist_acl` writes.",
				Computed: true,
			},
			"downstreams": schema.ListNestedAttribute{
				MarkdownDescription: "The backend servers, as Lua configured them.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":    schema.StringAttribute{Computed: true},
						"address": schema.StringAttribute{Computed: true},
						"state": schema.StringAttribute{
							Computed: true,
							MarkdownDescription: "Current health. Read this rather than " +
								"a failure count, which is cumulative since start.",
						},
						"pools":   schema.ListAttribute{Computed: true, ElementType: types.StringType},
						"queries": schema.Int64Attribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *dnsdistServerDataSource) Configure(
	_ context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	d.clients = configureClients(req, resp)
}

func (d *dnsdistServerDataSource) Read(
	ctx context.Context,
	_ datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	client, diags := d.clients.RequireDNSDist("the powerdns_dnsdist_server data source")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	server, err := client.GetServer(ctx)
	if err != nil {
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error reading the dnsdist server", "localhost", err))
		return
	}

	state := dnsdistServerDataSourceModel{
		ID:      types.StringValue("localhost"),
		Version: types.StringValue(server.Version),
		ACL:     types.StringValue(server.ACL),
	}

	downstreams := make([]dnsdistDownstreamModel, 0, len(server.Servers))
	for _, backend := range server.Servers {
		pools, listDiags := types.ListValueFrom(ctx, types.StringType, backend.Pools)
		resp.Diagnostics.Append(listDiags...)
		if resp.Diagnostics.HasError() {
			return
		}

		downstreams = append(downstreams, dnsdistDownstreamModel{
			Name:    types.StringValue(backend.Name),
			Address: types.StringValue(backend.Address),
			State:   types.StringValue(backend.State),
			Pools:   pools,
			Queries: types.Int64Value(int64(backend.Queries)), //nolint:gosec // a counter
		})
	}

	list, listDiags := types.ListValueFrom(ctx,
		types.ObjectType{AttrTypes: map[string]attr.Type{
			"name":    types.StringType,
			"address": types.StringType,
			"state":   types.StringType,
			"pools":   types.ListType{ElemType: types.StringType},
			"queries": types.Int64Type,
		}}, downstreams)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Downstreams = list

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
