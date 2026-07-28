# One kind per resource, deliberately. PowerDNS sets SOA-EDIT-API on every zone
# it creates, so a resource owning the whole collection would try to delete it
# on every apply. What this provider does not manage, it does not touch.

resource "powerdns_zone_metadata" "transfers" {
  zone   = powerdns_zone.example.id
  kind   = "ALLOW-AXFR-FROM"
  values = ["192.0.2.0/24", "2001:db8::/32"]
}

# SOA-EDIT-API and API-RECTIFY are not addressable here: PowerDNS lists them
# under a zone's metadata and answers 422 when they are read or written by
# name, because they exist as attributes of the zone. Use powerdns_zone.
