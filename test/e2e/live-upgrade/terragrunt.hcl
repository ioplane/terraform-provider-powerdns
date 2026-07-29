# The upgrade unit: one state file, applied by two builds of the provider.
#
# Its own state key, like every other unit. Sharing one would mean a scenario
# elsewhere could leave this unit's resources in a state written by the wrong
# version, which is the single thing this unit is measuring.
#
# The provider requirement is generated rather than written, because it is the
# variable under test. `get_env` with a default keeps a bare `terragrunt apply`
# in this directory working: it applies the released version, which is the
# state the upgrade starts from.

terraform {
  source = "git::https://127.0.0.1:19300/e2e/dns-modules.git//modules/upgrade?ref=main"
}

remote_state {
  backend = "s3"

  generate = {
    path      = "backend.tf"
    if_exists = "overwrite_terragrunt"
  }

  config = {
    bucket = "e2e-state"
    key    = "dns-upgrade/terraform.tfstate"
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

# The whole point of the unit. The version comes from the environment so one
# state file can be applied by one build and then by another, which is what an
# upgrade is.
generate "versions" {
  path      = "versions.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<-EOT
    terraform {
      required_providers {
        powerdns = {
          source  = "registry.terraform.io/ioplane/powerdns"
          version = "${get_env("E2E_PROVIDER_VERSION", "0.1.1")}"
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
  zone = "upgraded.e2e.example."
  ttl  = 3600
}
