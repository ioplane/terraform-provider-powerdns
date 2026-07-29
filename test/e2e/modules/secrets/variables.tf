variable "zone" {
  description = "The zone to sign."
  type        = string
}

variable "nameservers" {
  description = "Create-only; PowerDNS does not report them back."
  type        = list(string)
}

variable "tsig_name" {
  description = "The TSIG key's name. Changing it replaces the key."
  type        = string
}

variable "tsig_secret" {
  description = "Base64 secret. Write-only: it reaches the provider and stops there."
  type        = string
  sensitive   = true
  ephemeral   = true
}
