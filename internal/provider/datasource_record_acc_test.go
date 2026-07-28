//go:build acceptance

package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccRecordDataSource reads a set back and checks the values survive the
// round trip, including the disabled flag.
func TestAccRecordDataSource(t *testing.T) {
	zone := acceptanceZoneName(t, "ds-rec")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + fmt.Sprintf(`
resource "powerdns_record" "mx" {
  zone   = powerdns_zone.host.id
  name   = %[1]q
  type   = "MX"
  ttl    = 3600
  values = ["10 mail1.%[1]s", "20 mail2.%[1]s"]
}

data "powerdns_record" "mx" {
  zone       = powerdns_zone.host.id
  name       = %[1]q
  type       = "MX"
  depends_on = [powerdns_record.mx]
}
`, zone),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.powerdns_record.mx", "ttl", "3600"),
					resource.TestCheckResourceAttr("data.powerdns_record.mx", "values.#", "2"),
					resource.TestCheckResourceAttr("data.powerdns_record.mx", "disabled", "false"),
					resource.TestCheckResourceAttr("data.powerdns_record.mx", "id",
						fmt.Sprintf("%[1]s/%[1]s/MX", zone)),
				),
			},
		},
	})
}

// TestAccRecordDataSource_Missing pins the choice that an absent set is an
// error rather than an empty result.
//
// An empty result would let a configuration proceed on values that do not
// exist, and fail somewhere further along with no mention of the record.
func TestAccRecordDataSource_Missing(t *testing.T) {
	zone := acceptanceZoneName(t, "ds-rec-missing")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + fmt.Sprintf(`
data "powerdns_record" "absent" {
  zone       = powerdns_zone.host.id
  name       = "nothing.%s"
  type       = "A"
  depends_on = [powerdns_zone.host]
}
`, zone),
				ExpectError: regexp.MustCompile(`Record set not found`),
			},
		},
	})
}

// TestAccZoneMetadataDataSource_UnsetKind covers the shape PowerDNS chose:
// an unset kind answers 200 with an empty list, not 404, so the data source
// returns an empty list rather than failing.
//
// That is the opposite of the record data source above, and deliberately so —
// there the absence means the configuration is wrong, here it means the kind
// is simply not set, which is a state worth branching on.
func TestAccZoneMetadataDataSource_UnsetKind(t *testing.T) {
	zone := acceptanceZoneName(t, "ds-meta")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + `
data "powerdns_zone_metadata" "unset" {
  zone       = powerdns_zone.host.id
  kind       = "ALLOW-DNSUPDATE-FROM"
  depends_on = [powerdns_zone.host]
}

`,
				Check: resource.TestCheckResourceAttr(
					"data.powerdns_zone_metadata.unset", "values.#", "0"),
			},
		},
	})
}

// TestAccZoneMetadata_KindOnlyOnTheZone covers the boundary the API draws and
// does not explain.
//
// SOA-EDIT-API appears in GET /metadata and answers 422 "Unsupported metadata
// kind" on GET /metadata/SOA-EDIT-API. The provider rejects it before the
// request, naming the zone attribute that does work — the server's own message
// does not mention that the value is settable elsewhere.
func TestAccZoneMetadata_KindOnlyOnTheZone(t *testing.T) {
	zone := acceptanceZoneName(t, "ds-meta-attr")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + `
data "powerdns_zone_metadata" "unsupported" {
  zone       = powerdns_zone.host.id
  kind       = "SOA-EDIT-API"
  depends_on = [powerdns_zone.host]
}
`,
				ExpectError: regexp.MustCompile(`Set powerdns_zone\.soa_edit_api instead`),
			},
		},
	})
}

// TestAccZoneExportDataSource checks the export is a zone file rather than the
// JSON envelope PowerDNS wraps it in.
//
// That envelope is specification defect 5: the schema declares this endpoint
// returns a string, and the server sends an object.
func TestAccZoneExportDataSource(t *testing.T) {
	zone := acceptanceZoneName(t, "ds-export")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + `
data "powerdns_zone_export" "export" {
  zone       = powerdns_zone.host.id
  depends_on = [powerdns_zone.host]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// A zone file, so it names the zone and carries an SOA.
					resource.TestMatchResourceAttr("data.powerdns_zone_export.export",
						"zone_file", regexp.MustCompile(`\sSOA\s`)),
					resource.TestMatchResourceAttr("data.powerdns_zone_export.export",
						"zone_file", regexp.MustCompile(`\sNS\s`)),
					// Not the JSON envelope.
					resource.TestMatchResourceAttr("data.powerdns_zone_export.export",
						"zone_file", regexp.MustCompile(`^[^{]`)),
				),
			},
		},
	})
}
