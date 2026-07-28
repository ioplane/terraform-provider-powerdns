# True when the name already ends in a dot. The empty string is false: it is
# not a name at all.
output "already_qualified" {
  value = provider::powerdns::is_fqdn("example.com.") # true
}
