# Actions, ephemeral resources and an autoprimary.
#
# Actions require Terraform 1.14 or later, so this unit is the one place in the
# suite pinned to Terraform rather than OpenTofu. That is not a preference: the
# capability does not exist in the other engine, and pretending otherwise would
# make the whole suite fail on a difference that is documented upstream.

terraform {
  required_version = ">= 1.14"
  required_providers {
    powerdns = {
      source  = "registry.terraform.io/ioplane/powerdns"
      version = "0.1.1"
    }
  }
}

resource "powerdns_zone" "primary" {
  name        = var.zone
  kind        = "Master"
  nameservers = var.nameservers

  # Rectify after every change. A zone edited through the API is not
  # automatically rectified, and an unrectified signed zone answers wrongly
  # rather than failing loudly — which is exactly the sort of thing worth
  # attaching to the lifecycle rather than remembering to run.
  lifecycle {
    action_trigger {
      events  = [after_create, after_update]
      actions = [action.powerdns_rectify_zone.primary]
    }
  }
}

resource "powerdns_record" "host" {
  zone   = powerdns_zone.primary.name
  name   = "host.${var.zone}"
  type   = "A"
  ttl    = 300
  values = ["192.0.2.50"]

  lifecycle {
    action_trigger {
      events  = [after_create, after_update]
      actions = [action.powerdns_notify_zone.primary]
    }
  }
}

action "powerdns_rectify_zone" "primary" {
  config {
    zone = var.zone
  }
}

action "powerdns_notify_zone" "primary" {
  config {
    zone = var.zone
  }
}

# An autoprimary: the server this one will accept zone creation from.
resource "powerdns_autoprimary" "peer" {
  ip         = var.autoprimary_ip
  nameserver = var.autoprimary_ns
  account    = "e2e"
}

# The secret of a key that already exists, read through the ephemeral resource.
#
# It reads a key created outside this configuration, and that is not laziness.
# An ephemeral resource is opened while the graph is being walked, before the
# resources in the same apply exist — `depends_on` does not defer it, and the
# first version of this module tried exactly that and met a 404. A secret is
# something you read because it is already there.
ephemeral "powerdns_tsigkey_secret" "readable" {
  name = var.tsig_name
}

# The secret was actually returned, asserted without storing it anywhere. A
# check block runs during apply and reports; it cannot persist what it read.
check "the_ephemeral_secret_is_real" {
  assert {
    condition     = length(ephemeral.powerdns_tsigkey_secret.readable.secret) > 0
    error_message = "the ephemeral read returned an empty secret"
  }
}
