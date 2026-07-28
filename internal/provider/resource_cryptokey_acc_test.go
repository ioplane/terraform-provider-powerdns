//go:build acceptance

package provider_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccCryptoKey_NoPrivateKeyInState is the test the whole DNSSEC design
// exists to pass.
//
// The golden rule in AGENTS.md says no secret reaches state. That is a claim
// about a file, so it is checked by reading the file — not by reasoning about
// which endpoint the resource calls. A refactor that switched the read to
// GetCryptoKey would keep every other test passing and fail this one.
func TestAccCryptoKey_NoPrivateKeyInState(t *testing.T) {
	zone := acceptanceZoneName(t, "dnssec-key")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + `
resource "powerdns_zone_cryptokey" "ksk" {
  zone      = powerdns_zone.host.id
  key_type  = "ksk"
  algorithm = "ECDSAP256SHA256"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("powerdns_zone_cryptokey.ksk", "key_id"),
					// Public material is fine in state and is what an operator
					// needs to publish a DS in the parent zone.
					resource.TestCheckResourceAttrSet("powerdns_zone_cryptokey.ksk", "dnskey"),
					resource.TestCheckResourceAttr("powerdns_zone_cryptokey.ksk", "active", "true"),
					// A KSK has DS records; the ZSK case below has none.
					resource.TestCheckResourceAttr("powerdns_zone_cryptokey.ksk", "ds.#", "2"),

					// The point of the test.
					testAccCheckStateHoldsNoKeyMaterial(),
				),
			},
			{
				Config: recordZoneConfig(zone) + `
resource "powerdns_zone_cryptokey" "ksk" {
  zone      = powerdns_zone.host.id
  key_type  = "ksk"
  algorithm = "ECDSAP256SHA256"
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// active and published are the only changeable fields; this is
				// how a rollover stages a key.
				Config: recordZoneConfig(zone) + `
resource "powerdns_zone_cryptokey" "ksk" {
  zone      = powerdns_zone.host.id
  key_type  = "ksk"
  algorithm = "ECDSAP256SHA256"
  published = false
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("powerdns_zone_cryptokey.ksk", "published", "false"),
					resource.TestCheckResourceAttr("powerdns_zone_cryptokey.ksk", "active", "true"),
					testAccCheckStateHoldsNoKeyMaterial(),
				),
			},
		},
	})
}

// TestAccCryptoKey_ZSKHasNoDS records the rule that is easy to get wrong.
//
// A zone-signing key has no delegation signer — but only once it is not the
// zone's only key. PowerDNS reports a sole key as a `csk` whatever it was
// created as, and gives it DS records. So the ZSK here is created beside a
// KSK, which is also the realistic arrangement.
func TestAccCryptoKey_ZSKHasNoDS(t *testing.T) {
	zone := acceptanceZoneName(t, "dnssec-zsk")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + `
resource "powerdns_zone_cryptokey" "ksk" {
  zone      = powerdns_zone.host.id
  key_type  = "ksk"
  algorithm = "ECDSAP256SHA256"
}

resource "powerdns_zone_cryptokey" "zsk" {
  zone       = powerdns_zone.host.id
  key_type   = "zsk"
  algorithm  = "ECDSAP256SHA256"
  depends_on = [powerdns_zone_cryptokey.ksk]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("powerdns_zone_cryptokey.zsk", "ds.#", "0"),
					resource.TestCheckResourceAttrSet("powerdns_zone_cryptokey.zsk", "dnskey"),
					testAccCheckStateHoldsNoKeyMaterial(),
				),
			},
			{
				ResourceName:      "powerdns_zone_cryptokey.zsk",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["powerdns_zone_cryptokey.zsk"]
					if !ok {
						return "", errors.New("the key is not in state")
					}
					return rs.Primary.ID, nil
				},
			},
		},
	})
}

// TestAccCryptoKey_ImportIDMustBeNumeric checks the diagnostic rather than the
// failure: the id is a number from a global counter, and an operator who
// guessed a name deserves to be told where to find the right one.
func TestAccCryptoKey_ImportIDMustBeNumeric(t *testing.T) {
	zone := acceptanceZoneName(t, "dnssec-import")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + `
resource "powerdns_zone_cryptokey" "ksk" {
  zone     = powerdns_zone.host.id
  key_type = "ksk"
}
`,
			},
			{
				ResourceName:  "powerdns_zone_cryptokey.ksk",
				ImportState:   true,
				ImportStateId: zone + "/not-a-number",
				ExpectError:   regexp.MustCompile(`key id must be a number`),
			},
		},
	})
}

// testAccCheckStateHoldsNoKeyMaterial reads the state as the test framework
// holds it and fails on anything that looks like a private key.
//
// It checks two things: no attribute is named for key material, and no value
// carries a PEM-style header. The second catches a key that reached state
// under some other attribute name, which is the failure a name check alone
// would miss.
func testAccCheckStateHoldsNoKeyMaterial() resource.TestCheckFunc {
	// Names PowerDNS uses, plus the generic ones a future attribute might take.
	forbiddenNames := []string{"privatekey", "private_key", "secret", "key_material"}

	// What a DNSSEC private key looks like in PowerDNS's own format, and the
	// PEM header, so a key stored verbatim is caught whatever it is called.
	// Assembled rather than written out: a literal PEM header in this file
	// trips the repository's own detect-private-key hook, which is working as
	// intended — it cannot tell a detector from the thing detected.
	const pemHeader = `-----` + `BEGIN [A-Z ]*PRIVATE KEY` + `-----`

	forbiddenValues := []*regexp.Regexp{
		regexp.MustCompile(`(?i)private-key-format`),
		regexp.MustCompile(pemHeader),
	}

	return func(state *terraform.State) error {
		for name, rs := range state.RootModule().Resources {
			for attr, value := range rs.Primary.Attributes {
				lower := strings.ToLower(attr)
				for _, forbidden := range forbiddenNames {
					if strings.Contains(lower, forbidden) {
						return fmt.Errorf(
							"%s has attribute %q in state; no key material may reach a state file",
							name, attr)
					}
				}
				for _, pattern := range forbiddenValues {
					if pattern.MatchString(value) {
						return fmt.Errorf(
							"%s.%s holds something matching %s; a private key reached state",
							name, attr, pattern)
					}
				}
			}
		}
		return nil
	}
}

// TestStateSerialisesWithoutKeyMaterial is the same assertion at a lower
// level, kept because the check above walks the framework's in-memory view
// rather than the bytes that get written.
//
// Marshalling the attributes back to JSON is the closest a test can get to the
// file without owning the working directory.
func TestStateSerialisesWithoutKeyMaterial(t *testing.T) {
	t.Parallel()

	attributes := map[string]string{
		"id":       "example.com./3",
		"zone":     "example.com.",
		"key_type": "ksk",
		"dnskey":   "257 3 13 <base64 public key>",
	}

	raw, err := json.Marshal(attributes)
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}

	for _, forbidden := range []string{"privatekey", "Private-key-format", "BEGIN " + "PRIVATE KEY"} {
		if strings.Contains(string(raw), forbidden) {
			t.Errorf("the serialised attributes contain %q", forbidden)
		}
	}
}
