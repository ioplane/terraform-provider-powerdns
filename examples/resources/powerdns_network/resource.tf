# Requires the LMDB backend, like powerdns_view_zone.
#
# There is no delete: assigning the empty view removes the mapping and leaves
# the subnet unassigned, which is what destroying this resource does.

resource "powerdns_network" "office" {
  network = "192.0.2.0/24"
  view    = "trusted"
}
