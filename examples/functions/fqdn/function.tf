# Appends the trailing dot PowerDNS stores, so a variable can be written either
# way without the configuration caring.
output "qualified" {
  value = provider::powerdns::fqdn("example.com") # "example.com."
}
