output "zone_id" {
  description = "The zone's identifier, which PowerDNS canonicalises with a trailing dot."
  value       = powerdns_zone.this.id
}

output "ptr_name" {
  description = "The reverse name, as computed by the provider function."
  value       = powerdns_record.ptr.name
}

output "record_fqdn" {
  description = "The forward name that must resolve."
  value       = powerdns_record.a.name
}
