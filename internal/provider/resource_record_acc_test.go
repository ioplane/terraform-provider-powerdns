//go:build acceptance

package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// recordZoneConfig is the zone every record test hangs off.
func recordZoneConfig(zone string) string {
	return fmt.Sprintf(`
resource "powerdns_zone" "host" {
  name        = %q
  kind        = "Native"
  nameservers = ["ns1.%s"]
}
`, zone, zone)
}

// TestAccRecord_AddressNormalisation covers the two rewrites PowerDNS applies
// to an AAAA set: the owner name is lowercased, and the address is compressed.
//
// Both were observed against auth-5.1.3 before the resource was written. The
// empty plan after apply is what proves they are handled.
func TestAccRecord_AddressNormalisation(t *testing.T) {
	zone := acceptanceZoneName(t, "rec-v6")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + fmt.Sprintf(`
resource "powerdns_record" "v6" {
  zone   = powerdns_zone.host.id
  name   = "V6.%s"
  type   = "AAAA"
  ttl    = 300
  values = ["2001:0db8:0000:0000:0000:0000:0000:0001"]
}
`, zone),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("powerdns_record.v6", "type", "AAAA"),
					resource.TestCheckResourceAttr("powerdns_record.v6", "values.#", "1"),
					resource.TestCheckResourceAttr("powerdns_record.v6", "disabled", "false"),
				),
			},
			{
				// Same configuration again: the server has since lowercased the
				// name and compressed the address, and neither is a change.
				Config: recordZoneConfig(zone) + fmt.Sprintf(`
resource "powerdns_record" "v6" {
  zone   = powerdns_zone.host.id
  name   = "V6.%s"
  type   = "AAAA"
  ttl    = 300
  values = ["2001:0db8:0000:0000:0000:0000:0000:0001"]
}
`, zone),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// The address respelled the way the server stores it, and the
				// name in lower case. Still the same set.
				Config: recordZoneConfig(zone) + fmt.Sprintf(`
resource "powerdns_record" "v6" {
  zone   = powerdns_zone.host.id
  name   = "v6.%s"
  type   = "AAAA"
  ttl    = 300
  values = ["2001:db8::1"]
}
`, zone),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// A different address is a real change and must plan one. A
				// modifier that suppressed this would lose an edit silently.
				Config: recordZoneConfig(zone) + fmt.Sprintf(`
resource "powerdns_record" "v6" {
  zone   = powerdns_zone.host.id
  name   = "v6.%s"
  type   = "AAAA"
  ttl    = 300
  values = ["2001:db8::2"]
}
`, zone),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectNonEmptyPlan()},
				},
			},
		},
	})
}

// TestAccRecord_MultiValueSet is the case that motivates modelling an RRSet
// rather than a record: three A records under one name are one object, and
// removing one is an edit to that object rather than a delete.
func TestAccRecord_MultiValueSet(t *testing.T) {
	zone := acceptanceZoneName(t, "rec-multi")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + fmt.Sprintf(`
resource "powerdns_record" "www" {
  zone   = powerdns_zone.host.id
  name   = "www.%s"
  type   = "A"
  ttl    = 60
  values = ["192.0.2.1", "192.0.2.2", "192.0.2.3"]
}
`, zone),
				Check: resource.TestCheckResourceAttr("powerdns_record.www", "values.#", "3"),
			},
			{
				// Reordered only. The server does not promise an order and the
				// set is the same.
				Config: recordZoneConfig(zone) + fmt.Sprintf(`
resource "powerdns_record" "www" {
  zone   = powerdns_zone.host.id
  name   = "www.%s"
  type   = "A"
  ttl    = 60
  values = ["192.0.2.3", "192.0.2.1", "192.0.2.2"]
}
`, zone),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// One value removed: an update, and the set must end up with
				// exactly two.
				Config: recordZoneConfig(zone) + fmt.Sprintf(`
resource "powerdns_record" "www" {
  zone   = powerdns_zone.host.id
  name   = "www.%s"
  type   = "A"
  ttl    = 60
  values = ["192.0.2.1", "192.0.2.2"]
}
`, zone),
				Check: resource.TestCheckResourceAttr("powerdns_record.www", "values.#", "2"),
			},
			{
				ResourceName:      "powerdns_record.www",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/www.%s/A", zone, zone), nil
				},
			},
		},
	})
}

// TestAccRecord_TXTIsComparedExactly is the negative half of the content
// comparison. A TXT record's quoting and spacing are significant, so nothing
// about it may be treated as equivalent-but-different.
func TestAccRecord_TXTIsComparedExactly(t *testing.T) {
	zone := acceptanceZoneName(t, "rec-txt")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + fmt.Sprintf(`
resource "powerdns_record" "txt" {
  zone     = powerdns_zone.host.id
  name     = "txt.%s"
  type     = "TXT"
  ttl      = 300
  values   = ["\"v=spf1 -all\""]
  disabled = true
}
`, zone),
				Check: resource.ComposeAggregateTestCheckFunc(
					// disabled must survive the round trip: dropping it would
					// silently start serving a record somebody turned off.
					resource.TestCheckResourceAttr("powerdns_record.txt", "disabled", "true"),
					resource.TestCheckResourceAttr("powerdns_record.txt", "values.0", `"v=spf1 -all"`),
				),
			},
			{
				Config: recordZoneConfig(zone) + fmt.Sprintf(`
resource "powerdns_record" "txt" {
  zone     = powerdns_zone.host.id
  name     = "txt.%s"
  type     = "TXT"
  ttl      = 300
  values   = ["\"v=spf1 -all\""]
  disabled = true
}
`, zone),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// A changed value inside the quotes is a real change.
				Config: recordZoneConfig(zone) + fmt.Sprintf(`
resource "powerdns_record" "txt" {
  zone     = powerdns_zone.host.id
  name     = "txt.%s"
  type     = "TXT"
  ttl      = 300
  values   = ["\"v=spf1 ~all\""]
  disabled = true
}
`, zone),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectNonEmptyPlan()},
				},
			},
		},
	})
}
