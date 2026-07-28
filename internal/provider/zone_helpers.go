package provider

import (
	"context"
	"errors"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-powerdns/internal/api/auth"
	"github.com/ioplane/terraform-provider-powerdns/internal/api/transport"
)

// canonicalName appends the trailing dot PowerDNS stores.
//
// Applied on the way in rather than left to the server, so the id in state is
// the id the API will answer to. Without it, a zone configured as
// "example.com" is created as "example.com." and every subsequent read by the
// configured name 404s.
func canonicalName(name string) string {
	if name == "" || strings.HasSuffix(name, ".") {
		return name
	}
	return name + "."
}

// elementsAs extracts a list into a Go slice, leaving it nil when the
// attribute is unset.
//
// A null list and an empty list mean different things to PowerDNS — unset
// versus explicitly empty — and conflating them would let an empty
// configuration clear a value the operator never mentioned.
func elementsAs(ctx context.Context, list types.List, out *[]string) diag.Diagnostics {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}
	return list.ElementsAs(ctx, out, false)
}

// stringListValue converts a slice back into a list attribute, mapping an
// empty slice to null so a server that omits a field does not read as an
// operator setting it to empty.
func stringListValue(ctx context.Context, values []string) (types.List, diag.Diagnostics) {
	if len(values) == 0 {
		return types.ListNull(types.StringType), nil
	}
	return types.ListValueFrom(ctx, types.StringType, values)
}

// applyMode says whose value wins when the server and the plan disagree about
// how to spell the same thing.
type applyMode int

const (
	// afterWrite is Create and Update. Terraform requires that a create or
	// update return exactly the planned value for every attribute the
	// configuration set: returning `Native` where the plan said `native`
	// fails with "Provider produced inconsistent result after apply", which
	// is a framework contract and not something a plan modifier can rescue.
	//
	// So the planned spelling is kept, and the semantic modifiers earn their
	// keep on the *next* plan, when state holds the server's spelling.
	afterWrite applyMode = iota

	// afterRead is Read. Here the server's value is the truth: writing it into
	// state is how drift becomes visible, and the semantic modifiers are what
	// stop a mere respelling from looking like drift.
	afterRead
)

// applyZone writes a server response into the model.
//
// Computed attributes always come from the server — there is no configured
// value to conflict with. Everything else depends on the mode; see applyMode.
func applyZone(
	ctx context.Context,
	zone *auth.Zone,
	model *zoneModel,
	mode applyMode,
) diag.Diagnostics {
	var diags diag.Diagnostics

	// Computed, always the server's.
	model.ID = types.StringValue(zone.ID)
	model.Serial = types.Int64Value(int64(zone.Serial))
	model.EditedSerial = types.Int64Value(int64(zone.EditedSerial))

	// Optional and Computed: the server assigns DEFAULT when the operator says
	// nothing, so an unknown or null plan value takes what came back. A value
	// the operator did set is kept, because changing it would break the same
	// contract as kind.
	if mode == afterRead || model.SOAEditAPI.IsUnknown() || model.SOAEditAPI.IsNull() {
		model.SOAEditAPI = types.StringValue(zone.SOAEditAPI)
	}

	// dnssec is Optional and Computed with no default, because adding a
	// powerdns_zone_cryptokey turns it on server-side. An unknown plan value
	// must be resolved here: Terraform rejects an unknown that survives apply.
	if mode == afterRead || model.DNSSEC.IsUnknown() || model.DNSSEC.IsNull() {
		model.DNSSEC = types.BoolValue(derefBool(zone.DNSSEC))
	}
	if mode == afterRead || model.NSEC3Param.IsUnknown() || model.NSEC3Param.IsNull() {
		model.NSEC3Param = types.StringValue(zone.NSEC3Param)
	}
	if mode == afterRead || model.NSEC3Narrow.IsUnknown() || model.NSEC3Narrow.IsNull() {
		model.NSEC3Narrow = types.BoolValue(derefBool(zone.NSEC3Narrow))
	}
	if mode == afterRead || model.Presigned.IsUnknown() || model.Presigned.IsNull() {
		model.Presigned = types.BoolValue(derefBool(zone.Presigned))
	}

	if mode == afterRead {
		model.Kind = types.StringValue(zone.Kind)
		model.APIRectify = types.BoolValue(derefBool(zone.APIRectify))

		// Optional and not returned when unset: mapping "" to null keeps an
		// unset attribute unset rather than flipping it to the empty string.
		model.Account = optionalString(zone.Account)
		model.Catalog = optionalString(zone.Catalog)
		model.SOAEdit = optionalString(zone.SOAEdit)

		masters, d := stringListValue(ctx, zone.Masters)
		diags.Append(d...)
		model.Masters = masters
	}

	// Computed as well as Optional: PowerDNS returns an empty list rather than
	// omitting these, so an unknown plan value has to be resolved here.
	if mode == afterRead || model.MasterTSIGKeyIDs.IsUnknown() {
		list, d := types.ListValueFrom(ctx, types.StringType, zone.MasterTSIGKeyIDs)
		diags.Append(d...)
		model.MasterTSIGKeyIDs = list
	}
	if mode == afterRead || model.SlaveTSIGKeyIDs.IsUnknown() {
		list, d := types.ListValueFrom(ctx, types.StringType, zone.SlaveTSIGKeyIDs)
		diags.Append(d...)
		model.SlaveTSIGKeyIDs = list
	}

	return diags
}

// applyDNSSECAttributes copies the signing attributes onto a request body.
//
// Each is Optional and Computed, so an unknown value means "the configuration
// said nothing" and must not be sent — sending the zero value would turn NSEC3
// off on a zone that has it.
func applyDNSSECAttributes(model zoneModel, zone *auth.Zone) {
	if !model.NSEC3Param.IsUnknown() && !model.NSEC3Param.IsNull() {
		zone.NSEC3Param = model.NSEC3Param.ValueString()
	}
	if !model.NSEC3Narrow.IsUnknown() && !model.NSEC3Narrow.IsNull() {
		zone.NSEC3Narrow = model.NSEC3Narrow.ValueBoolPointer()
	}
	if !model.Presigned.IsUnknown() && !model.Presigned.IsNull() {
		zone.Presigned = model.Presigned.ValueBoolPointer()
	}
}

// applyTSIGKeyLists copies the zone's TSIG key lists onto a request body.
func applyTSIGKeyLists(ctx context.Context, model zoneModel, zone *auth.Zone) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.Append(elementsAs(ctx, model.MasterTSIGKeyIDs, &zone.MasterTSIGKeyIDs)...)
	diags.Append(elementsAs(ctx, model.SlaveTSIGKeyIDs, &zone.SlaveTSIGKeyIDs)...)
	return diags
}

// optionalString maps the empty string to null.
func optionalString(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

// derefBool reads a tri-state bool as false when absent.
func derefBool(value *bool) bool {
	return value != nil && *value
}

// listRequiresReplace is RequiresReplace for a list attribute.
func listRequiresReplace() planmodifier.List {
	return listplanmodifier.RequiresReplace()
}

// zoneDiagnostic turns a client error into an operator-facing diagnostic.
//
// The transport has already classified capability failures, and its message
// names the backend setting to change. This adds only what the transport
// cannot know: which zone, and what was being attempted.
func zoneDiagnostic(action, zoneID string, err error) diag.Diagnostic {
	summary := "Error " + action + " the zone"

	var apiErr *transport.APIError
	if errors.As(err, &apiErr) && apiErr.Capability != transport.CapabilityNone {
		// A capability failure is a configuration problem on the server, not a
		// transient one. Saying so keeps an operator from retrying.
		return diag.NewErrorDiagnostic(
			summary+": the server cannot do this as configured",
			"Zone "+zoneID+".\n\n"+err.Error(),
		)
	}

	return diag.NewErrorDiagnostic(summary, "Zone "+zoneID+".\n\n"+err.Error())
}
