# By zone name. A trailing dot is added if absent.
terraform import powerdns_zone.example example.com.

# An import block carries the identity instead. Note that a zone created with
# nameservers cannot round-trip this way: the attribute is create-only and
# never read back, so the difference forces a replacement.
#
#   import {
#     to = powerdns_zone.example
#     identity = {
#       zone_name = "example.com."
#     }
#   }
