# AUDIT-04 — provider efficiency and duplication

**Date:** 2026-08-26

**Bead:** `tfp-bqt.7`

**Status:** local candidate approved; GitHub Actions and merge remain open

## Scope and method

The audit covers all nine first-party Go packages: the root provider command
and eight packages under `internal/`. The final candidate contains 46
production and 39 test Go files. An unused future Go lab control plane was
removed because no Taskfile or workflow invoked it and the executable Python
lifecycle passed E2E.

The package and import graph came from `go list`. All 46 production files were
checked together with `gopls check`; no production diagnostic was reported.
The graph is acyclic.

Duplication was measured with golangci-lint's pinned clone analyzer and then
classified with `gopls` references and call hierarchy. The default threshold
of 150 tokens reports zero groups. Lower thresholds report 9 production groups
at 100 tokens, 73 at 50, and 182 at 30. The nine largest groups are Terraform
Framework or product-client scaffolding with different return types, endpoints,
or side effects. Lowering the global threshold would add noise and would still
miss short semantic duplicates, so the threshold is not changed by this task.

## Package cross-check

| Package | Production/test files at stable snapshot | Plan and duplication result |
| --- | ---: | --- |
| root | 1/0 | Provider bootstrap matches the Phase 7 boundary; no duplicate algorithm. |
| `internal/api/auth` | 9/4 | Phase 2 client; repeated CRUD shapes remain product-typed and endpoint-specific. |
| `internal/api/dnsdist` | 2/2 | Phase 2 client; similar wrappers have different response and endpoint semantics. |
| `internal/api/rec` | 3/2 | Phase 2 client; repeated HTTP shapes are product-local adapters. |
| `internal/api/transport` | 4/5 | Shared transport boundary; floating-point retry backoff is replaced by bounded integer arithmetic. |
| `internal/provider` | 22/17 | Phases 3–6 implementation; largest similarities are typed Framework schema and CRUD scaffolding with distinct side effects. |
| `internal/provider/normalise` | 1/3 | Canonical semantic-comparison owner; multiset comparison is linear for large inputs. |
| `internal/provider/planmodify` | 1/1 | Delegates multiset comparison to `normalise`; no second implementation remains. |
| `internal/testutil` | 3/5 | Phase 1 test scaffolding; no production clone. |

Product-local constructors, `basePath` helpers, Framework description methods,
and domain aliases such as `TSIGKeyID` and `RecordName` are retained. They name
different product or domain contracts even where their current bodies are
similar. A shared helper is introduced only when both result and ownership
semantics are identical.

## Complexity inventory

`gocyclo` v0.6.0 and `gocognit` v1.2.1 were run over every Go declaration.
The table records every production function above 15 in either measurement;
test helpers are governed by the separate 500-line test-file limit.

| Function group | Cyclomatic/cognitive | Disposition |
| --- | ---: | --- |
| `provider.applyZone` | 21/≤15 | One ordered create/update orchestration with API error mapping; resource tests cover both branches. |
| `normalise.foldKey` | 16/16 | Unicode simple-fold representative plus ASCII and invalid-UTF-8 fast paths; independent differential and native fuzz tests bind equivalence. |
| `transport.Client.Do` | ≤15/16 | One bounded HTTP retry loop; transport status, body, cancellation, and saturation tests cover it. |
| `flushCacheAction.Invoke` | ≤15/16 | Typed action request/response and diagnostic mapping; action tests retain product semantics. |

No threshold violation is an unexplained duplicate. The new multiset function
was split into bounded small-list and counted large-list helpers after the
inventory; it is no longer above the cognitive threshold.

## Direct module graph

Every direct module has a current import path proved by `go mod why -m`.

| Module | Version | First-party owner |
| --- | --- | --- |
| `github.com/getkin/kin-openapi` | `v0.147.0` | `internal/testutil` OpenAPI fixture server |
| `github.com/hashicorp/terraform-plugin-framework` | `v1.19.0` | root command and `internal/provider` |
| `github.com/hashicorp/terraform-plugin-framework-validators` | `v0.19.0` | `internal/provider` schema validation |
| `github.com/hashicorp/terraform-plugin-go` | `v0.31.0` | provider protocol through Framework server startup |
| `github.com/hashicorp/terraform-plugin-log` | `v0.11.0` | `internal/api/transport` and provider logging |
| `github.com/hashicorp/terraform-plugin-testing` | `v1.16.0` | `internal/provider` acceptance tests |

No direct module is orphaned. The versions and their upstream identifiers are
already verified by [AUDIT-02](AUDIT-02-go-1.27-toolchain.md).

## Non-Go and delivery surfaces

The final snapshot contains 36 Python files. A syntax-tree comparison that
ignores function names, docstrings, and local variable spelling found no exact
production function clone. The only exact bodies are test-local context-manager
`__enter__` and `__exit__` methods. They remain local because they manage
different fake process boundaries and do not own production behavior.

`scripts/automation/e2e.py` remains a deliberate integration driver: it owns
remote state, a private module source, two Terraform engines and cleanup in one
place. Taskfile and workflows invoke that tested executable boundary directly;
there is no parallel Go implementation.

## Correctness and performance changes

Tests were added before each production change.

- Ten resource attributes reproduced semantic normalization running after
  `RequiresReplace`. Their semantic modifier now runs first, matching the
  Terraform Plugin Framework's ordered modifier contract.
- `math.Pow`-based retry backoff was replaced by monotonic saturating integer
  doubling. Unit and fuzz cases include negative attempts and `math.MaxInt`.
- `normalise.StringSet` and `planmodify.setsMatch` were one algorithm under two
  names. One `StringMultiset` implementation now owns order-insensitive,
  multiplicity-sensitive comparison through canonical keys.
- Fuzzing found malformed upstream `[` was not idempotent: repeated
  canonicalization appended a default port repeatedly. Review then found that
  empty hosts or ports made `host:` equal `host` and `""` equal `:53`.
  Malformed bracket, colon, empty-host, empty-port, and repeated-root-label
  forms now remain unchanged.
- A registration invariant enumerates every resource, data source, action,
  ephemeral resource, and function. Names must be canonical and unique within
  their Terraform namespace, and the exact expected inventory prevents silent
  deletion or product-prefix drift.

The old quadratic multiset benchmark measured about 351–375 ns/op for eight
DNS values and 4.57–4.98 ms/op for 1,024 values. The current hybrid algorithm
measures about 623–643 ns/op with zero allocations for eight values and 111–134
µs/op with five allocations for 1,024 values on the same container and CPU.
The large case is about 34–45 times faster without changing multiplicity or
canonical equivalence.

The statistical gate used `benchstat` from
`golang.org/x/perf@v0.0.0-20260825160852-19be9d8e6c70` over 15 paired
100-millisecond samples built from exact `HEAD` baseline bytes and the current
worktree in the same dev container. Median DNS time changed from 356 ns to
624 ns at size 8 (`+75.28%`, `p=0.000`, `n=15`) and from 4,639.4 µs to
115.0 µs at size 1,024 (`-97.52%`, `p=0.000`, `n=15`). The large case trades
one 1 KiB allocation for five 53.328 KiB allocations; the measured speedup is
therefore explicit rather than presented as an allocation improvement.

Realistic 1,024-entry ACL/CIDR and A-record RRSet cases measure 200–208 µs/op
and 181–190 µs/op respectively. Integer backoff is 1.6–2.1 ns/op for the first
attempt and 4.5–4.7 ns/op at `math.MaxInt`, with zero allocations. Enumerating
all 29 provider registrations is 1.8–2.0 ns/op with zero allocations. These
benchmarks remain in the package gate so later complexity or deduplication
changes have an executable baseline.

Native fuzz runs for canonical-key idempotence, independent legacy
no-broadening, equality laws and backoff saturation pass. Focused race, shuffle
and atomic-coverage tests pass for transport, provider, normalization and plan
modification. A fresh `go test ./... -count=1` passes all nine packages. The
local E2E fixture passes 59/59 cases across Terraform and OpenTofu, including
state upgrade, drift, private module transport and DNS behavior, and leaves
zero residue after teardown.

The final local candidate passes `task all`, both complete `task verify`
matrices (PowerDNS Authoritative 5.1 and 5.0), and the 59-case E2E suite. The
runtime stacks and generated fixture files are absent after teardown. The
audit remains open only until the `dev` GitHub Actions and merge gate pass.
