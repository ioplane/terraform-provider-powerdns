output "zone_name" {
  description = "The zone as the server normalised it."
  value       = powerdns_zone.this.name
}

output "record_values" {
  description = "The RRSet's values, read back from state."
  value       = powerdns_record.a.values
}
