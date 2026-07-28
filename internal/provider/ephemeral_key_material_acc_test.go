//go:build acceptance

package provider_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestAccEphemeral_KeyMaterialStaysOffDisk is the pair to the state-file test.
//
// The managed resources cannot return key material, which closes the leak and
// leaves a real need unmet: handing a DNSSEC key to a signing appliance, or a
// TSIG secret to a secondary. An ephemeral resource meets it — Terraform
// fetches the value during the operation and discards it.
//
// So this asserts two things at once: the value was genuinely fetched (the
// apply succeeds, and a write-only attribute consumes it), and it is absent
// from state afterwards.
func TestAccEphemeral_KeyMaterialStaysOffDisk(t *testing.T) {
	zone := acceptanceZoneName(t, "ephemeral")
	keyName := "acc-ephemeral-" + runID

	resource.Test(t, resource.TestCase{
		PreCheck: func() { acceptancePreCheck(t) },
		// Ephemeral resources need Terraform 1.10, write-only attributes 1.11.
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + fmt.Sprintf(`
resource "powerdns_zone_cryptokey" "ksk" {
  zone      = powerdns_zone.host.id
  key_type  = "ksk"
  algorithm = "ECDSAP256SHA256"
}

# Reads the private key the managed resource refuses to expose.
ephemeral "powerdns_cryptokey_material" "ksk" {
  zone   = powerdns_zone.host.id
  key_id = powerdns_zone_cryptokey.ksk.key_id
}

resource "powerdns_tsigkey" "secondary" {
  name      = %q
  algorithm = "hmac-sha256"
}

# Reads the generated secret, again without storing it.
ephemeral "powerdns_tsigkey_secret" "secondary" {
  name = powerdns_tsigkey.secondary.id
}
`, keyName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckStateHoldsNoKeyMaterial(),
					// Nothing named for an ephemeral value is in state at all:
					// ephemeral results are not persisted, so there is no
					// "ephemeral.*" entry to find.
					testAccCheckNoEphemeralInState(),
				),
			},
		},
	})
}

// TestAccEphemeral_ConsumedByAWriteOnlyAttribute is the shape the feature
// exists for.
//
// An ephemeral value may only be consumed by another ephemeral or write-only
// attribute — the restriction is what stops something downstream persisting
// it. Here a TSIG secret is read ephemerally and handed straight to a second
// key's write-only secret, so the value passes through Terraform without ever
// being stored on either side.
func TestAccEphemeral_ConsumedByAWriteOnlyAttribute(t *testing.T) {
	source := "acc-src-" + runID
	target := "acc-dst-" + runID

	resource.Test(t, resource.TestCase{
		PreCheck: func() { acceptancePreCheck(t) },
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckTSIGKeyDestroyed(source+"."),
			testAccCheckTSIGKeyDestroyed(target+"."),
		),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "powerdns_tsigkey" "source" {
  name      = %[1]q
  algorithm = "hmac-sha256"
}

ephemeral "powerdns_tsigkey_secret" "source" {
  name = powerdns_tsigkey.source.id
}

# The same secret on a second key, copied without either end storing it.
resource "powerdns_tsigkey" "target" {
  name      = %[2]q
  algorithm = "hmac-sha256"
  secret_wo = ephemeral.powerdns_tsigkey_secret.source.secret
}
`, source, target),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("powerdns_tsigkey.target", "id", target+"."),
					testAccCheckStateHoldsNoKeyMaterial(),
					// The proof that the copy actually happened: both keys
					// authenticate with the same secret on the server.
					testAccCheckTSIGSecretsMatch(source+".", target+"."),
				),
			},
		},
	})
}

// testAccCheckNoEphemeralInState fails if anything ephemeral was persisted.
func testAccCheckNoEphemeralInState() resource.TestCheckFunc {
	return func(state *terraform.State) error {
		for name := range state.RootModule().Resources {
			if strings.HasPrefix(name, "ephemeral.") {
				return fmt.Errorf("%s is in state; ephemeral results must not be persisted",
					name)
			}
		}
		return nil
	}
}

// testAccCheckTSIGSecretsMatch asks the server whether two keys share a secret.
//
// The assertion has to happen server-side: neither secret is in state, which
// is the whole point, so there is nothing local to compare.
func testAccCheckTSIGSecretsMatch(first, second string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		client, err := acceptanceAuthClient()
		if err != nil {
			return err
		}

		left, err := client.GetTSIGKey(t0(), first)
		if err != nil {
			return fmt.Errorf("reading %s: %w", first, err)
		}
		right, err := client.GetTSIGKey(t0(), second)
		if err != nil {
			return fmt.Errorf("reading %s: %w", second, err)
		}

		if left.Key == "" || right.Key == "" {
			return fmt.Errorf("one of the keys has no secret: %s=%d bytes, %s=%d bytes",
				first, len(left.Key), second, len(right.Key))
		}
		if left.Key != right.Key {
			return fmt.Errorf("the secrets differ; the ephemeral value did not reach %s",
				second)
		}
		return nil
	}
}
