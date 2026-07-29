output "record_id" {
  description = "The composite id, so a test can see what identity resolved to."
  value       = powerdns_record.adopted.id
}
