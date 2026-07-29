# Changelog

All notable changes to this project are documented here.

The format follows [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/)
and this project adheres to [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).
The mapping between commit type, changelog section and version bump is in
[`docs/standards/naming-conventions.md`](docs/standards/naming-conventions.md) §1.

## [Unreleased]

## [0.1.1] — 2026-07-29

### Fixed

- The Terraform Registry refused `v0.1.0` with "missing files in request body",
  naming all thirteen SBOMs. It parses `SHA256SUMS` as the list of files
  belonging to the version, and goreleaser had folded the SBOMs into it — so
  every line the Registry could not resolve to a file it accepts failed the
  whole submission. `checksum.ids` now restricts the file to the archives; the
  SBOMs are still published beside them.

  Nothing caught it because nothing asked the right question. Every check
  established that the artefacts were correct — signed, digested, complete —
  and none established that they were the ones the Registry ingests.
  `scripts/check-release-artifacts.sh` asks that now, of a snapshot build,
  before a tag exists: `SHA256SUMS` may list archives and the manifest and
  nothing else, every archive must match its digest, and the manifest must be
  the repository's own and declare a protocol.

### Added

- `syft` in the dev image, and `task release:dryrun`. The release could not be
  rehearsed locally — the image had no syft, so `goreleaser release --snapshot`
  failed on a machine where the failure was cheap and succeeded in CI where it
  was not.


### Added

- `CODEOWNERS`, a pull-request template, issue templates and a code of conduct
  — the four files GitHub looks for, and the ones that tell somebody arriving
  how this project works before they have read a standard. They carry the rules
  that already applied: `task verify` for any resource change, `docs/plan.md`
  updated in the same commit, and which backend a bug was seen on, because
  gpgsql and LMDB differ.
- A "Using it" section in the README, with the `required_providers` block.
- Branch protection on `main`: nine required checks, branches current before
  merge, linear history, no force-push or deletion. Administrators are not
  included on purpose — a required check hanging on somebody else's outage
  would otherwise leave a one-maintainer project unable to merge the fix.
- Repository settings brought in line with the documented workflow: squash-only
  merges, branches deleted after merge, secret scanning and push protection on,
  topics and homepage set.

### Fixed

- The README said "Status: in development … Not yet published to the Terraform
  Registry", under a heading called *Planned* surface, on a repository with a
  signed `v0.1.0` and every API operation covered.

## [0.1.0] — 2026-07-29

First release. 29 artefacts across 13 platforms, `SHA256SUMS` signed with an
RSA-4096 key, an SBOM beside every archive, protocol 6.


### Added

- Repository foundation: `AGENTS.md` with `CLAUDE.md` and `CODEX.md` as
  symlinks, ten standards, seven architectural decision records, the delivery
  plan and the methodology.
- Naming standard synthesised from Harvard HMS, IT Glue, SemVer 2.0.0,
  Conventional Commits 1.0.0 and Keep a Changelog 1.1.0. Its core is the chain
  that carries one name from a branch through a commit type into a changelog
  section and out into a version bump, plus a table of which rules are enforced
  mechanically.
- Dev image on `golang:1.26-trixie` pinned by digest, carrying Go 1.26.5,
  golangci-lint v2.12.2, Task, Terraform, OpenTofu, Terragrunt, tfplugindocs,
  goreleaser, and the uv/ruff/ty Python toolchain.
- Five-service lab: Authoritative on PostgreSQL 17 and on LMDB, Recursor with
  `api_dir`, dnsdist, and PostgreSQL. Every image pinned by `sha256` digest.
  Driven through podman-py.
- `scripts/check-pins.sh` — every image digest and Action SHA must resolve, and
  nothing may float. Verified against a fixture containing one fabricated
  digest and one floating tag.
- GitLab CI as the quality gate: build, unit, contract, lint across Go, Python
  and pins, security, and an acceptance matrix across both authoritative
  backends.
- GitHub Actions for release only — goreleaser, GPG signing, registry
  publication.
- `golangci-lint` v2 with an explicit allowlist of 82 linters and no path
  exclusions: the first line of code faces the full gate.
- Empty provider on `terraform-plugin-framework` v1.19.0, protocol 6, with the
  three-product configuration schema.

- HTTP transport shared by all three products: `X-Api-Key`, a TLS 1.2 floor,
  timeouts, and a retry policy that backs off on `5xx`, `429` and transport
  errors while failing fast on `4xx` — a `4xx` is an answer, and retrying it
  turns a fast failure into a slow one.
- Capability classifier. Four distinct "this installation cannot do that"
  conditions across the three products each surface as a bare `4xx`; the
  transport classifies them once, so every resource reports the same actionable
  diagnostic naming the setting to change.
- `internal/testutil` — the contract test layer: recorded HTTP fixtures, a mock
  server that replays them, and an OpenAPI cross-check. Ten fixtures recorded
  from the five-service lab. The layer needs no containers, so it runs on every
  commit; the lab is needed to record a fixture, not to use one.
- `deployments/compose/compose.dev.yml` — the development container. Host
  networking, so a lab endpoint means the same thing inside it and out.
- `task fixtures:record` — re-records the fixtures against a running lab.
  Deliberately manual and in no gate: a fixture that re-records itself is not a
  fixture.

- `internal/api/auth` — the Authoritative client, starting with the ten zone
  operations: list, search by name, create, read, update, patch rrsets, delete,
  notify, axfr-retrieve, export and rectify. Twelve contract tests against
  recorded fixtures, no containers needed.

- `internal/api/auth` metadata, cryptokeys, tsigkeys and autoprimaries — 19
  more operations, 29 of the 68 now covered. Twenty-three contract tests.
- `TestFixturesCarryNoKeyMaterial` — a recorded fixture may not contain a
  DNSSEC private key or a TSIG secret. Fixtures are committed, so a bad one
  means rewriting history rather than deleting a file. Verified by planting a
  fixture holding a private key and watching the check fail.

- `internal/api/auth` views, networks, server, config, statistics, search and
  cache flush — the Authoritative client is complete at all 42 operations,
  31 contract tests. `TestSurfaceIsComplete` fails if an operation is dropped.

- `internal/api/rec` — the Recursor client, all 16 operations, 11 contract
  tests. The two writable settings are named constants rather than a
  `Get(name)`/`Set(name)` pair, because every other name answers 404 and the
  client refuses those before reaching the wire.

- `internal/api/dnsdist` — the dnsdist client, all 10 operations of which 2
  write, 10 contract tests. Phase 2 closes at 68 of 68 operations across the
  three products, with 52 contract tests over 26 recorded fixtures.

- `powerdns_zone` — the first resource, with create, read, update, delete and
  import. Acceptance tests on both authoritative backends assert an empty plan
  after apply, which is the property the whole normalisation layer exists for.
- `internal/provider/normalise` and `internal/provider/planmodify` — semantic
  comparison for zone kinds, DNS names, IP addresses, CIDRs, Recursor
  upstreams and unordered lists, and the plan modifiers that apply them. Each
  is tested for what it must suppress *and* for what it must not.
- The provider now builds its three clients and hands them to resources, with
  a diagnostic naming the missing argument when a product is unconfigured. It
  had carried a `TODO(phase-1)` in `Configure` since phase 0.

- `powerdns_record` — an RRSet rather than a single record, because PowerDNS
  has no per-record identity and two resources on one name would silently
  overwrite each other. Acceptance covers address normalisation, multi-value
  sets, ordering, `disabled` surviving a round trip, and exact comparison for
  `TXT`.

- `powerdns_zone_metadata` — one metadata kind per resource, so the
  `SOA-EDIT-API` PowerDNS assigns itself is never deleted. Acceptance asserts
  it survives.
- `powerdns_zone` and `powerdns_zones` data sources, for zones the
  configuration reads but does not own.
- `docs/contract.md` — what the provider promises to users and what it does
  not: resource identity, the semantic comparisons users rely on, the four
  capability diagnostics, the non-goals, and which change bumps which version
  component. The methodology called this a hard external contract from the
  start; it had never been written down.

- `powerdns_record`, `powerdns_zone_metadata` and `powerdns_zone_export` data
  sources. Phase 3 closes with three resources, five data sources and fourteen
  acceptance tests on both authoritative backends.
- `SOA-EDIT-API` and `API-RECTIFY` are rejected before the request with a
  diagnostic naming the zone attribute to use instead. Both appear in a zone's
  metadata collection and answer 422 by name; the server's message does not
  mention that the value is settable elsewhere.

- `powerdns_zone_cryptokey` — DNSSEC keys, reconciled against the collection
  endpoint so no private key can reach state. Enforced by a test that reads
  state and fails on an attribute named for key material or a value carrying a
  private-key header, rather than by reasoning about which endpoint is called.
- `powerdns_zone.dnssec` is Computed with no default. Creating a key turns
  DNSSEC on server-side, and a default of `false` made the zone plan to turn it
  back off on every run.

- `powerdns_tsigkey`, with the secret as a write-only attribute — Terraform
  sends it to the provider and stores it in neither state nor the plan file.
  Leaving it unset has PowerDNS generate one, which is then unreadable through
  the provider.
- Zone DNSSEC attributes: `nsec3param`, `nsec3narrow` and `presigned`.
- A behaviour test: a zone signed through the provider is queried with
  `dig +dnssec` and must answer with an RRSIG. Every other test asserts what an
  API returned; this one asks the DNS server.

- Ephemeral `powerdns_cryptokey_material` and `powerdns_tsigkey_secret` — the
  only place the provider returns key material, and safe because Terraform
  discards an ephemeral value rather than persisting it. Phase 4 closes.
- Zone `master_tsig_key_ids` and `slave_tsig_key_ids`, compared as canonical
  key names and ignoring order.

- ADR 0008: reviews happen on GitHub only. `.gitlab-ci.yml` has never run —
  no GitLab remote was ever configured — and stays as the gate definition for a
  mirror that does not exist. One pull request per sprint, squash-merged.

- `powerdns_view_zone`, `powerdns_network` and `powerdns_autoprimary`. Views
  and networks need the LMDB backend; on a relational one the diagnostic names
  the requirement rather than repeating a 422.
- Negative tests asserting each capability diagnostic by its **text**. A test
  that only checked "this errored" would pass for a provider that surfaced a
  bare status code.

- `powerdns_recursor_zone`, `powerdns_recursor_acl` and `powerdns_dnsdist_acl`.
  The family surface is complete: all three products have resources.
- Repository hooks and skills, so the rules in `AGENTS.md` are enforced rather
  than only written: `main` is blocked for direct commits, and a `file:line`
  citation without a revision is warned about.

- Actions: `powerdns_notify_zone`, `powerdns_axfr_retrieve`,
  `powerdns_rectify_zone` and `powerdns_flush_cache`. Terraform 1.14 or later.
  The capability map listed 24 operations as uncoverable because Terraform had
  nowhere to put an imperative operation; actions cover 19 of them.
- Functions: `fqdn`, `is_fqdn`, `reverse_zone_name`, `ptr_name` and
  `soa_serial`. Pure and offline — a data source would make a plan depend on a
  server for an answer that is a string operation.

- Resource identity on the nine resources with a stable natural key, and
  import by identity for zones and records. The two ACL resources deliberately
  have none: their key is the same on every installation, so any identity would
  be false.

- `powerdns_recursor_zone` and `powerdns_dnsdist_server` data sources. Both
  products are mostly read-only over HTTP, so most of what they expose is only
  useful to read — dnsdist's downstreams and pools are Lua or YAML and have no
  write path at all.
- Examples for the whole surface — 11 resources, 7 data sources, 5 functions,
  2 ephemeral resources and the provider — and generated registry
  documentation, validated with `tfplugindocs validate`.

- The quality gate runs in GitHub Actions. `ci.yml` mirrors `task all` job for
  job, so a failure it reports is one a developer can reproduce before pushing;
  `acceptance.yml` starts the five-container lab on a hosted runner and runs
  the acceptance suite against it; `security.yml` adds CodeQL, Semgrep,
  osv-scanner and Trivy, reporting into code scanning; `scorecard.yml` and
  `dependency-review.yml` cover supply chain and licence.
- `scripts/check-tool-versions.sh` — the workflows install the same tool
  versions the dev image is built with, or the gate fails. Versions live in
  `Containerfile.dev`; a workflow line naming one carries `# pin: <ARG>`. A
  bumped version and a deleted marker are both caught.
- `task lint:actions`, `task lint:shell`, `task lint:containers` — actionlint,
  zizmor, shellcheck, shfmt and hadolint. The scripts and the Containerfile had
  no linter at all; a workflow is otherwise only executed by pushing it.
  Between them they found a dead variable in `check-pins.sh`, five
  `readonly X="$(cmd)"` that swallow the command's exit status, a `curl | gpg`
  that could not fail the image build, sixteen checkouts leaving the token in
  `.git/config`, and a module cache restored into the release job that signs
  the artefacts.
- `yamllint`, `markdownlint-cli2`, `cspell`, `commitlint`, `podman-compose` and
  `podman-py` are pinned. They were installed at `@latest` in the dev image
  since phase 0, so "everything is pinned" had six exceptions nobody had
  counted.
- The ban on AI attribution is enforced on the branch. The `commit-msg` hook
  only ever protected a clone that had run `task hooks`; CI now checks every
  commit in a pull request, and its title and body, because those become the
  squash message.
- Dependabot for Go modules and Actions, grouped so a reviewer reads them.
  Container digests are excluded: the pinned PowerDNS versions are the
  reference the provider's behavioural claims were measured against, and moving
  one is a decision to re-verify those claims.
- Release artefacts now carry an SBOM per archive, generated by syft.
- A release gate. `release.yml` built, signed and published after `go test` and
  nothing else; the Registry treats a published version as immutable, so that
  was the one path where a mistake could not be corrected. Nothing is built now
  until the signing secrets exist, `VERSION` agrees with the tag, the changelog
  has a non-empty section for it, the manifest's protocol matches what the
  provider serves, the tag is an ancestor of `main`, `CI` and `Acceptance` were
  green for that exact commit, the generated documentation is in sync, and the
  tree is clean. `task release:check` answers the local half before the tag is
  pushed.
- A CI badge on the README, reporting `ci.yml`. S0-21 removed one because there
  was no CI to report; there is now, and `check-badges.sh` keeps a badge's query
  string when it verifies the endpoint — without it the check asks a different
  question than the badge does and rejects one that renders correctly.
- `check-badges.sh` recomputes the delivery plan's task counter from its own
  tables, retries a badge three times before reporting it, and distinguishes
  "shieldcn did not answer" from "this badge is wrong" — the first is somebody
  else's outage and is not fatal, the second is a broken image on the front
  page. CI reported an outage as a failure the first time it ran. The audit found that badge reading 67 against 62; recomputing it
  fixed the number, this stops it drifting again.

### Removed

- `.gitlab-ci.yml`. It had never run — no GitLab remote was ever configured —
  and by the time it was removed it called two scripts that do not exist, ran
  the contract tests with a build tag they do not carry, and split the
  acceptance matrix in a way the suite has not used since phase 5. An
  unexecuted pipeline does not stay correct; it reads as a gate while enforcing
  nothing. See ADR 0009.

### Fixed

The pre-merge gate had never run end-to-end, because the compose file every
`task` target depends on was missing. Adding it surfaced six failures that the
gate had been unable to report:

- `compose.dev.yml` did not exist, while `DC`, `EXEC` and the `_dev-running`
  precondition all referenced it. Every containerised target was unreachable.
- `test:contract` ran `go test -tags contract ./internal/api/...` and matched
  nothing: no file carried the tag and the contract tests live in
  `internal/testutil`. It now runs the tests it names.
- `tf:fmt` and `tf:fmt:check` failed on the absence of `examples/`, which
  arrives in phase 7. They now skip when there is nothing to format.
- `MD060` failed on every table in the repository: the delimiter rows were
  compact while the content rows were spaced. All 18 affected files normalised
  to `| --- | --- |`.
- `MD042` failed on 36 badges linking to `](#)` — clickable, leading nowhere,
  jumping the reader to the top of the page. Badges without a destination are
  now images.
- `MD013` failed on the two header URLs, which cannot be wrapped, in each of 22 files. The
  exemption is now scoped to the header block rather than the whole file, so
  the prose is still held to 100 columns.
- `cspell` reported 197 unknown words against an `en` dictionary while the
  prose is written in British English. Adding `en-GB` resolved 135 of them; the
  remaining 62 are technical identifiers, now listed and grouped by origin.
- `markdownlint` and `cspell` both linted `.venv/`, which uv materialises in
  the working tree. Ignored, and added to `.gitignore`.

- The `commit-msg` hook was configured but never installed. `.git/hooks` is not
  tracked, so every clone of this repository — including the only one that
  exists — started unprotected, and the ban on AI attribution in `AGENTS.md`
  was enforced by nothing. `task hooks` now installs the hooks and hangs off
  `task up`; `scripts/check-hooks.sh` asserts in the gate that they are present
  and that the checker both rejects an AI trailer and accepts an ordinary
  message.
- The phase 2 table listed 40 of the 42 Authoritative operations and 8 of the
  16 Recursor ones, and double-counted zone actions across two rows. Recounted
  against the capability map; each row now carries its operation count so the
  68-operation exit gate can be checked without recounting.
- Plan corrections found by re-verifying it against the repository rather than
  against itself: the Taskfile has 40 tasks, not 37; `golangci-lint` has no
  *blanket* path exclusions but does carry five enumerated per-rule exemptions,
  which is a weaker claim than the one the plan made.
- The `gofumpt` pre-commit hook called a binary the dev image does not carry,
  so it could never have run. Replaced by `golangci-lint fmt`, which is already
  configured with `gofmt`, `gofumpt` and `goimports` — running the formatter
  from two configurations is how they come to disagree.
- `yamllint` reported 30 warnings against the vendored PowerDNS specification,
  whose formatting is not ours to fix. Excluded, with `.venv/`.

- Nine of the first ten commits violated the repository's own `commitlint`
  rules, having all been written before the hook existed. Rewritten; trees are
  byte-identical and authorship unchanged. `commitlint` now also runs in the
  GitLab pipeline, because a hook only guards clones that installed it.

- The `ws-auth.cc` line cited for the undocumented `POST cryptokeys/{id}` was
  given without a revision. It is right for `master` `a74d89a8`, which is what
  the upstream issue cites, and wrong for the pinned `auth-5.1.3`, where the
  same registration is line 3349. Both revisions are now stated. The upstream
  issue itself was checked and is correct.

- The transport classified **every** dnsdist 405 as a missing `setAPIWritable`.
  `isMethodAllowed` admits `GET` unconditionally, `PUT` only behind the flag,
  and `DELETE` only for `/api/v1/cache`; everything else falls through, so a
  `POST` answers 405 on a path that exists and has nothing to do with the flag.
  The diagnostic would have sent an operator to change a setting that would not
  help. Now scoped to `PUT`, with tests asserting it stays silent otherwise.
- Documentation said `setAPIWritable` gates *every* dnsdist write. It gates
  configuration writes; `DELETE /api/v1/cache` is admitted without consulting
  it, so a cache flush works on a server that refuses every config write.

- `csk` is compared as compatible with `ksk` and `zsk`. PowerDNS derives the
  key type from DNSKEY flags and the number of keys in the zone, so a sole key
  reads back as `csk` and is renamed when a second appears. Comparing the
  string literally would have made adding a second key replace the first —
  destroying the signing key of a live zone.

- Changing a TSIG algorithm now replaces the key. `PUT /tsigkeys/{id}` deletes
  the previous entry only when the name changed, so changing the algorithm
  alone left the old key in place and added a second under the same id — three
  PUTs produced three entries. The zone would then authenticate against
  whichever the backend returned first.

- The provider set `ResourceData` and `DataSourceData` but not
  `EphemeralResourceData`, so every ephemeral resource received a nil client
  bundle and panicked at apply. The `Require*` accessors now answer a nil
  bundle with a diagnostic naming the omission rather than dereferencing it.

Writing the workflows found four more, in things already believed correct:

- The release workflow ran GoReleaser at `latest`, on the one path where
  "whatever was current that day" is least acceptable. Pinned, and pinned to
  the version the dev image carries.
- It also took the release notes from whichever changelog section was on top
  rather than the one matching the tag, so a tag cut while `[Unreleased]` was
  open would have shipped that section as its notes. It now selects by version
  and fails if the changelog has no section for the tag.
- `check-pins.sh` matched `owner/repo@sha` greedily and so read
  `github/codeql-action/init` as the repository `codeql-action/init`. Every
  subpath action reported NOT FOUND while being correctly pinned. It now
  reduces to the first two segments, and rejects a floating subpath action,
  which the previous pattern could not see at all.
- `task semgrep` ran on the host, against AGENTS.md's own rule that the host
  carries no toolchain, and with no pinned version anywhere. It runs in the
  container at a pinned version — which is also what lets CI agree with it.

The first run of the workflows found four more, none of them typos:

- A job with `container:` gets `sh`, not `bash`, so `${GITHUB_SHA::8}` failed
  as "Bad substitution". Every workflow now sets `shell: bash`.
- The Go image has no `unzip` and Terraform ships as a zip. That job moved to
  the runner, which has both.
- `check-pins.sh` treated any `gh` failure as "this commit does not exist", so
  one valid action SHA out of twenty-four reported as NOT FOUND and a
  rate-limited run would have reported all of them as fabricated. It retries,
  and distinguishes a 404 from a failed call.
- `markdownlint-cli2`, `cspell` and `commitlint` were installed at `@latest` in
  CI and in the dev image, and `podman-compose` was unpinned. All pinned, and
  installed with `--ignore-scripts`; downloads refuse a redirect off HTTPS, and
  `UV_NO_BUILD` stops `uv` building a source distribution, which is arbitrary
  Python at install time.

And one that could not be fixed by trying harder: `check-pins.sh` cannot verify
`aquasecurity/trivy-action`'s commit SHA from a hosted runner, because that
organisation has an IP allow list and the API answers 403 for every runner
address. A pin nothing can check is not a pin, and a named exception in the
checker for one action is how a rule becomes a preference — so the scan runs
from the published trivy image, pinned by digest, which resolves from anywhere.

### Security

- Four reachable vulnerabilities, all reached through indirect dependencies of
  `terraform-plugin-framework`: `golang.org/x/net` to v0.55.0 (GO-2026-5026),
  `golang.org/x/text` to v0.39.0 (GO-2026-5970), and `google.golang.org/grpc`
  to v1.82.1 (GO-2026-4762, GO-2026-6061). `govulncheck` now reports zero.

- One dev container per checkout. The container was bind-mounted on whichever
  tree started it and its name was fixed, so a worktree's `task test` compiled
  the code in `main`. Found on the first sprint to actually use a worktree.

- Removing an ACL resource leaves the setting on the server and warns. There is
  no unset state for an ACL, and writing an empty list would refuse every
  client.

- A zone created with `nameservers` cannot round-trip through an import block:
  the attribute is create-only and never read back, so the difference forces a
  replacement. Use `terraform import`, or manage NS records with
  `powerdns_record`.

### Notes

- semgrep's `uv-missing-dependency-cooldown` is knowingly not applied.
  `exclude-newer` makes this project's exact version pins unsatisfiable —
  `ruff==0.16.0` was published inside a seven-day window — and resolution fails
  rather than falling back. Exact pins, a committed `uv.lock` with hashes and a
  digest-pinned base image cover reproducibility; they do not cover a release
  compromised before it was pinned, which is the gap the rule names and this
  project accepts. The reasoning is in `pyproject.toml` beside the setting.

- Phases 0 to 4 were committed directly to `main`, which contradicts the
  workflow `AGENTS.md` has stated since phase 0. Recorded rather than quietly
  corrected; the rule holds from phase 5 onward.

- ADR 0006 records two dnsdist findings that are absent from its documentation
  and were discovered while standing up the lab: `setAPIWritable`, not
  `apiConfigDir`, gates every write, and `DELETE /api/v1/cache` answers `404`
  when the pool has no packet cache.
- The vendored OpenAPI document is a cross-check, never a source of code, and
  nothing is generated from it. Four divergences between it and the
  implementation are now recorded in `docs/plan.md` §Phase 1. Two were reported
  as [PowerDNS/pdns#17807](https://github.com/PowerDNS/pdns/issues/17807); two
  more were found by this cross-check itself and are neither visible to a
  reader nor yet reported: `Record.modified_at` is misindented into a stray
  sibling key, and `autoprimaries_url` is sent by every `Server` object while
  the schema omits it under `additionalProperties: false`.

[Unreleased]: https://github.com/ioplane/terraform-provider-powerdns/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/ioplane/terraform-provider-powerdns/releases/tag/v0.1.1
[0.1.0]: https://github.com/ioplane/terraform-provider-powerdns/releases/tag/v0.1.0
