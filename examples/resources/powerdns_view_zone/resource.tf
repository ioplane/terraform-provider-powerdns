# Requires the LMDB backend. On a relational one the write answers 422 and this
# provider reports the requirement rather than the status code.
#
# The resource is the membership, not the view: a view has no object of its own
# — it exists because zones point at it and disappears when the last one is
# removed.

resource "powerdns_view_zone" "internal" {
  view = "trusted"
  zone = powerdns_zone.example.id
}

resource "powerdns_network" "office" {
  network = "192.0.2.0/24"
  view    = powerdns_view_zone.internal.view
}

# Compared as a subnet, so an uncompressed IPv6 prefix is not a change.
resource "powerdns_network" "vpn" {
  network = "2001:db8::/32"
  view    = powerdns_view_zone.internal.view
}
