# The name a PTR record sits at, for either address family.
resource "powerdns_record" "ptr" {
  zone   = provider::powerdns::reverse_zone_name("192.0.2.0/24")
  name   = provider::powerdns::ptr_name("192.0.2.1")
  type   = "PTR"
  ttl    = 3600
  values = ["www.example.com."]
}
