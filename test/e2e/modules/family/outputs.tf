output "forward_kind" {
  description = "Read back through the data source, not from the resource."
  value       = data.powerdns_recursor_zone.readback.kind
}

output "dnsdist_version" {
  description = "Proves the dnsdist client reached a real server."
  value       = data.powerdns_dnsdist_server.self.version
}
