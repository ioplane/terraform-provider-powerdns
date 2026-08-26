package provider

import (
	"context"
	"errors"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-powerdns/internal/api/auth"
	"github.com/ioplane/terraform-provider-powerdns/internal/api/transport"
	"github.com/ioplane/terraform-provider-powerdns/internal/provider/normalise"
)

// recordIDParts is the number of segments in `<zone>/<name>/<type>`.
const recordIDParts = 3

// recordID builds the composite id.
//
// The three parts are what the API needs to address the set, and PowerDNS
// offers no id of its own — an RRSet is identified by where it sits, not by a
// key it carries.
func recordID(zone, name, recordType string) string {
	return zone + "/" + name + "/" + recordType
}

// rrsetFromModel turns the model into a request body.
func rrsetFromModel(
	ctx context.Context,
	model recordModel,
	changeType string,
) (auth.RRSet, diag.Diagnostics) {
	var diags diag.Diagnostics

	var values []string
	diags.Append(elementsAs(ctx, model.Values, &values)...)
	if diags.HasError() {
		return auth.RRSet{}, diags
	}

	disabled := model.Disabled.ValueBool()
	records := make([]auth.Record, 0, len(values))
	for _, value := range values {
		// One disabled flag for the whole set. PowerDNS stores it per record,
		// but disabling half a set is not a state anyone models deliberately,
		// and offering it per value would make the attribute a parallel list
		// that has to stay index-aligned with values.
		records = append(records, auth.Record{Content: value, Disabled: disabled})
	}

	rrset := auth.RRSet{
		Name:       canonicalName(model.Name.ValueString()),
		Type:       model.Type.ValueString(),
		TTL:        uint32(model.TTL.ValueInt64()), //nolint:gosec // a TTL is bounded by the schema
		ChangeType: changeType,
		Records:    records,
	}

	if comment := model.Comment.ValueString(); comment != "" {
		rrset.Comments = []auth.Comment{{Content: comment}}
	}

	return rrset, diags
}

// findRRSet locates a set within a zone read.
//
// Comparison is semantic on the name — PowerDNS lowercases it — and exact on
// the type, which the API does not rewrite.
func findRRSet(sets []auth.RRSet, name, recordType string) *auth.RRSet {
	for i := range sets {
		if normalise.RecordName(name, sets[i].Name) && sets[i].Type == recordType {
			return &sets[i]
		}
	}
	return nil
}

// applyRRSet writes a server response into the model.
//
// Only called from Read, so the server's values always win: this is where
// drift becomes visible. The name is the exception — it stays as configured,
// because the semantic modifier already established the two are the same name
// and rewriting it would show a diff for a case change nobody made.
func applyRRSet(
	ctx context.Context,
	rrset *auth.RRSet,
	model *recordModel,
) diag.Diagnostics {
	var diags diag.Diagnostics

	model.TTL = types.Int64Value(int64(rrset.TTL))

	values := make([]string, 0, len(rrset.Records))
	var anyDisabled bool
	for _, record := range rrset.Records {
		values = append(values, record.Content)
		if record.Disabled {
			anyDisabled = true
		}
	}

	// The model carries one flag for the set. A set that is partly disabled
	// was not made by this provider, and reporting it as disabled is the
	// honest reading: something in it is not being served.
	model.Disabled = types.BoolValue(anyDisabled)

	list, d := types.ListValueFrom(ctx, types.StringType, values)
	diags.Append(d...)
	model.Values = list

	if len(rrset.Comments) > 0 {
		model.Comment = types.StringValue(rrset.Comments[0].Content)
	} else {
		model.Comment = types.StringNull()
	}

	return diags
}

// recordDiagnostic turns a client error into an operator-facing diagnostic.
func recordDiagnostic(action string, rrset auth.RRSet, err error) diag.Diagnostic {
	summary := "Error " + action + " the record set"
	detail := rrset.Type + " " + rrset.Name + ".\n\n" + err.Error()

	var apiErr *transport.APIError
	if errors.As(err, &apiErr) && apiErr.Capability != transport.CapabilityNone {
		return diag.NewErrorDiagnostic(
			summary+": the server cannot do this as configured", detail)
	}
	return diag.NewErrorDiagnostic(summary, detail)
}

// recordValuesModifier compares an RRSet's values semantically.
//
// It cannot live in the planmodify package because the comparison depends on
// a sibling attribute: an A record's content is an address and compares
// numerically, a TXT record's content is a string and does not. The modifier
// reads `type` from the plan to decide which equivalence applies.
type recordValuesModifier struct{}

func (recordValuesModifier) Description(_ context.Context) string {
	return "address values are compared numerically and order is ignored; " +
		"every other type is compared exactly"
}

func (m recordValuesModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (recordValuesModifier) PlanModifyList(
	ctx context.Context,
	req planmodifier.ListRequest,
	resp *planmodifier.ListResponse,
) {
	if req.StateValue.IsNull() || req.PlanValue.IsUnknown() || req.PlanValue.IsNull() {
		return
	}

	var recordType types.String
	resp.Diagnostics.Append(
		req.Plan.GetAttribute(ctx, path.Root("type"), &recordType)...)
	if resp.Diagnostics.HasError() || recordType.IsUnknown() || recordType.IsNull() {
		return
	}

	var planned, current []string
	resp.Diagnostics.Append(req.PlanValue.ElementsAs(ctx, &planned, false)...)
	resp.Diagnostics.Append(req.StateValue.ElementsAs(ctx, &current, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key := func(value string) string {
		return normalise.RecordContentKey(recordType.ValueString(), value)
	}
	if normalise.StringMultiset(planned, current, key) {
		resp.PlanValue = req.StateValue
	}
}

// metadataKindsOnlyOnTheZone are the kinds PowerDNS refuses to serve by name.
//
// Both appear in GET /metadata and both answer 422 "Unsupported metadata kind"
// on GET /metadata/{kind}. Verified against auth-5.1.3. They are the two kinds
// that exist only as attributes of the zone object — `soa_edit_api` and
// `api_rectify` — so the metadata endpoint reports them and refuses to address
// them.
//
// Note the boundary is not "every kind that is also a zone attribute":
// NSEC3PARAM and PRESIGNED are both, and both are addressable. Only these two
// are not, which is why the list is enumerated rather than derived.
var metadataKindsOnlyOnTheZone = map[string]string{
	"SOA-EDIT-API": "soa_edit_api",
	"API-RECTIFY":  "api_rectify",
}

// checkMetadataKind rejects a kind the API will not address, naming the
// attribute to use instead.
//
// Rejected before the request, because the server's 422 says only "Unsupported
// metadata kind" and does not mention that the value is settable elsewhere.
func checkMetadataKind(kind string) diag.Diagnostics {
	var diags diag.Diagnostics

	attribute, unsupported := metadataKindsOnlyOnTheZone[strings.ToUpper(kind)]
	if !unsupported {
		return diags
	}

	diags.AddError(
		"Metadata kind "+kind+" is not addressable",
		"PowerDNS lists this kind under a zone's metadata but answers 422 "+
			"\"Unsupported metadata kind\" when it is read or written by name, "+
			"because it exists as an attribute of the zone itself.\n\n"+
			"Set powerdns_zone."+attribute+" instead.",
	)
	return diags
}
