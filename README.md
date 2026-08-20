<!-- markdownlint-disable MD013 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=terraform-provider-powerdns&subtitle=Authoritative+%2B+Recursor+%2B+dnsdist&logo=terraform&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="terraform-provider-powerdns" src="https://shieldcn.dev/header/graph.svg?title=terraform-provider-powerdns&subtitle=Authoritative+%2B+Recursor+%2B+dnsdist&logo=terraform&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>
<!-- markdownlint-enable MD013 -->

<div align="center">

[![ci](https://shieldcn.dev/github/ci/ioplane/terraform-provider-powerdns.svg?workflow=ci.yml&variant=secondary)](https://github.com/ioplane/terraform-provider-powerdns/actions/workflows/ci.yml)
[![license](https://shieldcn.dev/github/license/ioplane/terraform-provider-powerdns.svg?variant=secondary)](LICENSE)
[![last-commit](https://shieldcn.dev/github/last-commit/ioplane/terraform-provider-powerdns.svg?variant=secondary)](https://github.com/ioplane/terraform-provider-powerdns/commits/main)
[![contributors](https://shieldcn.dev/github/contributors/ioplane/terraform-provider-powerdns.svg?variant=secondary)](https://github.com/ioplane/terraform-provider-powerdns/graphs/contributors)
[![issues](https://shieldcn.dev/github/issues/ioplane/terraform-provider-powerdns.svg?variant=secondary)](https://github.com/ioplane/terraform-provider-powerdns/issues)
<br>
[![go 1.27.0](https://shieldcn.dev/badge/go-1.27.0-00ADD8.svg?variant=branded&logo=go&logoColor=white)](https://go.dev/doc/go1.27)
[![framework v1.19](https://shieldcn.dev/badge/framework-v1.19-7B42BC.svg?variant=secondary&logo=terraform&logoColor=white)](https://developer.hashicorp.com/terraform/plugin/framework)
[![terraform 1.11+](https://shieldcn.dev/badge/terraform-1.11+-7B42BC.svg?variant=secondary&logo=terraform&logoColor=white)](https://developer.hashicorp.com/terraform)
[![protocol 6.0](https://shieldcn.dev/badge/protocol-6.0-7B42BC.svg?variant=secondary)](docs/adr/0003-framework-protocol-6.md)
<br>
[![commits conventional](https://shieldcn.dev/badge/commits-conventional-FE5196.svg?variant=secondary&logo=conventionalcommits&logoColor=white)](https://www.conventionalcommits.org/en/v1.0.0/)
[![semver 2.0.0](https://shieldcn.dev/badge/semver-2.0.0-3fb950.svg?variant=secondary)](https://semver.org/spec/v2.0.0.html)
[![changelog keep_a_changelog](https://shieldcn.dev/badge/changelog-keep_a_changelog-3fb950.svg?variant=secondary)](https://keepachangelog.com/en/1.1.0/)
[![PRs welcome](https://shieldcn.dev/badge/PRs-welcome-3fb950.svg?variant=secondary)](CONTRIBUTING.md)

</div>

# terraform-provider-powerdns

A Terraform provider for the **PowerDNS family** — Authoritative Server,
Recursor and dnsdist — on `terraform-plugin-framework`, protocol 6.

> **Status: `v0.1.0` released.** Every operation of all three APIs is covered
> and the acceptance suite runs against a real five-service lab on every push
> to `main`. Registry publication is pending; until then, install from the
> [GitHub release](https://github.com/ioplane/terraform-provider-powerdns/releases/tag/v0.1.0)
> or build from source. [`docs/plan.md`](docs/plan.md) records exactly where
> everything stands.

## Using it

```terraform
terraform {
  required_providers {
    powerdns = {
      source  = "ioplane/powerdns"
      version = "~> 0.1"
    }
  }
}

provider "powerdns" {
  # Configure only the products you use; each is optional.
  server_url = "https://ns1.example.com:8081" # or PDNS_SERVER_URL
  api_key    = var.pdns_api_key               # or PDNS_API_KEY
}
```

Pre-1.0, a MINOR release may carry what MAJOR will carry afterwards — pin
accordingly. See [`docs/standards/versioning.md`](docs/standards/versioning.md).

Every resource, data source, function and ephemeral resource has a worked
example under [`examples/`](examples/), and the generated reference is in
[`docs/`](docs/).

## Why one provider for three products

They share one credential model — `X-API-Key` on separate web servers — and are
deployed together. Each gets its own endpoint and its own key, and each product
prefixes its resources except Authoritative, which is unprefixed:

```hcl
provider "powerdns" {
  server_url          = "https://auth.example.com:8081"
  api_key             = var.auth_key
  recursor_server_url = "https://rec.example.com:8082"
  recursor_api_key    = var.rec_key
  dnsdist_server_url  = "https://ddist.example.com:8083"
  dnsdist_api_key     = var.ddist_key
}

resource "powerdns_zone" "example" { ... }
resource "powerdns_recursor_zone" "forward" { ... }
resource "powerdns_dnsdist_acl" "clients" { ... }
```

Configure only the products you use.

## Surface, as delivered

| Product | API operations | Resources | Data sources | Actions |
| --- | ---: | ---: | ---: | ---: |
| Authoritative 5.1.3 | 42 | 9 | 9 | 4 |
| Recursor 5.4.4 | 16 | 2 | 3 | 1 |
| dnsdist 2.1.0 | 10 | 1 | 4 | 1 |

Plus five provider functions and two ephemeral resources.

**dnsdist is thin because its API is.** Two of its ten operations write:
`PUT /config/allow-from` and `DELETE /api/v1/cache`. Rules, pools and
downstream servers are Lua or YAML and are not reachable over HTTP. See
[ADR 0006](docs/adr/0006-dnsdist-scope.md).

## Tested against

| Product | Versions the acceptance suite runs against |
| --- | --- |
| Authoritative | 5.1.3 and 5.0.6 |
| Recursor | 5.4.4 |
| dnsdist | 2.1.0 |

The authoritative branch is a matrix, not a single pin: the same 203
assertions run on 5.1.3 and on 5.0.6, on every pull request, and both must
pass. Nothing else in the
fixture moves between the two runs, so a difference is attributable to the
branch. The API facts in this repository are cited against `auth-5.1.3` —
that is the tag they were read from, and it is a different claim from the
range the provider is exercised over.

Run either yourself: `task lab:up AUTH=5.0 && task testacc`.

## Surface

```mermaid
flowchart LR
  subgraph AUTH["Authoritative · 42 ops"]
    A1["9 resources"]
    A2["9 data sources"]
    A3["4 actions"]
  end
  subgraph REC["Recursor · 16 ops"]
    R1["2 resources"]
    R2["3 data sources"]
    R3["1 action"]
  end
  subgraph DD["dnsdist · 10 ops"]
    D1["1 resource"]
    D2["4 data sources"]
    D3["1 action"]
  end

  classDef full fill:#1a7f37,stroke:#116329,color:#fff
  classDef part fill:#9a6700,stroke:#7d4e00,color:#fff
  class A1,A2,A3,R1,R2,R3 full
  class D1,D2,D3 part
```

Plus five provider functions and two ephemeral resources. dnsdist is amber
because its API is: two of its ten operations write.

## What this provider does differently

**It refuses at plan what the server would refuse at apply.** PowerDNS has
several conditions where an operation is impossible on a given installation —
views need the LMDB backend, recursor writes need `api_dir`, dnsdist writes
need `setAPIWritable` — and each surfaces as a bare `4xx`. The client
classifies them once and every resource reports the same actionable diagnostic.

**Secrets do not reach state.** DNSSEC private keys and TSIG secrets are
ephemeral resources or write-only attributes. Terraform state is not encrypted,
and `Sensitive` only redacts console output.

**Server-side normalisation is compared semantically.** PowerDNS rewrites what
it stores — `native` becomes `Native`, `:0000:` becomes `:0:` — and string
comparison makes such configurations permanently dirty.

## Development

```sh
task up        # dev container — no host toolchain needed
task lab:up    # five-service lab
task all       # the pre-merge gate
```

Start at [`AGENTS.md`](AGENTS.md); the documentation index is
[`docs/README.md`](docs/README.md).

## Licence

Apache-2.0.
