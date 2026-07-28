# Delivery plan

Living document. The method is in [`methodology.md`](methodology.md); this is
its execution record. **A task's status changes in the commit that does the
work**, never retrospectively — a plan updated afterwards is a report, not a
control.

**Status:** phase 0 **closed**; phase 1 (transport) next
**Last updated:** 2026-07-28

## Legend

| Mark | Meaning |
|---|---|
| `[x]` | done, gate green, evidence in the commit body |
| `[~]` | in progress |
| `[ ]` | not started |
| `[!]` | blocked — the blocker is named in the row |
| `[-]` | dropped — the reason is named in the row |

Roles per [`methodology.md`](methodology.md): **PM**, **ARC**, **DEV**, **QA**,
**OPS**.

---

## Phase 0 — Foundation · `[x]` closed 2026-07-28

**Goal:** the toolchain, the lab and the pipelines everything else depends on.
**Exit gate:** `task all` green on an empty provider; `task lab:verify` green
across five services.

### Sprint S0 — Repository and toolchain

| ID | Task | Role | Depends | Status |
|---|---|---|---|---|
| S0-01 | Repository `ioplane/terraform-provider-powerdns`, Apache-2.0 | PM | — | `[x]` |
| S0-02 | `AGENTS.md`, symlinks `CLAUDE.md` and `CODEX.md` | PM | S0-01 | `[x]` |
| S0-03 | Naming standard synthesised from the five sources | PM | — | `[x]` |
| S0-04 | Standards: versioning, commits, changelog | PM | S0-03 | `[x]` |
| S0-05 | Standards: Go 1.26, provider practices, Terragrunt, API discipline, Python, verified identifiers | ARC | — | `[x]` |
| S0-06 | ADR 0001–0007 | ARC | — | `[x]` |
| S0-07 | Dev image on `golang:1.26-trixie`, pinned by digest | OPS | — | `[x]` |
| S0-08 | Five-service lab, every image pinned by digest | OPS | S0-07 | `[x]` |
| S0-09 | `lab.py` on podman-py: up, down, status, verify | OPS | S0-08 | `[x]` |
| S0-10 | Taskfile — 37 tasks, guards on container and lab | OPS | S0-07 | `[x]` |
| S0-11 | `golangci-lint` v2, allowlist of 82, **no path exclusions** | OPS | — | `[x]` |
| S0-12 | Python gate: uv, ruff, ty, `pyproject.toml` | OPS | S0-07 | `[x]` |
| S0-13 | `scripts/check-pins.sh` — digests and action SHAs resolve, none float | OPS | — | `[x]` |
| S0-14 | `scripts/check-no-ai-attribution.sh` and the commit hook | OPS | — | `[x]` |
| S0-15 | GitLab CI: build, test, lint, security, acceptance matrix | OPS | S0-10 | `[x]` |
| S0-16 | GitHub Actions: release only — goreleaser, GPG, registry | OPS | S0-10 | `[x]` |
| S0-17 | `main.go` + empty framework provider, protocol 6, three-product schema | DEV | S0-10 | `[x]` |
| S0-18 | `CHANGELOG.md`, `VERSION`, README, CONTRIBUTING, SECURITY, docs index | PM | — | `[x]` |

**Exit gate met.** `go build` clean, `golangci-lint` 0 issues with **no path
exclusions**, `check-pins.sh` verifies 10 references, `lab:verify` green across
five services. The Taskfile parses with 37 tasks.

**Two of my own mistakes were caught by the gate I had just written.** The
GitLab pipeline went in with placeholder digests of all zeros, and
`check-pins.sh` itself had a resolution bug — it handed skopeo a reference
carrying both a tag and a digest, which skopeo rejects, so five valid digests
reported as not found. Both fixed; the checker is now verified against a
fixture containing one fabricated digest and one floating tag.

**S0-08 already earned its keep.** Standing up dnsdist found that
`setAPIWritable`, not `apiConfigDir`, gates every write — without it each `PUT`
answers `405` — and that `DELETE /api/v1/cache` answers `404` when the pool has
no packet cache. Neither is in the documentation. Recorded in
[ADR 0006](adr/0006-dnsdist-scope.md).

---

## Phase 1 — Transport · `[ ]`

**Exit gate:** contract tests pass against fixtures recorded from all three
products.

| ID | Task | Role | Depends | Status |
|---|---|---|---|---|
| S1-01 | ARC: error taxonomy and capability classification, signed | ARC | — | `[ ]` |
| S1-02 | `transport.Client`: HTTP, `X-API-Key`, timeouts | DEV | S1-01 | `[ ]` |
| S1-03 | Retry: `5xx` and transport errors back off; `4xx` fails fast | DEV | S1-02 | `[ ]` |
| S1-04 | `transport.APIError`: status, server message, capability class | DEV | S1-01 | `[ ]` |
| S1-05 | Capability classifier: LMDB-only, `api_dir` unset, `setAPIWritable` unset, no packet cache | DEV | S1-04 | `[ ]` |
| S1-06 | Fixture recorder — capture real responses from the lab into `testutil` | QA | S0-08 | `[ ]` |
| S1-07 | Contract tests for every error class | QA | S1-05, S1-06 | `[ ]` |

S1-05 is the piece the whole design turns on. Four different "this installation
cannot do that" conditions exist across the three products; each surfaces as a
bare `4xx`. Classifying them once means every resource reports the same
actionable diagnostic.

---

## Phase 2 — Clients · `[ ]`

**Exit gate:** all 68 targeted operations implemented and contract-tested.

| ID | Task | Role | Depends | Status |
|---|---|---|---|---|
| S2-01 | `api/auth`: zones, rrsets | DEV | S1-07 | `[ ]` |
| S2-02 | `api/auth`: metadata, cryptokeys, tsigkeys, autoprimaries | DEV | S2-01 | `[ ]` |
| S2-03 | `api/auth`: views, networks, server, search, cache, zone actions | DEV | S2-01 | `[ ]` |
| S2-04 | `api/rec`: zones, the two writable settings, statistics | DEV | S1-07 | `[ ]` |
| S2-05 | `api/dnsdist`: acl, cache, statistics, pool, rings, config | DEV | S1-07 | `[ ]` |
| S2-06 | Contract tests per client | QA | S2-01…S2-05 | `[ ]` |

---

## Phase 3 — Core resources · `[ ]`

**Exit gate:** acceptance green on both authoritative backends.

| ID | Task | Role | Depends | Status |
|---|---|---|---|---|
| S3-01 | ARC: zone and record schemas, identity, signed | ARC | S2-06 | `[ ]` |
| S3-02 | `powerdns_zone` | DEV | S3-01 | `[ ]` |
| S3-03 | `powerdns_record` | DEV | S3-02 | `[ ]` |
| S3-04 | `powerdns_zone_metadata` | DEV | S3-02 | `[ ]` |
| S3-05 | Data sources: zone, zones, record, zone_metadata, zone_export | DEV | S3-03 | `[ ]` |
| S3-06 | Semantic-normalisation plan modifiers: case, IPv6, server defaults | DEV | S3-02 | `[ ]` |
| S3-07 | Acceptance on both backends for each | QA | S3-02…S3-05 | `[ ]` |

S3-06 is not optional polish. Three normalisations are already known — `kind`
title-casing, IPv6 zero compression, `soa_edit_api` assignment — and each
produces a permanent diff if compared as a string.

---

## Phase 4 — Security surface · `[ ]`

**Exit gate:** a zone signed through Terraform validates under `dig +dnssec`.

| ID | Task | Role | Depends | Status |
|---|---|---|---|---|
| S4-01 | ARC: key-material handling — ephemeral versus write-only, signed | ARC | S3-07 | `[ ]` |
| S4-02 | `powerdns_zone_cryptokey` | DEV | S4-01 | `[ ]` |
| S4-03 | Zone DNSSEC attributes: `dnssec`, `nsec3param`, `nsec3narrow`, `presigned`, `api_rectify` | DEV | S4-02 | `[ ]` |
| S4-04 | `powerdns_tsigkey`; zone `master_tsig_key_ids` / `slave_tsig_key_ids` | DEV | S3-07 | `[ ]` |
| S4-05 | Ephemeral `powerdns_cryptokey_material`, `powerdns_tsigkey_secret` | DEV | S4-01 | `[ ]` |
| S4-06 | Behaviour test: signed zone validates under `dig +dnssec` | QA | S4-03 | `[ ]` |
| S4-07 | Test asserting no key material reaches state | QA | S4-05 | `[ ]` |

S4-07 is a security property, so it is tested rather than asserted: the test
reads the state file and fails if it finds key material.

---

## Phase 5 — Family surface · `[ ]`

| ID | Task | Role | Depends | Status |
|---|---|---|---|---|
| S5-01 | `powerdns_view_zone`, `powerdns_network` — LMDB only | DEV | S3-07 | `[ ]` |
| S5-02 | `powerdns_autoprimary` | DEV | S3-07 | `[ ]` |
| S5-03 | `powerdns_recursor_zone`, `powerdns_recursor_acl` | DEV | S2-04 | `[ ]` |
| S5-04 | `powerdns_dnsdist_acl` | DEV | S2-05 | `[ ]` |
| S5-05 | Data sources for recursor and dnsdist | DEV | S5-03, S5-04 | `[ ]` |
| S5-06 | Negative tests asserting each capability diagnostic | QA | S5-01…S5-04 | `[ ]` |

S5-06 asserts the **message**, not the failure. A bare `422` reaching a user is
the defect this provider exists to avoid.

---

## Phase 6 — Beyond resources · `[ ]`

| ID | Task | Role | Depends | Status |
|---|---|---|---|---|
| S6-01 | Actions: `notify_zone`, `axfr_retrieve`, `rectify_zone`, `flush_cache` | DEV | S2-03 | `[ ]` |
| S6-02 | Actions: `recursor_flush_cache`, `dnsdist_flush_cache` | DEV | S2-04, S2-05 | `[ ]` |
| S6-03 | Functions: `fqdn`, `is_fqdn`, `reverse_zone_name`, `ptr_name`, `soa_serial` | DEV | — | `[ ]` |
| S6-04 | Resource identity on every resource with a stable one | DEV | S3-07 | `[ ]` |
| S6-05 | Gate actions at Terraform 1.14 via client capability | DEV | S6-01 | `[ ]` |

S6-03 has no dependency on the clients: the functions are pure and offline,
which is why they are functions. They replace the name arithmetic that other
providers smear into resources.

---

## Phase 7 — Release · `[ ]`

| ID | Task | Role | Status |
|---|---|---|---|
| S7-01 | Examples for every resource, action and function | DEV | `[ ]` |
| S7-02 | Registry documentation generated and validated | DEV | `[ ]` |
| S7-03 | Version matrix: acceptance against auth 5.0.x as well as 5.1.3 | QA | `[ ]` |
| S7-04 | Signed `v0.1.0` | PM | `[ ]` |
| S7-05 | Terraform Registry submission | PM | `[ ]` |

---

## Risk register

| Risk | Effect | Response |
|---|---|---|
| Server-side normalisation not caught | Permanent diff for the user | S3-06; every resource gets an `ExpectEmptyPlan` check |
| Key material reaches state | Secret exposure | S4-01 decides before implementation; S4-07 tests the state file |
| Capability conditions handled per resource | Divergent diagnostics | S1-05 classifies once in the transport |
| dnsdist support attracts rule-management requests | Repeated disappointment | Ceiling documented on the resource page with the numbers |
| PowerDNS 5.2 ships mid-build | Pinned claims go stale | `task lab:verify` fails loudly on a version change |
| Actions raise the Terraform floor | Older CLIs lose the provider | S6-05 gates actions, not the provider |
| Two pipelines drift | Contradictory gates | They do not overlap: GitLab owns quality, GitHub owns release only |

## How this document is maintained

- Status changes in the commit that does the work.
- A task that turns out to be wrong is marked `[-]` with the reason, not
  deleted.
- A task discovered mid-sprint is added with a note of where it came from.
- Phase closures are recorded in `CHANGELOG.md`.
