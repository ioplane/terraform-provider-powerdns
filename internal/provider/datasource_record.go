package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*recordDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*recordDataSource)(nil)

	_ datasource.DataSource              = (*zoneMetadataDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*zoneMetadataDataSource)(nil)

	_ datasource.DataSource              = (*zoneExportDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*zoneExportDataSource)(nil)
)

// recordDataSource reads one RRSet.
//
// There is no endpoint for a single RRSet: reading one means reading the whole
// zone and finding it. That is the API's shape, and the cost is worth knowing
// before putting this in a loop over a large zone.
type recordDataSource struct {
	clients *Clients
}

// NewRecordDataSource returns the data source factory.
func NewRecordDataSource() datasource.DataSource {
	return &recordDataSource{}
}

type recordDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Zone     types.String `tfsdk:"zone"`
	Name     types.String `tfsdk:"name"`
	Type     types.String `tfsdk:"type"`
	TTL      types.Int64  `tfsdk:"ttl"`
	Values   types.List   `tfsdk:"values"`
	Disabled types.Bool   `tfsdk:"disabled"`
}

func (d *recordDataSource) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_record"
}

func (d *recordDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads one RRSet.\n\n" +
			"PowerDNS has no endpoint for a single RRSet, so this reads the zone and " +
			"selects from it. On a large zone that is not cheap; prefer one read and " +
			"local lookups over this in a loop.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "`<zone>/<name>/<type>`.",
				Computed:            true,
			},
			"zone": schema.StringAttribute{
				MarkdownDescription: "The zone holding the set.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The owner name.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The record type.",
				Required:            true,
			},
			"ttl": schema.Int64Attribute{
				MarkdownDescription: "Time to live, in seconds.",
				Computed:            true,
			},
			"values": schema.ListAttribute{
				MarkdownDescription: "The record values.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"disabled": schema.BoolAttribute{
				MarkdownDescription: "True when any record in the set is not served.",
				Computed:            true,
			},
		},
	}
}

func (d *recordDataSource) Configure(
	_ context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	d.clients = configureClients(req, resp)
}

func (d *recordDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	var config recordDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := d.clients.RequireAuth("the powerdns_record data source")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneID := canonicalName(config.Zone.ValueString())
	zone, err := client.GetZone(ctx, zoneID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading the zone holding this record",
			"Zone "+zoneID+".\n\n"+err.Error(),
		)
		return
	}

	name := canonicalName(config.Name.ValueString())
	recordType := config.Type.ValueString()

	rrset := findRRSet(zone.RRSets, name, recordType)
	if rrset == nil {
		// A data source asking for something absent is an error. Returning an
		// empty result would let a configuration proceed on values that do not
		// exist.
		resp.Diagnostics.AddError(
			"Record set not found",
			fmt.Sprintf("No %s set for %s in zone %s. The zone holds %d set(s).",
				recordType, name, zoneID, len(zone.RRSets)),
		)
		return
	}

	values := make([]string, 0, len(rrset.Records))
	var anyDisabled bool
	for _, record := range rrset.Records {
		values = append(values, record.Content)
		if record.Disabled {
			anyDisabled = true
		}
	}

	list, listDiags := types.ListValueFrom(ctx, types.StringType, values)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	config.ID = types.StringValue(recordID(zoneID, rrset.Name, rrset.Type))
	config.TTL = types.Int64Value(int64(rrset.TTL))
	config.Values = list
	config.Disabled = types.BoolValue(anyDisabled)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// zoneMetadataDataSource reads one metadata kind.
type zoneMetadataDataSource struct {
	clients *Clients
}

// NewZoneMetadataDataSource returns the data source factory.
func NewZoneMetadataDataSource() datasource.DataSource {
	return &zoneMetadataDataSource{}
}

type zoneMetadataDataSourceModel struct {
	ID     types.String `tfsdk:"id"`
	Zone   types.String `tfsdk:"zone"`
	Kind   types.String `tfsdk:"kind"`
	Values types.List   `tfsdk:"values"`
}

func (d *zoneMetadataDataSource) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_zone_metadata"
}

func (d *zoneMetadataDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads one metadata kind on a zone.\n\n" +
			"An unset kind is not an error: PowerDNS answers with an empty list rather " +
			"than a 404, so `values` is empty and the configuration can branch on it.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "`<zone>/<kind>`.",
				Computed:            true,
			},
			"zone": schema.StringAttribute{
				MarkdownDescription: "The zone.",
				Required:            true,
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "The metadata kind.",
				Required:            true,
			},
			"values": schema.ListAttribute{
				MarkdownDescription: "The values, empty when the kind is unset.",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (d *zoneMetadataDataSource) Configure(
	_ context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	d.clients = configureClients(req, resp)
}

func (d *zoneMetadataDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	var config zoneMetadataDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := d.clients.RequireAuth("the powerdns_zone_metadata data source")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	kind := config.Kind.ValueString()
	resp.Diagnostics.Append(checkMetadataKind(kind)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneID := canonicalName(config.Zone.ValueString())

	entry, err := client.GetMetadata(ctx, zoneID, kind)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading the zone metadata",
			kind+" on "+zoneID+".\n\n"+err.Error(),
		)
		return
	}

	list, listDiags := types.ListValueFrom(ctx, types.StringType, entry.Metadata)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	config.ID = types.StringValue(zoneID + "/" + kind)
	config.Values = list

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// zoneExportDataSource reads a zone in presentation format.
type zoneExportDataSource struct {
	clients *Clients
}

// NewZoneExportDataSource returns the data source factory.
func NewZoneExportDataSource() datasource.DataSource {
	return &zoneExportDataSource{}
}

type zoneExportDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Zone     types.String `tfsdk:"zone"`
	ZoneFile types.String `tfsdk:"zone_file"`
}

func (d *zoneExportDataSource) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_zone_export"
}

func (d *zoneExportDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Exports a zone as a zone file.\n\n" +
			"For backups and for diffing against a file kept in version control. " +
			"The whole zone lands in state, so this is not something to run against " +
			"a zone with thousands of records on every refresh.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The canonical zone name.",
				Computed:            true,
			},
			"zone": schema.StringAttribute{
				MarkdownDescription: "The zone to export.",
				Required:            true,
			},
			"zone_file": schema.StringAttribute{
				MarkdownDescription: "The zone in presentation format, tabs and newlines " +
					"included.",
				Computed: true,
			},
		},
	}
}

func (d *zoneExportDataSource) Configure(
	_ context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	d.clients = configureClients(req, resp)
}

func (d *zoneExportDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	var config zoneExportDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := d.clients.RequireAuth("the powerdns_zone_export data source")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneID := canonicalName(config.Zone.ValueString())
	zoneFile, err := client.ExportZone(ctx, zoneID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error exporting the zone",
			"Zone "+zoneID+".\n\n"+err.Error(),
		)
		return
	}

	config.ID = types.StringValue(zoneID)
	config.ZoneFile = types.StringValue(zoneFile)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// configureClients is the Configure body every data source shares.
//
// Five copies of the same type assertion is five places for it to be written
// slightly differently, and the one that is wrong fails at apply rather than
// at compile time.
func configureClients(
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
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
