# Changelog

All notable changes to this project are documented here.

The format follows [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/)
and this project adheres to [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).
The mapping between commit type, changelog section and version bump is in
[`docs/standards/naming-conventions.md`](docs/standards/naming-conventions.md) §1.

## [Unreleased]

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

### Security

- Four reachable vulnerabilities, all reached through indirect dependencies of
  `terraform-plugin-framework`: `golang.org/x/net` to v0.55.0 (GO-2026-5026),
  `golang.org/x/text` to v0.39.0 (GO-2026-5970), and `google.golang.org/grpc`
  to v1.82.1 (GO-2026-4762, GO-2026-6061). `govulncheck` now reports zero.

### Notes

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

[Unreleased]: https://github.com/ioplane/terraform-provider-powerdns/commits/main
