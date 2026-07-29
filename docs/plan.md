<!-- markdownlint-disable MD013 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=Delivery+plan&subtitle=Live+execution+record&logo=githubactions&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="Delivery plan" src="https://shieldcn.dev/header/graph.svg?title=Delivery+plan&subtitle=Live+execution+record&logo=githubactions&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>
<!-- markdownlint-enable MD013 -->

<div align="center">

![phase 9_of_10](https://shieldcn.dev/badge/phase-9_of_10-0969da.svg?variant=secondary)
![phases_closed 8](https://shieldcn.dev/badge/phases_closed-8-3fb950.svg?variant=secondary)
![tasks_done 95](https://shieldcn.dev/badge/tasks_done-95-3fb950.svg?variant=secondary)
[![last-commit](https://shieldcn.dev/github/last-commit/ioplane/terraform-provider-powerdns.svg?variant=secondary)](https://github.com/ioplane/terraform-provider-powerdns/commits/main)

</div>

# Delivery plan

Living document. The method is in [`methodology.md`](methodology.md) and what the
provider promises to users is in [`contract.md`](contract.md); this is
its execution record. **A task's status changes in the commit that does the
work**, never retrospectively — a plan updated afterwards is a report, not a
control.

**Status:** phase 8 — the gate now runs in GitHub Actions. Phase 7 stays open
alongside it: the version matrix and the signed release are release decisions,
not CI work.
**Last updated:** 2026-07-29

## How a sprint runs

One worktree per sprint, one pull request per sprint, squash-merged.

```console
task worktree:new BRANCH=sprint/<phase>-<name>
# ... work, task all, task verify ...
gh pr create --fill
# ... squash-merge ...
task worktree:rm BRANCH=sprint/<phase>-<name>
```

Phases 0 to 4 were committed directly to `main`. That contradicted
`AGENTS.md`, which has said `main` is never committed to directly since phase
0. Recorded here rather than quietly corrected: the rule holds from phase 5,
and the history stands as it happened.

## Legend

| Mark | Meaning |
| --- | --- |
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
| --- | --- | --- | --- | --- |
| S0-01 | Repository `ioplane/terraform-provider-powerdns`, Apache-2.0 | PM | — | `[x]` |
| S0-02 | `AGENTS.md`, symlinks `CLAUDE.md` and `CODEX.md` | PM | S0-01 | `[x]` |
| S0-03 | Naming standard synthesised from the five sources | PM | — | `[x]` |
| S0-04 | Standards: versioning, commits, changelog | PM | S0-03 | `[x]` |
| S0-05 | Standards: Go 1.26, provider practices, Terragrunt, API discipline, Python, verified identifiers | ARC | — | `[x]` |
| S0-06 | ADR 0001–0007 | ARC | — | `[x]` |
| S0-07 | Dev image on `golang:1.26-trixie`, pinned by digest | OPS | — | `[x]` |
| S0-08 | Five-service lab, every image pinned by digest | OPS | S0-07 | `[x]` |
| S0-09 | `lab.py` on podman-py: up, down, status, verify | OPS | S0-08 | `[x]` |
| S0-10 | Taskfile — 40 tasks, guards on container, lab and hooks | OPS | S0-07 | `[x]` |
| S0-11 | `golangci-lint` v2, allowlist of 82, **no blanket path exclusions** | OPS | — | `[x]` |
| S0-12 | Python gate: uv, ruff, ty, `pyproject.toml` | OPS | S0-07 | `[x]` |
| S0-13 | `scripts/check-pins.sh` — digests and action SHAs resolve, none float | OPS | — | `[x]` |
| S0-14 | `scripts/check-no-ai-attribution.sh` and the `commit-msg` hook | OPS | — | `[x]` script written; **installation was not — see S1-10** |
| S0-15 | GitLab CI: build, test, lint, security, acceptance matrix | OPS | S0-10 | `[x]` |
| S0-16 | GitHub Actions: release only — goreleaser, GPG, registry | OPS | S0-10 | `[x]` |
| S0-17 | `main.go` + empty framework provider, protocol 6, three-product schema | DEV | S0-10 | `[x]` |
| S0-18 | `CHANGELOG.md`, `VERSION`, README, CONTRIBUTING, SECURITY, docs index | PM | — | `[x]` |
| S0-19 | **Added.** Markdown standard: template, shieldcn badges, mermaid; `check-badges.sh` | PM | S0-18 | `[x]` |
| S0-21 | **Added.** Badges corrected: dynamic over static, default size, clickable, no erroring endpoint | PM | S0-19 | `[x]` |
| S0-20 | **Added.** Correct commit authorship to the `gh auth` identity | OPS | — | `[x]` |

**Exit gate met.** `go build` clean, `golangci-lint` 0 issues, `check-pins.sh`
verifies 10 references, `lab:verify` green across five services. The Taskfile
parses with 40 tasks.

On S0-11: `exclusions.paths` is `["$^"]`, a pattern that matches nothing, so no
file is exempt from the linter as a whole. What does exist is five enumerated
per-rule exemptions — `funlen` off inside framework `Schema` methods, `gosec`
and friends off in `_test.go`, `paralleltest` off in `_acc_test.go` — each
naming both the path and the linters it relaxes. That is a weaker claim than
"no exclusions" and the row now makes it.

**Two of my own mistakes were caught by the gate I had just written.** The
GitLab pipeline went in with placeholder digests of all zeros, and
`check-pins.sh` itself had a resolution bug — it handed skopeo a reference
carrying both a tag and a digest, which skopeo rejects, so five valid digests
reported as not found. Both fixed; the checker is now verified against a
fixture containing one fabricated digest and one floating tag.

**S0-21** corrected the first attempt, twice. The badges went in as bare
`<img>` rather than links, and static where a dynamic endpoint existed —
`status-phase_1_of_8` is a claim that rots, `github/last-commit` is a fact.
Sizing then overshot: `xs` was unreadable, `lg` dominated the heading it
followed. The answer was to set no size at all and take the `sm` default, which
is what the sibling renders.

It also removed a CI badge that would have shipped broken. `github/ci` answers
`200` for this repository but renders "not found", because the quality pipeline
is GitLab and GitHub holds only the release workflow. `check-badges.sh` now
fetches the `.json` behind every dynamic badge rather than trusting the status
code, and is verified against exactly that case.

**S0-19** came from the observation that a standards-heavy repository fails in a
specific way: every document is individually reasonable and collectively
unnavigable. The template fixes classification, not prose — a reader can tell a
binding standard from a design note from a status record without reading past
the first screen. Badges come from [shieldcn](https://shieldcn.dev), diagrams
are mermaid, and `scripts/check-badges.sh` verifies both, excluding fenced
examples so a placeholder cannot pass.

**S0-20** corrected an error of mine. Every commit had been authored with an
address passed on the command line rather than the one `git config` and
`gh auth` agree on. Rewritten here and in the sibling repositories; the seven
branches that are heads of open upstream pull requests are deliberately left
alone, because force-pushing them would disturb a third party's review in
flight.

**S0-08 already earned its keep.** Standing up dnsdist found that
`setAPIWritable`, not `apiConfigDir`, gates every write — without it each `PUT`
answers `405` — and that `DELETE /api/v1/cache` answers `404` when the pool has
no packet cache. Neither is in the documentation. Recorded in
[ADR 0006](adr/0006-dnsdist-scope.md).

---

## Phase 1 — Transport · `[x]` closed 2026-07-28

**Exit gate:** contract tests pass against fixtures recorded from all three
products. **Met** — ten fixtures recorded from the five-service lab, replayed
without containers, and cross-checked against the vendored specification.

| ID | Task | Role | Depends | Status |
| --- | --- | --- | --- | --- |
| S1-01 | ARC: error taxonomy and capability classification, signed | ARC | — | `[x]` |
| S1-02 | `transport.Client`: HTTP, `X-Api-Key`, TLS floor, timeouts | DEV | S1-01 | `[x]` |
| S1-03 | Retry: `5xx` and transport errors back off; `4xx` fails fast | DEV | S1-02 | `[x]` |
| S1-04 | `transport.APIError`: status, server message, capability class | DEV | S1-01 | `[x]` |
| S1-05 | Capability classifier: LMDB-only, `api_dir` unset, `setAPIWritable` unset, no packet cache | DEV | S1-04 | `[x]` |
| S1-06 | Fixture recorder — capture real responses from the lab into `testutil` | QA | S0-08 | `[x]` |
| S1-07 | Contract tests for every error class | QA | S1-05, S1-06 | `[x]` |
| S1-08 | `compose.dev.yml` — the file every `task` target already assumed | OPS | S0-04 | `[x]` |
| S1-09 | Make `task all` pass end to end: six latent gate failures | OPS | S1-08 | `[x]` |
| S1-10 | Install the git hooks S0-14 only configured; `check-hooks.sh` in the gate | OPS | S0-14 | `[x]` |
| S1-11 | Rewrite the nine commit messages that predate the hook; `commitlint` in CI | OPS | S1-10 | `[x]` |

The classifier is verified twice over. `classify_test.go` asserts the mapping,
including the cases that must **not** fire — a rule that classifies too eagerly
tells an operator to change a backend setting when the real problem is a typo.
`live_acc_test.go` asserts the premise against real servers: if PowerDNS
changes a status code, the unit tests keep passing while the provider silently
stops explaining itself.

S1-05 is the piece the whole design turns on. Four different "this installation
cannot do that" conditions exist across the three products; each surfaces as a
bare `4xx`. Classifying them once means every resource reports the same
actionable diagnostic.

### The gate had never run

S1-08 is a correction, not a feature. `Taskfile.yml` referenced
`deployments/compose/compose.dev.yml` from `DC`, `EXEC` and the `_dev-running`
precondition, and the file did not exist — so every containerised target was
unreachable and phase 0's work had only ever been verified by running the tools
by hand on the host.

Writing it turned `task all` green for the first time, and in doing so exposed
six failures the gate had been unable to report: a `test:contract` target
matching no files, `terraform fmt` failing on a directory that arrives in phase
7, three markdownlint rules failing repo-wide, and `cspell` running an American
dictionary over British prose. All are listed in the changelog.

The lesson is in [`docs/standards/verified-identifiers.md`](standards/verified-identifiers.md):
a gate that has not been observed to fail is not known to run.

S1-10 is the same failure a second time, and it mattered more. S0-14 was marked
done on the strength of `.pre-commit-config.yaml` and
`scripts/check-no-ai-attribution.sh` both existing. Neither is a hook.
`.git/hooks` is not tracked, so a fresh clone starts unprotected and this one
always had been: the ban on AI attribution — a binding rule in `AGENTS.md`, and
the one the author cares about most — was enforced by nothing at all.

`task hooks` now installs them and hangs off `task up`, so the first command
anybody runs arms the ban. `scripts/check-hooks.sh` asserts both halves in the
gate: the hooks are present, **and** the checker rejects a message carrying an
AI trailer while accepting an ordinary one. Testing only the rejection would
pass for a checker that rejects everything.

Installing the hook then rejected the very commit that installed it — header 91
characters, `hooks` not in the scope enum, body lines over 72 — which is the
most direct evidence available that it works. Running `commitlint` over the
history it had never guarded found **nine of the first ten commits in
violation**, mostly on header and body length.

S1-11 rewrote all nine. Trees are byte-identical before and after
(`1d626ab938cd`), authorship is unchanged, and every commit now passes. The
pre-rewrite history is kept locally on `backup/pre-commitlint-rewrite`.

A hook only guards commits made in a clone that ran `task hooks`, so the branch
needs its own check: `.gitlab-ci.yml` now runs `commitlint --from` the merge
base on every merge request and on the default branch.

### The test scaffolding

Three pieces, in `internal/testutil`:

- **`fixture.go`** — a recorded request and response, carrying
  `recorded_against` so a stale fixture is visible rather than merely old.
  Recording is manual (`task fixtures:record`) by design: a fixture that
  re-records itself is not a fixture, because the value is that a change in
  what PowerDNS sends arrives as a diff somebody reads.
- **`mock.go`** — replays fixtures over HTTP. It **fails the test** on an
  unmatched request rather than answering 404, because a 404 would be
  indistinguishable from a genuine not-found fixture and would quietly pass.
- **`spec.go`** — cross-checks recorded responses against the vendored
  Authoritative specification with `kin-openapi`.

Ten fixtures: five Authoritative, three Recursor, two dnsdist. The contract
layer needs no containers, so it runs on every commit; the lab is needed to
*record* a fixture, not to use one.

### The specification is a cross-check, never a source of code

The vendored OpenAPI document exists to catch drift in the shapes we use. It
does not decide whether an endpoint exists, and no code is generated from it.
It cannot be trusted for either, as this project has now demonstrated nine
times. Six of the nine were found by this machinery rather than by reading, and
five of those arrived in a single afternoon of phase 2 — which is the argument
for having built it:

| # | Defect | Found by | Status |
| --- | --- | --- | --- |
| 1 | `GET /config/{name}` documented, no handler — the server answers 404 | reading `ws-auth.cc` | [pdns#17807](https://github.com/PowerDNS/pdns/issues/17807) |
| 2 | `POST cryptokeys/{id}` implemented, undocumented — `apiZoneCryptokeysPOST`, `ws-auth.cc:3349` at tag `auth-5.1.3` and `:3361` on `master` `a74d89a8` | reading `ws-auth.cc` | [pdns#17807](https://github.com/PowerDNS/pdns/issues/17807) |
| 3 | `Record.modified_at` indented one level too far out — a stray sibling of `properties`, so the property is simultaneously missing and the schema structurally invalid | `kin-openapi` refusing to load the file | to report |
| 4 | `autoprimaries_url` sent by every `Server` object, absent from a schema declaring `additionalProperties: false` — a generated client would reject a real response | this cross-check, on a recorded fixture | to report |
| 5 | `GET zones/{id}/export` declared `type: string`; the server sends `{"zone": "…"}`. The documentation calls it "AXFR format", which reads like `text/plain` and is not | this cross-check, phase 2 | to report |
| 6 | `PUT zones/{id}/rectify` declared `type: string`; the server sends `{"result": "Rectified"}` | this cross-check, phase 2 | to report |
| 7 | `GET /autoprimaries` declared as a single `Autoprimary` object; the server sends an array. A generated client could not decode the list endpoint at all | this cross-check, phase 2 | to report |
| 8 | `Cryptokey` omits `flags` — 256 for a ZSK, 257 for a KSK — under `additionalProperties: false` | this cross-check, phase 2 | to report |
| 9 | PowerDNS stamps a `type` discriminator onto `Metadata`, `Cryptokey`, `TSIGKey` and `ConfigSetting`; none declares it, and all four set `additionalProperties: false` | this cross-check, phase 2 | to report |

Defect 9 is the broadest: four schemas reject a field the server puts on every
one of their objects. Any code generated from this document would fail to
decode an ordinary metadata read.

Defects 5 and 6 arrived the moment phase 2
recorded its first fixtures against those two routes, and 5 had already been
caught by hand an hour earlier: `ExportZone` was written to return the body
verbatim, on the strength of the documentation's phrase "AXFR format", and a
single `curl` against the lab showed `Content-Type: application/json`. The
cross-check then found the same thing without being asked.

Both are tolerated rather than failed — listed in `KnownDivergences` and
`knownSpecDefects`, reported once per run. A check that is permanently red is a
check nobody reads. **Removing an entry when PowerDNS fixes it is the point:**
the list shrinking is how upstream progress becomes visible here.

Recursor and dnsdist publish no specification at all. Their fixtures are
checked for well-formedness and nothing more; that is a property of PowerDNS,
not a gap here.

---

## Phase 2 — Clients · `[x]` closed 2026-07-28

**Exit gate:** all 68 operations implemented and contract-tested — 42
Authoritative, 16 Recursor, 10 dnsdist. **Met.** Each client carries a
`TestSurfaceIsComplete` asserting its own count against the source, and 52
contract tests run against 26 recorded fixtures without a container.

The operation counts are in the table because the exit gate is a number: a
phase that claims 68 and delivers 61 should be visible as such without anybody
recounting. Each row's count is the sum of its domains in
[`CM-03`](https://github.com/dantte-lp/powerdns-capability-map).

| ID | Task | Role | Depends | Ops | Status |
| --- | --- | --- | --- | --- | --- |
| S2-01 | `api/auth`: zones and rrsets — including notify, axfr-retrieve, export, rectify | DEV | S1-07 | 10 | `[x]` |
| S2-02 | `api/auth`: metadata (5), cryptokeys (6), tsigkeys (5), autoprimaries (3) | DEV | S2-01 | 19 | `[x]` |
| S2-03 | `api/auth`: views (4), networks (3), servers (2), config (1), statistics (1), search (1), cache flush (1) | DEV | S2-01 | 13 | `[x]` |
| S2-04 | `api/rec`: zones (5), config (5), statistics (2), search (1), cache (1), servers (2) | DEV | S1-07 | 16 | `[x]` |
| S2-05 | `api/dnsdist`: all ten, of which two write | DEV | S1-07 | 10 | `[x]` |
| S2-06 | Contract tests per client, against recorded fixtures | QA | S2-01…S2-05 | — | `[x]` |

**Authoritative is complete: all 42 operations, verified by
`TestSurfaceIsComplete`.** That test lists every operation `ws-auth.cc`
registers against the client method covering it, and fails if the count
drifts. It proves nothing works — the contract tests do that — but it catches
the failure a per-method suite cannot, because a method nobody wrote has no
test to fail.

**42 + 16 + 10 = 68.** The previous version of this table listed neither
`config` nor `statistics` for Authoritative, and gave Recursor as "zones, the
two writable settings, statistics" — which is 8 of its 16. It also listed
"zone actions" under S2-03 while S2-01 already owned them. Corrected by
recounting against the capability map rather than against the earlier plan.

### Key material is asymmetric, and the asymmetry is load-bearing

S2-02 established a property the whole DNSSEC design rests on. PowerDNS returns
key material from the single-object read and withholds it from the collection:

| Endpoint | Secret field | Present? |
| --- | --- | --- |
| `GET zones/{id}/cryptokeys` | `privatekey` | **omitted entirely** |
| `GET zones/{id}/cryptokeys/{key_id}` | `privatekey` | yes |
| `POST zones/{id}/cryptokeys` | `privatekey` | yes |
| `GET tsigkeys` | `key` | **present but blank** |
| `GET tsigkeys/{id}` | `key` | yes |
| `POST tsigkeys` | `key` | yes |

So the rule for the resource layer is mechanical: **reconcile against the
collection, never against a get.** Note the two products of the same rule are
not identical — cryptokeys omit the field, TSIG keys blank it, so a caller
testing for presence rather than emptiness would believe it held a secret.

Two guards enforce this rather than a comment asking nicely. Contract tests
fail if either collection ever starts carrying material, and
`TestFixturesCarryNoKeyMaterial` fails if a recorded fixture does — a fixture
taken from a single-key endpoint would put a DNSSEC private key in git
permanently, where the fix is rewriting history rather than deleting a file.
That check was verified by planting a fixture containing a private key and
watching it fail.

### The counts were verified against the source, not carried forward

Every operation count in this table was re-derived from a local checkout of
[PowerDNS/pdns](https://github.com/PowerDNS/pdns) at the exact tags the lab
runs, rather than taken from the capability map that produced them earlier:

| Product | Tag | Where | Count |
| --- | --- | --- | --- |
| Authoritative | `auth-5.1.3` | `pdns/ws-auth.cc`, `registerApiHandler` under `/api/v1/servers` | 42 |
| Recursor | `rec-5.4.4` | `pdns/recursordist/ws-recursor.cc`, lines 873–888 | 16 |
| dnsdist | `dnsdist-2.1.0` | `pdns/dnsdistdist/dnsdist-web.cc`, `registerWebHandler` | 10 |

All three matched. Two things the re-derivation corrected:

- **The `ws-auth.cc` line number for defect 2 was ambiguous.** It read
  `ws-auth.cc:3361` with no revision. That is correct on `master` `a74d89a8`,
  which is what [pdns#17807](https://github.com/PowerDNS/pdns/issues/17807)
  cites, and wrong for the tag this project pins: at `auth-5.1.3` the same
  registration is line **3349**. Both are now stated with their revision, per
  [`verified-identifiers.md`](standards/verified-identifiers.md).
- **dnsdist registers paths, not method-and-path pairs.** `dnsdist-web.cc`
  hangs one handler on each path and dispatches on method inside it, so its
  eight registrations plus `PUT` on `config/allow-from` and `DELETE` on
  `/api/v1/cache` are what make the ten. Counting registrations alone would
  have given eight and quietly lost the only two writes the product has.

For the Recursor, four handlers sit in the same registration block and are
excluded for different reasons: `/jsonstat` is marked legacy dispatch, `/api`
and `/api/v1` are discovery endpoints, and `/metrics` is a `registerWebHandler`
rather than an API handler at all. PowerDNS publishes no specification for the
Recursor or for dnsdist, so for those two the source is the specification.

Two notes that shape the clients rather than merely describe them:

- **Recursor's `/config/{name}` is not a general handler.** Only `allow-from`
  and `allow-notify-from` are registered; every other name answers `404` on
  both read and write. The client exposes two settings, not a map.
- **dnsdist has ten operations and two of them write** — `PUT
  config/allow-from` and `DELETE /api/v1/cache`. Rules, pools, downstreams and
  dynblocks are Lua or YAML, never HTTP. The client cannot be given a shape
  that implies otherwise, and `TestSurfaceIsComplete` fails if the write count
  ever leaves two, because that would mean ADR 0006 needs revisiting.

### What the clients found that reading would not have

Each of these was written one way from the documentation, then corrected by a
request against the lab. They are listed because the pattern is the argument
for the fixture layer, not because any one of them is remarkable.

| Product | Assumed | Actual |
| --- | --- | --- |
| auth | `/export` returns `text/plain` | JSON, `{"zone": "…"}` |
| auth | `/rectify` refuses when `api_rectify` is on | 200, and for an unsigned Native zone too |
| auth | a PATCH without `changetype` gets an opaque 422 | the server names the field |
| auth | an unset metadata kind is a 404 | 200 with an empty list |
| auth | a fresh zone has no metadata | `SOA-EDIT-API DEFAULT`, server-assigned |
| auth | a TSIG key keeps the name it was given | canonicalised, `probe` → `probe.` |
| rec | an upstream is stored as given | `192.0.2.53` → `192.0.2.53:53` |
| dnsdist | `setAPIWritable` gates every write | only `PUT`; `DELETE /cache` is admitted regardless |
| dnsdist | an unknown pool returns an empty list | 404, with an empty body and no message |
| dnsdist | the ACL appears in the config dump as `allow-from` | it appears as `acl` |
| dnsdist | the flush count is a number | a string, `"0"` |

The last three matter most for the resource layer. A 405 classified as the
write gate would tell an operator to add a Lua call that would not help — the
classifier now fires only on `PUT`, and two tests assert it stays silent on
`POST` and on `DELETE` elsewhere. The `acl` versus `allow-from` mismatch means
reconciling the dump against the writable endpoint by name finds nothing. And
a count declared as an int fails to decode a successful flush.

---

## Phase 3 — Core resources · `[x]` closed 2026-07-29

**Exit gate:** acceptance green on both authoritative backends. **Met** —
three resources, five data sources, fourteen acceptance tests.

| ID | Task | Role | Depends | Status |
| --- | --- | --- | --- | --- |
| S3-01 | ARC: zone and record schemas, identity, signed | ARC | S2-06 | `[x]` |
| S3-02 | `powerdns_zone` | DEV | S3-01 | `[x]` |
| S3-03 | `powerdns_record` — one RRSet, not one record | DEV | S3-02 | `[x]` |
| S3-04 | `powerdns_zone_metadata` — one kind, not the collection | DEV | S3-02 | `[x]` |
| S3-05 | Data sources: `zone`, `zones`, `record`, `zone_metadata`, `zone_export` | DEV | S3-03 | `[x]` |
| S3-06 | Semantic-normalisation plan modifiers: case, IPv6, server defaults | DEV | S3-02 | `[x]` |
| S3-07 | Acceptance on both backends for each | QA | S3-02…S3-05 | `[x]` |

S3-06 is not optional polish, and phase 2 raised the count from three to seven.
Each produces a permanent diff if compared as a string, so
`internal/provider/normalise` answers "are these the same thing?" per kind of
value and `internal/provider/planmodify` turns that into a plan modifier.

The modifiers are tested in both directions. A test that only proves a
respelling is suppressed would pass for a modifier that suppresses everything —
and a modifier that suppresses too much hides a real change, which is worse
than a spurious diff because no plan ever shows it. So every case has its
`differ` twin: a substituted master, a different port, a duplicate that must
not satisfy two slots.

### `powerdns_record` is an RRSet, and that is not a design preference

PowerDNS has no per-record identity. The addressable unit is the RRSet — every
record sharing an owner name and a type — and a PATCH replaces it wholesale.
There is no way to add one A record to a name without sending every A record
that name should end up with.

A resource modelling a single record would therefore have to read the set,
splice itself in, and write it back on every change. Two such resources on the
same name would race: each reads a set without the other's record, and
whichever applies second deletes the first. That failure is silent, survives a
plan, and surfaces as a record that vanished.

Modelling the set makes ownership explicit, and Terraform's own graph then
prevents two resources from claiming the same name-and-type pair.

The content comparison could not go in `planmodify`, because it depends on a
sibling attribute: an `A` value is an address and compares numerically, a `TXT`
value is a string whose quoting is significant and compares exactly. The
modifier reads `type` from the plan to choose. Three more normalisations were
measured for it — the owner name is lowercased, `AAAA` content is compressed,
and `disabled` survives a round trip.

### `powerdns_zone_metadata` owns one kind, not the collection

The reason is a normalisation found in phase 2: PowerDNS sets `SOA-EDIT-API` on
every zone it creates, unasked. A resource owning the whole collection would
have to delete anything it did not recognise, so it would try to remove that
kind on the first apply, on every zone, for ever.

Owning one kind avoids it entirely, and gives the right answer for a server
whose metadata was partly set by `pdnsutil` or another tool. The acceptance
test asserts the server-assigned kind is still present after Terraform has
finished — it fails if the resource ever grows to own the collection.

### Two resources can disagree about one server-side flag

Creating a `powerdns_zone_cryptokey` turns DNSSEC on for the zone. With
`dnssec` defaulted to `false` on `powerdns_zone`, the zone then planned to turn
it back off on every run and the two resources fought for ever.

`dnssec` is now Optional and Computed with no default: unset adopts whatever
the server has. Setting it explicitly still signs a zone with a
server-generated CSK, which is the other way to do it — and the schema says to
pick one.

### A metadata boundary the API draws and does not explain

Two kinds appear in `GET /metadata` and answer **422 "Unsupported metadata
kind"** when read or written by name: `SOA-EDIT-API` and `API-RECTIFY`. Both
exist as attributes of the zone object, so the metadata endpoint reports them
and refuses to address them.

The boundary is not "every kind that is also a zone attribute". `NSEC3PARAM`
and `PRESIGNED` are both, and both are addressable — which is why the provider
enumerates the two rather than deriving the rule. Measured across ten kinds
against auth-5.1.3.

The provider rejects them before the request, naming the zone attribute that
does work. The server's own message says only "Unsupported metadata kind" and
never mentions the value is settable elsewhere.

### Two framework contracts the acceptance tests found

Both were written wrong first and corrected by running against the lab.

**A create must return exactly what was planned.** Writing the server's
`Native` into state after a create configured as `native` fails with *"Provider
produced inconsistent result after apply"* — a framework contract no plan
modifier can rescue. So `applyZone` has two modes: after a write the planned
spelling is kept, and only after a read does the server's value land. The
semantic modifiers then earn their keep on the *next* plan, comparing a
configured `native` against a state holding `Native`.

**A Computed attribute without `UseStateForUnknown` makes every plan
non-empty.** `serial` and `edited_serial` planned as "known after apply" on
each run, so `ExpectEmptyPlan` failed for a reason that had nothing to do with
normalisation. The diagnosis was only visible because the test prints the plan;
guessing would have led to the modifiers, which were working.

---

## Phase 4 — Security surface · `[x]` closed 2026-07-29

**Exit gate:** a zone signed through Terraform validates under `dig +dnssec`.
**Met** — S4-06 queries the lab's authoritative port and requires an RRSIG.

| ID | Task | Role | Depends | Status |
| --- | --- | --- | --- | --- |
| S4-01 | ARC: key-material handling — reconcile against the collection | ARC | S3-07 | `[x]` |
| S4-02 | `powerdns_zone_cryptokey` | DEV | S4-01 | `[x]` |
| S4-03 | Zone DNSSEC attributes: `dnssec`, `nsec3param`, `nsec3narrow`, `presigned`, `api_rectify` | DEV | S4-02 | `[x]` |
| S4-04 | `powerdns_tsigkey`; zone `master_tsig_key_ids` / `slave_tsig_key_ids` | DEV | S3-07 | `[x]` |
| S4-05 | Ephemeral `powerdns_cryptokey_material`, `powerdns_tsigkey_secret` | DEV | S4-01 | `[x]` |
| S4-06 | Behaviour test: signed zone validates under `dig +dnssec` | QA | S4-03 | `[x]` |
| S4-07 | Test asserting no key material reaches state | QA | S4-05 | `[x]` |

S4-07 is a security property, so it is tested rather than asserted. The check
walks every attribute of every resource in state and fails on two things: an
attribute *named* for key material, and a *value* matching PowerDNS's
`Private-key-format` header or a PEM private-key header. The second is what
catches a key that reached state under some other name — the failure a name
check alone would miss.

It is deliberately not a proof that the right endpoint was called. A refactor
switching the read to `GetCryptoKey` would keep every other test passing and
fail this one, which is the point.

### The ephemeral resources are where the secret rule pays for itself

Refusing to return key material closes the leak and leaves a real need unmet:
an operator has to hand a DNSSEC key to a signing appliance, or a TSIG secret
to a secondary server. Saying "no" to that is not a security posture, it is an
unusable provider.

`powerdns_cryptokey_material` and `powerdns_tsigkey_secret` meet the need
without reopening the leak. Terraform fetches the value during an operation and
discards it — nothing to state, nothing to the plan file — and an ephemeral
value may only be consumed by another ephemeral or write-only attribute. That
restriction is the feature: it is what stops something downstream persisting
what this deliberately did not.

These are the only two places in the provider that call `GetCryptoKey` or
`GetTSIGKey`, the endpoints that carry secrets. The acceptance test reads a
generated TSIG secret ephemerally and feeds it straight into a second key's
write-only attribute, then asks the *server* whether both keys share a secret —
because neither is in state, so there is nothing local to compare.

It also exposed a bug of the kind a type checker cannot: the provider sets
`ResourceData` and `DataSourceData`, and ephemeral resources read a third
field, `EphemeralResourceData`. Omitting it is a nil dereference at apply
rather than a compile error. `RequireAuth` and its siblings now answer a nil
bundle with a diagnostic naming the omission.

### Defect 10: a TSIG `PUT` adds a key instead of changing one

`apiServerTSIGKeyDetailPUT` calls `setTSIGKey(name, algorithm, key)` and then
deletes the previous entry **only when the name changed**
(`pdns/ws-auth.cc:1932` at tag `auth-5.1.3`). Changing only the algorithm
therefore leaves the old key in place and adds a second under the same id.
Measured: three consecutive PUTs produced three entries, all reading
`algprobe.`.

This is the first defect this project has found in the *implementation* rather
than in the specification, and it is why `algorithm` and `name` both force
replacement on `powerdns_tsigkey`. An in-place update would appear to succeed
and leave the zone authenticating against whichever entry the backend happened
to return first. The acceptance test asserts the count, not just the value.

`Update` on that resource consequently does nothing, and says so: every
attribute forces replacement, including `secret_wo`, whose value cannot be
compared against state because it is never stored there.

### S4-06 is the only test that asks the DNS server

Everything else in this suite asserts what an API returned. `dig +dnssec`
against the lab's authoritative port asks whether the zone actually serves
signatures — the gap between "the request succeeded" and "the thing works".

It caught a mistake immediately, though mine rather than the provider's: `dig`
takes one type per query and reads a second type name as a hostname, so
`dig … A RRSIG` silently asked about a host called RRSIG and returned nothing.

### `keytype` is derived, not stored, and that is a production hazard

PowerDNS stores the DNSKEY flags and computes `keytype` from them **together
with how many keys the zone holds**. Measured against auth-5.1.3:

| Requested | Zone contents | Read back | Flags | DS |
| --- | --- | --- | --- | --- |
| `ksk` | no other key | `csk` | 257 | 2 |
| `zsk` | no other key | `csk` | 256 | 2 |
| `ksk` | a `zsk` beside it | `ksk` | 257 | 2 |
| `zsk` | a `ksk` beside it | `zsk` | 256 | 0 |

`csk` is not a third kind of key. It is what PowerDNS calls whichever key does
every job because it is the only one, and the same key is *renamed* — not
replaced — the moment a second appears. Same id, same material.

Comparing that string literally is a trap that springs only in production: a
second resource adding a key flips the first one's type, and `RequiresReplace`
on that attribute would destroy and recreate the signing key of a live zone,
losing the DS the parent publishes. So `csk` is compared as compatible with
both, and the semantic modifier runs before `RequiresReplace` sees the plan.

The same measurement corrected "a ZSK has no DS", which is true only once the
ZSK is not the zone's only key. The earlier reading came from a probe with
three keys in the zone and did not generalise.

---

## Phase 5 — Family surface · `[x]` closed 2026-07-29 — S5-05 landed as S7-00

| ID | Task | Role | Depends | Status |
| --- | --- | --- | --- | --- |
| S5-01 | `powerdns_view_zone`, `powerdns_network` — LMDB only | DEV | S3-07 | `[x]` |
| S5-02 | `powerdns_autoprimary` | DEV | S3-07 | `[x]` |
| S5-03 | `powerdns_recursor_zone`, `powerdns_recursor_acl` | DEV | S2-04 | `[x]` |
| S5-04 | `powerdns_dnsdist_acl` | DEV | S2-05 | `[x]` |
| S5-05 | Data sources for recursor and dnsdist | DEV | S5-03, S5-04 | `[x]` delivered as S7-00 |
| S5-06 | Negative tests asserting each capability diagnostic | QA | S5-01…S5-04 | `[x]` |

S5-06 asserts the **message**, not the failure. A bare `422` reaching a user is
the defect this provider exists to avoid, and a test that only checked "this
errored" would pass for a provider that surfaced the status and left the
operator to work it out.

Two cases are covered so far, and the split between them is worth recording:

- **Views on a relational backend** is an acceptance test. It needs two real
  servers with different backends, which is exactly what ADR 0005 built the lab
  for. Its twin asserts that meeting the requirement works, because a
  diagnostic naming a fix is only useful if the fix does something.
- **An unconfigured product** could not be an acceptance test. The lab
  configures every product through the environment and the provider reads the
  environment by design, so there is no way to have one unconfigured on a
  machine where the suite runs. It is a unit test on the `Require*` accessors
  instead, asserting the argument name and the environment variable appear in
  the text.

The Recursor's `api_dir` case is skipped with its reason recorded: the lab's
Recursor has it configured, and provoking the failure would need a second
Recursor with it left out. The classifier itself is covered by
`classify_test.go`, including the cases that must not fire.

### The two ACL resources are singletons and say so

Neither product has a collection of ACLs. The Recursor exposes two named
netmask settings and dnsdist exactly one, so a resource per ACL is a resource
that can only exist once.

Terraform cannot prevent a second one of the same type, so the schema says what
the API means and `Delete` does the safe thing: it leaves the setting alone and
warns. There is no unset state for an ACL — writing an empty list would refuse
every client — so removing the resource removes Terraform's knowledge of the
setting rather than the setting.

### `ImportStateVerify` does not run plan modifiers

Importing a Recursor zone failed on `servers`: state held `192.0.2.54` from the
configuration and the import read `192.0.2.54:53` from the server. That is
precisely the normalisation the semantic modifier exists to absorb, and it is
invisible in ordinary use.

`ImportStateVerify` compares raw strings without running modifiers, so every
attribute the server respells has to be listed in `ImportStateVerifyIgnore`
with the reason. Three resources now carry such a list, and the pattern is
worth knowing before writing the fourth.

### Hooks and skills, so the rules are not only written down

`AGENTS.md` said "main is never committed to directly" for five phases while
fourteen commits went straight to main. The rule was fine; nothing enforced it.

- `.claude/hooks/guard-main-branch.sh` blocks `git commit` and `git push` on
  main, and allows the sanctioned paths — `gh pr merge`, a fast-forward pull.
- `.claude/hooks/guard-unverified-identifiers.sh` warns when a `file:line`
  citation reaches a file without a revision beside it, which is how
  `ws-auth.cc:3361` was written unqualified when the pinned tag has it at 3349.
- `.claude/skills/sprint-workflow` and `.claude/skills/powerdns-facts` carry
  the workflow and the measure-don't-assume rule, including the table of
  eleven comments this project wrote from documentation and then corrected
  against the lab.

Both hooks were verified by running them against inputs that must fail and
inputs that must pass, which is the same standard `check-hooks.sh` applies to
the commit-msg hook.

### One worktree per sprint, starting here

Phase 5 is the first sprint to follow the workflow `AGENTS.md` has always
described. Doing so immediately exposed a defect in it: the dev container is
bind-mounted on whichever checkout started it, and its name was fixed, so a
worktree's `task test` compiled the code in `main` instead of its own.

`DEV_SUFFIX` now derives a per-checkout container name from the directory, and
each worktree gets its own. Verified by inspecting the mount: the worktree's
container is mounted on the worktree.

---

## Phase 6 — Beyond resources · `[x]` closed 2026-07-29

| ID | Task | Role | Depends | Status |
| --- | --- | --- | --- | --- |
| S6-01 | Actions: `notify_zone`, `axfr_retrieve`, `rectify_zone`, `flush_cache` | DEV | S2-03 | `[x]` |
| S6-02 | Recursor and dnsdist cache flush | DEV | S2-04, S2-05 | `[x]` folded into one `flush_cache` action |
| S6-03 | Functions: `fqdn`, `is_fqdn`, `reverse_zone_name`, `ptr_name`, `soa_serial` | DEV | — | `[x]` |
| S6-04 | Resource identity on every resource with a stable one | DEV | S3-07 | `[x]` |
| S6-05 | Gate actions at Terraform 1.14 via client capability | DEV | S6-01 | `[x]` the framework negotiates it; the tests skip below 1.14 |

S6-03 has no dependency on the clients: the functions are pure and offline,
which is why they are functions rather than data sources. A data source would
make a plan depend on a server for an answer that is a string operation.

They replace the name arithmetic other providers smear into a locals block —
`join(".", reverse(split(".", var.subnet)))`, copied between modules and subtly
wrong for IPv6 or for a prefix off a byte boundary.

Two decisions in them are worth recording, because both refuse rather than
guess:

- **`reverse_zone_name` rejects a prefix off the boundary.** A /25 spans two
  /24 reverse zones and a /20 spans sixteen, so there is no single name to
  return. Returning the enclosing /24 would look right and put half the PTRs in
  the wrong zone; RFC 2317 delegation is a different shape and not something a
  name function can invent.
- **`soa_serial` takes the date as an argument.** A function reading the clock
  would change the plan every day and never converge. The revision is bounded
  at 99 because the `YYYYMMDDnn` convention has two digits for it, and rolling
  silently into the next day's serial would produce a number that looks fine
  and sorts wrong.

The first test run caught a real defect: `/0` produced `.in-addr.arpa.` with a
leading dot, which is a different name and not a valid one.

### Identity has one property PowerDNS cannot give it

The framework asks three things of an identity: it addresses at most one remote
object, it lets the provider decide whether that object exists, and it does not
change for the object's lifetime.

The second and third hold outright here. Every identity is the natural key
PowerDNS itself uses, and every attribute composing one already forces
replacement — so stability is by construction rather than by promise.

The first is stricter than PowerDNS can support. *"At most one remote object
per provider, across all instances of that provider"* — but `example.com.`
names one zone on one server, and two servers can each hold a zone by that
name. Nothing in the API distinguishes them: `/servers/{id}` answers only for
`localhost`, so there is no server identifier to compose in.

Adding the endpoint URL would not fix it and would break the third property: a
server that moves gets a new URL while remaining the same object, and the
identity would change underneath it. So the boundary is the server, and
`docs/contract.md` says so rather than leaving a user to discover it.

The two ACL resources have **no** identity, deliberately. Their natural key is
the setting name, which is `allow-from` on every installation there has ever
been — an identity of `allow-from` would address one object per server and
every object across servers. Leaving it out is honest; declaring it would not
be.

### `nameservers` cannot round-trip through an import block

Found by the identity tests. A zone created with `nameservers` imports without
them — the attribute is create-only and never read back — so the imported
object has none, the configuration has some, and the difference forces
replacement. Importing such a zone by identity therefore plans a destroy and
create rather than a no-op.

That is a property of the API, not of the identity: PowerDNS consumes
nameservers once and does not report them afterwards. Recorded in the contract
so it is a known limit rather than a surprise.

### One flush action, not three

S6-02 asked for `recursor_flush_cache` and `dnsdist_flush_cache` beside the
Authoritative one. They are folded into a single `powerdns_flush_cache` with a
`product` argument instead.

Flushing a name is one operation an operator wants and three endpoints that
implement it. Three actions would make the caller work out which applies, and
the answer is always "the product this name lives on" — which the configuration
already knows.

### S6-05 needed no gate

The plan expected to gate actions behind a client capability check. The
framework negotiates that itself: a Terraform below 1.14 never learns the
provider has actions, and the configuration simply fails to parse the block.
The acceptance tests skip below 1.14 rather than fail with a syntax error that
says nothing about the cause.

---

## Phase 7 — Release · `[~]` in progress

### Documentation is generated, and `docs/` now holds two things

`tfplugindocs` writes registry documentation into `docs/`, which is also where
this project's own documents live — the plan, the contract, the standards, the
ADRs. Both now share the directory.

`tfplugindocs validate` was run against it and passes: it recognised exactly the
30 provider documents and ignored everything else, because the Registry reads
`index.md` and the known subdirectories — `resources/`, `data-sources/`,
`functions/`, `ephemeral-resources/`, `actions/`, `guides/` — rather than every
file it finds.

That was an inference from the validator's behaviour rather than a guarantee,
so it was left open for S7-05 to confirm before publication.

**Confirmed, and nothing moves.** The Registry's own documentation enumerates
what it reads under `docs/`: `index.md`, and the subdirectories `guides/`,
`resources/`, `data-sources/`, `functions/`, `ephemeral-resources/`,
`actions/` and `list-resources/`. It is a fixed list, not a sweep. `plan.md`,
`contract.md`, `standards/`, `adr/`, `methodology.md` and `development.md` are
not in it and are not published.

Source: [Provider documentation — directory
structure](https://developer.hashicorp.com/terraform/registry/providers/docs).

### Every example is a claim that has to keep working

The examples are not decoration. `tfplugindocs` embeds them in the generated
pages, so an example that drifts is documentation that lies, and `task
tf:fmt:check` covers only their formatting.

Each one therefore states the constraint it is demonstrating, in the words the
schema uses: that a zone's `nameservers` are create-only, that `csk` and `ksk`
are the same key, that destroying an ACL resource leaves the ACL alone, that a
prefix off a boundary has no reverse zone. A reader who copies an example
copies the caveat with it.

| ID | Task | Role | Status |
| --- | --- | --- | --- |
| S7-00 | Data sources for recursor and dnsdist — deferred from S5-05 | DEV | `[x]` |
| S7-01 | Examples for every resource, action, function, data source and ephemeral | DEV | `[x]` |
| S7-02 | Registry documentation generated and validated | DEV | `[x]` |
| S7-03 | Version matrix: acceptance against auth 5.0.x as well as 5.1.3 | QA | `[x]` 5.0.6 and 5.1.3, identical results |
| S7-04 | Signed `v0.1.0` | PM | `[x]` |
| S7-05 | Terraform Registry submission | PM | `[x]` published, `v0.1.1`, 13 platforms, protocol 6.0 |
| S7-07 | **Added.** OpenTofu Registry submission | PM | `[~]` prepared to two prefilled links; the forms refuse anything but the browser |
| S7-08 | **Added.** `v0.1.1` — the release the Registry will accept | OPS | `[x]` |
| S7-06 | **Added.** An RSA-4096 signing key, and `GPG_PRIVATE_KEY`/`PASSPHRASE` in repository secrets | PM | `[x]` |

### The version matrix, and what it did not find

S7-03 asked whether the provider works on the 5.0 branch as well as 5.1, and
the honest answer before this sprint was that nobody knew — every acceptance
run in the project's history had used one image.

The fixture now takes `--auth`, and `compose.lab-auth-50.yml` overrides exactly
two lines: the images of the authoritative pair, pinned by digest to 5.0.6, the
last release on that branch. PostgreSQL, the recursor, dnsdist, every
configuration file and every port stay where they were. That restraint is the
whole design — if the two runs differ, the difference is attributable to the
authoritative branch and to nothing else.

**Both runs are identical: 203 assertions, 0 failures, the same two skips.**
Views and networks work on LMDB under 5.0.6, `/views` answers, and no
capability diagnostic changed. The matrix found no defect, which is the result
it was most likely to have and still worth the run: "supports 5.0 and 5.1" was
a sentence in the documentation and is now two green jobs a reader can open.

Acceptance in CI is a matrix over the two branches with `fail-fast: false`.
When one breaks, the useful question is whether the other did too, and
cancelling the sibling job destroys that answer.

**A precondition that failed open.** `_lab-running` asserted
`podman container exists`, which is also true for a *stopped* container — the
same defect found in the end-to-end driver a sprint earlier and not looked for
anywhere else. The first replacement used `--format '{{.State.Running}}'` and
was worse: Task interpolates Go template braces as its own before podman sees
them, so the check silently compared an empty string and failed open in the
other direction. It now filters on `--filter status=running`, which needs no
template at all.

### v0.1.0 was refused, and why nothing caught it

The Terraform Registry rejected it: *"missing files in request body"*, naming
all thirteen SBOMs.

It parses `SHA256SUMS` as the list of files belonging to the version. Adding
`sboms:` to the release configuration made goreleaser fold thirteen SBOM
entries into that file, and every line the Registry cannot resolve to a file it
accepts fails the whole submission. The SBOMs were a good idea implemented
without checking what else consumed the checksum file.

**The release gate had been written the day before and did not catch it**, and
the reason is worth keeping. Every check it makes establishes that the
artefacts are *correct* — signed, digested, complete, matching the manifest,
built from a commit that passed CI. Not one asked whether they are the artefacts
the *Registry* accepts. Those are different questions, and only the second one
was going to be answered by a stranger's validator.

`scripts/check-release-artifacts.sh` asks it now, of a snapshot build, before a
tag exists: `SHA256SUMS` may list archives and the manifest and nothing else,
every archive must match its digest, and the manifest must be the repository's
own and declare a protocol. Verified by putting an SBOM line back and watching
it fail.

**And the release could not be rehearsed.** The dev image had no `syft`, so
`goreleaser release --snapshot` failed locally — on the machine where failure
is free — and succeeded in CI, where it is not. `syft` is in the image and
`task release:dryrun` runs the whole thing in about twenty seconds. That the
rehearsal was impossible is the reason the mistake reached a tag.

### v0.1.0 is released

29 artefacts across 13 platforms, a `SHA256SUMS` signed with the new key, an
SBOM beside every archive, and the manifest declaring protocol 6. Verified the
way the Registry will: the detached signature checks against the public key,
the archive matches its recorded digest, and the binary in it answers.

The release gate ran first and passed in 2m33s — signing secrets present,
`VERSION` and tag agreeing, a non-empty changelog section, the manifest
matching the served protocol, the tag an ancestor of `main`, `CI` and
`Acceptance` both green for `81a7ecc`, and the generated documentation in sync.

What remains is not a build step, and deliberately cannot be automated.

**Every prerequisite is met.** The repository is public and named
`terraform-provider-powerdns`; `v0.1.0` carries 29 artefacts, a signed
`SHA256SUMS` and a manifest declaring protocol 6; the signing key is RSA-4096,
which is what both registries accept; and membership of the `ioplane`
organisation is now public, which the OpenTofu submission validates
automatically and would otherwise have rejected.

**Both registries then need a human in a browser.**

The Terraform Registry publishes through an OAuth sign-in and a form — there is
no public API for it. The API that exists is for HCP Terraform's *private*
registry and does not apply.

The OpenTofu Registry takes two issues, and its templates open with a line
worth quoting: *"Submissions MUST be made through the GitHub issue form UI, not
via the API, gh CLI, or by manually creating issues. The automated validation
and processing pipeline depends on the structured data from the issue form."*
So they were not filed from here. A submission that circumvents the instruction
at the top of the form is a submission that fails validation and leaves noise
in somebody else's repository.

**On the namespace.** `ioplane/powerdns` is free in both registries. Seven
`powerdns` providers already exist in the Terraform Registry, of which
`pan-net/powerdns` accounts for over ten million downloads. That is the thing
this provider is arriving next to, and the reason the README leads with what it
does differently rather than with what it supports.

### Preparing it was blocked on a key that did not exist

`CHANGELOG.md` is cut: `[0.1.0] — 2026-07-29` holds everything that was under
`[Unreleased]`, which is now empty, and `check-release.sh` agrees the release
is otherwise ready.

Two things then came out of preparing it.

**The repository has no secrets at all.** `GPG_PRIVATE_KEY` and `PASSPHRASE`
do not exist, so the release gate stops in its first job — which is what it is
for, and cheaply, but the tag cannot be signed and the Registry rejects an
unsigned `SHA256SUMS`.

**And the key on hand could not be used.** It was ed25519, and the Registry
"supports RSA and DSA keys, but not ECC keys"
([Preparing and adding a signing key](https://developer.hashicorp.com/terraform/registry/providers/publishing)).
A release signed with it would be built, uploaded and then refused at
publication — after the tag was spent. RSA-4096 with no expiry is what the
publishing tutorial specifies. That is S7-06, and it is a decision about an
identity rather than a chore: the private key signs everything this provider
ever publishes, and its public half is what the Registry pins.

**One defect found while preparing.** The release-notes extraction ran to
end-of-file for the oldest section, so `v0.1.0`'s notes ended with the
changelog's markdown link-reference block. It stops at that block now — a
defect that only ever appears on the first release, which is exactly the one
nobody gets to rehearse.

**And a documented check that never existed.**
`docs/standards/changelog.md` said `scripts/check-changelog.sh` verifies the
heading format before the tag. There is no such script and never was. Same
failure as the pipeline nobody ran: a documented check reads as a check and
enforces nothing. The standard now names `check-release.sh`, which does it.

---

## Phase 8 — Continuous integration · `[~]` in progress

### The gate was never enforced anywhere

Until this phase the quality gate was `task all`, run by a developer and quoted
in a commit body. [ADR 0008](adr/0008-github-only-review.md) recorded that as
"weaker than a pipeline enforcing it, and the accepted cost until a runner
exists", and kept `.gitlab-ci.yml` as the gate's definition for a mirror that
might exist later — to be "kept current".

It was not kept current. By the time it was removed it called two scripts that
do not exist, ran the contract tests with a build tag they do not carry, and
split the acceptance matrix in a way the suite has not worked in since phase 5.

Nobody noticed, because nothing ran it. That is the finding, and it generalises
past this file: **an unexecuted pipeline does not stay correct, and reads as a
gate while enforcing nothing.** It is now deleted, and the gate runs where the
code is reviewed ([ADR 0009](adr/0009-github-actions-is-the-gate.md)).

### One toolchain, in two places, that cannot drift

CI cannot cheaply run the dev container — building that image costs more per
job than the job. So the workflows install the same tools themselves, and the
toolchain now exists twice.

Two versions of a linter is not a cosmetic problem: it produces a finding on
one machine and not the other, and the argument that follows is about which
machine is right rather than about the code. So `Containerfile.dev` holds every
version, a workflow line naming one carries `# pin: <ARG>`, and
`scripts/check-tool-versions.sh` — part of `task all` — fails on a mismatch
**and** on a deleted marker. Both directions were tested against real
mutations, because a check that only ever passes proves nothing.

### What the gate found on its way in

Writing the workflows surfaced four defects in things already believed correct:

| Where | Defect |
| --- | --- |
| `release.yml` | ran GoReleaser at `latest` — the one path where "whatever was current that day" is least acceptable |
| `release.yml` | took the release notes from whichever changelog section was on top, not the one matching the tag |
| `check-pins.sh` | read `github/codeql-action/init` as a repository, so every subpath action reported NOT FOUND |
| `task semgrep` | ran on the host with no pinned version, against AGENTS.md's own no-host-toolchain rule |

The plan's task counter is also derived now rather than asserted: the audit
recomputed it, `check-badges.sh` recomputes it on every run. That was the
audit's own finding left half-finished.

### And what the first run found, which is the point

The workflows were written blind — `actionlint` parses them, nothing local
executes them — so the first run on the pull request was the real test. It
failed four ways, each of them a fact about GitHub Actions rather than a typo:

- A job with `container:` gets `sh`, not `bash`. `${GITHUB_SHA::8}` is bash
  syntax and died as "Bad substitution". Every workflow now sets `shell: bash`.
- The Go image has no `unzip`, and HashiCorp ships Terraform as a zip. That job
  moved to the runner, which already has both.
- `check-pins.sh` reported one action of twenty-four as NOT FOUND — a valid,
  resolvable SHA. It treated any `gh` failure as "this commit does not exist",
  so a rate-limited run would report two dozen correct pins as fabricated. It
  now retries, and distinguishes "GitHub says no" from "the call did not
  succeed".
- SonarCloud, already wired to this repository, failed its quality gate on
  nineteen findings — all in the workflows just written. Most were the same
  finding this repository already has a rule about: `npm install -g pkg@latest`
  in CI *and* in the dev image, and an unpinned `podman-compose`. Now pinned,
  installed with `--ignore-scripts`, and every download refuses a redirect off
  HTTPS.

The second run then found the one that could not be fixed by trying harder.
`check-pins.sh` could not verify `aquasecurity/trivy-action`'s SHA from a
hosted runner at all: that organisation has an IP allow list, and the API
answers 403 for every runner address. A pin nothing can check is not a pin,
and a named exception in the checker for one action is how a rule becomes a
preference — so the action was replaced by the published trivy image, pinned
by digest, which skopeo resolves from anywhere.

The acceptance workflow then failed its own first run on `main`, which is what
it was merged to find out. `uv pip install --system` is refused by the runner's
Debian-managed interpreter (PEP 668) — the dev image passes
`--break-system-packages` for exactly that reason and the workflow did not.
It is the same class as the `sh`-not-`bash` finding: a difference between the
image and the runner that no amount of reading catches, and one run does.

Adding `--break-system-packages` got past PEP 668 and straight into the next
one — `/usr/local/lib/python3.12/dist-packages` is not writable by the runner's
user, and the dev image only gets away with it by being root. The right answer
was not a third override: `podman-py` is already a declared dependency in
`pyproject.toml`, so the workflow runs the driver with `uv run` and the project
environment supplies it at the pinned version.

Which surfaced a version living in a third place. `pyproject.toml` still said
`podman==5.7.0` after the bump to 5.8.0 — a drift this sprint's own checker
introduced and did not catch, because it only read the workflows.
`check-tool-versions.sh` now reads `pyproject.toml` too, and the three versions
it declares carry `# pin:` markers like everything else.

One thing still fails, and it is not a change to this repository:

- SonarCloud's quality gate — it objects to `go install pkg@v1.2.3` on the
  grounds that it is not a lock-file, which for a pinned module version it
  effectively is. Marking a finding as a false positive needs access to the
  SonarCloud project, not a commit here. **S8-12.**

### The linters that were missing entirely

Six shell scripts and a Containerfile had no linter at all — the gate covered
Go, Python, Markdown, YAML and spelling, and stopped there. Four now run, in
the dev image and in `ci.yml`, pinned in the same place as everything else:

| Linter | Covers | Found |
| --- | --- | --- |
| `shellcheck` | the scripts | a dead `IMAGE_RE` in `check-pins.sh`, and five `readonly X="$(cmd)"` that swallow the command's exit status |
| `shfmt` | the scripts | seven files, reformatted once |
| `hadolint` | `Containerfile.dev` | two `curl … \| …` that could not fail the build |
| `zizmor` | the workflows | sixteen checkouts leaving the token in `.git/config`, and a module cache restored into the signing job |

`shellcheck` is not only for the scripts: `actionlint` hands it every `run:`
block, so the shell inside a workflow is now held to the same standard as a
script — and without it installed, actionlint silently checks less than it
appears to.

`hadolint` also produced the sprint's one wrong first answer. Its finding was
that a pipe hides the exit status of everything but the last command, and the
usual remedy is `SHELL ["/bin/bash", "-o", "pipefail", "-c"]` — which podman
accepts, warns about, and ignores, because OCI has no `SHELL` instruction. It
would have satisfied the linter and protected nothing. The pipes were removed
instead, which works whatever builds the image.

The two `zizmor` classes are worth naming because both are about a release
this repository has not cut yet. `actions/checkout` leaves its token in
`.git/config` for the rest of the job, where anything that archives the
workspace carries it out too; and `actions/setup-go` with `cache: true` in
`release.yml` would let a cache entry written by an earlier run reach the job
whose output is signed and published as immutable.

Super-Linter is deliberately absent. It bundles its own versions of
golangci-lint, ruff, markdownlint and yamllint, which this repository pins —
so it would put a second opinion about the same files into the tree, which is
the thing [ADR 0009](adr/0009-github-actions-is-the-gate.md) exists to prevent.
What it would have added over the gate is exactly the four above, and those are
installed at named versions instead.

What SonarCloud was right about is already fixed: `@latest` npm installs in CI
*and* in the dev image since phase 0, an unpinned `podman-compose`, downloads
that would follow a redirect off HTTPS, and `uv` willing to build a source
distribution — which is arbitrary Python at install time — now refused by
`UV_NO_BUILD`.

| ID | Task | Role | Status |
| --- | --- | --- | --- |
| S8-01 | Retire `.gitlab-ci.yml`; ADR 0009 | OPS | `[x]` |
| S8-02 | `ci.yml` — the gate, job for job against `task all` | OPS | `[x]` |
| S8-03 | `acceptance.yml` — the five-container lab on a hosted runner | OPS | `[x]` |
| S8-04 | `security.yml` — CodeQL, Semgrep, osv-scanner, Trivy, SARIF | OPS | `[x]` |
| S8-05 | `scorecard.yml`, `dependency-review.yml`, `dependabot.yml` | OPS | `[x]` |
| S8-06 | `release.yml` — pinned, tested, SBOMs, notes by version | OPS | `[x]` |
| S8-07 | `check-tool-versions.sh` and `lint:actions` in the gate | OPS | `[x]` |
| S8-08 | Acceptance moves to pull requests once its run history says it is stable | QA | `[x]` 15 consecutive green runs on `main` |
| S8-09 | Branch protection: require the gate before merge | PM | `[x]` |
| S8-10 | README CI badge, once `ci.yml` has a run on `main` to point at | PM | `[x]` |
| S8-11 | Enable Dependency graph for the organisation, so dependency review can run | PM | `[x]` |
| S8-12 | Triage SonarCloud's `go install pkg@version` findings in the project | PM | `[x]` the Python move replaced them with six agent-safety findings, all fixed |
| S8-14 | **Added.** Repository hygiene: CODEOWNERS, templates, code of conduct, settings, branch protection | OPS | `[x]` |
| S8-15 | **Added.** The gate's checks move from shell to Python, with tests | OPS | `[x]` |
| S8-16 | **Added.** Required status checks are compared against the jobs that exist | OPS | `[x]` |

---

## Phase 9 — The consumer's path · `[x]` closed 2026-07-29

### What the acceptance suite never touched

It proves the provider is correct against PowerDNS. It says nothing about the
path a configuration travels to reach it: remote state, a module fetched from a
remote, an engine holding a lock. That is where a provider which passes every
test still fails in somebody's pipeline, and none of it had ever been exercised.

`test/e2e` runs that path. Terragrunt over an S3 backend on SeaweedFS, the module
fetched over HTTP from a private Forgejo repository, against the running lab —
forty-four scenarios across eight units, all green, repeatable back to back.

| Scenario | Establishes |
| --- | --- |
| apply fetches the module over an authenticated remote | it comes from a remote that asks who is calling, not from a path beside the configuration |
| a bad token fails and says so | the credential error people actually meet, which an anonymous remote could not produce |
| the second plan is empty | idempotence, the property the whole normalisation layer serves |
| DNS answers the forward records | a name resolves — an HTTP 200 did not establish that |
| DNS answers the reverse record | the PTR sits where the provider function computed it |
| the RRSet holds both values | one set with two values, not two resources fighting over a name |
| the database holds the rows | gpgsql, past both the API and anything caching in front of it |
| state is in the bucket | S3, not on disk beside the configuration |
| no lock is left behind | a finished run releases it; a stale lock is indistinguishable from a live one |
| a held lock blocks a second run | the lock is planted rather than raced, so it fails for one reason only |
| a TTL change does not replace the record | a replacement is an outage nobody asked for |
| an out-of-band zone imports | adoption, in its own unit with its own state |
| destroy removes it everywhere | gone from DNS and from storage, not merely from state |

### The module remote is a forge, and the second attempt is the honest one

It was `git daemon` first, on the argument that a daemon is smaller and adds no
image to pin. The argument was wrong about what was being tested. `git://` is
anonymous, and nobody sources a production module that way — real
configurations use HTTPS with a token or SSH with a key, and the failure people
actually meet when wiring one up is a credential error. An anonymous remote
cannot produce that failure, so the scenario could not exist.

Forgejo rootless, 74 MB, the whole fixture built through its REST API: create
the administrator, issue a token, create the repository, push the module. The
rootless variant also drops privilege by construction rather than because a
scanner asked, which is what had just happened to the daemon.

**And the first version of the authentication scenario proved nothing.** The
repository was created public, so the token in the source URL was decoration
and a deliberately wrong token still fetched the module. The test failed,
which is how that surfaced. The repository is private now, and clearing the
cache is part of the scenario: Terragrunt keys its download on the source with
the credentials stripped, so with the module already cached a wrong token never
reaches the network.

### Forty-four scenarios, and what the last thirty-one established

Thirteen covered the happy path. The rest cover what the provider actually
promises.

| Area | What it establishes |
| --- | --- |
| **Secrets in remote state** | the DNSSEC private key and the TSIG secret are absent from the state object **in the bucket** — the risk the claim exists to answer, checked for the first time where it lives rather than in a local file |
| **DNSSEC** | the zone serves a DNSKEY and answers carry an RRSIG: signed, not merely holding a key |
| **Recursor and dnsdist** | two of the three products, which no consumer path had reached |
| **Views and networks** | the LMDB-only resources, and the capability diagnostic when they are asked of gpgsql |
| **Drift** | a value changed, a TTL changed and a record deleted behind Terraform's back, each visible in the plan and repaired by apply |
| **Both engines** | the same unit under `tofu` and `terraform`, each verified from its own log prefix |
| **Actions** | `rectify` and `notify` attached to a lifecycle, reported by Terraform as invoked |
| **Ephemeral** | a secret read and provably not stored |
| **Autoprimary** | a server-side list with no other manager |

### The fixture was leaking its own token

Terragrunt prints a module source verbatim, and the source carried
`http://e2e:<token>@…`. Every log line naming the module printed the
credential, and every `git` it spawned carried it in the process list.

It is a fixture token that is regenerated on every `task e2e:up`, so nothing
was at risk — but the shape is the one that leaks a real credential in a real
pipeline, and a test suite is a place people copy from. The credential now
lives in git's credential store, the source URL has none, and a scenario
asserts the URL stays that way.

The remote also serves TLS now. A self-signed certificate for `127.0.0.1`,
generated before the fixture starts, and the dev container told to trust that
one certificate for that one host — not `sslVerify = false`, which would have
worked and taught the wrong lesson. A scenario overrides the URL-specific
`sslCAInfo` and asserts git refuses with "certificate signer not trusted";
the first version of that check overrode the *generic* `http.sslCAInfo`, which
git ignores when a more specific one exists, and so passed against a broken
premise.

Every scanner finding on this driver went away as a consequence rather than
by suppression: the credential is no longer in a URL, the URL is no longer
plain HTTP, and the push runs in a `mktemp -d` directory rather than a
predictable name under a shared `/tmp`. The `NOSONAR` markers added first are
gone — a suppression that survives is a finding nobody fixed, and this
repository has a standard about checks that only look like checks.

### The suite runs where nobody owns the machine

`e2e.yml`: the lab, the S3 gateway, the forge, both engines, eight units. On
main, nightly and on demand — not on pull requests, for the reason
`acceptance.yml` was not either. Seven containers and eight applies is a slow
gate before it has a run history, and an unreliable one until it does.

**The driver had to stop assuming a dev container.** On a developer's machine
the toolchain is in one; on a runner CI installs it onto the machine, and
building the dev image to have somewhere to `exec` into costs more per run than
the run. `Runner` executes locally when no dev container is running, and every
path it uses is derived from the checkout rather than written as `/app` — one
definition that is correct in both places, with the single translation to the
container's mount point where it belongs.

Two things that cost a cycle each. `podman container exists` is true for a
*stopped* container, so the driver tried to `exec` into one and got 255 with
its own command echoed back; it asks whether the container is running now. And
`/root/.git-credentials` is right in the container and wrong on a runner, where
the user is not root — the paths come from `Path.home()`.

**Terragrunt's version turned out to be load-bearing.** This host carries
v0.66.7, which predates the 1.0 CLI freeze: it has no `run` subcommand and
forwards the word to the engine, which answers *"OpenTofu has no command named
run"*. The suite drives every command through `run` because
`standards/terragrunt-integration.md` requires it, so the workflow installs the
pinned v1.1.1 and `check-tool-versions.sh` now treats Terragrunt and OpenTofu
as tools CI must match the image on. A version that was a formality for the
dev image is a hard dependency for the suite.

**The first CI run passed 43 of 44**, and the one failure was a sentence
again. Git's TLS refusal reads "certificate signer not trusted" on this host
and "server certificate verification failed" on a hosted runner — same git,
same command, different build. The assertion is about the certificate now,
not about the wording, which is the second time in this phase a test has been
pinned to a phrase somebody else owns.

**And the local path was writing on the developer's machine.** The fixture
configures `credential.helper store` and writes `~/.git-credentials`; run
outside the container, that is not a test fixture, it is a change to the
person's computer. It happened here before the reviewer named it: the global
helper I set went on to capture three of this machine's real credentials and
write them to disk in plaintext, where before they lived only in `gh` and
`glab`. The helper is removed and the code redirects `HOME` to a directory
under the fixture, so `--global` means that directory and nothing else.
Verified by checksumming the real file across a local run.

**And `uv.lock` was decorative.** It was in the repository and nothing required
it, so any `uv run` could resolve a different set of libraries than the one
pinned. `--locked` everywhere now — the same failure the image digests and
action SHAs exist to prevent, in the one ecosystem that had been left out of
the rule.

### The object store is SeaweedFS

Not on features — either serves the S3 API the backend needs, and the suite
moved across without a scenario changing. Two reasons that are about the
project rather than the software.

**Licence.** MinIO is AGPL-3.0; SeaweedFS is Apache-2.0, which is this
repository's own. Nothing links against a test fixture, so this was never a
compliance question — it is a consistency one, and consistency is cheaper to
keep than to restore.

**Publication.** MinIO's newest source release had no published image, so
"latest release" and "latest image" were already different things for it, and
the pin had to be reasoned about rather than read. SeaweedFS publishes an image
per release.

Two details the move surfaced. `weed server -s3` runs master, volume, filer and
the gateway in one process — more moving parts than an object store ought to
be, and fewer containers than running them apart. And the gateway has no health
endpoint: it answers `403` to an unsigned list, which is a served request and
the thing to wait for. Waiting for a `200`, as the MinIO probe did, waits
forever.

Creating the bucket also goes through the S3 API now. Against MinIO the fixture
made a directory in the server's data root, which worked because MinIO lays a
bucket out that way. SeaweedFS keeps buckets in its filer, and a fixture that
reaches behind a server's interface breaks when the server changes its mind
about storage.

Two things the reviewer caught in the move itself, and the first is the
uncomfortable one: **the commit said the bucket was created through the S3 API
and the code still made a directory.** The edit had not applied — the source
had been reformatted since the pattern was written — and the suite passed
anyway, because SeaweedFS tolerated writes to a bucket it had never been told
about. A green run and a commit message agreeing with each other, both wrong.
It creates the bucket with `CreateBucket` now, and asserts `head_bucket`
afterwards so a silent tolerance cannot stand in for a bucket again.

The healthcheck was the second: `wget || exit 1` against an endpoint whose
correct answer is `403` left the container `starting` forever, and anything
waiting on health — `up --wait`, a monitor — would have waited with it. It
matches the HTTP status line instead, because any response means the gateway
is serving and a refused connection produces none. BusyBox `wget` exits 1 for
a 403 rather than the 8 GNU `wget` uses, so the exit code was never the thing
to read.

### The changelog had been rewriting history

Twelve entries had been written into `[0.1.1]` after that version shipped, and
two into `[0.1.0]`. Each was true about the work and false about the release:
the section says what went out under that tag, and these had not.

It would also have lost them. The release cut reads `[Unreleased]` and nothing
else, so entries parked in a closed section are dropped from the next set of
notes as well as being wrong in this one.

`check-release.sh` compares each released section against its tag and fails on
a line that has appeared since. Removals pass — that is how the mistake is
corrected, and the check had to permit the correction it was written to
prompt. It found the two lines in `[0.1.0]` immediately, which I had put there
myself when recording that `v0.1.0` was released.

### Four defects a reviewer found that the suite could not

The suite was green throughout, and none of these would have made it red —
which is the point of them being found by reading rather than by running.

**The mirror was built for one architecture.** `linux_amd64`, hard-coded, in a
dev image whose Containerfile supports amd64 and arm64 on purpose. On an arm64
host the engine looks in `linux_arm64`, finds nothing, and reports the provider
unavailable — a wrong path presenting itself as a missing provider. It now
asks `go env`.

**The lock-file cleanup covered two units of eight.** It was written when there
were two. Rebuilding the provider — the ordinary reason to run this suite —
changes the binary's checksum, and any unit whose lock file was not cleared
refuses the rebuilt package. It clears every `live*` unit now, which is also
the shape that survives the ninth.

**Every command was a bare `terragrunt apply`.** Terragrunt 1.0 froze the CLI
contract and `standards/terragrunt-integration.md` requires the `run`
subcommand. The legacy form still works, which is exactly what makes it easy to
keep using until it stops.

**`task e2e:down` orphaned the zones.** The lab outlives the fixture, so
removing MinIO takes the state and leaves the zones; the next `up` starts with
empty state and meets a 409 on a zone it believes it is creating. `down` now
deletes them by name first — by name rather than by `terragrunt destroy`,
because destroy needs the module, the mirror and a reachable remote, and `down`
has to work when the reason for running it is that one of those is broken.

### ADR 0005 is imprecise, and the provider is not

It says views and networks are "unimplemented by gpgsql". The read endpoint
exists and answers `200 {"views": []}`; only the write fails, with `405` on a
`PUT` and `422` on a `POST`. A test asserting a 404 on the read — which this
suite did first — fails against a server behaving exactly as designed.

The provider's own diagnostic already says it correctly: *"a read returns an
empty list while a write fails like this. Check the launch= setting."* The ADR
is the document that is behind, and it is left as written because an ADR
records what was decided; the precise behaviour is recorded here and asserted
by `test_family.py`.

### What Terraform enforces that the provider only claims

An ephemeral value could not be got out of the module at all. An ordinary
output derived from one is refused — "not declared as returning an ephemeral
value" — and declaring the output ephemeral is refused too, because a root
module has nowhere to return one to. Both errors were met, in that order,
writing the module.

And an ephemeral resource is opened while the graph is walked, before the
resources in the same apply exist. `depends_on` does not defer it. A secret is
something you read because it is already there, and the module now does.

### Actions exist on one engine

The imperative unit is pinned to Terraform. Actions are a 1.14 feature,
OpenTofu does not have them, and `test_imperative.py` asserts the refusal
rather than letting the pin be a preference nobody revisits.

### It runs on OpenTofu, which nobody had checked

Terragrunt drives `tofu`, not `terraform`. ADR 0004 called OpenTofu a co-equal
target and the dev image has carried it since phase 0, but every test until now
went through Terraform. The provider's first end-to-end exercise turns out to
have been on the other engine, and it passed.

### Four defects in the harness, all mine

Written down because each is a class, not a typo.

**`bash -lc` a second time.** A login shell re-reads the profile and drops
`/usr/local/go/bin` from PATH, so `go build` failed with 127. I had already met
this exact failure earlier in the day and reintroduced it.

**The lab check asked without the API key.** PowerDNS answers 401, `http_ok`
saw "not 200", and the driver reported a running lab as absent.

**The import test removed a live resource from state.** It imported into
`powerdns_zone.this` — the address the live unit manages — and cleaned up with
`state rm`, orphaning the real zone. Two tests later an apply met a 409 whose
cause was nowhere near it. The adoption scenario now has its own unit and its
own state key.

**And one the fixture found in CI, not here — a tag with two digests.**
`check-pins.sh` blessed the MinIO image locally and CI reported it NOT FOUND.
Neither was wrong. The tag is published as both an OCI image index and a Docker
manifest list, which are different documents with different digests, and the
registry serves whichever the client's `Accept` header allows. The local skopeo
asks for both and resolved the OCI digest; the older skopeo on a hosted runner
asks only for the Docker one and could not find it.

The pin is now the Docker manifest list, which every client can fetch. Two
things follow that are worth keeping: a digest is only as portable as the media
types the client requesting it accepts, and "verified on my machine" is a
weaker claim for a digest than it looks. The image check also now separates
"the registry said no such digest" from "the request did not get through" —
the same distinction the action check learned earlier in phase 8, which had not
been carried across.

My first reading of this was that the digest did not exist upstream, on the
strength of a `curl` whose own `Accept` header excluded the format it was
looking for. The tool used to check the claim had the same blind spot as the
tool that produced it.

**A destroyed zone answers REFUSED, not NXDOMAIN.** With the zone gone the
server is no longer authoritative and declines the name; dnspython raises that
as "all nameservers failed", which is also what a dead server looks like. The
suite asks with a low-level query and asserts the rcode, so a server that is
simply down now fails the test instead of passing it.

| ID | Task | Role | Status |
| --- | --- | --- | --- |
| S9-01 | `compose.e2e.yml` — an S3 gateway and a Forgejo remote beside the lab | OPS | `[x]` |
| S9-02 | A module and a Terragrunt unit that consume the provider | DEV | `[x]` |
| S9-03 | `e2e.py` — fixture lifecycle on podman-py, driven by uv | OPS | `[x]` |
| S9-04 | The suite on pytest, with boto3, dnspython and psycopg | QA | `[x]` |
| S9-05 | The e2e suite in CI | OPS | `[x]` |
| S9-06 | DNSSEC and the second backend in the e2e path | QA | `[x]` |
| S9-08 | **Added.** Provider upgrade against existing state, and adoption by identity | QA | `[x]` |
| S9-07 | **Added.** Secrets, drift, both engines, both other products, actions and ephemeral | QA | `[x]` |
| S8-13 | A release gate: nothing is built until the release is checkable | OPS | `[x]` |

S8-10 reopened something S0-21 closed. A `github/ci` badge was removed then
because it rendered "not found": the endpoint answered `200`, but GitHub held
only the release workflow, so there was no CI to report.

It reports now, with one correction on the way in. The bare endpoint still
answers "not found" — this repository has six workflows and the badge has to be
told which one it means, `?workflow=ci.yml`. And `check-badges.sh` was dropping
the query string before asking the `.json` endpoint, so it would have rejected
a badge that renders correctly. It keeps the query now, because a check that
asks a different question than the badge does is not checking the badge.

### The release path had no gate

`release.yml` ran `go test` and then built, signed and published. That is the
one path in this repository where a mistake cannot be corrected: the Terraform
Registry treats a published version as immutable, so a wrong release is
answered by publishing another one and leaving the wrong one visible forever.

Everything answerable before building is now answered before building:

| Asserted | Why it cannot wait until after |
| --- | --- |
| the signing secrets exist | discovering it at the signing step spends the tag — it is pushed, and the fix is a new version |
| `VERSION` and the tag agree | `versioning.md` calls `VERSION` the source of truth, and the Registry will believe the tag |
| the changelog has a non-empty section for this version | the release notes come from it; without it a release ships nothing, or `[Unreleased]` |
| the manifest's protocol matches what the provider serves | the Registry advertises the manifest, so a wrong number breaks every consumer's `terraform init` |
| the tag is an ancestor of `main` | otherwise any branch can be tagged and published |
| `CI` and `Acceptance` were green **for that commit** | not re-run — a re-run proves a different execution, not the one anyone looked at |
| the generated documentation is in sync | stale pages are published as describing this version and cannot be corrected in place |
| the tag does not already exist elsewhere | moving a tag changes what a signature covers without changing what anyone downloaded |
| the working tree is clean | `git describe` would stamp `-dirty` on an archive corresponding to no commit |

`scripts/check-release.sh` and `task release:check` are the local half, so the
answer is reachable before the tag is pushed rather than after. Verified
against five reintroduced faults — a version that disagrees with `VERSION`, one
that is not semver, a manifest claiming protocol 5, a missing changelog section
and an empty one.

**It already found the thing blocking S7-04.** `CHANGELOG.md` has no
`## [0.1.0]` section — everything is still under `[Unreleased]` — so tagging
today would have failed after the tests, the GPG import and the syft download,
having pushed the tag. It now fails in the first job, before anything is built,
and names the file to edit.

### The settings, which nobody had looked at either

| Was | Is | Why |
| --- | --- | --- |
| merge commits and rebase allowed | squash only | the documented workflow is one squash-merged PR per sprint; the other two buttons contradicted it |
| branches kept after merge | deleted | every sprint left a branch behind |
| no topics, no homepage | ten topics, homepage set | the only discovery surface a repository has before it is in a registry |
| secret scanning off | on, with push protection | it is free on a public repository, and this one handles DNSSEC keys and TSIG secrets |
| description omitted the scale | names all 68 operations | "supports PowerDNS" is what every abandoned provider also says |

### What the repository looked like from outside

Everything inside was gated; the repository itself had never been read as a
stranger reads it. Four things were missing and one was false.

`CODEOWNERS`, a pull-request template, issue templates and a code of conduct
did not exist — the four files GitHub itself looks for, and the ones that tell
somebody arriving how this project works before they have read a standard.
They now carry the rules that already applied rather than inventing new ones:
the PR template asks for `task verify` on any resource change and for
`docs/plan.md` updated *in the same commit*, the bug template asks which
backend, because gpgsql and LMDB differ, and the feature template says up
front that dnsdist's rules and pools have no HTTP write path at all.

**And the README said the provider was not released.** "Status: in
development … Not yet published to the Terraform Registry", under a heading
called *Planned* surface, on a repository with a signed `v0.1.0` and a
complete API surface. The one document most readers see first was the one
nobody had reread.

**The acceptance workflow is green on `main`.** 203 assertions across eight
packages against the real five-container lab on a hosted runner, with the same
two skips it has locally, in 2m23s including pulling four images. That is the
claim this phase existed to make good: the gate is no longer something a
developer asserts having run.

**S8-09 is done, now that the checks have a run history to point at.** Nine
required contexts on `main`, branches must be current before merging, linear
history, no force-push, no deletion, conversations resolved. Administrators are
deliberately *not* included: a required check that hangs on somebody else's
outage would otherwise leave a one-maintainer project with no way to merge a
fix, and this repository has already met two such outages in a day. The
protection is the default path, not a cage.

Acceptance is not among the nine — that is S8-08, and it is still open for the
reason below.

S8-08 is deliberately open. Acceptance is five services and a
ninety-minute ceiling, and it has never run on hardware nobody here controls;
making every pull request wait on it before it has proven itself buys an
unreliable gate rather than a slow one. Branch protection is worth setting only
once the checks it would require have a run history to point at.

---

### The checks were shell, and shell has no tests

Nine scripts under `scripts/`, 961 lines of bash, ran the gate. Their only test
was running them against the repository's own state, which means a branch that
had never executed was indistinguishable from one that worked — and two of them
had shipped with exactly that defect: `check-pins.sh` passed locally and failed
in CI because the two machines took different paths through it, and the
`_lab-running` precondition asserted `container exists`, which is also true for
a stopped container, so it had never once done its job.

They are now `scripts/checks/`, one module per check, with `test/scripts/`
importing them. Eighty-seven assertions, and the ones worth naming are the
cases the shell could not reach at all:

| What is now tested | Why the shell version could not |
| --- | --- |
| A single timeout is retried, three are unreachable | Would need an outage to reproduce |
| A dynamic badge answering 200 with an error card | Would need a broken endpoint |
| The task and phase counters drifting | Would need a hand-edited plan |
| A line added to a released changelog section | Would need a tag and a rewrite |
| `codeql-action/init` reduced to `github/codeql-action` | Reachable, never asserted |
| A comment satisfying a version pin on its own | Reachable, never asserted |

Every conversion was checked against the script it replaced before the script
was deleted: the same corpus of commit messages through both attribution
checkers, the same 27 pins, the same 106 badges, the same verdicts from the
release gate. The badge check also came out eight times faster, because a
hundred independent HTTP requests are a thread pool in Python and a loop in
bash.

**Two defects the rewrite surfaced rather than introduced.** `task py:typecheck`
resolved `boto3` and `tenacity` only when a previous `task e2e` had left them in
the virtualenv, so whether the gate passed depended on what had been run before
it; it now names `--group e2e`. And `lint:shell` had been linting `scripts/*.sh`
— with those gone it now checks the `terraform import` snippets under
`examples/`, which are the only shell left and the only shell a reader copies.
`shfmt` went with them, having nothing left to format.

**What this cost.** `scripts/check-pins.sh` and its eight siblings are named
throughout `CHANGELOG.md` and in the older entries of this document. Those are
records of what was true when they were written and are left alone; the rename
is recorded here so a reader who finds one can follow it.

**Review found a place the rewrite was laxer than the script it replaced.**
`parse_sums` required exactly two fields and dropped anything else, so a
malformed `SHA256SUMS` line would have passed the check and then been read by
the Registry, which reads every line. The bash version fed such a line's second
field to the listing check and rejected it there. The parity comparison did not
catch this because the real file has no malformed lines — a reminder that
comparing two implementations on the input you have only proves they agree on
that input.

**SonarCloud refused the branch, and was right.** The quality gate failed on
"Security Rating C on New Code" with six findings, all in code written this
sprint: three paths built from `argv`, two git arguments built from a branch
name, and one URL path built from a registry response. The temptation is to read
these as noise about a developer's command line. They are not — the rules are
aimed at code an agent invokes, which is exactly what this repository is, and
`task worktree:new BRANCH=../../elsewhere` really does put a worktree outside
`.worktrees`. `scripts/checks/paths.py` now bounds all six, and the branch-name
rule is the one the naming standard already stated, enforced for the first time.

### An hour of nothing, and the deadline that replaces it

The end-to-end job on `main` was killed at its sixty-minute ceiling with the
lab still starting. GitHub records a timeout as "cancelled", the step's log was
never written, and the re-run finished in two minutes — so the diagnosis, a
stalled image pull, was an inference rather than a reading. Nothing in the
driver bounded anything: `wait_for_apis` had a 120-second deadline, and the
`podman-compose up` in front of it had none at all.

`scripts/automation/run.py` now carries every subprocess the drivers issue, with
a deadline and a sentence about what it was doing. Ten minutes for anything that
pulls, one for anything local, and ten for a terragrunt command — under pytest's
own 900-second ceiling on purpose, because when both would fire the failure that
names the command is the useful one.

**And the packaging trap underneath it.** `lab.py` was invoked by path, so
`sys.path[0]` was `scripts/automation/` and the new `from scripts.automation.run
import …` failed with `No module named 'scripts'`. Both drivers are modules now.
It was caught by running them; the acceptance and end-to-end workflows would
have found it on the next push, which is later and more expensively.

### The two things the end-to-end suite had never asked

**Whether a new version reads an old version's state.** Every scenario until now
applied one build to state that same build had written. The fixture now mirrors
two: 0.1.1 built from the released tag, 0.1.2 built from the working tree, with
`git archive` unpacking the tag on the host — the container cannot run git,
because `/app` is a worktree whose `.git` points at a directory that is not
mounted. Nine scenarios follow a consumer's bump: apply, check the *lock file*
to prove which build ran, `init -upgrade`, then an empty plan, no replacement,
every resource still in state.

Building HEAD twice would have been far less work and would have proved only
that Terraform can change a version number.

**Whether anyone can use an identity.** Nine resources declare identity schemas,
and the contract table has been compared against those declarations in both
directions — which establishes that the code and the document agree, and nothing
about whether the schema is usable. Six scenarios adopt a record through an
`import { identity = … }` block, against a record and zone created outside
Terraform, and read the identity back out of state. The unit is pinned to
Terraform: OpenTofu has neither the block nor the plumbing.

### Required checks now have to name a job that exists

The audit compared branch protection's contexts against the workflows' job names
by hand and found them consistent. `scripts/checks/protection.py` does it every
run, expanding matrix names — the fragile case, and the one that just became
real: `Lab acceptance (auth ${{ matrix.auth }} · …)` is two contexts whose names
change when the axis does. It warns rather than fails where the protection rule
is unreadable, so the gate does not depend on who is running it.

Writing it found the first regex counting nine spaces where the schema puts
eight, which returned the job name with the interpolation still in it.

## Audit, 2026-07-29 — before phase 7

A full re-check of the project against itself before phase 7: contract against
code, plan against its own tables, changelog against the surface, and the whole
tree through semgrep, the gate and the acceptance suite.

### What was verified mechanically

| Check | Result |
| --- | --- |
| Resource, data source, ephemeral, action and function type names | 27 in code, 27 in the contract, no difference either way |
| Identity schemas versus the contract's table | 9 resources, attributes identical |
| `ResourceWithIdentity` assertions versus `IdentitySchema` methods | the same 9, no resource declaring one without the other |
| `RequiresReplace` versus the contract's "replacement forced by" | matched for all 11 resources |
| Provider arguments and their environment variables | 12 and 12, matched |
| Changelog against every registered type | all present |
| `semgrep` — 1369 rules including OWASP Top Ten | 1 finding, addressed below |
| `task all` | green |
| `task testacc` | 129 assertions, 0 failures, 2 skips with recorded reasons |

### What it found

**The task counter had drifted.** The badge read 67 against 62 tasks actually
marked `[x]`. It was hand-incremented each sprint, which is exactly the sort of
number that rots; it is now derived from the tables.

**A deferred task had gone missing.** S5-05 was deferred "to phase 7" and never
added there, so it existed only as a note on a row nobody would look at again.
It is now S7-00.

**Phase 5 is `[~]`, and I described it as closed.** The plan was right and the
sentence was wrong. The heading now names the reason rather than leaving a
reader to find the one incomplete row.

**semgrep: no dependency cooldown for `uv`.** Real, and knowingly not applied —
`exclude-newer` makes this project's exact pins unsatisfiable, because
`ruff==0.16.0` was published inside the window and resolution fails rather than
falling back. The reasoning sits beside the setting in `pyproject.toml`.

### What it did not find

No divergence between the contract and the code. That is the check worth
repeating before every release, because the contract is the thing a user's
state file is written against — and it is also the check most likely to pass
for the wrong reason, since both sides were written by the same hand on the
same day. The mechanical comparison above is what makes it more than an
opinion.

## Audit, 2026-07-29 — after phase 9

The second full re-check, this time with a released provider, a published
registry entry and an end-to-end fixture behind it. The question was narrower
than the first audit's: not "does the code match the contract" — that check is
now mechanical and ran again clean — but "does every claim this repository
makes about its own enforcement still name something that exists".

### What was verified mechanically

| Check | Result |
| --- | --- |
| Surface counts in code | 11 resources, 7 data sources, 4 actions, 5 functions, 2 ephemeral |
| Type names, code against `docs/contract.md`, both directions | no difference either way |
| Generated registry pages | 29, in sync with the schema |
| `task …` references in documentation | 26, all resolve to a real task |
| Released changelog sections against their tags | `[0.1.1]` and `[0.1.0]` have gained nothing |
| Branch protection contexts against CI job names | 9 required, 9 jobs, exact match |
| `task all`, `task lab:verify` | green |
| `task testacc` | 8 packages ok |
| `task e2e` | 44 scenarios passed |
| Workflows on `main` | CI, Acceptance, End-to-end, Security, Scorecard all green |

### What it found

**Four standards named enforcement that does not exist.** A standard that cites
a script is making a promise the reader cannot check without looking, and three
of these had been wrong since the file was written:

| Document | Claimed | Reality |
| --- | --- | --- |
| `go-1.26-style.md` | layout `internal/resources/<area>`, `internal/client/pdns` | `internal/provider`, `internal/api/{transport,auth,rec,dnsdist}`, `internal/testutil` |
| `go-1.26-style.md` | paralleltest excluded for `test/acceptance/` | excluded by the `_acc_test.go` suffix; there is no such directory |
| `verified-identifiers.md` | `scripts/check-action-pins.sh` | renamed to `scripts/check-pins.sh` when it grew image checks |
| `naming-conventions.md` | `scripts/check-changelog.sh`, `scripts/check-version.sh` | neither was ever written; both jobs live in `scripts/check-release.sh` |

The last row is the interesting one. The naming standard listed those two
scripts in a table headed "enforced by", and the release gate did in fact
enforce both rules — under a different name. The table was aspirational at the
time it was written and nothing ever reconciled it. Three other references —
`scripts/check-changelog.sh` in the changelog standard and the plan,
`scripts/ci/*.sh` in ADR 0009 — are deliberate narration about things that
never existed, and are left as they are.

**The phase counter had drifted, the same way the task counter did.** The badge
read 6 against 8 phases marked `[x]` — and phase 9 was itself still `[~]`
although every one of S9-01…S9-07 was closed. The first audit derived the task
counter from the tables and added a check; it did not occur to me that the
counter sitting next to it had the same defect. `scripts/check-badges.sh` now
verifies both, and the phase check has a negative test.

**`task docs:drift` compared the whole `docs/` tree.** `tfplugindocs` writes six
paths; `docs/` also holds the plan, the contract, the standards and the ADRs.
Editing any of those and running the check reported "the generated
documentation is out of date", naming files no generator touches. The release
workflow was never actually exposed to this — it asserts a clean tree first —
but the standalone task lied, and a check that lies is worse than no check.
It now diffs only the generated paths.

### What it did not find

No drift between the contract and the code, and nothing added to a released
changelog section since the tag. Both were defects the first audit or a review
caught, both now have a script behind them, and both stayed clean without
anyone thinking about them. That is the whole argument for turning a finding
into a check rather than a resolution.

## Risk register

| Risk | Effect | Response |
| --- | --- | --- |
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
