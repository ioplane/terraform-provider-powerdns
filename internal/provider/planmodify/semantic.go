// Package planmodify turns the semantic comparisons in the normalise package
// into Terraform plan modifiers.
//
// The problem they solve: PowerDNS rewrites values, so what comes back from a
// read is not textually what was configured. Terraform compares state to
// configuration as strings, so every one of those rewrites is a diff that
// never converges — plan, apply, plan, same diff.
//
// A semantic modifier keeps the value already in state when the configured
// value means the same thing. It never keeps a value that means something
// different, which is the failure mode worth guarding against: a modifier that
// suppresses too much hides a real change and no plan ever shows it.
package planmodify

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// stringComparer answers whether a configured value and a state value are the
// same thing.
type stringComparer func(configured, actual string) bool

// semanticString keeps the state value when it means the same as the plan.
type semanticString struct {
	description string
	same        stringComparer
}

func (m semanticString) Description(_ context.Context) string { return m.description }

func (m semanticString) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m semanticString) PlanModifyString(
	_ context.Context,
	req planmodifier.StringRequest,
	resp *planmodifier.StringResponse,
) {
	// Unknown is the framework asking what a computed value will be; there is
	// nothing to compare against yet. Null state is creation.
	if req.StateValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	// A null plan against a non-null state is removal, which is a real change.
	if req.PlanValue.IsNull() {
		return
	}

	if m.same(req.PlanValue.ValueString(), req.StateValue.ValueString()) {
		resp.PlanValue = req.StateValue
	}
}

// SemanticString builds a modifier from a comparison and its explanation.
//
// The description is not decoration: it appears in the generated documentation
// as the answer to "why does this attribute not diff when I change its case?".
func SemanticString(description string, same stringComparer) planmodifier.String {
	return semanticString{description: description, same: same}
}

// semanticSet keeps the state list when it holds the same values as the plan,
// in any order.
type semanticSet struct {
	description string
	same        stringComparer
}

func (m semanticSet) Description(_ context.Context) string { return m.description }

func (m semanticSet) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m semanticSet) PlanModifyList(
	ctx context.Context,
	req planmodifier.ListRequest,
	resp *planmodifier.ListResponse,
) {
	if req.StateValue.IsNull() || req.PlanValue.IsUnknown() || req.PlanValue.IsNull() {
		return
	}

	planned, ok := stringsFromList(ctx, req.PlanValue, resp)
	if !ok {
		return
	}
	current, ok := stringsFromList(ctx, req.StateValue, resp)
	if !ok {
		return
	}

	if setsMatch(planned, current, m.same) {
		resp.PlanValue = req.StateValue
	}
}

// SemanticSet builds a list modifier that ignores order and compares elements
// with same.
//
// The attribute stays a list rather than a set because PowerDNS returns these
// as JSON arrays and a set would lose the server's ordering entirely; what is
// wanted is to ignore order when comparing, not to discard it.
func SemanticSet(description string, same stringComparer) planmodifier.List {
	return semanticSet{description: description, same: same}
}

// stringsFromList extracts the elements, reporting a conversion failure into
// the response rather than silently comparing nothing.
func stringsFromList(
	ctx context.Context,
	list types.List,
	resp *planmodifier.ListResponse,
) ([]string, bool) {
	var out []string
	diags := list.ElementsAs(ctx, &out, false)
	resp.Diagnostics.Append(diags...)
	return out, !diags.HasError()
}

// setsMatch is normalise.StringSet, inlined here to keep this package free of
// a dependency on the comparison it is given.
func setsMatch(planned, current []string, same stringComparer) bool {
	if len(planned) != len(current) {
		return false
	}

	used := make([]bool, len(current))
	for _, want := range planned {
		var found bool
		for i, got := range current {
			if used[i] || !same(want, got) {
				continue
			}
			used[i], found = true, true
			break
		}
		if !found {
			return false
		}
	}
	return true
}

// AttributePath is re-exported so a resource can build a diagnostic pointing
// at the attribute a modifier acted on without importing path directly.
type AttributePath = path.Path
