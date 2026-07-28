//go:build acceptance

package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// Identity is what lets Terraform recognise a remote object without parsing an
// id string, and importing by identity is what it is for.
//
// The tests below import with an identity object rather than an id, which is
// the path that would break silently if an identity were declared and never
// populated: the schema would exist, the plan would accept the block, and the
// import would find nothing.

// identityVersionCheck gates on the Terraform that understands identity in an
// import block.
func identityVersionCheck() []tfversion.TerraformVersionCheck {
	return []tfversion.TerraformVersionCheck{
		tfversion.SkipBelow(tfversion.Version1_12_0),
	}
}

// TestAccIdentity_ImportZoneByIdentity imports with an identity object.
func TestAccIdentity_ImportZoneByIdentity(t *testing.T) {
	zone := acceptanceZoneName(t, "identity-zone")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		TerraformVersionChecks:   identityVersionCheck(),
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				// No nameservers: the attribute is create-only and never read
				// back, so a zone created with it cannot round-trip through an
				// import block — the imported object has none, the
				// configuration has some, and the difference forces
				// replacement. That is a property of the API rather than of
				// the identity, and the contract says so.
				Config: fmt.Sprintf(`
resource "powerdns_zone" "host" {
  name = %q
  kind = "Native"
}
`, zone),
			},
			{
				ResourceName:    "powerdns_zone.host",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
			},
		},
	})
}

// TestAccIdentity_ImportRecordByIdentity is the case that shows why an
// identity beats an id string.
//
// The record's id is `<zone>/<name>/<type>`, and importing by it means the
// caller assembles and Terraform splits that string. The identity carries the
// three parts as three attributes, so nothing parses anything.
func TestAccIdentity_ImportRecordByIdentity(t *testing.T) {
	zone := acceptanceZoneName(t, "identity-record")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		TerraformVersionChecks:   identityVersionCheck(),
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
`, zone),
			},
			{
				ResourceName:    "powerdns_record.www",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
			},
		},
	})
}

// TestAccIdentity_ZoneIdentityIsTheZoneName pins the value rather than only
// the round trip, because an identity that imported correctly while holding
// the wrong value would still be wrong for anything else reading it.
func TestAccIdentity_ZoneIdentityIsTheZoneName(t *testing.T) {
	zone := acceptanceZoneName(t, "identity-value")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		TerraformVersionChecks:   identityVersionCheck(),
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentityValue(
						"powerdns_zone.host",
						tfjsonpath.New("zone_name"),
						knownvalue.StringExact(zone),
					),
				},
			},
		},
	})
}

// TestAccIdentity_MetadataIdentityCarriesBothParts checks a two-attribute
// identity, where an implementation that set only the first would still import
// and quietly address the wrong object.
func TestAccIdentity_MetadataIdentityCarriesBothParts(t *testing.T) {
	zone := acceptanceZoneName(t, "identity-meta")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		TerraformVersionChecks:   identityVersionCheck(),
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + `
resource "powerdns_zone_metadata" "axfr" {
  zone   = powerdns_zone.host.id
  kind   = "ALLOW-AXFR-FROM"
  values = ["192.0.2.0/24"]
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentityValue(
						"powerdns_zone_metadata.axfr",
						tfjsonpath.New("zone_name"),
						knownvalue.StringExact(zone),
					),
					statecheck.ExpectIdentityValue(
						"powerdns_zone_metadata.axfr",
						tfjsonpath.New("kind"),
						knownvalue.StringExact("ALLOW-AXFR-FROM"),
					),
				},
			},
		},
	})
}
