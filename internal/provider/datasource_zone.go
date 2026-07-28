package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*zoneDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*zoneDataSource)(nil)
)

// zoneDataSource reads a zone that something else manages.
//
// The usual case is a zone created by another team or by pdnsutil, whose
// records this configuration needs to add to. A data source rather than an
// import keeps the ownership honest: reading a zone is not the same as taking
// responsibility for it.
type zoneDataSource struct {
	clients *Clients
}

// NewZoneDataSource returns the data source factory.
func NewZoneDataSource() datasource.DataSource {
	return &zoneDataSource{}
}

type zoneDataSourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	Kind types.String `tfsdk:"kind"`

	Serial       types.Int64 `tfsdk:"serial"`
	EditedSerial types.Int64 `tfsdk:"edited_serial"`

	Masters    types.List   `tfsdk:"masters"`
	Account    types.String `tfsdk:"account"`
	Catalog    types.String `tfsdk:"catalog"`
	DNSSEC     types.Bool   `tfsdk:"dnssec"`
	SOAEdit    types.String `tfsdk:"soa_edit"`
	SOAEditAPI types.String `tfsdk:"soa_edit_api"`

	RecordCount types.Int64 `tfsdk:"record_count"`
}

func (d *zoneDataSource) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_zone"
}

func (d *zoneDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a zone the configuration does not manage.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The canonical zone name.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The zone to read. A trailing dot is added if absent.",
				Required:            true,
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "`Native`, `Master`, `Slave`, `Producer` or `Consumer`.",
				Computed:            true,
			},
			"serial": schema.Int64Attribute{
				MarkdownDescription: "The SOA serial as stored.",
				Computed:            true,
			},
			"edited_serial": schema.Int64Attribute{
				MarkdownDescription: "The serial a query would see, after SOA-EDIT.",
				Computed:            true,
			},
			"masters": schema.ListAttribute{
				MarkdownDescription: "Primary servers, for a `Slave` zone.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"account": schema.StringAttribute{
				MarkdownDescription: "Free-form owner label.",
				Computed:            true,
			},
			"catalog": schema.StringAttribute{
				MarkdownDescription: "The catalog zone this zone belongs to.",
				Computed:            true,
			},
			"dnssec": schema.BoolAttribute{
				MarkdownDescription: "Whether the zone is signed.",
				Computed:            true,
			},
			"soa_edit": schema.StringAttribute{
				MarkdownDescription: "SOA-EDIT policy.",
				Computed:            true,
			},
			"soa_edit_api": schema.StringAttribute{
				MarkdownDescription: "SOA-EDIT-API policy.",
				Computed:            true,
			},
			"record_count": schema.Int64Attribute{
				MarkdownDescription: "How many RRSets the zone holds. The sets themselves " +
					"are not exposed: a zone with thousands of them would put all of " +
					"them in state on every refresh, and `powerdns_record` is the way " +
					"to address one.",
				Computed: true,
			},
		},
	}
}

func (d *zoneDataSource) Configure(
	_ context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	d.clients = configureClients(req, resp)
}

func (d *zoneDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	var config zoneDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := d.clients.RequireAuth("the powerdns_zone data source")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneID := canonicalName(config.Name.ValueString())
	zone, err := client.GetZone(ctx, zoneID)
	if err != nil {
		// A data source reading a missing object is an error, not an empty
		// result: the configuration asked for something that is not there.
		resp.Diagnostics.AddError(
			"Error reading the zone",
			"Zone "+zoneID+".\n\n"+err.Error(),
		)
		return
	}

	config.ID = types.StringValue(zone.ID)
	config.Kind = types.StringValue(zone.Kind)
	config.Serial = types.Int64Value(int64(zone.Serial))
	config.EditedSerial = types.Int64Value(int64(zone.EditedSerial))
	config.Account = types.StringValue(zone.Account)
	config.Catalog = types.StringValue(zone.Catalog)
	config.SOAEdit = types.StringValue(zone.SOAEdit)
	config.SOAEditAPI = types.StringValue(zone.SOAEditAPI)
	config.DNSSEC = types.BoolValue(derefBool(zone.DNSSEC))
	config.RecordCount = types.Int64Value(int64(len(zone.RRSets)))

	masters, listDiags := types.ListValueFrom(ctx, types.StringType, zone.Masters)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.Masters = masters

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// zonesDataSource lists every zone.
type zonesDataSource struct {
	clients *Clients
}

var (
	_ datasource.DataSource              = (*zonesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*zonesDataSource)(nil)
)

// NewZonesDataSource returns the data source factory.
func NewZonesDataSource() datasource.DataSource {
	return &zonesDataSource{}
}

type zonesDataSourceModel struct {
	ID    types.String       `tfsdk:"id"`
	Names types.List         `tfsdk:"names"`
	Zones []zoneSummaryModel `tfsdk:"zones"`
}

type zoneSummaryModel struct {
	ID     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	Kind   types.String `tfsdk:"kind"`
	Serial types.Int64  `tfsdk:"serial"`
}

func (d *zonesDataSource) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_zones"
}

func (d *zonesDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Every zone on the server.\n\n" +
			"The list endpoint omits records, so this is cheap even on a server " +
			"holding thousands of zones. Reading one zone's records needs " +
			"`powerdns_zone` and `powerdns_record`.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Fixed: this data source reads the whole server.",
				Computed:            true,
			},
			"names": schema.ListAttribute{
				MarkdownDescription: "Zone names, for the common case of iterating them.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"zones": schema.ListNestedAttribute{
				MarkdownDescription: "Each zone with its kind and serial.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":     schema.StringAttribute{Computed: true},
						"name":   schema.StringAttribute{Computed: true},
						"kind":   schema.StringAttribute{Computed: true},
						"serial": schema.Int64Attribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *zonesDataSource) Configure(
	_ context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	d.clients = configureClients(req, resp)
}

func (d *zonesDataSource) Read(
	ctx context.Context,
	_ datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	client, diags := d.clients.RequireAuth("the powerdns_zones data source")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zones, err := client.ListZones(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing the zones", err.Error())
		return
	}

	state := zonesDataSourceModel{
		ID:    types.StringValue("zones"),
		Zones: make([]zoneSummaryModel, 0, len(zones)),
	}

	names := make([]string, 0, len(zones))
	for _, zone := range zones {
		names = append(names, zone.Name)
		state.Zones = append(state.Zones, zoneSummaryModel{
			ID:     types.StringValue(zone.ID),
			Name:   types.StringValue(zone.Name),
			Kind:   types.StringValue(zone.Kind),
			Serial: types.Int64Value(int64(zone.Serial)),
		})
	}

	list, listDiags := types.ListValueFrom(ctx, types.StringType, names)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Names = list

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
