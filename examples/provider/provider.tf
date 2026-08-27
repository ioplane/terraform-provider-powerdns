# The provider talks to up to three PowerDNS products, each a separate web
# server with its own endpoint and key. Configure the ones you run; a resource
# belonging to an unconfigured product fails with a diagnostic naming the
# argument to set.

terraform {
  required_providers {
    powerdns = {
      source  = "ioplane/powerdns"
      version = "~> 0.2"
    }
  }
}

provider "powerdns" {
  # Authoritative. Also readable from PDNS_SERVER_URL and PDNS_API_KEY.
  server_url = "https://ns1.example.com:8081"
  api_key    = var.powerdns_api_key

  # Recursor. The key falls back to api_key when both run under one key.
  recursor_server_url = "https://resolver.example.com:8082"

  # dnsdist.
  dnsdist_server_url = "https://dnsdist.example.com:8083"

  # An installation with its own certificate authority.
  ca_certificate = file("${path.module}/internal-ca.pem")

  # Defaults shown; both are also read from PDNS_TIMEOUT_SECONDS and
  # PDNS_RETRY_ATTEMPTS.
  timeout_seconds = 30
  retry_attempts  = 5
}

variable "powerdns_api_key" {
  description = "PowerDNS API key. Keep it out of the configuration file."
  type        = string
  sensitive   = true
}
