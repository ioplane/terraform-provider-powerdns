output "is_fqdn_check" {
  description = "The is_fqdn function, which nothing else exercises."
  value       = provider::powerdns::is_fqdn(var.zone)
}

output "dnskey" {
  description = "The public half. Public by definition — it is served in DNS."
  value       = powerdns_zone_cryptokey.ksk.dnskey
}

output "key_id" {
  description = "The key's identifier, for the ephemeral lookup to use."
  value       = powerdns_zone_cryptokey.ksk.key_id
}
