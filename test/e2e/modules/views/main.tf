# Views and networks, which exist only on LMDB.
#
# gpgsql does not implement either — it answers 422 — so a single authoritative
# instance cannot cover this provider (ADR 0005), and an end-to-end path that
# only ever spoke to gpgsql left two of the eleven resources unreachable.

terraform {
  required_version = ">= 1.10"
  required_providers {
    powerdns = {
      source  = "registry.terraform.io/ioplane/powerdns"
      version = "0.1.1"
    }
  }
}

resource "powerdns_zone" "internal" {
  name        = var.zone
  kind        = "Native"
  nameservers = var.nameservers
}

# A view is a named set of zone variants. The membership is the resource; the
# view itself has no separate existence to create.
resource "powerdns_view_zone" "internal" {
  view = var.view
  zone = powerdns_zone.internal.name
}

# A network maps a client prefix to a view. Both arrived in PowerDNS 5.0.
resource "powerdns_network" "internal" {
  network = var.network
  view    = powerdns_view_zone.internal.view
}
