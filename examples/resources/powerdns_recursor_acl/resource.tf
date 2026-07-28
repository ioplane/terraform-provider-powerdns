# One of the Recursor's two writable settings. ws-recursor.cc registers
# allow-from and allow-notify-from as separate handlers rather than one
# parameterised route, so every other name answers 404 on read as well as
# write — and this resource rejects any other name before sending.
#
# Destroying it leaves the setting as it is and warns. There is no unset state
# for an ACL, and writing an empty list would refuse every client.

resource "powerdns_recursor_acl" "clients" {
  setting  = "allow-from"
  netmasks = ["10.0.0.0/8", "192.168.0.0/16", "2001:db8::/32"]
}

resource "powerdns_recursor_acl" "notifies" {
  setting  = "allow-notify-from"
  netmasks = ["192.0.2.53/32"]
}
