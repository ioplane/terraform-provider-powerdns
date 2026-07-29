# A zone and the records in it.
#
# Deliberately ordinary: the end-to-end fixture is testing the path from a
# Terragrunt configuration through remote state to a running PowerDNS, not the
# cleverness of the module. What it does exercise is the provider's own
# claims — an RRSet with several values, a TTL that changes in place, and a
# name computed by a provider function rather than written out.

terraform {
  required_version = ">= 1.10"
  required_providers {
    powerdns = {
      source  = "registry.terraform.io/ioplane/powerdns"
      version = "0.1.1"
    }
  }
}

resource "powerdns_zone" "this" {
  name        = var.zone
  kind        = "Native"
  nameservers = var.nameservers
}

resource "powerdns_record" "a" {
  zone   = powerdns_zone.this.name
  name   = "${var.host}.${var.zone}"
  type   = "A"
  ttl    = var.ttl
  values = var.addresses
}

resource "powerdns_record" "txt" {
  zone   = powerdns_zone.this.name
  name   = var.zone
  type   = "TXT"
  ttl    = var.ttl
  values = ["\"managed by terragrunt\""]
}

# The reverse side, with both the zone name and the record name computed by
# provider functions rather than spelled out.
resource "powerdns_zone" "reverse" {
  name        = provider::powerdns::reverse_zone_name(var.cidr)
  kind        = "Native"
  nameservers = var.nameservers
}

resource "powerdns_record" "ptr" {
  zone   = powerdns_zone.reverse.name
  name   = provider::powerdns::ptr_name(var.addresses[0])
  type   = "PTR"
  ttl    = var.ttl
  values = ["${var.host}.${var.zone}"]
}
