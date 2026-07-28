<!-- markdownlint-disable MD033 MD041 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=Methodology&subtitle=Gated-iterative+delivery&logo=target&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="Methodology" src="https://shieldcn.dev/header/graph.svg?title=Methodology&subtitle=Gated-iterative+delivery&logo=target&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>

<div align="center">

[![method gated_iterative](https://shieldcn.dev/badge/method-gated_iterative-0969da.svg?variant=secondary)](#the-choice-gated-iterative-delivery)
[![phases 8](https://shieldcn.dev/badge/phases-8-3fb950.svg?variant=secondary)](#macro-phases)
[![roles 5](https://shieldcn.dev/badge/roles-5-3fb950.svg?variant=secondary)](#roles)
[![sprint 2_weeks](https://shieldcn.dev/badge/sprint-2_weeks-3fb950.svg?variant=secondary)](#iteration-mechanics)

</div>

# Development methodology

## The choice: gated-iterative delivery

**Phase-gated macro-lifecycle, two-week sprints inside implementation,
trunk-based development, a per-item Definition of Done, and evidence
discipline.**

### Why this and not Scrumban

A Terraform provider has a **hard external contract**: schema, state, resource
identity, registry publication, SemVer. You cannot refactor away a shipped
breaking schema, so the contract has to be decided before it is implemented —
which is what a design gate is for.

But the surface is **large and independently deliverable** — 12 resources, 16
data sources, 6 actions, 5 functions across three products — and each item
needs feedback from a live server. That rewards short iterations with a crisp
per-item done-definition.

Scrumban was the right answer for contributing fixes to somebody else's
provider, where work arrives as a stream of independent defects. It is the
wrong answer here, where the first three months build a contract that lasts
years.

## Roles

One person may wear several hats. The separation exists so the same person does
not silently approve their own contract decisions.

| Role | Owns |
|---|---|
| **PM** | Scope, sprint goals, phase gates, changelog and release cadence, the risk register |
| **ARC** (architect) | Provider, schema, state and identity contract; ADRs; transport and error design; non-goals |
| **DEV** | Implementation, tests, documentation, gates green |
| **QA** | Acceptance harness, the backend matrix, negative cases, drift |
| **OPS** | Dev image, lab, pipelines, pin verification |

Hard rules:

- DEV does not approve their own ADR.
- The author of a resource does not write its acceptance test.
- ARC signs the schema before DEV implements it. A schema changed after
  implementation is a design failure, not an iteration.

## Macro-phases

| Phase | Output | Exit gate |
|---|---|---|
| 0. Foundation | Repository, standards, dev image, lab, pipelines | `task all` green on an empty provider; `task lab:verify` green on five services |
| 1. Transport | `internal/api/transport` — HTTP, auth, retry, typed errors, capability classification | Contract tests pass against recorded fixtures from all three products |
| 2. Clients | `auth`, `rec`, `dnsdist` clients | All 68 targeted operations implemented and contract-tested |
| 3. Core resources | zone, record, metadata + their data sources | Acceptance green on both authoritative backends; parity with the best existing provider |
| 4. Security surface | DNSSEC, TSIG, ephemeral key material | A zone signed by Terraform validates under `dig +dnssec` |
| 5. Family surface | views, networks, autoprimaries, recursor, dnsdist | Negative tests assert the diagnostic, not merely the failure |
| 6. Beyond resources | actions, functions, resource identity | Actions verified on Terraform 1.14+ and gated below it |
| 7. Release | Signed release, registry submission | Registry listing live; `terraform init` resolves it |

A downstream phase does not open until the upstream gate is signed off in
`plan.md`.

**Phase 1 before phase 2, and phase 2 before any resource.** The transport
being second-class is what produces inconsistent status handling and duplicated
error interpretation in every provider we examined.

## Definition of Done

### Per client operation

1. Request construction lives in `internal/api/*` and nowhere else.
2. Status examined before the body is decoded.
3. Errors returned as typed values carrying status, server message and
   capability classification.
4. Contract test against a fixture recorded from the real server.
5. Godoc naming the endpoint and the tag it was verified against.

### Per resource

1. `Schema` with `MarkdownDescription` on every attribute; secrets write-only
   or ephemeral, never `Sensitive`-and-stored.
2. `Create`, `Read`, `Update`, `Delete` sharing one model mapper.
3. `ImportState`, and `IdentitySchema` where the object has a stable identity.
4. Re-apply of an unchanged configuration produces an empty plan, asserted with
   `plancheck.ExpectEmptyPlan`.
5. Server-side normalisation handled by semantic comparison, not string
   equality.
6. Validators for what the API will reject; diagnostics naming the requirement
   for what cannot be known at plan time.
7. `UpgradeState` if the state shape changed.
8. At least five unit edge cases: empty, maximum-length identifier, idempotent
   re-create, conflict, transport error.
9. At least one acceptance test with an `ImportState` verify step, green on
   **every backend the resource supports**.
10. Drift: an out-of-band change is reported by `plan -refresh-only`.
11. Registry documentation regenerated; `task docs:check` clean.
12. `CHANGELOG.md` and `docs/plan.md` updated in the same commit.

## Iteration mechanics

- **Sprints are two weeks.** Work lands on `sprint/<id>-<scope>`; the merge
  request cites the exit criteria it satisfies.
- **Trunk-based.** `main` is always shippable; short-lived branches;
  squash-merge.
- **Quality gates** (`task all`, `task verify`) are the automated part. An item
  is not done until they are green with evidence quoted.

## Evidence discipline

Every factual claim about PowerDNS behaviour carries a source: the sources at a
pinned tag, plus a live round-trip. This is not ceremony — in the work that
preceded this repository it caught, among others:

- the published OpenAPI documenting an endpoint that does not exist and omitting
  a method that does;
- views returning `200` with an empty list on a backend that cannot store them;
- `setAPIWritable`, not `apiConfigDir`, gating every dnsdist write;
- three server-side normalisations that each produced a permanent diff.

Not one of those was discoverable from documentation.

## Definition of Ready

Before an item enters a sprint:

- Endpoint and method identified **in the sources**, not only in the
  specification.
- Backend and configuration preconditions known and recorded.
- Schema shape drafted and signed by ARC.
- Test approach sketched, including which backends it must run on.
