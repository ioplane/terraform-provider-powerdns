<!-- markdownlint-disable MD013 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=Commits&subtitle=Conventional+Commits+1.0.0&logo=conventionalcommits&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="Commits" src="https://shieldcn.dev/header/graph.svg?title=Commits&subtitle=Conventional+Commits+1.0.0&logo=conventionalcommits&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>
<!-- markdownlint-enable MD013 -->

<div align="center">

[![status normative](https://shieldcn.dev/badge/status-normative-cf222e.svg?variant=secondary)](../README.md)
![conventional 1.0.0](https://shieldcn.dev/badge/conventional-1.0.0-0969da.svg?variant=secondary)
![enforced see the table](https://shieldcn.dev/badge/enforced-see_the_table-3fb950.svg?variant=secondary)

</div>

# Commit conventions

[Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/),
enforced by `commitlint` in the `commit-msg` hook and against merge-request
titles in CI.

## Shape

```text
<type>(<scope>): <summary, imperative, lower case, ≤ 72 chars>

<body — WHY, wrapped at 72, blank line before>

<footer — Refs:, Closes:, BREAKING CHANGE:>
```

## Types

| Type | Use | Changelog | Version |
| --- | --- | --- | --- |
| `feat` | New user-facing capability | `Added` | MINOR |
| `fix` | Bug fix | `Fixed` | PATCH |
| `perf` | Performance change | `Changed` | PATCH |
| `docs` | Documentation only | usually none | none |
| `refactor` | Behaviour-preserving change | none | none |
| `test` | Tests only | none | none |
| `build` | Containerfile, Taskfile, compose, `go.mod` | none | none |
| `ci` | Pipelines | none | none |
| `style` | Formatting only | none | none |
| `chore` | Repository hygiene | none | none |
| `revert` | Revert of a prior commit | mirrors the reverted entry | mirrors |

The mapping is not advisory — it is the chain in
[`naming-conventions.md`](naming-conventions.md) §1, and the changelog gate
checks that a `feat` in a release range produced an `Added` entry.

## Scopes

Closed list in `.commitlintrc.yaml`: `provider`, `zone`, `record`, `metadata`,
`view`, `network`, `dnssec`, `tsig`, `autoprimary`, `recursor`, `dnsdist`,
`transport`, `auth`, `actions`, `functions`, `ephemeral`, `docs`, `examples`,
`lab`, `ci`, `build`, `deps`, `release`, `test`, `lint`, `repo`, `standards`.

## Breaking changes

`feat(scope)!: …` or a `BREAKING CHANGE:` footer naming the migration path.
Every breaking change also gets a `CHANGELOG.md` entry under `Changed` with a
`BREAKING:` prefix.

## Evidence in the body

The body is where the *why* lives, and for this project that includes the
evidence golden rule 3 requires. A commit relying on a claim about PowerDNS
behaviour cites it:

```text
fix(zone): compare masters semantically rather than as strings

PowerDNS stores the compressed form of an IPv6 address, so a master written
fd92:81e1:e314:ea7b:0000:1234:5678:60ab is returned as
fd92:81e1:e314:ea7b:0:1234:5678:60ab and every subsequent plan wants to change
the zone.

Verified against auth-5.1.3: POST /zones echoes the compressed form.

6/6 acceptance tests pass on both backends.

Closes: #12
```

## Prohibited

No mention of AI, assistants or generated authorship in any part of a commit —
subject, body, footer or trailer. Enforced by
`scripts/check-no-ai-attribution.sh` in the `commit-msg` hook. Human
co-authorship uses the ordinary `Co-Authored-By:` trailer.
