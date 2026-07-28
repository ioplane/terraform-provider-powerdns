package planmodify_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-powerdns/internal/provider/normalise"
	"github.com/ioplane/terraform-provider-powerdns/internal/provider/planmodify"
)

// TestSemanticString covers the four states a plan modifier sees, and the two
// outcomes that matter: a respelling is suppressed, a real change is not.
func TestSemanticString(t *testing.T) {
	t.Parallel()

	modifier := planmodify.SemanticString(
		"zone kinds are compared case-insensitively", normalise.ZoneKind)

	tests := []struct {
		name  string
		state types.String
		plan  types.String
		// want is the value the plan should hold afterwards.
		want types.String
	}{
		{
			// The case this exists for: configured `native`, state `Native`.
			name:  "a respelling keeps the state value",
			state: types.StringValue("Native"),
			plan:  types.StringValue("native"),
			want:  types.StringValue("Native"),
		},
		{
			name:  "a real change survives",
			state: types.StringValue("Native"),
			plan:  types.StringValue("Master"),
			want:  types.StringValue("Master"),
		},
		{
			// Creation: nothing in state to keep.
			name:  "null state is left alone",
			state: types.StringNull(),
			plan:  types.StringValue("native"),
			want:  types.StringValue("native"),
		},
		{
			// The framework asking what a computed value will be.
			name:  "unknown plan is left alone",
			state: types.StringValue("Native"),
			plan:  types.StringUnknown(),
			want:  types.StringUnknown(),
		},
		{
			// Removing an optional attribute is a change, not a respelling.
			name:  "null plan against a set state survives",
			state: types.StringValue("Native"),
			plan:  types.StringNull(),
			want:  types.StringNull(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := planmodifier.StringRequest{
				StateValue: tt.state,
				PlanValue:  tt.plan,
			}
			resp := &planmodifier.StringResponse{PlanValue: tt.plan}

			modifier.PlanModifyString(context.Background(), req, resp)

			if !resp.PlanValue.Equal(tt.want) {
				t.Errorf("plan = %v, want %v", resp.PlanValue, tt.want)
			}
			if resp.Diagnostics.HasError() {
				t.Errorf("unexpected diagnostics: %v", resp.Diagnostics)
			}
		})
	}
}

func TestSemanticSet(t *testing.T) {
	t.Parallel()

	modifier := planmodify.SemanticSet(
		"addresses are compared by value, ignoring order", normalise.IPAddress)

	list := func(values ...string) types.List {
		elements := make([]types.String, 0, len(values))
		for _, v := range values {
			elements = append(elements, types.StringValue(v))
		}
		out, diags := types.ListValueFrom(context.Background(), types.StringType, elements)
		if diags.HasError() {
			t.Fatalf("building the list: %v", diags)
		}
		return out
	}

	tests := []struct {
		name       string
		state      types.List
		plan       types.List
		wantKeeper bool
	}{
		{
			"reordering is suppressed",
			list("192.0.2.1", "192.0.2.2"),
			list("192.0.2.2", "192.0.2.1"),
			true,
		},
		{
			// Both at once: the IPv6 respelling PowerDNS applies, and a
			// different order.
			"respelling and reordering together are suppressed",
			list("192.0.2.1", "2001:db8::1"),
			list("2001:db8:0:0::1", "192.0.2.1"),
			true,
		},
		{
			"an added entry survives",
			list("192.0.2.1"),
			list("192.0.2.1", "192.0.2.2"),
			false,
		},
		{
			"a removed entry survives",
			list("192.0.2.1", "192.0.2.2"),
			list("192.0.2.1"),
			false,
		},
		{
			"a substituted entry survives",
			list("192.0.2.1", "192.0.2.2"),
			list("192.0.2.1", "192.0.2.3"),
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := planmodifier.ListRequest{StateValue: tt.state, PlanValue: tt.plan}
			resp := &planmodifier.ListResponse{PlanValue: tt.plan}

			modifier.PlanModifyList(context.Background(), req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}

			keptState := resp.PlanValue.Equal(tt.state)
			if keptState != tt.wantKeeper {
				t.Errorf("kept state = %v, want %v (plan is now %v)",
					keptState, tt.wantKeeper, resp.PlanValue)
			}
		})
	}
}

// TestSemanticSet_NullAndUnknown covers the states where there is nothing to
// compare, which must be left exactly as the framework supplied them.
func TestSemanticSet_NullAndUnknown(t *testing.T) {
	t.Parallel()

	modifier := planmodify.SemanticSet("", normalise.IPAddress)
	listType := types.ListNull(types.StringType)

	for _, tt := range []struct {
		name  string
		state types.List
		plan  types.List
	}{
		{"null state", listType, types.ListValueMust(types.StringType, nil)},
		{"unknown plan", types.ListValueMust(types.StringType, nil), types.ListUnknown(types.StringType)},
		{"null plan", types.ListValueMust(types.StringType, nil), listType},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := planmodifier.ListRequest{StateValue: tt.state, PlanValue: tt.plan}
			resp := &planmodifier.ListResponse{PlanValue: tt.plan}

			modifier.PlanModifyList(context.Background(), req, resp)

			if !resp.PlanValue.Equal(tt.plan) {
				t.Errorf("plan was changed to %v; it must be left alone", resp.PlanValue)
			}
		})
	}
}
