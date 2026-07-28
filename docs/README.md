# Documentation

| Document | Contents |
| --- | --- |
| [`../AGENTS.md`](../AGENTS.md) | **Start here.** Golden rules, architecture, workflow, gates. |
| [`methodology.md`](methodology.md) | Delivery method, roles, phase gates, Definition of Done. |
| [`contract.md`](contract.md) | What the provider promises to users, and what it does not |
| [`plan.md`](plan.md) | **Live delivery plan.** Task status, updated with the work. |

## Standards

Normative. Read the standard before changing the thing it governs.

| Standard | Governs |
| --- | --- |
| [`standards/naming-conventions.md`](standards/naming-conventions.md) | **The synthesis.** Files, branches, resources, and the chain from branch to version bump. |
| [`standards/versioning.md`](standards/versioning.md) | SemVer, what "breaking" means for a provider, dependency pinning. |
| [`standards/commits.md`](standards/commits.md) | Conventional Commits, evidence in the body. |
| [`standards/changelog.md`](standards/changelog.md) | Keep a Changelog, the closed vocabulary, release cut. |
| [`standards/go-1.26-style.md`](standards/go-1.26-style.md) | Go patterns, antipatterns, tooling. |
| [`standards/terraform-provider-best-practices.md`](standards/terraform-provider-best-practices.md) | Provider design and the per-resource Definition of Done. |
| [`standards/terragrunt-integration.md`](standards/terragrunt-integration.md) | How consumers orchestrate this provider. |
| [`standards/powerdns-api-discipline.md`](standards/powerdns-api-discipline.md) | How to establish a fact about PowerDNS. |
| [`standards/python-tooling.md`](standards/python-tooling.md) | uv, ruff, ty. |
| [`standards/verified-identifiers.md`](standards/verified-identifiers.md) | Never write a digest, SHA or version from memory. |

## Decisions

Immutable and numbered. A reversal adds a superseding record rather than
editing one.

| ADR | Decision |
| --- | --- |
| [`adr/0001`](adr/0001-methodology.md) | Gated-iterative delivery. |
| [`adr/0002`](adr/0002-one-provider-for-the-family.md) | One provider for Authoritative, Recursor and dnsdist. |
| [`adr/0003`](adr/0003-framework-protocol-6.md) | Plugin framework, protocol 6, from scratch. |
| [`adr/0004`](adr/0004-podman-oci-dev-workflow.md) | Podman, OCI, Compose, digest pinning. |
| [`adr/0005`](adr/0005-two-backend-test-matrix.md) | Acceptance on two authoritative backends. |
| [`adr/0006`](adr/0006-dnsdist-scope.md) | dnsdist in scope, sized to its API. |
| [`adr/0007`](adr/0007-taskfile-over-make.md) | Task is the command interface. |

## Related

| Repository | Holds |
| --- | --- |
| `powerdns-capability-map` | Analysis of the PowerDNS API surface and the existing provider ecosystem. Cited here, never copied. |
| `PowerDNS/pdns` | The authority on API behaviour, at the pinned tags. |
