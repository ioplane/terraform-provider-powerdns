---
name: sprint-workflow
description: Use when starting, running or finishing a sprint in terraform-provider-powerdns — creating a worktree, running the gate, opening the pull request, merging and cleaning up. Use when about to commit anything in this repository.
---

# Running a sprint

One worktree per sprint, one pull request per sprint, squash-merged.

`main` is never committed to directly. That has been in `AGENTS.md` since
phase 0 and was broken fourteen times before a hook enforced it — a rule in a
document is a rule that gets forgotten at the moment it applies.

## Opening

```console
scripts/worktree.sh new sprint/<phase>-<name>
cd ../.worktrees/sprint/<phase>-<name>
task up
```

`task up` builds a container **for this checkout**. Each worktree gets its own,
named from the directory, because a shared one is bind-mounted on whichever
tree started it — and `task test` would compile the wrong code.

## Working

Every claim about PowerDNS behaviour is measured, not assumed. See the
`powerdns-facts` skill; the short version is that eleven comments written from
the documentation in this project were later corrected by a single request
against the lab.

Before the pull request:

| Command | Covers |
| --- | --- |
| `task all` | build, unit, contract, lint, pins, semgrep, python, docs, vulncheck |
| `task verify` | the above plus acceptance on both authoritative backends |

A resource change needs `task verify`. Quote the acceptance result in the
commit body.

## Updating the record

Four documents, and each answers a different question:

| File | Question |
| --- | --- |
| `docs/plan.md` | What was built, and what was learned doing it |
| `docs/contract.md` | What the provider promises users |
| `CHANGELOG.md` | What changed, for someone upgrading |
| `docs/adr/` | Why a decision was taken, when it was not obvious |

A task's status changes **in the commit that does the work**. A plan updated
afterwards is a report, not a control.

## Closing

```console
gh pr create --fill
gh pr merge --squash --delete-branch
scripts/worktree.sh rm sprint/<phase>-<name>
podman rm -f terraform-provider-powerdns-dev-<name>
```

Reviews happen on GitHub only ([ADR 0008](../../docs/adr/0008-github-only-review.md)).
`.gitlab-ci.yml` has never run: no GitLab remote is configured.

## Commit messages

Conventional Commits, enforced by `commitlint` in the `commit-msg` hook:

- header ≤ 72 characters, body lines ≤ 72
- subject lower-case — `PowerDNS` in a subject fails `subject-case`
- scope from the enum in `.commitlintrc.yaml`
- **no AI attribution**, enforced by `scripts/check-no-ai-attribution.sh`

Write the body to a file and use `git commit -F`. Heredocs through the Bash
tool mangle the wrapping, and the length rules then fail on something invisible.
