<!-- markdownlint-disable MD013 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=Provider+practices&subtitle=Design+and+Definition+of+Done&logo=terraform&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="Provider practices" src="https://shieldcn.dev/header/graph.svg?title=Provider+practices&subtitle=Design+and+Definition+of+Done&logo=terraform&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>
<!-- markdownlint-enable MD013 -->

<div align="center">

[![status normative](https://shieldcn.dev/badge/status-normative-cf222e.svg?variant=secondary)](../README.md)
![framework v1.19](https://shieldcn.dev/badge/framework-v1.19-0969da.svg?variant=secondary)
![enforced see the table](https://shieldcn.dev/badge/enforced-see_the_table-3fb950.svg?variant=secondary)

</div>

# Terraform provider best practices

The rules this provider is held to, distilled from HashiCorp's
[provider design principles](https://developer.hashicorp.com/terraform/plugin/best-practices/hashicorp-provider-design-principles),
the [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
documentation, and this project's target — PowerDNS over HTTP. Grounded against
the framework through the `context7` MCP.

## 1. Design principles

1. **One problem domain.** This provider manages the PowerDNS family —
   Authoritative Server, Recursor and dnsdist. They are one domain because they
   share one credential model, `X-API-Key` on separate web servers, and are
   deployed together. Each product's surface is sized to what its API permits
   (ADR 0006).
2. **One API object per resource.** `powerdns_zone` is a zone,
   `powerdns_tsigkey` is a TSIG key. Orchestration across objects belongs in
   Terraform modules and Terragrunt, not in a mega-resource.
3. **The schema mirrors the API.** Attribute names track PowerDNS field names.
   Simplification belongs in modules, not in provider magic. Where PowerDNS is
   itself inconsistent, the API name still wins.
4. **Every resource is importable.** `ResourceWithImportState` is mandatory —
   an existing PowerDNS installation must be adoptable with `terraform import`.
5. **State continuity and SemVer.** State shape is a contract; a change ships a
   state upgrader. Versioning per [`versioning.md`](versioning.md).
6. **Provider functions are pure and offline.** Anything needing the network is
   a data source, not a function.
7. **Ephemeral resources and write-only attributes for sensitive data.**
   Terraform state is not encrypted. A DNSSEC private key or a TSIG secret that
   does not need drift detection must not be written to state at all.

## 2. Framework conventions

- Use **`terraform-plugin-framework`** v1.19+, protocol 6. Not SDKv2.
- Implement the optional interfaces deliberately:
  - `ResourceWithConfigure` — receive the client bundle from the provider.
  - `ResourceWithImportState` — always.
  - `ResourceWithModifyPlan` — for computed-on-change and `RequiresReplace`.
  - `ResourceWithConfigValidators` / `ValidateConfig` — cross-attribute rules.
  - `ResourceWithUpgradeState` — whenever the stored state shape changes.
- Prefer declarative validators from `terraform-plugin-framework-validators`
  over hand-rolled checks.
- Mark secrets `Sensitive: true`; prefer **write-only attributes**
  (Terraform 1.11+) where drift detection is not needed.
- Every attribute carries a `MarkdownDescription`. Documentation is generated
  from the schema by `tfplugindocs` — the schema is the source of truth.
- A re-apply of an unchanged configuration produces an empty plan. Idempotency
  is asserted, not assumed: `plancheck.ExpectEmptyPlan` after apply.

## 3. Validate what the server will reject

This is the rule this project exists to learn, and it is not in HashiCorp's
list, and it is the rule this provider exists to apply. Existing PowerDNS
providers accept configuration the *server* then rejects, so the failure
surfaces at `apply` instead of `plan` — see the sibling `powerdns-capability-map`,
`CM-04` §5, for the measurement.

**A provider knows things about its API that the user does not, and the plan is
where that knowledge belongs.**

| Situation | Where it must fail |
| --- | --- |
| Value outside the API's accepted set (recursor config name) | `plan`, via `ValidateFunc` |
| Malformed input the provider itself parses (IPv6 in `masters`) | `plan`, via a validator |
| Capability absent from the server's backend (views on gpgsql) | `apply` — but the diagnostic must name the requirement |
| Server-side configuration missing (recursor `api_dir`) | `apply` — but the diagnostic must name the setting |

The last two cannot be known at plan time without a network call, which the
plan phase should not make. There the obligation is different: **the error
message must state the requirement**, not relay a bare `422`.

## 4. Error, retry, and logging behaviour

- **Diagnostics, not panics.** `resp.Diagnostics.AddError` /
  `AddAttributeError` with an actionable detail.
- **Status before body.** Examine `resp.StatusCode` before decoding. Decoding
  an error response into a success type is how a server failure becomes a
  silent empty result (defect D-08).
- **Retry policy.** Transient transport failures and `5xx` retry with
  exponential backoff, five attempts, 1–16 s. `4xx` fails fast — `404`, `409`
  and `422` are semantic answers, not flakes. `401` and `403` fail fast with a
  hint naming the API-key argument.
- **Translate the server's vocabulary.** A `422` from `/views` means "this
  backend has no views", not "unprocessable entity". The provider knows the
  mapping; the user should not have to.
- **Logging** through `tflog` with structured fields. Never log an API key, a
  TSIG secret or a DNSSEC private key.

## 5. Testing

- **Acceptance tests** (`terraform-plugin-testing`, `TF_ACC=1`) exercise real
  plan, apply, refresh and destroy against the lab. Every resource has at least
  one.
- **The backend matrix is mandatory.** Resources touching views or networks run
  against the LMDB instance; everything else runs against PostgreSQL. A single
  backend cannot cover this provider.
- Use plan checks and state checks (`statecheck`, `knownvalue`) rather than
  brittle string assertions.
- Always include an `ImportState` step with `ImportStateVerify: true`.
- Unit tests cover schema validation, plan modifiers and payload marshalling
  without a network.

## 6. Definition of done, per resource

1. `Schema` with a `MarkdownDescription` on every attribute; secrets
   `Sensitive`, and write-only where they need not persist.
2. `Create`, `Read`, `Update`, `Delete` implemented against the client.
3. `ImportState` implemented.
4. Plan is idempotent — same configuration twice yields an empty plan.
5. Validators for what the API will reject; diagnostics that name the
   requirement for what it cannot know locally (§3).
6. `UpgradeState` if the state shape changed.
7. At least five unit edge cases: empty, maximum-length identifier, idempotent
   re-create, conflict, transport error.
8. At least one acceptance test including an `ImportState` verify step, green
   against the lab, on **every backend the resource supports**.
9. Drift: an out-of-band change in PowerDNS is reported by
   `terraform plan -refresh-only`.
10. Registry documentation regenerated (`task docs`), `task docs:check` clean.
11. `CHANGELOG.md` `[Unreleased]` updated.
12. Any new claim about PowerDNS behaviour corroborated against the sources and
    the lab, and cited.

## 7. Documentation and registry

- `tfplugindocs` generates `docs/` from schema, `examples/` and `templates/`.
- Registry publishing needs an OSI licence (Apache-2.0), a valid
  top-level `terraform-registry-manifest.json` declaring protocol 6.0, a
  GPG-signed release, and example programs.
- A resource whose availability depends on the server's backend or
  configuration says so **in its own documentation page**, not only in a
  release note.
