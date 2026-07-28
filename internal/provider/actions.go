package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Actions are the imperative operations PowerDNS has and Terraform previously
// had nowhere to put.
//
// Notifying secondaries, triggering a transfer, rectifying a zone and flushing
// a cache are all things an operator does *to* a zone rather than states *about*
// it. They have no state to converge on, so modelling them as resources means
// inventing one — a `powerdns_zone_notify` resource whose only content is
// "when did I last run", which then drifts and re-runs.
//
// This was the conclusion of CM-04 §3 in the capability map before Terraform
// 1.14 shipped actions, and it was the reason 24 operations were listed as
// uncoverable. Actions covered 19 of them.
//
// Every action here is idempotent in the sense that matters: running it twice
// does no harm. None of them can be undone, which is why none of them is a
// resource.

var (
	_ action.Action              = (*notifyZoneAction)(nil)
	_ action.ActionWithConfigure = (*notifyZoneAction)(nil)

	_ action.Action              = (*axfrRetrieveAction)(nil)
	_ action.ActionWithConfigure = (*axfrRetrieveAction)(nil)

	_ action.Action              = (*rectifyZoneAction)(nil)
	_ action.ActionWithConfigure = (*rectifyZoneAction)(nil)

	_ action.Action              = (*flushCacheAction)(nil)
	_ action.ActionWithConfigure = (*flushCacheAction)(nil)
)

// zoneActionModel is the configuration every zone action takes.
type zoneActionModel struct {
	Zone types.String `tfsdk:"zone"`
}

// zoneActionSchema is the schema every zone action shares.
func zoneActionSchema(description string) schema.Schema {
	return schema.Schema{
		MarkdownDescription: description,
		Attributes: map[string]schema.Attribute{
			"zone": schema.StringAttribute{
				MarkdownDescription: "The zone to act on.",
				Required:            true,
			},
		},
	}
}

// notifyZoneAction sends a NOTIFY to a zone's secondaries.
type notifyZoneAction struct {
	clients *Clients
}

// NewNotifyZoneAction returns the action factory.
func NewNotifyZoneAction() action.Action { return &notifyZoneAction{} }

func (a *notifyZoneAction) Metadata(
	_ context.Context,
	req action.MetadataRequest,
	resp *action.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_notify_zone"
}

func (a *notifyZoneAction) Schema(
	_ context.Context,
	_ action.SchemaRequest,
	resp *action.SchemaResponse,
) {
	resp.Schema = zoneActionSchema(
		"Sends a NOTIFY to a zone's secondaries.\n\n" +
			"PowerDNS answers `Notification queued` even for a `Native` zone with no " +
			"secondaries to notify. Queued is not delivered, and this reports the " +
			"former — there is no API that reports the latter.")
}

func (a *notifyZoneAction) Configure(
	_ context.Context,
	req action.ConfigureRequest,
	resp *action.ConfigureResponse,
) {
	a.clients = configureActionClients(req, resp)
}

func (a *notifyZoneAction) Invoke(
	ctx context.Context,
	req action.InvokeRequest,
	resp *action.InvokeResponse,
) {
	var config zoneActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := a.clients.RequireAuth("the powerdns_notify_zone action")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneID := canonicalName(config.Zone.ValueString())
	if err := client.NotifyZone(ctx, zoneID); err != nil {
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error notifying the zone's secondaries", zoneID, err))
	}
}

// axfrRetrieveAction triggers a transfer from a zone's primary.
type axfrRetrieveAction struct {
	clients *Clients
}

// NewAXFRRetrieveAction returns the action factory.
func NewAXFRRetrieveAction() action.Action { return &axfrRetrieveAction{} }

func (a *axfrRetrieveAction) Metadata(
	_ context.Context,
	req action.MetadataRequest,
	resp *action.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_axfr_retrieve"
}

func (a *axfrRetrieveAction) Schema(
	_ context.Context,
	_ action.SchemaRequest,
	resp *action.SchemaResponse,
) {
	resp.Schema = zoneActionSchema(
		"Triggers a zone transfer from the zone's primary.\n\n" +
			"`Slave` zones only. Any other kind answers 422 naming the reason, which " +
			"this reports rather than the status code.")
}

func (a *axfrRetrieveAction) Configure(
	_ context.Context,
	req action.ConfigureRequest,
	resp *action.ConfigureResponse,
) {
	a.clients = configureActionClients(req, resp)
}

func (a *axfrRetrieveAction) Invoke(
	ctx context.Context,
	req action.InvokeRequest,
	resp *action.InvokeResponse,
) {
	var config zoneActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := a.clients.RequireAuth("the powerdns_axfr_retrieve action")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneID := canonicalName(config.Zone.ValueString())
	if err := client.AXFRRetrieveZone(ctx, zoneID); err != nil {
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error retrieving the zone by AXFR", zoneID, err))
	}
}

// rectifyZoneAction recomputes DNSSEC ordering and NSEC records.
type rectifyZoneAction struct {
	clients *Clients
}

// NewRectifyZoneAction returns the action factory.
func NewRectifyZoneAction() action.Action { return &rectifyZoneAction{} }

func (a *rectifyZoneAction) Metadata(
	_ context.Context,
	req action.MetadataRequest,
	resp *action.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_rectify_zone"
}

func (a *rectifyZoneAction) Schema(
	_ context.Context,
	_ action.SchemaRequest,
	resp *action.SchemaResponse,
) {
	resp.Schema = zoneActionSchema(
		"Recomputes DNSSEC ordering and NSEC records for a zone.\n\n" +
			"Succeeds on an unsigned zone and on one with `api_rectify` already on — " +
			"neither is refused, contrary to what the name suggests. A `Slave` zone " +
			"with nothing transferred answers 422 `No SOA known`.")
}

func (a *rectifyZoneAction) Configure(
	_ context.Context,
	req action.ConfigureRequest,
	resp *action.ConfigureResponse,
) {
	a.clients = configureActionClients(req, resp)
}

func (a *rectifyZoneAction) Invoke(
	ctx context.Context,
	req action.InvokeRequest,
	resp *action.InvokeResponse,
) {
	var config zoneActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := a.clients.RequireAuth("the powerdns_rectify_zone action")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneID := canonicalName(config.Zone.ValueString())
	if err := client.RectifyZone(ctx, zoneID); err != nil {
		resp.Diagnostics.Append(capabilityDiagnostic(
			"Error rectifying the zone", zoneID, err))
	}
}

// flushCacheAction drops a domain from a cache.
//
// One action for all three products, chosen by which endpoint is configured,
// because "flush this name" is one operation an operator wants and three
// endpoints that implement it. Three actions would make the caller work out
// which applies.
type flushCacheAction struct {
	clients *Clients
}

// NewFlushCacheAction returns the action factory.
func NewFlushCacheAction() action.Action { return &flushCacheAction{} }

type flushCacheModel struct {
	Domain  types.String `tfsdk:"domain"`
	Product types.String `tfsdk:"product"`
	Type    types.String `tfsdk:"type"`
	Pool    types.String `tfsdk:"pool"`
}

func (a *flushCacheAction) Metadata(
	_ context.Context,
	req action.MetadataRequest,
	resp *action.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_flush_cache"
}

func (a *flushCacheAction) Schema(
	_ context.Context,
	_ action.SchemaRequest,
	resp *action.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Drops a domain from a cache.\n\n" +
			"One action for all three products: flushing a name is one operation an " +
			"operator wants, and three endpoints that implement it. `product` chooses " +
			"which.\n\n" +
			"A count of zero is a successful flush — nothing cached means nothing to " +
			"drop. On dnsdist, a pool with no packet cache answers 404, which this " +
			"reports as the missing cache rather than a missing endpoint.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				MarkdownDescription: "The name to drop.",
				Required:            true,
			},
			"product": schema.StringAttribute{
				MarkdownDescription: "`auth`, `recursor` or `dnsdist`.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The record type, for dnsdist only. Defaults to " +
					"`ANY`.",
				Optional: true,
			},
			"pool": schema.StringAttribute{
				MarkdownDescription: "The dnsdist pool, for dnsdist only. The default " +
					"pool is the empty string, which is also the default here.",
				Optional: true,
			},
		},
	}
}

func (a *flushCacheAction) Configure(
	_ context.Context,
	req action.ConfigureRequest,
	resp *action.ConfigureResponse,
) {
	a.clients = configureActionClients(req, resp)
}

func (a *flushCacheAction) Invoke(
	ctx context.Context,
	req action.InvokeRequest,
	resp *action.InvokeResponse,
) {
	var config flushCacheModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain := canonicalName(config.Domain.ValueString())

	switch product := config.Product.ValueString(); product {
	case "auth":
		client, diags := a.clients.RequireAuth("the powerdns_flush_cache action")
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if _, err := client.FlushCache(ctx, domain); err != nil {
			resp.Diagnostics.Append(capabilityDiagnostic(
				"Error flushing the Authoritative cache", domain, err))
		}

	case "recursor":
		client, diags := a.clients.RequireRecursor("the powerdns_flush_cache action")
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if _, err := client.FlushCache(ctx, domain); err != nil {
			resp.Diagnostics.Append(capabilityDiagnostic(
				"Error flushing the Recursor cache", domain, err))
		}

	case "dnsdist":
		client, diags := a.clients.RequireDNSDist("the powerdns_flush_cache action")
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		recordType := config.Type.ValueString()
		if recordType == "" {
			recordType = "ANY"
		}
		if _, err := client.FlushCache(ctx, config.Pool.ValueString(), domain, recordType); err != nil {
			resp.Diagnostics.Append(capabilityDiagnostic(
				"Error flushing the dnsdist cache", domain, err))
		}

	default:
		resp.Diagnostics.AddError(
			"Unknown product "+product,
			"product must be auth, recursor or dnsdist.",
		)
	}
}

// configureActionClients is the Configure body every action shares.
func configureActionClients(
	req action.ConfigureRequest,
	resp *action.ConfigureResponse,
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
