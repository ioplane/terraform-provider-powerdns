<!-- markdownlint-disable MD013 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=ADR+0009&subtitle=GitHub+Actions+is+the+gate&logo=githubactions&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="ADR 0009" src="https://shieldcn.dev/header/graph.svg?title=ADR+0009&subtitle=GitHub+Actions+is+the+gate&logo=githubactions&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>
<!-- markdownlint-enable MD013 -->

<div align="center">

![status accepted](https://shieldcn.dev/badge/status-accepted-3fb950.svg?variant=secondary)
![date 2026--07--29](https://shieldcn.dev/badge/date-2026--07--29-0969da.svg?variant=secondary)
[![adr 0009](https://shieldcn.dev/badge/adr-0009-6e7781.svg?variant=secondary)](../README.md)

</div>

# ADR 0009 — GitHub Actions is the gate

- **Status:** accepted
- **Date:** 2026-07-29
- **Deciders:** PM, OPS
- **Supersedes:** the third consequence of
  [ADR 0008](0008-github-only-review.md) — "GitHub Actions stays release-only"

## Context

[ADR 0008](0008-github-only-review.md) settled that review happens on GitHub,
and kept `.gitlab-ci.yml` in the tree "as the definition of the gate for a
mirror that may exist later", to be "kept current — the job list matches
`task all`".

It was not kept current. By the time this decision was taken the file referred
to `scripts/ci/start-lab.sh` and `scripts/ci/lab-diagnostics.sh`, neither of
which exists; it ran the contract tests as `go test -tags contract
./internal/api/...`, and the contract tests carry no build tag and live in
`internal/testutil`; and its acceptance matrix split gpgsql and lmdb into two
jobs, where the suite reads both endpoints in one run and skips what a backend
cannot do.

None of that was noticed, because nothing ran it. That is the finding, and it
generalises: a pipeline nobody executes does not stay correct, and its
correctness cannot be assumed from the fact that somebody wrote it carefully.
It reads as a gate in review while enforcing nothing.

Meanwhile the actual gate was `task all` on a developer's machine, quoted in a
commit body. ADR 0008 recorded that as "weaker than a pipeline enforcing it,
and the accepted cost until a runner exists". A runner exists: GitHub-hosted,
already used for release.

## Decision

GitHub Actions runs the gate. `.gitlab-ci.yml` is removed.

Six workflows, split by what they answer to:

| Workflow | Answers | When |
| --- | --- | --- |
| `ci.yml` | `task all` — build, test, lint, docs, commits | every push and pull request |
| `acceptance.yml` | `task testacc` against the real lab | main, nightly, on demand |
| `security.yml` | CodeQL, Semgrep, osv-scanner, Trivy | push, pull request, weekly |
| `scorecard.yml` | OpenSSF Scorecard | main, weekly |
| `dependency-review.yml` | new dependencies: severity and licence | pull request |
| `release.yml` | signed artefacts for the Registry | version tags |

Three properties make this different from writing the gate a second time.

**`ci.yml` mirrors `task all`, job for job.** Every job names the task it
corresponds to. A developer can reproduce any failure it reports before
pushing, which is the difference between a gate and an obstacle.

**The toolchain is pinned once.** `deployments/containers/Containerfile.dev`
holds every version. A workflow line that names one carries `# pin: <ARG>`, and
`scripts/checks/tool_versions.py` — part of `task all` — fails if the two
disagree, or if a marker is deleted. Without this the duplication ADR 0008
feared is real: a linter at one version locally and another in CI produces an
argument about which machine is right rather than about the code.

**Scanners are separated from the gate.** `ci.yml` reports what the code says;
`security.yml` reports what a vulnerability database says today. The second
changes without the code changing, so its findings arrive as code-scanning
alerts rather than as a red tick on a pull request that introduced none of
them. `govulncheck` is the exception and stays in the gate: it reports only
what the binary can actually reach, so it is about this code.

## Consequences

- The AI-attribution ban (AGENTS.md golden rule 6) is enforced on the branch
  for the first time. The commit-msg hook only ever protected a clone that had
  run `task hooks`; `ci.yml` checks every commit in a pull request, and the
  title and body, because those become the squash message.
- Acceptance runs in CI at all, which is unusual for a provider: the lab is
  five digest-pinned containers with nothing to authenticate against, so a
  hosted runner can start the whole thing. It is not yet a pull-request gate —
  ninety minutes and a first run on hardware nobody here controls is an
  unreliable gate rather than a slow one. It moves to pull requests once a run
  history says it is stable.
- `task semgrep` now runs in the dev container rather than on the host. It had
  no pinned version anywhere, which is fine for a tool nobody else runs and not
  fine for one CI has to agree with.
- Two defects in `release.yml` are fixed as part of this: it ran GoReleaser at
  `latest`, and it took the release notes from whichever changelog section was
  on top rather than the one matching the tag. Both would have shipped.
- The dead `.gitlab-ci.yml` is gone. If a mirror is ever wanted, the gate to
  port is the one that runs.
