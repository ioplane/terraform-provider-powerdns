//go:build acceptance

package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccZoneDataSource reads a zone the configuration also manages.
//
// That is not the intended use — the point of the data source is a zone owned
// elsewhere — but it is the only way to assert the values without depending on
// something the test did not create.
func TestAccZoneDataSource(t *testing.T) {
	zone := acceptanceZoneName(t, "ds-zone")

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
  values = ["192.0.2.1"]
}

data "powerdns_zone" "read" {
  name       = powerdns_zone.host.id
  depends_on = [powerdns_record.www]
}
`, zone),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.powerdns_zone.read", "id", zone),
					resource.TestCheckResourceAttr("data.powerdns_zone.read", "kind", "Native"),
					// The server's spelling, not a configured one: a data
					// source has no plan to stay consistent with.
					resource.TestCheckResourceAttr("data.powerdns_zone.read", "soa_edit_api", "DEFAULT"),
					resource.TestCheckResourceAttrSet("data.powerdns_zone.read", "serial"),
					// SOA, NS and the A record just created.
					resource.TestCheckResourceAttr("data.powerdns_zone.read", "record_count", "3"),
				),
			},
		},
	})
}

// TestAccZonesDataSource lists the zones and looks for the one just created.
//
// It asserts presence rather than a count: the lab is shared with the other
// tests in this package, which run in parallel and create zones of their own.
func TestAccZonesDataSource(t *testing.T) {
	zone := acceptanceZoneName(t, "ds-zones")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + `
data "powerdns_zones" "all" {
  depends_on = [powerdns_zone.host]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.powerdns_zones.all", "id", "zones"),
					resource.TestCheckTypeSetElemAttr(
						"data.powerdns_zones.all", "names.*", zone),
				),
			},
		},
	})
}
