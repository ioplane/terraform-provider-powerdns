//go:build acceptance

package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// recursorPreCheck skips when the Recursor is not in the lab.
func recursorPreCheck(t *testing.T) {
	t.Helper()

	acceptancePreCheck(t)
	if os.Getenv("PDNS_RECURSOR_SERVER_URL") == "" {
		t.Skip("PDNS_RECURSOR_SERVER_URL is not set; run task lab:up")
	}
}

// TestAccRecursorZone_UpstreamPortIsDefaulted is the normalisation this
// resource exists to absorb.
//
// The Recursor stores `192.0.2.53` as `192.0.2.53:53`. Compared as a string
// that is a diff on every plan, for ever.
func TestAccRecursorZone_UpstreamPortIsDefaulted(t *testing.T) {
	zone := acceptanceZoneName(t, "rec-fwd")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { recursorPreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckRecursorZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "powerdns_recursor_zone" "forward" {
  name    = %q
  kind    = "Forwarded"
  servers = ["192.0.2.53", "192.0.2.54"]
}
`, zone),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("powerdns_recursor_zone.forward",
						"id", zone),
					resource.TestCheckResourceAttr("powerdns_recursor_zone.forward",
						"kind", "Forwarded"),
					resource.TestCheckResourceAttr("powerdns_recursor_zone.forward",
						"servers.#", "2"),
				),
			},
			{
				// The server has appended :53 to both by now. Not a change.
				Config: fmt.Sprintf(`
resource "powerdns_recursor_zone" "forward" {
  name    = %q
  kind    = "Forwarded"
  servers = ["192.0.2.53", "192.0.2.54"]
}
`, zone),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// Written the way the server stores them, and reordered.
				// Still not a change.
				Config: fmt.Sprintf(`
resource "powerdns_recursor_zone" "forward" {
  name    = %q
  kind    = "forwarded"
  servers = ["192.0.2.54:53", "192.0.2.53:53"]
}
`, zone),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// A different port is a real change: the comparison defaults
				// the port, it does not ignore it.
				Config: fmt.Sprintf(`
resource "powerdns_recursor_zone" "forward" {
  name    = %q
  kind    = "Forwarded"
  servers = ["192.0.2.53:5353", "192.0.2.54"]
}
`, zone),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectNonEmptyPlan()},
				},
			},
			{
				ResourceName:      "powerdns_recursor_zone.forward",
				ImportState:       true,
				ImportStateId:     zone,
				ImportStateVerify: true,
				// ImportStateVerify compares strings, and these are exactly the
				// attributes whose spelling the server changes:
				//
				//   servers  state holds "192.0.2.54" from the configuration,
				//            the import reads "192.0.2.54:53" from the server
				//
				// The plan modifier resolves that as equal, which is why no
				// plan is empty-checked here and why the difference is
				// invisible in ordinary use. ImportStateVerify does not run
				// plan modifiers, so it sees the raw strings.
				//
				// recursion_desired and notify_allowed have schema defaults,
				// so the imported values come from the server while the
				// configured ones come from the default.
				ImportStateVerifyIgnore: []string{
					"servers", "recursion_desired", "notify_allowed",
				},
			},
		},
	})
}

// TestAccRecursorACL covers the two named settings and the rejection of any
// other name.
func TestAccRecursorACL(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { recursorPreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `
resource "powerdns_recursor_acl" "allow" {
  setting  = "allow-from"
  netmasks = ["10.0.0.0/8", "2001:db8::/32"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("powerdns_recursor_acl.allow",
						"id", "allow-from"),
					resource.TestCheckResourceAttr("powerdns_recursor_acl.allow",
						"netmasks.#", "2"),
				),
			},
			{
				// Reordered, and one prefix respelled. Neither is a change.
				Config: `
resource "powerdns_recursor_acl" "allow" {
  setting  = "allow-from"
  netmasks = ["2001:db8:0:0::/32", "10.0.0.0/8"]
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccDNSDistACL covers the one setting dnsdist's API can write.
func TestAccDNSDistACL(t *testing.T) {
	if os.Getenv("PDNS_DNSDIST_SERVER_URL") == "" {
		t.Skip("PDNS_DNSDIST_SERVER_URL is not set; run task lab:up")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `
resource "powerdns_dnsdist_acl" "allow" {
  netmasks = ["10.0.0.0/8", "127.0.0.0/8", "::1/128"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("powerdns_dnsdist_acl.allow",
						"id", "allow-from"),
					resource.TestCheckResourceAttr("powerdns_dnsdist_acl.allow",
						"netmasks.#", "3"),
				),
			},
			{
				// The IPv6 prefix uncompressed, and the list reordered.
				Config: `
resource "powerdns_dnsdist_acl" "allow" {
  netmasks = ["0000:0000:0000:0000:0000:0000:0000:0001/128", "10.0.0.0/8", "127.0.0.0/8"]
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// testAccCheckRecursorZoneDestroyed asserts the zone is gone from the
// Recursor, asking the server rather than trusting state.
func testAccCheckRecursorZoneDestroyed(zone string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		client, err := acceptanceRecursorClient()
		if err != nil {
			return err
		}

		zones, err := client.ListZones(t0())
		if err != nil {
			return fmt.Errorf("listing Recursor zones: %w", err)
		}
		for _, z := range zones {
			if z.ID == zone {
				return fmt.Errorf("Recursor zone %s still exists after destroy", zone)
			}
		}
		return nil
	}
}
