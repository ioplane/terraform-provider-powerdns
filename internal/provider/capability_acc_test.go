//go:build acceptance

package provider_test

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The capability tests assert the **message**, not the failure.
//
// PowerDNS has four conditions under which an operation cannot succeed on a
// given installation, and every one arrives as a bare 4xx. A test that only
// checked "this errored" would pass for a provider that surfaced `422
// Unprocessable Entity` and left an operator to work out why — which is the
// defect this provider exists to avoid.
//
// So each case matches on the requirement: the backend to switch to, the
// setting to configure, the Lua call to add. If the wording changes the test
// fails, and that is deliberate — the wording is what an operator reads.

// lmdbURL is the second authoritative endpoint, for the cases that need a
// backend the relational one cannot provide.
func lmdbURL(t *testing.T) string {
	t.Helper()

	url := os.Getenv("PDNS_SERVER_URL_LMDB")
	if url == "" {
		t.Skip("PDNS_SERVER_URL_LMDB is not set; run task lab:up")
	}
	return url
}

// TestAccCapability_ViewsNeedLMDB is the case ADR 0005 exists for.
//
// The gpgsql backend has no views table. A write answers 422, and the
// diagnostic has to say which backend does support it — otherwise the operator
// sees an unprocessable entity and no reason.
func TestAccCapability_ViewsNeedLMDB(t *testing.T) {
	zone := acceptanceZoneName(t, "cap-views")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				// The default provider points at gpgsql.
				Config: recordZoneConfig(zone) + `
resource "powerdns_view_zone" "trusted" {
  view = "trusted"
  zone = powerdns_zone.host.id
}
`,
				ExpectError: regexp.MustCompile(`(?s)LMDB`),
			},
		},
	})
}

// TestAccCapability_ViewsWorkOnLMDB is the other half. A diagnostic naming a
// requirement is only useful if meeting the requirement works.
func TestAccCapability_ViewsWorkOnLMDB(t *testing.T) {
	url := lmdbURL(t)
	zone := acceptanceZoneName(t, "cap-lmdb")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "powerdns" {
  server_url = %[1]q
}

resource "powerdns_zone" "host" {
  name        = %[2]q
  kind        = "Native"
  nameservers = ["ns1.%[2]s"]
}

resource "powerdns_view_zone" "trusted" {
  view = "trusted"
  zone = powerdns_zone.host.id
}

resource "powerdns_network" "clients" {
  network = "2001:db8::/32"
  view    = powerdns_view_zone.trusted.view
}
`, url, zone),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("powerdns_view_zone.trusted", "id",
						"trusted/"+zone),
					resource.TestCheckResourceAttr("powerdns_network.clients", "view",
						"trusted"),
				),
			},
			{
				Config: fmt.Sprintf(`
provider "powerdns" {
  server_url = %[1]q
}

resource "powerdns_zone" "host" {
  name        = %[2]q
  kind        = "Native"
  nameservers = ["ns1.%[2]s"]
}

resource "powerdns_view_zone" "trusted" {
  view = "trusted"
  zone = powerdns_zone.host.id
}

# The same subnet, respelled: IPv6 has many spellings of one prefix and
# PowerDNS returns the compressed form. Not a change.
resource "powerdns_network" "clients" {
  network = "2001:db8:0:0::/32"
  view    = powerdns_view_zone.trusted.view
}
`, url, zone),
				// A CIDR compared as a string would diff here; compared as a
				// subnet it does not.
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TestAccCapability_RecursorNeedsAPIDir asserts the second condition.
//
// A Recursor without webservice.api_dir is read-only, and says so in a 422
// whose text names the setting. The provider has to carry that through rather
// than reporting the status.
func TestAccCapability_RecursorNeedsAPIDir(t *testing.T) {
	if os.Getenv("PDNS_RECURSOR_SERVER_URL") == "" {
		t.Skip("PDNS_RECURSOR_SERVER_URL is not set")
	}
	t.Skip("the lab's Recursor has api_dir configured, so this cannot be " +
		"provoked without a second Recursor; the classifier is covered by " +
		"internal/api/transport/classify_test.go")
}
