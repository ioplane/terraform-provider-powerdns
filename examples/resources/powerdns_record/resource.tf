# A record resource owns an RRSet — every record sharing a name and a type —
# because PowerDNS has no per-record identity. Two resources managing the same
# name and type would silently overwrite each other.

resource "powerdns_record" "www" {
  zone   = powerdns_zone.example.id
  name   = "www.example.com."
  type   = "A"
  ttl    = 300
  values = ["192.0.2.1", "192.0.2.2", "192.0.2.3"]
}

# Address values are compared numerically, and DNS targets such as CNAME are
# compared as names. Remaining content is exact, so a TXT record's quoting and
# whitespace are significant. Value order is ignored for every type.
resource "powerdns_record" "spf" {
  zone   = powerdns_zone.example.id
  name   = "example.com."
  type   = "TXT"
  ttl    = 3600
  values = ["\"v=spf1 mx -all\""]
}

resource "powerdns_record" "mail" {
  zone   = powerdns_zone.example.id
  name   = "example.com."
  type   = "MX"
  ttl    = 3600
  values = ["10 mail1.example.com.", "20 mail2.example.com."]
}

# Kept in the zone without being served. The flag survives a round trip, so a
# record turned off stays off.
resource "powerdns_record" "staged" {
  zone     = powerdns_zone.example.id
  name     = "new.example.com."
  type     = "A"
  ttl      = 60
  values   = ["192.0.2.9"]
  disabled = true
}
