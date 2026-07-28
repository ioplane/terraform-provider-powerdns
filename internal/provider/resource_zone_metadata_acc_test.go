//go:build acceptance

package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccZoneMetadata covers the kind this resource exists to avoid touching.
//
// PowerDNS sets SOA-EDIT-API on every zone it creates. A resource owning the
// whole metadata collection would try to delete it on the first apply, on
// every zone, for ever — so this one owns a single kind, and the test asserts
// the server-assigned kind is still there once Terraform has finished.
func TestAccZoneMetadata(t *testing.T) {
	zone := acceptanceZoneName(t, "meta")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + `
resource "powerdns_zone_metadata" "axfr" {
  zone   = powerdns_zone.host.id
  kind   = "ALLOW-AXFR-FROM"
  values = ["192.0.2.0/24", "2001:db8::/32"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("powerdns_zone_metadata.axfr", "values.#", "2"),
					resource.TestCheckResourceAttr("powerdns_zone_metadata.axfr", "id",
						zone+"/ALLOW-AXFR-FROM"),
					// The kind nobody configured is still present. If the
					// resource ever grows to own the collection, this fails.
					testAccCheckMetadataKindPresent(zone, "SOA-EDIT-API"),
				),
			},
			{
				Config: recordZoneConfig(zone) + `
resource "powerdns_zone_metadata" "axfr" {
  zone   = powerdns_zone.host.id
  kind   = "ALLOW-AXFR-FROM"
  values = ["192.0.2.0/24", "2001:db8::/32"]
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// A PUT replaces the values outright, so removing one is an
				// update rather than a delete-and-create.
				Config: recordZoneConfig(zone) + `
resource "powerdns_zone_metadata" "axfr" {
  zone   = powerdns_zone.host.id
  kind   = "ALLOW-AXFR-FROM"
  values = ["192.0.2.0/24"]
}
`,
				Check: resource.TestCheckResourceAttr(
					"powerdns_zone_metadata.axfr", "values.#", "1"),
			},
			{
				ResourceName:      "powerdns_zone_metadata.axfr",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return zone + "/ALLOW-AXFR-FROM", nil
				},
			},
		},
	})
}

// testAccCheckMetadataKindPresent asserts a kind exists on the server, whether
// or not Terraform manages it.
func testAccCheckMetadataKindPresent(zone, kind string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		client, err := acceptanceAuthClient()
		if err != nil {
			return err
		}

		entries, err := client.ListMetadata(t0(), zone)
		if err != nil {
			return fmt.Errorf("listing metadata on %s: %w", zone, err)
		}

		for _, entry := range entries {
			if entry.Kind == kind {
				return nil
			}
		}
		return fmt.Errorf("%s is missing from %s; the provider deleted metadata it "+
			"does not manage", kind, zone)
	}
}
