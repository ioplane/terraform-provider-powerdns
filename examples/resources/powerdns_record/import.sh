# <zone>/<name>/<type>
terraform import powerdns_record.www 'example.com./www.example.com./A'

# Or by identity, which carries the three parts as attributes and needs no
# parsing:
#
#   import {
#     to = powerdns_record.www
#     identity = {
#       zone_name   = "example.com."
#       record_name = "www.example.com."
#       record_type = "A"
#     }
#   }
