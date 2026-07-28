# There is no endpoint for a single RRSet, so this reads the zone and selects
# from it. On a large zone that is not cheap; prefer one powerdns_zone read and
# local lookups over this in a loop.
data "powerdns_record" "mail" {
  zone = "example.com."
  name = "example.com."
  type = "MX"
}

output "mail_exchangers" {
  value = data.powerdns_record.mail.values
}
