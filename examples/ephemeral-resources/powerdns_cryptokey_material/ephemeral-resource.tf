# The only place this provider returns a DNSSEC private key. Terraform fetches
# the value during the operation and discards it: nothing reaches the state
# file or the plan file.
#
# An ephemeral value can only be consumed by another ephemeral or write-only
# attribute, which is what stops something downstream persisting it.
ephemeral "powerdns_cryptokey_material" "ksk" {
  zone   = powerdns_zone.example.id
  key_id = powerdns_zone_cryptokey.ksk.key_id
}

# Handing it to a signing appliance, a secret manager, or anything else that
# takes a write-only argument.
resource "vault_kv_secret_v2" "dnssec" {
  mount                = "secret"
  name                 = "dnssec/example.com"
  data_json_wo         = jsonencode({ private_key = ephemeral.powerdns_cryptokey_material.ksk.private_key })
  data_json_wo_version = 1
}
