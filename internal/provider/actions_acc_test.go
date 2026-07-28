//go:build acceptance

package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// Actions need Terraform 1.14. Below that the configuration does not parse, so
// the whole file skips rather than failing with a syntax error that says
// nothing about the cause.
func actionVersionCheck() []tfversion.TerraformVersionCheck {
	return []tfversion.TerraformVersionCheck{
		tfversion.SkipBelow(tfversion.Version1_14_0),
	}
}

// TestAccAction_NotifyAndRectify runs the two zone actions that succeed on an
// ordinary zone.
//
// An action has no state, so there is nothing to assert afterwards beyond the
// apply having succeeded — which is the point. The value of the test is that
// the action is wired up, reaches the right endpoint and does not error on a
// zone the provider itself created.
func TestAccAction_NotifyAndRectify(t *testing.T) {
	zone := acceptanceZoneName(t, "action-zone")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		TerraformVersionChecks:   actionVersionCheck(),
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + fmt.Sprintf(`
action "powerdns_notify_zone" "notify" {
  config {
    zone = powerdns_zone.host.id
  }
}

action "powerdns_rectify_zone" "rectify" {
  config {
    zone = powerdns_zone.host.id
  }
}

resource "powerdns_record" "www" {
  zone   = powerdns_zone.host.id
  name   = "www.%s"
  type   = "A"
  ttl    = 60
  values = ["192.0.2.1"]

  lifecycle {
    action_trigger {
      events  = [after_create, after_update]
      actions = [action.powerdns_notify_zone.notify, action.powerdns_rectify_zone.rectify]
    }
  }
}
`, zone),
				Check: resource.TestCheckResourceAttr("powerdns_record.www", "values.#", "1"),
			},
		},
	})
}

// TestAccAction_AXFRRetrieveOnANativeZone asserts the diagnostic rather than
// the failure, like the capability tests.
//
// A transfer only means something for a Slave zone; PowerDNS says so in a 422
// whose text names the reason, and that text has to survive.
func TestAccAction_AXFRRetrieveOnANativeZone(t *testing.T) {
	zone := acceptanceZoneName(t, "action-axfr")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		TerraformVersionChecks:   actionVersionCheck(),
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + fmt.Sprintf(`
action "powerdns_axfr_retrieve" "pull" {
  config {
    zone = powerdns_zone.host.id
  }
}

resource "powerdns_record" "www" {
  zone   = powerdns_zone.host.id
  name   = "www.%s"
  type   = "A"
  ttl    = 60
  values = ["192.0.2.1"]

  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.powerdns_axfr_retrieve.pull]
    }
  }
}
`, zone),
				// Terraform wraps a diagnostic body, so the phrase can carry a
				// newline. Matching it literally passes locally and fails on
				// a terminal of a different width.
				ExpectError: regexp.MustCompile(`not a secondary\s+domain`),
			},
		},
	})
}

// TestAccAction_FlushCache covers the one action that spans all three
// products, and the count-of-zero case.
func TestAccAction_FlushCache(t *testing.T) {
	zone := acceptanceZoneName(t, "action-flush")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		TerraformVersionChecks:   actionVersionCheck(),
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + fmt.Sprintf(`
action "powerdns_flush_cache" "auth" {
  config {
    domain  = powerdns_zone.host.id
    product = "auth"
  }
}

resource "powerdns_record" "www" {
  zone   = powerdns_zone.host.id
  name   = "www.%s"
  type   = "A"
  ttl    = 60
  values = ["192.0.2.1"]

  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.powerdns_flush_cache.auth]
    }
  }
}
`, zone),
				// Nothing was cached, so the flush dropped nothing. That is a
				// successful flush and the apply must not fail.
				Check: resource.TestCheckResourceAttr("powerdns_record.www", "ttl", "60"),
			},
		},
	})
}

// TestAccAction_UnknownProductIsRejected checks the message for a value the
// schema cannot constrain, since the product is a free string.
func TestAccAction_UnknownProductIsRejected(t *testing.T) {
	zone := acceptanceZoneName(t, "action-bad")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acceptancePreCheck(t) },
		TerraformVersionChecks:   actionVersionCheck(),
		ProtoV6ProviderFactories: testAccProviderFactories(),
		CheckDestroy:             testAccCheckZoneDestroyed(zone),
		Steps: []resource.TestStep{
			{
				Config: recordZoneConfig(zone) + fmt.Sprintf(`
action "powerdns_flush_cache" "wrong" {
  config {
    domain  = powerdns_zone.host.id
    product = "postgres"
  }
}

resource "powerdns_record" "www" {
  zone   = powerdns_zone.host.id
  name   = "www.%s"
  type   = "A"
  ttl    = 60
  values = ["192.0.2.1"]

  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.powerdns_flush_cache.wrong]
    }
  }
}
`, zone),
				ExpectError: regexp.MustCompile(`auth,\s+recursor\s+or\s+dnsdist`),
			},
		},
	})
}
