# A zone is created with its nameservers, because PowerDNS generates the SOA
# and NS records at that moment and will not do so later. Changing them
# afterwards replaces the zone; manage NS records with powerdns_record instead.

resource "powerdns_zone" "example" {
  name        = "example.com."
  kind        = "Native"
  nameservers = ["ns1.example.com.", "ns2.example.com."]
}

# A secondary. Masters are compared by address value and ignoring order, so an
# IPv6 address written uncompressed does not produce a permanent diff.
resource "powerdns_zone" "secondary" {
  name    = "example.net."
  kind    = "Slave"
  masters = ["192.0.2.53", "2001:db8::53"]
}

# Signed, with PowerDNS generating a CSK itself. Manage keys here or with
# powerdns_zone_cryptokey — not both.
resource "powerdns_zone" "signed" {
  name        = "secure.example."
  kind        = "Native"
  nameservers = ["ns1.example.com."]
  dnssec      = true
  nsec3param  = "1 0 0 ab"
}
