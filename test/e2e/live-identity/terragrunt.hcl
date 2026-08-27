# Adoption by identity, in its own unit with its own state.
#
# Pinned to Terraform. Resource identity in an `import` block is a Terraform
# feature and OpenTofu has neither the identity plumbing nor the block syntax,
# so leaving the engine to Terragrunt's default — which is tofu — would make
# this unit fail for a reason that has nothing to do with the provider.
terraform_binary = "/usr/local/bin/terraform"

terraform {
  source = "git::https://127.0.0.1:19300/e2e/dns-modules.git//modules/identity?ref=main"
}

remote_state {
  backend = "s3"

  generate = {
    path      = "backend.tf"
    if_exists = "overwrite_terragrunt"
  }

  config = {
    bucket = "e2e-state"
    key    = "dns-identity/terraform.tfstate"
    region = "us-east-1"

    endpoints = {
      s3 = "http://127.0.0.1:19000"
    }

    access_key = "e2eaccesskey"
    secret_key = "e2esecretkey"

    skip_credentials_validation = true
    skip_metadata_api_check     = true
    skip_region_validation      = true
    skip_requesting_account_id  = true
    use_path_style              = true
    use_lockfile                = true
  }
}

generate "versions" {
  path      = "versions.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<-EOT
    terraform {
      required_providers {
        powerdns = {
          source  = "registry.terraform.io/ioplane/powerdns"
          version = "0.2.0"
        }
      }
    }
  EOT
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
  # The zone this unit does not manage. Created and removed by the scenario, so
  # the record it adopts genuinely predates Terraform knowing about it.
  zone      = "adopted.e2e.example."
  addresses = ["198.51.100.30", "198.51.100.31"]
  ttl       = 1800
}
