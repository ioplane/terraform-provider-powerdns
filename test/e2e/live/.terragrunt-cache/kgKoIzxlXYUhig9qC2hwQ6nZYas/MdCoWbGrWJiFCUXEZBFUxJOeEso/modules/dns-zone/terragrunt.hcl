# The consumer's side of the provider.
#
# Terragrunt rather than plain Terraform because the state backend, the module
# source and the provider inputs are exactly the three things a real
# configuration keeps out of the module — and the provider has never been
# driven through any of them.

terraform {
  # Fetched over the git protocol, not from a path. A module that resolves
  # because it happens to sit in the same directory tree proves nothing about
  # how anyone will actually consume this.
  source = "git::git://127.0.0.1:19418/dns-modules.git//modules/dns-zone?ref=main"
}

remote_state {
  backend = "s3"

  generate = {
    path      = "backend.tf"
    if_exists = "overwrite_terragrunt"
  }

  config = {
    bucket = "e2e-state"
    key    = "dns/terraform.tfstate"
    region = "us-east-1"

    endpoints = {
      s3 = "http://127.0.0.1:19000"
    }

    access_key = "e2eaccesskey"
    secret_key = "e2esecretkey"

    # MinIO is not AWS: there is no STS to ask who we are, no bucket in a real
    # region, and the bucket is addressed by path rather than by hostname.
    skip_credentials_validation = true
    skip_metadata_api_check     = true
    skip_region_validation      = true
    skip_requesting_account_id  = true
    use_path_style              = true

    # S3-native locking, which Terraform gained in 1.10. The alternative is a
    # DynamoDB table, and standing one up to lock a test would be a second
    # service existing only to be a lock.
    use_lockfile = true
  }
}

generate "provider" {
  path      = "provider.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<-EOT
    provider "powerdns" {
      server_url = "http://127.0.0.1:18081"
      api_key    = "labapikey"
    }
  EOT
}

inputs = {
  zone        = "e2e.example."
  host        = "www"
  cidr        = "198.51.100.0/24"
  addresses   = ["198.51.100.10", "198.51.100.11"]
  ttl         = 3600
  nameservers = ["ns1.e2e.example.", "ns2.e2e.example."]
}
