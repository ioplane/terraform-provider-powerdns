# A zone owned elsewhere — created by another team or by pdnsutil — whose
# records this configuration adds to.
data "powerdns_zone" "shared" {
  name = "example.com."
}

resource "powerdns_record" "ours" {
  zone   = data.powerdns_zone.shared.id
  name   = "service.example.com."
  type   = "A"
  ttl    = 300
  values = ["192.0.2.10"]
}

# record_count rather than the sets themselves: a zone with thousands of them
# would put all of them in state on every refresh.
output "size" {
  value = data.powerdns_zone.shared.record_count
}
