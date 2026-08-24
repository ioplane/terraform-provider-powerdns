# Go 1.27 and environment modernization design

**Status:** approved with PostgreSQL 18 required and combined cache/toolchain
boundary

**Date:** 2026-08-21

**Beads epic:** `tfp-bqt`

## Goal

Move the provider, its development image, CI, acceptance lab and end-to-end
fixture to current stable delivery channels while preserving provider protocol
6 and the documented Terraform, OpenTofu and PowerDNS compatibility contract.

The work also closes correctness and efficiency findings exposed by the
upgrade: semantic normalization must prevent false replacement, comparison
algorithms must scale with realistic ACL and RRSet sizes, backoff arithmetic
must be exact, and quality gates must verify that they receive meaningful
input.

## Delivery decision

The modernization is delivered as independent pull requests ordered by the
Beads dependency graph. Each pull request has one primary concern, updates
`docs/plan.md` with the owning change and carries its own rollback and gate
evidence.

One evidence-driven exception combines `tfp-bqt.2.1` and `tfp-bqt.3`. The
baseline branch could not build the development image because its persistent
module cache contained incomplete source trees. That repair was required before
the Go migration could be validated, and runtime parity is part of the same
image evidence. The user then required that the unpublished work contain no
intermediate Go 1.26.7 project state. The staged design documents, cache and
lifecycle repair, and Go 1.27 migration therefore collapse into one final
atomic commit and pull request. The user explicitly approved this
reconciliation. It does not combine any later service, workflow,
provider-correctness or gate-policy boundary.

Current stable patches are selected within existing compatibility channels,
with one deliberate major migration: the disposable lab database moves from
PostgreSQL 17.10 to PostgreSQL 18.6. Node stays on the current LTS channel.
PowerDNS keeps the documented lower-bound Authoritative 5.0 branch while the
current product branches receive patch updates.

A single big-bang pull request is rejected because it would combine language,
database, runtime, service, provider behavior and CI changes. A pins-only
update is rejected because measured correctness and gate defects would remain.

### Pull-request boundaries

| Bead | Pull-request boundary |
| --- | --- |
| `tfp-bqt.2` | Operational storage recovery; no repository change |
| `tfp-bqt.2.1` + `tfp-bqt.3` | **Approved exception:** cache and worktree lifecycle repair plus Go 1.27, direct modules, required analyzers and compatibility tests in one final unpublished-branch boundary |
| `tfp-bqt.4` | Remaining development-image tools and build layering |
| `tfp-bqt.5` | Workflow containers and GitHub Actions |
| `tfp-bqt.6.1` | PostgreSQL 18.6 only |
| `tfp-bqt.6.2` | PowerDNS, SeaweedFS and Forgejo Compose images |
| `tfp-bqt.7` | Semantic correctness, backoff and measured efficiency |
| `tfp-bqt.8` | Linter and scanner policy after code reaches its baseline |
| `tfp-bqt.9` | Historical review findings and naming invariants |
| `tfp-bqt.10` | Terraform, OpenTofu and Terragrunt compatibility |
| `tfp-bqt.11` | Final documentation reconciliation and release evidence |

PostgreSQL 18 is isolated because it changes a service major version and is the
largest fixture compatibility boundary. The other Compose image patches remain
separate so a PowerDNS or e2e regression cannot be attributed to PostgreSQL.
Grouped tool updates are one development-environment change set; exact tool
versions remain individually observable and bisectable in Git.

## Sources of truth

Version and behavior claims use the following order:

1. Go release notes and language specification for Go behavior.
2. Upstream source and release notes for libraries and tools.
3. OCI registry manifests for executable image identity.
4. `PowerDNS/pdns` source tags for API behavior.
5. The local lab for observable provider behavior.
6. GitHub GraphQL pagination for release, issue and review state.

Every executable reference is immutable. OCI images use a human-readable tag
and a manifest digest. GitHub Actions use the peeled 40-character commit.
Libraries and tools use exact versions. The pin checker validates every
occurrence, including workflow containers and Compose overlays.

## Target environment

### Language and provider stack

| Component | Target | Compatibility decision |
| --- | --- | --- |
| Go | 1.27.0 | Minimum and build toolchain |
| terraform-plugin-framework | 1.19.0 | Already current; protocol 6 |
| Terraform | 1.15.9 | Actions and identity remain Terraform-only |
| OpenTofu | 1.12.6 | Independent protocol 6 acceptance path |
| Terragrunt | 1.1.3 | Shared-state end-to-end driver |
| golangci-lint | 2.13.1 | Go 1.27-capable release |
| govulncheck | 1.7.0 | Go 1.27-capable vulnerability analyzer |

The Go base image is
`docker.io/library/golang:1.27-trixie@sha256:6212da3924947f4b6a939df02ea627c13f338f1a41d6c3fcb0dd9d076eef46c4`.
The same Go version and digest are used by the development image and every
containerized CI job that compiles or analyzes Go. Jobs that cannot run inside
that image use the commit-SHA-pinned `actions/setup-go` action with
`go-version-file: go.mod`; the exact `go 1.27.0` directive is their single
version source. The Go change-set verification covers both delivery paths.

A separate matching `toolchain go1.27.0` line is intentionally absent. Go 1.27
defines that suggestion implicitly, removes the redundant line during
`go mod tidy`, and refuses a subsequent build until that tidy delta is applied.
The exact compiler is instead enforced by the immutable OCI pin, inherited
`GOTOOLCHAIN=local`, runtime `GOVERSION` parity, and workflow parity checks.

`golangci-lint` and `govulncheck` move with the Go change set rather than the
general tool refresh. This keeps the Go 1.27 pull request independently
verifiable: an older analyzer must not be the reason that the new toolchain
cannot pass its mandatory gates. All other development-image tools remain in
`tfp-bqt.4`.

### Lab and end-to-end images

| Service | Target image |
| --- | --- |
| PostgreSQL | `docker.io/library/postgres:18.6@sha256:06cad38a5d9f5d24b4d83d86def30795d5e4b757fedbf5281172b576dedcd941` |
| PowerDNS Auth current | `docker.io/powerdns/pdns-auth-51:5.1.4@sha256:bb5b1c133bcca1dd455075321de7d55db4945a8d7f2ba23339e3c7bbe416b205` |
| PowerDNS Auth lower bound | `docker.io/powerdns/pdns-auth-50:5.0.6@sha256:c6d296669d720dd3f596bf2b42dc25d2e272f40a2b2493a1c476b3007a1a4a75` |
| PowerDNS Recursor | `docker.io/powerdns/pdns-recursor-54:5.4.5@sha256:33aadc74a8d6b68cb422b06a5bff0c032dbf0f712ba6e5a62cf5bd9739dbac70` |
| dnsdist | `docker.io/powerdns/dnsdist-21:2.1.1@sha256:28100b260120b2f4fdcef6e973f5553966977c2ec4576ed2d849b1dc9d3278ea` |
| SeaweedFS | `docker.io/chrislusf/seaweedfs:4.42@sha256:f7cbc8bdbbf60a1aaba7d61784a3bdff3ec1e0657f6ad0b26d5b6ab2cd9d0dc6` |
| Forgejo | `codeberg.org/forgejo/forgejo:16.0.3-rootless@sha256:214f4ae63ee78be1e445e58573c88dc7215e72091210852e0df94eaac1a25685` |

Workflow containers move to Semgrep 1.174.0 and Trivy 0.74.0 with their
verified manifest digests. The local development image remains locally named;
its base and installed executables are independently pinned.

## PostgreSQL 18 design

The current fixture uses PostgreSQL 17.10 and does not mount a persistent
database volume. PostgreSQL 18.6 therefore starts a new disposable cluster and
runs `test/lab/schema.pgsql.sql`; this work does not claim to test `pg_upgrade`
or dump and restore.

PostgreSQL 18 enables data checksums by default and changes several server
behaviors, but none changes the PowerDNS gpgsql schema contract by assumption.
The acceptance evidence must prove the contract instead:

- the container reports server version 18.6;
- the schema initializes without warnings or errors;
- the Authoritative gpgsql service becomes healthy;
- CRUD and direct SQL assertions observe the expected rows;
- both Authoritative compatibility branches run the same acceptance suite;
- end-to-end apply, upgrade and destroy leave no residue.

Ports, credentials, schema mount, health check and `service_healthy` ordering
remain unchanged. A future persistent PostgreSQL fixture requires a separate
data-migration design.

## Go 1.27 compatibility design

The migration accounts for these release changes:

- `go test` applies the `stdversion` vet analyzer by default;
- `encoding/json` v1 is backed by the v2 implementation;
- closing an unread HTTP/1 response body performs limited draining;
- system certificate pool behavior changes on Darwin and Windows;
- Unicode tables move to Unicode 17;
- generic methods and generalized function type inference become valid;
- `go mod tidy` may consolidate `require` blocks.

Regression tests cover JSON request and response contracts, empty and bounded
error bodies, connection reuse, custom CA, mTLS, TLS 1.2 and TLS 1.3, and DNS
normalization with a newly assigned Unicode 17 letter. The Unicode test proves
both the standard library's new category table and the provider's byte-preserving
trailing-dot behavior; it does not invent a case mapping for an uncased `Lo`
character. Release builds include a Darwin dry run because Go 1.27 raises the
minimum supported macOS release.

The module file is checked after `go mod tidy`; no manually added directive is
considered present until it survives the final tidy and diff check.

## Container and Compose design

`deployments/containers/Containerfile.dev` remains the single development
toolbox. Its active Go identity and cache contract are recorded in
[ADR 0010](../../adr/0010-go-1.27-development-toolchain.md). Every version
argument is updated from its delivery channel, every download has an integrity
check where the publisher provides one, and every installed executable reports
its version during the build.

Build, test, lint, generation and Go commands run inside that container. The
host is limited to worktree management, Podman and Compose orchestration,
storage diagnostics and other repository-defined host drivers.

The Go tool installation layer is measured before restructuring. If the cold
build still exhausts temporary storage, module downloads and tool compilation
are split into atomic stages and only executables are copied into a clean final
stage. Compiler caches may persist; incomplete module trees may not become
shared state.

Compose service contracts remain stable:

- `compose.dev.yml` keeps one isolated project, container and local image tag
  per worktree;
- `compose.lab.yml` keeps five services and both Authoritative backends;
- `compose.lab-auth-50.yml` remains an overlay changing only Auth images;
- `compose.e2e.yml` continues to extend the lab with S3 and HTTPS Git hosting;
- SELinux relabel flags, host networking, health checks and named API volumes
  are preserved.

Each base and overlay combination must render through `podman-compose config`
before images are pulled. Runtime validation then checks the observed image ID
and product version, not only the text in YAML.

Routine startup is non-destructive. Explicit recreation builds first and then
removes only an exact-name container whose Compose project label and canonical
`/app` bind source prove it belongs to the current worktree. Missing or
mismatched ownership evidence fails before removal.

## Provider correctness and performance

### Audit method

The audit covers every first-party Go package, not only functions already named
as suspicious. It starts only after the Go 1.27 dev container provides a working
compiler configuration and `gopls` index.

1. Record the package and dependency graph with `go list`; prove every direct
   module with imports and `go mod why`.
2. Inventory every declared function and method through syntax-aware tooling,
   then use `gopls` references and call hierarchy for reachability and callers.
3. Measure exact and structural duplication with independent analyzers and
   inspect each candidate at the symbol level. Schema declarations and product
   adapter symmetry are classified separately from duplicated algorithms.
4. Record cyclomatic and cognitive complexity for every function. Review every
   threshold violation for correctness, error paths and testability.
5. Benchmark comparison, normalization, transport and provider-registration hot
   paths with realistic small and large inputs. Record time and allocations.
6. Enumerate every canonicalizer and semantic comparator and test its applicable
   algebraic laws. Enumerate every Terraform-facing registration and schema name
   and validate the naming contract mechanically.
7. Record every actionable finding as a Bead with severity, evidence, owner,
   focused test and acceptance gate.

The audit exits only when all first-party packages and declared functions appear
in the inventory; every direct module is justified; no exact algorithm clone is
unexplained; no unbounded quadratic path remains without a documented bound and
benchmark; every canonicalizer has an idempotence property; every claimed
equivalence has its applicable relation laws; registration names are unique;
and every complexity exception has a written reason and focused tests. Benchmark
comparisons use repeated samples and statistical comparison rather than one
wall-clock result. Any unresolved correctness, security or data-integrity
finding blocks completion.

### Known starting findings

The duplicated multiset comparison in normalization and plan modification is
replaced only where canonicalization defines an equivalence relation. For DNS
names, IP addresses, CIDRs and upstream endpoints, canonical keys permit a
frequency-map comparison with expected linear time and multiplicity tracking.

DNSSEC key-type compatibility is not transitive. It remains a compatibility
predicate and is not converted into a hash-key equivalence.

Properties enforced by tests are:

- idempotence: `C(C(x)) = C(x)`;
- reflexivity, symmetry and transitivity for true equivalence relations;
- permutation invariance and multiplicity sensitivity for multisets;
- empty multiset identity;
- monotonicity and saturation for retry backoff.

Backoff uses bounded integer arithmetic instead of floating-point exponentiation.
Semantic plan modifiers run before `RequiresReplace`, so canonically equivalent
values cannot schedule replacement. Benchmarks cover realistic large ACL and
RRSet inputs and record allocations as well as elapsed time.

Provider registration receives an invariant test: every resource, data source,
function, action and ephemeral resource name is unique, canonical and consistent
with its product prefix.

## Lint and security gates

golangci-lint uses the maximum useful profile, not indiscriminate `enable-all`.
The initial required additions are `revive`, `cyclop`, `gocognit` and `gocyclo`.
Further linters are enabled only when their target construct exists and their
diagnostic does not duplicate the formatter or another gate. Database-, logger-
and framework-specific linters with zero input remain disabled with a reason.

The repository keeps one formatting path. New high-noise checks enter report
mode with a measured baseline, findings are corrected by consequence, and the
check becomes blocking when its owned scope reaches zero.

The security aggregate adds:

- secret scanning over the pull-request diff and repository history;
- direct `go vet` with a non-empty package assertion;
- OSV Scanner in the local aggregate;
- dependency licence policy;
- scheduled native Go fuzzing.

Every new gate is proven with a deliberately failing fixture, followed by the
same gate passing after removal. Scanner output is triaged as evidence, not
treated as a defect without reachability and context.

The repository-history secret scanner is proven against a disposable synthetic
Git repository. Its deliberate secret never enters this repository's object
database.

## Terraform and OpenTofu compatibility

Framework 1.19.0 and plugin protocol 6 remain the provider boundary. Terraform
and OpenTofu run independent acceptance paths; a green Terraform run does not
stand in for OpenTofu.

Both engines cover the operations they support: init, validate, plan, apply,
import, provider upgrade, empty plan, state portability, lock contention and
destroy. Terraform-only action and resource-identity fixtures stay explicitly
pinned to Terraform. OpenTofu receives explicit coverage for its ephemeral-plan
behavior and registry GPG verification path.

## Historical review findings

The 38 unresolved review threads are classified against current `main` as
fixed, obsolete, deliberate design or actionable. Actionable findings are
assigned to the pull request that owns the affected behavior. A thread is
resolved only after the corresponding code, test and GitHub state agree.

This work includes semantic modifier ordering, validators, empty-value drift,
record preservation, import identity, version-occurrence validation, action
examples, release and changelog checkers, pin immutability, no-secret policy and
Taskfile branch interpolation.

## Documentation and status

Beads is the durable task graph. `docs/plan.md` remains the public execution
record and changes in the same commit as the work whose status it reports.
User-visible changes update `CHANGELOG.md` under `Unreleased`. Version badges,
tested-version tables, standards, source citations and generated registry docs
change with their owning implementation.

The final documentation bead detects residual drift; it does not retroactively
mark implementation work complete.

The first executable change-set plan is
[`2026-08-21-go-1-27-toolchain-plan.md`](../plans/2026-08-21-go-1-27-toolchain-plan.md).
It implements the approved `tfp-bqt.2.1` plus `tfp-bqt.3` atomic exception;
subsequent pull-request boundaries receive their own reviewed plans before
implementation.

## Verification and rollback

Each pull request runs its focused red/green tests and the complete gates its
scope requires. The final sequence is:

1. cold and warm development-image builds;
2. `task all`;
3. Compose rendering for dev, lab, lower-bound overlay and e2e;
4. `task verify` against PostgreSQL 18.6 and both Auth branches;
5. Terraform and OpenTofu end-to-end suites;
6. vulnerability, secret, licence and OCI scans;
7. release dry run and generated-document drift check;
8. uncached final tests and clean Git status.

Every change set is independently revertible because it is delivered in the
pull-request boundary above. The approved combined cache/toolchain boundary is
reverted atomically because the former cache policy cannot produce a reliable
baseline build. PostgreSQL 18 has its own pull request. The prior immutable
digest remains visible in Git. Lab data is disposable; rollback recreates the
service from the prior pin rather than reusing a database directory across
major versions.

## Operational prerequisite

The original development-image build was blocked first by capacity and then by
the persistent incomplete module cache tracked in `tfp-bqt.2.1`. Capacity was
recovered under scoped authorization. No existing development object was
removed or replaced during implementation; disposable lab lifecycle actions
were separately authorized and recorded in the execution audit.
