# <zone>/<key_id>. The id is a number from a server-global counter, visible in
# GET /api/v1/servers/localhost/zones/<zone>/cryptokeys.
terraform import powerdns_zone_cryptokey.ksk 'example.com./3'
