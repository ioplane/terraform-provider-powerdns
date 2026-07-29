# A zone and a record, and deliberately no provider version.
#
# Every other module in this fixture pins `version = "0.1.1"` in its own
# `required_providers`, which is right for a module a consumer depends on and
# wrong here: this module exists to be applied by one version of the provider
# and then re-applied by another. The requirement is left to the root
# configuration, which the unit generates.
#
# What it manages is ordinary on purpose. The subject of the scenario is the
# state file, not the resources: an upgrade has to read what the previous
# version wrote, and the only way to find out is to write some.

terraform {
  required_version = ">= 1.10"
}

resource "powerdns_zone" "this" {
  name        = var.zone
  kind        = "Native"
  nameservers = ["ns1.${var.zone}"]
}

# Several values in one RRSet, a TTL, and a metadata resource: three different
# shapes in state, so an upgrade that mishandles one of them shows up.
resource "powerdns_record" "a" {
  zone   = powerdns_zone.this.name
  name   = "www.${var.zone}"
  type   = "A"
  ttl    = var.ttl
  values = ["198.51.100.20", "198.51.100.21"]
}

resource "powerdns_zone_metadata" "transfers" {
  zone   = powerdns_zone.this.name
  kind   = "ALLOW-AXFR-FROM"
  values = ["198.51.100.0/24"]
}
