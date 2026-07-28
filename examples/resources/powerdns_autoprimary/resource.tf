# A server permitted to create zones here by sending a NOTIFY.
#
# PowerDNS assigns no id: the pair of ip and nameserver is the key, so every
# attribute forces replacement — changing either means deleting one entry and
# creating another, which is what the API does too.

resource "powerdns_autoprimary" "hidden_primary" {
  ip         = "192.0.2.53"
  nameserver = "ns0.example.com."
  account    = "internal"
}
