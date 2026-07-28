# A forwarder somebody else configured, whose target this configuration needs
# to know.
data "powerdns_recursor_zone" "corp" {
  name = "corp.example."
}

output "forwarders" {
  # Reported with the port the Recursor defaulted: 192.0.2.53 reads back as
  # 192.0.2.53:53.
  value = data.powerdns_recursor_zone.corp.servers
}
