# Reads a TSIG secret without storing it. The usual reason is configuring a
# secondary server, which needs the same secret the primary authenticates with.
ephemeral "powerdns_tsigkey_secret" "transfer" {
  name = powerdns_tsigkey.transfer.id
}

# Copying it to a second key, without either end storing the value.
resource "powerdns_tsigkey" "secondary_copy" {
  name      = "transfer-secondary"
  algorithm = "hmac-sha256"
  secret_wo = ephemeral.powerdns_tsigkey_secret.transfer.secret
}
