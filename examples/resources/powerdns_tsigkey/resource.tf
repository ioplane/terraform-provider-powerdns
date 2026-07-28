# Leaving secret_wo unset asks PowerDNS to generate a secret, which then cannot
# be read back through this provider at all.
resource "powerdns_tsigkey" "transfer" {
  name      = "transfer"
  algorithm = "hmac-sha256"
}

# Importing a secret you already hold. secret_wo is write-only: Terraform sends
# it to the provider and stores it in neither the state file nor the plan file.
# Requires Terraform 1.11 or later.
resource "powerdns_tsigkey" "shared" {
  name      = "shared-with-secondary"
  algorithm = "hmac-sha256"
  secret_wo = var.tsig_secret
}

variable "tsig_secret" {
  description = "Base64 HMAC secret. Never stored by this provider."
  type        = string
  sensitive   = true
}

# The key's id is the canonical name, with a trailing dot.
resource "powerdns_zone" "secondary" {
  name                = "example.net."
  kind                = "Slave"
  masters             = ["192.0.2.53"]
  master_tsig_key_ids = [powerdns_tsigkey.transfer.id]
}
