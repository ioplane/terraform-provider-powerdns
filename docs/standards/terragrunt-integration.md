<!-- markdownlint-disable MD013 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=Terragrunt&subtitle=How+consumers+orchestrate+this+provider&logo=terraform&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="Terragrunt" src="https://shieldcn.dev/header/graph.svg?title=Terragrunt&subtitle=How+consumers+orchestrate+this+provider&logo=terraform&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>
<!-- markdownlint-enable MD013 -->

<div align="center">

[![status normative](https://shieldcn.dev/badge/status-normative-cf222e.svg?variant=secondary)](../README.md)
![terragrunt 1.1.1](https://shieldcn.dev/badge/terragrunt-1.1.1-0969da.svg?variant=secondary)
![enforced see the table](https://shieldcn.dev/badge/enforced-see_the_table-3fb950.svg?variant=secondary)

</div>

# Terragrunt integration

How consumers drive this provider with
[Terragrunt](https://terragrunt.gruntwork.io/), pinned **v1.1.1** in the dev
container.

## Why Terragrunt applies here

DNS is naturally hierarchical and multi-tenanted: a zone per environment, a
delegation per site, the same record set repeated across regions with different
values. That is the case Terragrunt's **unit** and **stack** model is for — one
unit per zone or per environment, composed into a stack, with the repetition in
`terragrunt.hcl` rather than copied HCL.

It matters more here than for most providers because of ordering. A DNSSEC key
must exist before the zone is published; a TSIG key must exist before the
secondary that references it. Those are unit-to-unit dependencies, not
intra-apply ones.

## Version baseline

- **Terragrunt 1.0** (2026-03-30) froze the CLI and HCL contract: `run`, `exec`,
  `find`, `list`; the unified `--filter` system; structured run reports; and
  automatic provider caching with OpenTofu 1.10+.
- **Terragrunt 1.1** (July 2026) adds stack dependencies declared in
  `terragrunt.stack.hcl`, the redesigned catalog as default, and read-based
  change detection.

Use `unit` and `stack` terminology, not the legacy "module", and the `run`
subcommand rather than a bare `terragrunt apply`.

## Recommended layout

```text
live/
├── root.hcl                       # remote_state + provider generate + common inputs
├── prod/
│   ├── zones/
│   │   ├── example-com/
│   │   │   └── terragrunt.hcl     # unit → one powerdns_zone + its records
│   │   └── example-net/
│   │       └── terragrunt.hcl
│   └── keys/
│       └── terragrunt.hcl         # unit → TSIG keys referenced by the zones
└── _catalog/                      # reusable unit templates
```

## Provider configuration generation

Generate the provider block so credentials come from the environment and never
from committed HCL:

```hcl
# root.hcl
generate "provider" {
  path      = "provider.tf"
  if_exists = "overwrite"
  contents  = <<-EOF
    terraform {
      required_providers {
        powerdns = {
          source  = "ioplane/powerdns"
          version = "~> 0.2"
        }
      }
    }
    provider "powerdns" {
      server_url          = "${get_env("PDNS_SERVER_URL")}"
      recursor_server_url = "${get_env("PDNS_RECURSOR_SERVER_URL", "")}"
      api_key             = "${get_env("PDNS_API_KEY")}"
    }
  EOF
}
```

- Pin the provider with a pessimistic constraint. While `0.x`, pin the minor
  (`~> 0.2`) — minors may break (see [`versioning.md`](versioning.md)).
- Commit `.terraform.lock.hcl` for reproducible provider selection.

## Dependencies between units

Prefer **stack dependencies** (1.1) over threading `dependency` output paths by
hand. Keep `mock_outputs` so `plan` works against a not-yet-applied dependency.

The dependency edges worth declaring for DNS:

| Downstream unit | Depends on | Because |
| --- | --- | --- |
| zone records | the zone | the zone must exist before an RRset is patched into it |
| secondary zone | TSIG keys | `slave_tsig_key_ids` references a key by id |
| signed zone | cryptokeys | publishing before the key exists yields an unsigned interval |
| recursor forward zone | the authoritative zone | forwarding to a zone that does not answer is a broken delegation |

## The backend caveat

A stack that uses `powerdns_view_zone_association` or `powerdns_network` only
works against an LMDB-backed installation
(capability map `CM-03` §5). If a stack spans several PowerDNS installations
with different backends, those units cannot be uniform — express the difference
in the stack rather than discovering it at apply time.

## CI and change detection

Use read-based change detection (`terragrunt find --filter`, the run queue) so
a CI run only touches units whose inputs actually changed. Do not auto-approve
production DNS changes from CI: a bad apply to a zone is visible to the
internet within the TTL, and `terraform destroy` on a zone is not recoverable
from state.

## Examples

Worked HCL and Terragrunt examples live under `examples/` and are checked by the
`terraform` CI job (`terraform fmt -check`).
