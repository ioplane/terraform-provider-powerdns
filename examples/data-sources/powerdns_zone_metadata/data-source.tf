# An unset kind is not an error: PowerDNS answers with an empty list rather
# than a 404, so a configuration can branch on it.
data "powerdns_zone_metadata" "transfers" {
  zone = "example.com."
  kind = "ALLOW-AXFR-FROM"
}

output "transfers_restricted" {
  value = length(data.powerdns_zone_metadata.transfers.values) > 0
}
