# AUDIT-05 — independent `v0.2.0` pre-release audit

**Date:** 2026-08-27

**Bead:** `tfp-34v`

**Status:** local remediation verified; hosted exact-SHA gates and publication
remain blocked

## Boundary and method

The audit starts at the last Terraform Registry release, `v0.1.1` commit
`c34fc02b96ed321b49a0ac10f61afbac22b582cb`, and ends at audited `main`
commit `37905257aeb1dc7590f5e149eddc6611b6d70211`. The boundary contains 14
commits and 167 changed files. It is deliberately wider than the most recent
pull request: publication exposes the complete delta from the version users
can install.

Three independent read-only reviews mapped that delta to:

- all nine Go packages, provider schema, semantic plan modifiers, transport,
  direct modules and gopls diagnostics;
- local tasks, six GitHub workflows, release tooling, downloaded executable
  assets, vulnerability scanners and branch protection;
- README, changelog, contract, standards, plan, Registry state, release history
  and current GitHub run evidence.

Exact identifiers came from `git`, paginated GitHub API queries, the Terraform
Registry API and upstream release assets. No publication workflow was
dispatched during the audit.

## Version decision

The next version is `v0.2.0`. The audited delta contains a `feat(provider)`
change and makes a semantic comparison stricter. Both map to MINOR under
[`naming-conventions.md`](../standards/naming-conventions.md) and
[`contract.md`](../contract.md). Before 1.0, the project's versioning standard
uses MINOR for this class of compatibility change. `v0.1.2` would therefore be
an incorrect release.

`VERSION` remains `0.1.1` and the changelog remains under `[Unreleased]` during
this remediation. Moving them, creating the annotated signed tag and
publishing are a separate, post-gate release commit.

## Independent findings

The provider-code review found no Critical or Important runtime defect. It did
find incomplete documentation of semantic attributes; the contract table now
names the missing DNS-name and TSIG multiset comparisons.

The delivery and documentation reviews found release blockers:

| Finding | Risk | Correction |
| --- | --- | --- |
| CI linted only `scripts/`, treated ty as advisory and never ran pytest | new release tooling could be untested while required CI was green | run the locked `task py` surface: Ruff and blocking ty over code and tests, then pytest |
| hosted pytest lacked the pinned Task binary used by the local gate | the real Taskfile expansion oracle passed locally but failed before execution on CI | install the development image's exact Task version in the Python job and bind it through the tool-version drift check |
| Security discarded OSV failures and Trivy did not fail on HIGH/CRITICAL findings | known vulnerable dependencies could coexist with a green Security workflow | preserve SARIF upload while returning each scanner's real status; update affected Go modules |
| Release required only CI and Acceptance | a tag could publish without the consumer-path E2E or Security result | require CI, Acceptance, End-to-end and Security from the exact `main` push SHA |
| duplicated version regexes accepted invalid SemVer and a source tag could be lightweight or unsigned | an immutable Registry version could start from an invalid version or unauthenticated source tag | one strict SemVer 2.0.0 parser; require an annotated tag that passes `git verify-tag` |
| the release trigger omitted valid build-metadata-only SemVer tags | a version accepted by the release checker could never enter its workflow | cover bare, prerelease and escaped literal-plus build-metadata tag forms; keep the strict parser as the final grammar authority |
| the release gate verified the source tag before a clean runner had its public key | a valid tag could not pass; importing the private key early would expose signing material before ancestry checks | commit the Registry-published public key and verify its fingerprint in the gate; keep the private key exclusively in the final GoReleaser job |
| signing secrets were repository-scoped | a workflow from an arbitrary SemVer tag could bypass the honest gate graph and read the Registry key | bind GoReleaser to a protected `release` environment; P10-15/tfp-34v.1 blocks publication until the secrets are moved there and repository copies removed |
| workflow and development-image downloads were TLS-only | a compromised or replaced release asset could execute before any repository gate | verify exact per-platform SHA-256 before extracting or installing every downloaded executable |
| public documentation named `v0.1.0`, pending Registry publication, obsolete scripts and stale pipeline behavior | consumers and maintainers would follow a contract that no longer exists | reconcile README, changelog, contract, standards, plan and this audit in the remediation change |

The checksum values were independently recomputed from the official Terraform,
OpenTofu, Terragrunt, shellcheck, hadolint and uv release assets and the
NodeSource repository key before being written into workflows or the
development image.

## Baseline evidence

Before remediation, the exact audited `main` passed locally in the worktree
development container:

- `gopls check` for all 85 Go files: no diagnostics;
- `task all`: green, including 260 Python tests and no govulncheck finding;
- `task verify AUTH=5.1` and `task verify AUTH=5.0`: green;
- local E2E: 59/59 passed in 17.81 seconds, followed by successful teardown;
- `task osv-scan`: zero affected packages;
- `task release:dryrun`: 13 archives, their digests and the Registry manifest
  verified successfully.

Those results establish the provider and consumer path at the baseline; they
do not approve the remediation bytes. Fresh focused, aggregate, image,
acceptance, E2E and release checks are required after the corrections.

The audited `main` SHA had successful GitHub runs: CI `33050786698`, Acceptance
`33050786739`, Coverage `33050786692`, End-to-end `33050786785`, Security
`33050786948` and Scorecard `33050786774`. These runs predate the remediation
and cannot authorize its publication.

## Remediated-candidate evidence

The final local candidate passed the required executable checks on 2026-08-27:

- `task all`, including race tests, 290 Python tests, Ruff, blocking ty,
  actionlint, zizmor, shellcheck, hadolint, Semgrep, documentation checks and
  govulncheck;
- `task verify AUTH=5.1` and `task verify AUTH=5.0`, including all four lab
  services and provider acceptance on PostgreSQL and LMDB;
- the consumer-path E2E suite: 59/59 passed in 17.77 seconds for Terraform,
  OpenTofu, Terragrunt, S3 state, the authenticated HTTPS module source and the
  `v0.1.1` to `v0.2.0` state upgrade; fixture and lab teardown then completed;
- OSV: zero affected packages; pinned Trivy 0.74.0: zero HIGH/CRITICAL
  vulnerabilities, secrets or misconfigurations in the clean-checkout source
  boundary;
- a complete development-image rebuild from the changed Containerfile,
  including successful verification of every pinned downloaded asset;
- `task release:dryrun`: all 13 release archives matched their recorded
  digests and the Registry manifest matched the repository.

The pre-cut `task release:check VERSION_ARG=0.2.0` remains deliberately red on
the release-only conditions that the separate release commit must satisfy:
`VERSION` still says `0.1.1`, the changelog has no `[0.2.0]` section, copied
provider constraints still describe the published `0.1.x` line, and this review
worktree is uncommitted. The checker requires the release commit to change the
README, provider example, generated Registry index and Terragrunt standard to
`~> 0.2` together. The `v0.2.0` tag is absent, released changelog sections
remain unchanged, and protocol 6 matches the manifest.

## Release decision

Publication is fail-closed. The local execution boundary is complete; a fresh
independent diff review still must approve the exact candidate bytes. A pull
request must then pass the amended exact-SHA GitHub gates before a separate
`v0.2.0` release commit and annotated signed tag are considered. The tag is
also blocked on `tfp-34v.1`. The live GitHub environment now has a selected tag
policy, the authenticated sole maintainer as its required reviewer,
`prevent_self_review=false`, and `can_admins_bypass=false`. Self-approval is the
only workable manual gate for this single-author repository and is explicitly
not independent review. Publication remains blocked until both GPG secrets are
environment-scoped and their repository-scoped copies are absent.
