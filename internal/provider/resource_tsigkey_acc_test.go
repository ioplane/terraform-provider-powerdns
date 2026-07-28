//go:build acceptance

package provider_test

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccTSIGKey_SecretNeverInState is the TSIG half of the state-file rule.
//
// The secret comes back from create, from update and from the single-key read.
// This resource asks the collection, which blanks it, and stores nothing a
// write returned.
func TestAccTSIGKey_SecretNeverInState(t *testing.T) {
	name := "acc-" + runID

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckTSIGKeyDestroyed(name + "."),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "powerdns_tsigkey" "transfer" {
  name      = %q
  algorithm = "hmac-sha256"
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					// The server canonicalises the name into the id.
					resource.TestCheckResourceAttr("powerdns_tsigkey.transfer", "id", name+"."),
					resource.TestCheckResourceAttr("powerdns_tsigkey.transfer", "algorithm",
						"hmac-sha256"),
					// The generated secret exists on the server and nowhere here.
					testAccCheckStateHoldsNoKeyMaterial(),
					testAccCheckTSIGSecretAbsentFromState("powerdns_tsigkey.transfer"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "powerdns_tsigkey" "transfer" {
  name      = %q
  algorithm = "hmac-sha256"
}
`, name),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// Changing the algorithm replaces the key. PowerDNS's PUT would
				// otherwise leave the old entry beside the new one.
				Config: fmt.Sprintf(`
resource "powerdns_tsigkey" "transfer" {
  name      = %q
  algorithm = "hmac-sha512"
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("powerdns_tsigkey.transfer", "algorithm",
						"hmac-sha512"),
					resource.TestCheckResourceAttr("powerdns_tsigkey.transfer", "id", name+"."),
					testAccCheckTSIGSecretAbsentFromState("powerdns_tsigkey.transfer"),
					// One key, not two: the replacement removed the old entry
					// that a bare PUT would have left behind.
					testAccCheckTSIGKeyCount(name+".", 1),
				),
			},
			{
				ResourceName:      "powerdns_tsigkey.transfer",
				ImportState:       true,
				ImportStateId:     name,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccTSIGKey_ImportedSecretIsWriteOnly checks that a secret supplied by the
// operator also stays off disk.
//
// secret_wo is a write-only attribute: Terraform hands it to the provider
// during apply and stores it in neither the state file nor the plan file.
func TestAccTSIGKey_ImportedSecretIsWriteOnly(t *testing.T) {
	name := "acc-imported-" + runID

	// A syntactically valid base64 HMAC secret, generated for this test alone.
	const secret = "dGVycmFmb3JtLXByb3ZpZGVyLXBvd2VyZG5zLWFjY2VwdGFuY2U="

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckTSIGKeyDestroyed(name + "."),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "powerdns_tsigkey" "imported" {
  name      = %q
  algorithm = "hmac-sha256"
  secret_wo = %q
}
`, name, secret),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("powerdns_tsigkey.imported", "id", name+"."),
					// The value the operator supplied is not in state either.
					testAccCheckValueAbsentFromState(secret),
					testAccCheckStateHoldsNoKeyMaterial(),
				),
			},
		},
	})
}

// TestAccZone_SignedZoneValidates is the behaviour test.
//
// Every other assertion in this suite is about what an API returned. This one
// asks the DNS server whether the zone actually answers with signatures — the
// only check that would catch a provider which configured DNSSEC correctly
// through the API and produced a zone that does not serve RRSIGs.
func TestAccZone_SignedZoneValidates(t *testing.T) {
	if _, err := exec.LookPath("dig"); err != nil {
		t.Skip("dig is not on PATH; the dev image carries dnsutils")
	}

	zone := acceptanceZoneName(t, "signed")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "powerdns_zone" "signed" {
  name        = %[1]q
  kind        = "Native"
  nameservers = ["ns1.%[1]s"]
}

resource "powerdns_zone_cryptokey" "ksk" {
  zone      = powerdns_zone.signed.id
  key_type  = "ksk"
  algorithm = "ECDSAP256SHA256"
}

resource "powerdns_record" "www" {
  zone       = powerdns_zone.signed.id
  name       = "www.%[1]s"
  type       = "A"
  ttl        = 60
  values     = ["192.0.2.1"]
  depends_on = [powerdns_zone_cryptokey.ksk]
}
`, zone),
				// dnssec is not asserted here: the zone was created before the
				// key, so state still holds what it was created with. The
				// refresh at the end of the step picks it up, and the check
				// that matters is the one below — whether the server actually
				// serves signatures.
				Check: testAccCheckZoneServesSignatures(zone),
			},
		},
	})
}

// testAccCheckZoneServesSignatures queries the lab's authoritative server and
// requires an RRSIG in the answer.
//
// The DNS port is where the lab publishes it. An API that reports a signed
// zone and a resolver that returns unsigned answers is exactly the gap between
// "the request succeeded" and "the thing works".
func testAccCheckZoneServesSignatures(zone string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		// One type per query: dig reads a second type name as a hostname, and
		// the query silently becomes something else. +dnssec is what asks for
		// the signature, so the RRSIG arrives in the answer section beside the
		// A record.
		// A context bounds the query: a lab that stopped answering should fail
		// the test rather than hang it.
		ctx, cancel := context.WithTimeout(t0(), 10*time.Second)
		defer cancel()

		out, err := exec.CommandContext(ctx, "dig", "+dnssec", "+norecurse",
			"@127.0.0.1", "-p", labDNSPort, "www."+zone, "A").CombinedOutput()
		if err != nil {
			return fmt.Errorf("dig against the lab: %w (%s)", err, out)
		}

		answer := string(out)
		if !strings.Contains(answer, "192.0.2.1") {
			return fmt.Errorf("the zone does not answer for www.%s at all:\n%s", zone, answer)
		}
		if !strings.Contains(answer, "RRSIG") {
			return fmt.Errorf("no RRSIG for www.%s; the API reports the zone signed and "+
				"the server serves it unsigned:\n%s", zone, answer)
		}
		return nil
	}
}

// labDNSPort is where compose.lab.yml publishes the authoritative server's DNS
// port. The API is on a different one; a behaviour test has to ask the thing
// that answers queries.
const labDNSPort = "15300"

// testAccCheckTSIGKeyDestroyed asserts the key is gone from the server.
func testAccCheckTSIGKeyDestroyed(keyID string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		client, err := acceptanceAuthClient()
		if err != nil {
			return err
		}

		keys, err := client.ListTSIGKeys(t0())
		if err != nil {
			return fmt.Errorf("listing TSIG keys: %w", err)
		}
		for _, key := range keys {
			if key.ID == keyID {
				return fmt.Errorf("TSIG key %s still exists after destroy", keyID)
			}
		}
		return nil
	}
}

// testAccCheckTSIGKeyCount asserts how many entries the collection holds for
// one id.
//
// PowerDNS's PUT can leave two keys under the same id, so "the algorithm is
// right" is not enough — the old one has to be gone.
func testAccCheckTSIGKeyCount(keyID string, want int) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		client, err := acceptanceAuthClient()
		if err != nil {
			return err
		}

		keys, err := client.ListTSIGKeys(t0())
		if err != nil {
			return fmt.Errorf("listing TSIG keys: %w", err)
		}

		var found int
		for _, key := range keys {
			if key.ID == keyID {
				found++
			}
		}
		if found != want {
			return fmt.Errorf("found %d entries for %s, want %d", found, keyID, want)
		}
		return nil
	}
}

// testAccCheckTSIGSecretAbsentFromState fails if the resource carries anything
// that could be a secret.
//
// Distinct from the generic key-material check: this one knows the attribute
// that would hold it and asserts it is absent rather than merely harmless.
func testAccCheckTSIGSecretAbsentFromState(address string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[address]
		if !ok {
			return fmt.Errorf("%s is not in state", address)
		}

		for _, attr := range []string{"secret_wo", "secret", "key"} {
			if value, present := rs.Primary.Attributes[attr]; present && value != "" {
				return fmt.Errorf("%s.%s holds %q; a TSIG secret reached state",
					address, attr, value)
			}
		}
		return nil
	}
}

// testAccCheckValueAbsentFromState looks for one specific string anywhere in
// state, which is how an operator-supplied secret is checked.
func testAccCheckValueAbsentFromState(needle string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		for name, rs := range state.RootModule().Resources {
			for attr, value := range rs.Primary.Attributes {
				if strings.Contains(value, needle) {
					return fmt.Errorf("%s.%s contains the configured secret", name, attr)
				}
			}
		}
		return nil
	}
}
