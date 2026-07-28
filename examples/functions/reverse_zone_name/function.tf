# The in-addr.arpa or ip6.arpa zone that holds a subnet's PTR records.
#
# Errors for a prefix off an octet boundary (IPv4) or a nibble boundary (IPv6):
# a /25 spans two /24 reverse zones and a /20 spans sixteen, so there is no
# single name to return.
resource "powerdns_zone" "reverse" {
  name        = provider::powerdns::reverse_zone_name("192.0.2.0/24")
  kind        = "Native"
  nameservers = ["ns1.example.com."]
}

output "ipv6_reverse" {
  value = provider::powerdns::reverse_zone_name("2001:db8::/32")
  # "8.b.d.0.1.0.0.2.ip6.arpa."
}
