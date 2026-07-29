# Everything the provider promises never to put in state.
#
# A DNSSEC key and a TSIG secret. The point of this module is not that it
# applies — it is that the state file it produces, sitting in an S3 bucket a
# team shares, contains no key material. That is the provider's central claim
# and until now it had only ever been checked against a local state file.

terraform {
  required_version = ">= 1.10"
  required_providers {
    powerdns = {
      source  = "registry.terraform.io/ioplane/powerdns"
      version = "0.1.1"
    }
  }
}

resource "powerdns_zone" "signed" {
  name        = var.zone
  kind        = "Native"
  nameservers = var.nameservers
}

# Signing the zone. PowerDNS generates the key; the private half is readable
# only through the single-key endpoint, which this resource never calls.
resource "powerdns_zone_cryptokey" "ksk" {
  zone      = powerdns_zone.signed.name
  key_type  = "csk"
  algorithm = "ECDSAP256SHA256"
  active    = true
  published = true
}

# The secret is write-only: Terraform sends it to the provider and stores it in
# neither state nor the plan file. Leaving it unset has PowerDNS generate one,
# which is then unreadable through the provider at all.
resource "powerdns_tsigkey" "transfer" {
  name       = var.tsig_name
  algorithm  = "hmac-sha256"
  secret_wo  = var.tsig_secret
}

# A record whose value is built by the pure functions, so the module exercises
# the three that the lifecycle module does not.
resource "powerdns_record" "soa_probe" {
  zone   = powerdns_zone.signed.name
  name   = provider::powerdns::fqdn("probe.${var.zone}")
  type   = "TXT"
  ttl    = 300
  values = ["\"${provider::powerdns::soa_serial("2026-07-29", 1)}\""]
}
