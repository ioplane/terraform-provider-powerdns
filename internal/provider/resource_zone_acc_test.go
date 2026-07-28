//go:build acceptance

package provider_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccZone_Native is the whole point of the plan modifiers.
//
// The zone is configured with `native` in lower case and an uncompressed IPv6
// master. PowerDNS stores `Native` and the compressed address, and assigns
// soa_edit_api DEFAULT unasked. Every one of those would be a permanent diff
// without the semantic comparisons, so the test that matters is the empty plan
// after apply — not that the zone was created.
func TestAccZone_Native(t *testing.T) {
	name := acceptanceZoneName(t, "native")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "powerdns_zone" "test" {
  name        = %q
  kind        = "native"
  nameservers = ["ns1.%s", "ns2.%s"]
}
`, name, name, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("powerdns_zone.test", "id", name),
					// State holds the *configured* spelling after an apply, not
					// the server's. Terraform requires a create to return what
					// was planned, so `Native` cannot appear here — it arrives
					// on the next refresh, and the semantic modifier is what
					// stops that from reading as a change.
					resource.TestCheckResourceAttr("powerdns_zone.test", "kind", "native"),
					// Computed, so the server's value does land immediately:
					// PowerDNS assigns DEFAULT although the configuration is
					// silent about it.
					resource.TestCheckResourceAttr("powerdns_zone.test", "soa_edit_api", "DEFAULT"),
					resource.TestCheckResourceAttrSet("powerdns_zone.test", "serial"),
				),
			},
			{
				// The regression guard. Re-applying the same configuration
				// must plan nothing: if any normalisation is unhandled, this
				// step fails with the offending attribute named.
				Config: fmt.Sprintf(`
resource "powerdns_zone" "test" {
  name        = %q
  kind        = "native"
  nameservers = ["ns1.%s", "ns2.%s"]
}
`, name, name, name),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				// Same zone, kind spelled differently and the name without its
				// trailing dot. Both are the same value semantically, so this
				// must also plan nothing.
				Config: fmt.Sprintf(`
resource "powerdns_zone" "test" {
  name        = %q
  kind        = "NATIVE"
  nameservers = ["ns1.%s", "ns2.%s"]
}
`, strings.TrimSuffix(name, "."), name, name),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      "powerdns_zone.test",
				ImportState:       true,
				ImportStateVerify: true,
				// nameservers is create-only and never read back, so an
				// imported zone cannot know what it was created with.
				ImportStateVerifyIgnore: []string{"nameservers"},
			},
		},
	})
}

// TestAccZone_SlaveWithMasters covers the IPv6 compression PowerDNS applies to
// a master list, and the ordering it does not promise to preserve.
func TestAccZone_SlaveWithMasters(t *testing.T) {
	name := acceptanceZoneName(t, "slave")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "powerdns_zone" "slave" {
  name    = %q
  kind    = "Slave"
  masters = ["192.0.2.53", "2001:db8:0:0:0:0:0:53"]
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("powerdns_zone.slave", "kind", "Slave"),
					resource.TestCheckResourceAttr("powerdns_zone.slave", "masters.#", "2"),
				),
			},
			{
				// The address respelled in compressed form, and the two
				// entries in the other order. Neither is a change.
				Config: fmt.Sprintf(`
resource "powerdns_zone" "slave" {
  name    = %q
  kind    = "Slave"
  masters = ["2001:db8::53", "192.0.2.53"]
}
`, name),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				// A genuinely different master must still plan a change; a
				// modifier that suppressed this would hide a real edit.
				Config: fmt.Sprintf(`
resource "powerdns_zone" "slave" {
  name    = %q
  kind    = "Slave"
  masters = ["192.0.2.54", "2001:db8::53"]
}
`, name),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccZone_OnLMDB runs the same zone against the LMDB backend.
//
// ADR 0005: a single authoritative instance cannot cover the provider, because
// views and networks are unimplemented by the relational backends. Running the
// ordinary zone lifecycle on both is what keeps a backend-specific behaviour
// from being discovered by a user.
func TestAccZone_OnLMDB(t *testing.T) {
	lmdbURL := os.Getenv("PDNS_SERVER_URL_LMDB")
	if lmdbURL == "" {
		t.Skip("PDNS_SERVER_URL_LMDB is not set")
	}

	name := acceptanceZoneName(t, "lmdb")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "powerdns" {
  server_url = %q
}

resource "powerdns_zone" "lmdb" {
  name        = %q
  kind        = "Native"
  nameservers = ["ns1.%s"]
}
`, lmdbURL, name, name),
				Check: resource.TestCheckResourceAttr("powerdns_zone.lmdb", "kind", "Native"),
			},
			{
				Config: fmt.Sprintf(`
provider "powerdns" {
  server_url = %q
}

resource "powerdns_zone" "lmdb" {
  name        = %q
  kind        = "Native"
  nameservers = ["ns1.%s"]
}
`, lmdbURL, name, name),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// testAccCheckZoneDestroyed asserts the zone is gone after the test.
//
// It queries by name rather than trusting the state, because a resource that
// failed to delete would still be absent from state.
func testAccCheckZoneDestroyed(name string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		client, err := acceptanceAuthClient()
		if err != nil {
			return err
		}

		zones, err := client.SearchZoneByName(t0(), name)
		if err != nil {
			return fmt.Errorf("checking that %s was destroyed: %w", name, err)
		}
		if len(zones) != 0 {
			return fmt.Errorf("zone %s still exists after destroy", name)
		}
		return nil
	}
}
