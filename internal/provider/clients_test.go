package provider

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// TestRequireNamesTheArgument covers the failure that is not a PowerDNS limit
// at all: a resource for a product the provider was never pointed at.
//
// It is a unit test rather than an acceptance one because the lab configures
// every product through the environment, and the provider reads the
// environment by design — there is no way to have an unconfigured product on a
// machine where the acceptance suite can run.
//
// The assertion is on the wording. "The client is nil" is not something an
// operator can act on; the argument name and the environment variable are.
func TestRequireNamesTheArgument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		call    func(*Clients) []string
		wantArg string
		wantEnv string
	}{
		{
			"authoritative",
			func(c *Clients) []string { _, d := c.RequireAuth("powerdns_zone"); return messages(d) },
			"server_url", "PDNS_SERVER_URL",
		},
		{
			"recursor",
			func(c *Clients) []string {
				_, d := c.RequireRecursor("powerdns_recursor_zone")
				return messages(d)
			},
			"recursor_server_url", "PDNS_RECURSOR_SERVER_URL",
		},
		{
			"dnsdist",
			func(c *Clients) []string {
				_, d := c.RequireDNSDist("powerdns_dnsdist_acl")
				return messages(d)
			},
			"dnsdist_server_url", "PDNS_DNSDIST_SERVER_URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// An empty bundle: every product unconfigured.
			got := strings.Join(tt.call(&Clients{}), "\n")

			if !strings.Contains(got, tt.wantArg) {
				t.Errorf("the diagnostic does not name %q:\n%s", tt.wantArg, got)
			}
			if !strings.Contains(got, tt.wantEnv) {
				t.Errorf("the diagnostic does not name %q:\n%s", tt.wantEnv, got)
			}
		})
	}
}

// TestRequireSurvivesANilBundle is the regression guard for the bug the
// ephemeral resources hit: the provider sets three separate fields in
// Configure, and omitting one hands a resource a nil bundle.
//
// A panic there is a crash at apply with no diagnostic. This must be an error
// naming the omission instead.
func TestRequireSurvivesANilBundle(t *testing.T) {
	t.Parallel()

	var nilBundle *Clients

	for name, call := range map[string]func() []string{
		"auth":     func() []string { _, d := nilBundle.RequireAuth("r"); return messages(d) },
		"recursor": func() []string { _, d := nilBundle.RequireRecursor("r"); return messages(d) },
		"dnsdist":  func() []string { _, d := nilBundle.RequireDNSDist("r"); return messages(d) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := strings.Join(call(), "\n")
			if !strings.Contains(got, "EphemeralResourceData") {
				t.Errorf("the diagnostic does not name the field that is likely missing:\n%s",
					got)
			}
		})
	}
}

// messages flattens diagnostics into their summaries and details.
func messages(diags diag.Diagnostics) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.Summary()+" "+d.Detail())
	}
	return out
}
