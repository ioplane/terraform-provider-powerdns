package provider

import (
	"context"
	"maps"
	"reflect"
	"regexp"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestRegisteredTypeNamesAreUniqueAndCanonical(t *testing.T) {
	t.Parallel()

	provider := &powerdnsProvider{}
	ctx := context.Background()
	const providerName = "powerdns"

	resources := collectNames(provider.Resources(ctx), func(instance resource.Resource) string {
		var response resource.MetadataResponse
		instance.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: providerName}, &response)
		return response.TypeName
	})
	assertCanonicalNames(t, "resource", resources, []string{
		"powerdns_zone", "powerdns_record", "powerdns_zone_metadata", "powerdns_zone_cryptokey",
		"powerdns_tsigkey", "powerdns_view_zone", "powerdns_network", "powerdns_autoprimary",
		"powerdns_recursor_zone", "powerdns_recursor_acl", "powerdns_dnsdist_acl",
	}, `^powerdns_[a-z0-9]+(?:_[a-z0-9]+)*$`)

	dataSources := collectNames(provider.DataSources(ctx), func(instance datasource.DataSource) string {
		var response datasource.MetadataResponse
		instance.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: providerName}, &response)
		return response.TypeName
	})
	assertCanonicalNames(t, "data source", dataSources, []string{
		"powerdns_zone", "powerdns_zones", "powerdns_record", "powerdns_zone_metadata",
		"powerdns_zone_export", "powerdns_recursor_zone", "powerdns_dnsdist_server",
	}, `^powerdns_[a-z0-9]+(?:_[a-z0-9]+)*$`)

	actions := collectNames(provider.Actions(ctx), func(instance action.Action) string {
		var response action.MetadataResponse
		instance.Metadata(ctx, action.MetadataRequest{ProviderTypeName: providerName}, &response)
		return response.TypeName
	})
	assertCanonicalNames(t, "action", actions, []string{
		"powerdns_notify_zone", "powerdns_axfr_retrieve", "powerdns_rectify_zone", "powerdns_flush_cache",
	}, `^powerdns_[a-z0-9]+(?:_[a-z0-9]+)*$`)

	ephemeralResources := collectNames(provider.EphemeralResources(ctx), func(instance ephemeral.EphemeralResource) string {
		var response ephemeral.MetadataResponse
		instance.Metadata(ctx, ephemeral.MetadataRequest{ProviderTypeName: providerName}, &response)
		return response.TypeName
	})
	assertCanonicalNames(t, "ephemeral resource", ephemeralResources, []string{
		"powerdns_cryptokey_material", "powerdns_tsigkey_secret",
	}, `^powerdns_[a-z0-9]+(?:_[a-z0-9]+)*$`)

	functions := collectNames(provider.Functions(ctx), func(instance function.Function) string {
		var response function.MetadataResponse
		instance.Metadata(ctx, function.MetadataRequest{}, &response)
		return response.Name
	})
	assertCanonicalNames(t, "function", functions, []string{
		"fqdn", "is_fqdn", "reverse_zone_name", "ptr_name", "soa_serial",
	}, `^[a-z0-9]+(?:_[a-z0-9]+)*$`)
}

func collectNames[T any](factories []func() T, name func(T) string) []string {
	result := make([]string, 0, len(factories))
	for _, factory := range factories {
		result = append(result, name(factory()))
	}
	return result
}

func assertCanonicalNames(t *testing.T, kind string, names, expected []string, pattern string) {
	t.Helper()
	if !slices.Equal(names, expected) {
		t.Errorf("%s registrations = %v, want exact %v", kind, names, expected)
	}
	want := regexp.MustCompile(pattern)
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if !want.MatchString(name) {
			t.Errorf("%s name %q is not canonical", kind, name)
		}
		if _, exists := seen[name]; exists {
			t.Errorf("duplicate %s name %q", kind, name)
		}
		seen[name] = struct{}{}
	}
}

func BenchmarkProviderRegistrations(b *testing.B) {
	provider := &powerdnsProvider{}
	ctx := context.Background()
	for b.Loop() {
		count := len(provider.Resources(ctx)) + len(provider.DataSources(ctx)) +
			len(provider.Actions(ctx)) + len(provider.EphemeralResources(ctx)) + len(provider.Functions(ctx))
		if count != 29 {
			b.Fatalf("registration count = %d, want 29", count)
		}
	}
}

// TestSemanticPlanModifiersPrecedeReplacement guards the execution order used
// by terraform-plugin-framework: each modifier receives the PlanValue returned
// by its predecessor, while a replacement decision cannot be revoked later.
func TestSemanticPlanModifiersPrecedeReplacement(t *testing.T) {
	t.Parallel()

	provider := &powerdnsProvider{}
	expected := map[string]semanticReplacementValues{
		"powerdns_zone.name":               {"Example.COM", "example.com.", "other.example."},
		"powerdns_record.zone":             {"Example.COM", "example.com.", "other.example."},
		"powerdns_record.name":             {"WWW.Example.COM", "www.example.com.", "api.example.com."},
		"powerdns_zone_metadata.zone":      {"Example.COM", "example.com.", "other.example."},
		"powerdns_zone_cryptokey.zone":     {"Example.COM", "example.com.", "other.example."},
		"powerdns_zone_cryptokey.key_type": {"csk", "ksk", "zsk"},
		"powerdns_tsigkey.name":            {"Transfer", "transfer.", "other."},
		"powerdns_view_zone.zone":          {"Example.COM", "example.com.", "other.example."},
		"powerdns_network.network":         {"192.0.2.4/24", "192.0.2.0/24", "198.51.100.0/24"},
		"powerdns_autoprimary.ip":          {"2001:0db8::1", "2001:db8::1", "2001:db8::2"},
		"powerdns_autoprimary.nameserver":  {"NS1.Example.COM", "ns1.example.com.", "ns2.example.com."},
		"powerdns_recursor_zone.name":      {"Example.COM", "example.com.", "other.example."},
	}
	remaining := make(map[string]semanticReplacementValues, len(expected))
	maps.Copy(remaining, expected)

	for _, factory := range provider.Resources(context.Background()) {
		instance := factory()
		var metadata resource.MetadataResponse
		instance.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "powerdns"}, &metadata)

		var response resource.SchemaResponse
		instance.Schema(context.Background(), resource.SchemaRequest{}, &response)
		for name, attribute := range response.Schema.Attributes {
			stringAttribute, ok := attribute.(schema.StringAttribute)
			if !ok {
				continue
			}
			semantic, replacement := modifierIndexes(stringAttribute.PlanModifiers)
			identifier := metadata.TypeName + "." + name
			values, required := expected[identifier]
			if !required {
				if semantic >= 0 && replacement >= 0 {
					t.Errorf("unexpected semantic replacement contract %s", identifier)
				}
				continue
			}
			delete(remaining, identifier)
			if semantic < 0 || replacement < 0 {
				t.Errorf("%s: semantic index %d, replacement index %d", identifier, semantic, replacement)
				continue
			}
			if semantic > replacement {
				t.Errorf("%s.%s: semantic modifier index %d follows replacement index %d",
					metadata.TypeName, name, semantic, replacement)
			}
			assertSemanticPreventsReplacement(t, identifier, stringAttribute.PlanModifiers[semantic], values)
			assertSemanticReplacementChain(t, identifier, stringAttribute.PlanModifiers, values)
		}
	}

	for identifier := range remaining {
		t.Errorf("required semantic replacement contract %s was not registered", identifier)
	}
}

type semanticReplacementValues struct {
	planned string
	current string
	changed string
}

func assertSemanticPreventsReplacement(
	t *testing.T,
	identifier string,
	modifier planmodifier.String,
	values semanticReplacementValues,
) {
	t.Helper()
	planned := types.StringValue(values.planned)
	current := types.StringValue(values.current)
	response := planmodifier.StringResponse{PlanValue: planned}
	modifier.PlanModifyString(context.Background(), planmodifier.StringRequest{
		ConfigValue: planned,
		PlanValue:   planned,
		StateValue:  current,
	}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("%s: semantic modifier returned diagnostics: %v", identifier, response.Diagnostics)
	}
	if !response.PlanValue.Equal(current) {
		t.Errorf("%s: semantic modifier left %v instead of state %v; replacement would trigger",
			identifier, response.PlanValue, current)
	}
}

func assertSemanticReplacementChain(
	t *testing.T,
	identifier string,
	modifiers []planmodifier.String,
	values semanticReplacementValues,
) {
	t.Helper()

	if runStringModifierChain(t, identifier, modifiers, values.planned, values.current) {
		t.Errorf("%s: equivalent spelling required replacement", identifier)
	}
	if !runStringModifierChain(t, identifier, modifiers, values.changed, values.current) {
		t.Errorf("%s: genuine change did not require replacement", identifier)
	}
}

func runStringModifierChain(
	t *testing.T,
	identifier string,
	modifiers []planmodifier.String,
	planned string,
	current string,
) bool {
	t.Helper()

	plannedValue := types.StringValue(planned)
	currentValue := types.StringValue(current)
	request := planmodifier.StringRequest{
		ConfigValue: plannedValue,
		PlanValue:   plannedValue,
		StateValue:  currentValue,
		Plan:        tfsdk.Plan{Raw: tftypes.NewValue(tftypes.String, planned)},
		State:       tfsdk.State{Raw: tftypes.NewValue(tftypes.String, current)},
	}
	response := planmodifier.StringResponse{PlanValue: plannedValue}
	for _, modifier := range modifiers {
		request.PlanValue = response.PlanValue
		modifier.PlanModifyString(context.Background(), request, &response)
		if response.Diagnostics.HasError() {
			t.Fatalf("%s: modifier chain returned diagnostics: %v", identifier, response.Diagnostics)
		}
	}
	return response.RequiresReplace
}

func modifierIndexes(modifiers []planmodifier.String) (int, int) {
	semantic, replacement := -1, -1
	for index, modifier := range modifiers {
		typeOf := reflect.TypeOf(modifier)
		if typeOf.Kind() == reflect.Pointer {
			typeOf = typeOf.Elem()
		}
		switch typeOf.PkgPath() {
		case "github.com/ioplane/terraform-provider-powerdns/internal/provider/planmodify":
			semantic = index
		case "github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier":
			if typeOf.Name() == "requiresReplaceIfModifier" {
				replacement = index
			}
		}
	}
	return semantic, replacement
}
