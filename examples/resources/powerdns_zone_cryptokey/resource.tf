# No private key reaches state. This resource reconciles against the collection
# endpoint, which omits key material, and never stores what the create response
# carried — so a generated private key cannot be read back through it. Use the
# powerdns_cryptokey_material ephemeral resource where the key genuinely has to
# leave PowerDNS.

resource "powerdns_zone_cryptokey" "ksk" {
  zone      = powerdns_zone.example.id
  key_type  = "ksk"
  algorithm = "ECDSAP256SHA256"
}

resource "powerdns_zone_cryptokey" "zsk" {
  zone      = powerdns_zone.example.id
  key_type  = "zsk"
  algorithm = "ECDSAP256SHA256"
}

# The DS records to publish in the parent zone.
output "delegation_signer" {
  value = powerdns_zone_cryptokey.ksk.ds
}

# Staging a rollover: active but not yet published.
resource "powerdns_zone_cryptokey" "next" {
  zone      = powerdns_zone.example.id
  key_type  = "ksk"
  algorithm = "ECDSAP256SHA256"
  published = false
}
