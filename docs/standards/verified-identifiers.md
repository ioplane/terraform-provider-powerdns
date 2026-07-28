<!-- markdownlint-disable MD033 MD041 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=Verified+identifiers&subtitle=Never+write+a+hash+from+memory&logo=keycdn&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="Verified identifiers" src="https://shieldcn.dev/header/graph.svg?title=Verified+identifiers&subtitle=Never+write+a+hash+from+memory&logo=keycdn&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>

<div align="center">

[![status normative](https://shieldcn.dev/badge/status-normative-cf222e.svg?variant=secondary)](../README.md)
[![enforced check-pins.sh](https://shieldcn.dev/badge/enforced-check--pins.sh-0969da.svg?variant=secondary)](#)
[![enforced see the table](https://shieldcn.dev/badge/enforced-see_the_table-3fb950.svg?variant=secondary)](#)

</div>

# Verified identifiers

An identifier that must be exact is never written from memory. It is looked up,
and the lookup is what goes in.

This standard exists because the rule was violated twice in this repository
within a day — a fabricated commit SHA for `astral-sh/setup-uv`, then another
for `go-task/setup-task`. Both were caught only because someone happened to run
a verification command. Twice is a pattern, and a pattern needs enforcement
rather than another resolution to be careful.

## 1. What counts as an exact identifier

Anything where a plausible-looking wrong value is indistinguishable from the
right one by inspection:

| Kind | Example | Where it comes from |
|---|---|---|
| Git commit SHA | `01a4adf9db2d…` | `gh api repos/<r>/git/ref/tags/<tag> --jq .object.sha` |
| Release tag | `v3.52.0` | `gh api repos/<r>/releases/latest --jq .tag_name` |
| Module version | `v1.19.0` | `curl -s https://proxy.golang.org/<mod>/@latest \| jq -r .Version` |
| Container digest | `sha256:…` | `skopeo inspect docker://<ref> --format '{{.Digest}}'` |
| Checksum | any | the artefact itself |
| Advisory identifier | `GO-2026-6061` | `govulncheck` output |
| Source location | `ws-auth.cc:3349` | the file, at the pinned tag |
| API endpoint or status | `422` on `POST /views/<v>` | a live round-trip |

## 2. The rule

**Recalling one of these is not permitted. Fetch it, then paste what came back.**

The failure mode is specific and worth naming: a fabricated SHA is *syntactically
valid*. It passes YAML parsing, passes review, passes every check except the one
that resolves it upstream. It fails later, in someone else's pull request, with
an error that points at the workflow rather than at the change that introduced
it. The cost is displaced onto whoever comes next.

The same applies to a version number that looks right, a line number that looks
right, and a status code that seems obvious. "Seems obvious" is exactly the
state in which this goes wrong.

## 3. Enforcement

| Identifier | Gate |
|---|---|
| Action pins in workflows | `scripts/check-action-pins.sh` — pre-commit hook, `task lint:pins`, and a CI job |
| Go module versions | `go.sum`, and `go mod tidy` in the gate |
| Container images | pinned tags in `Containerfile.dev`, mirrored into `compose.dev.yml`; `task lab:verify` asserts the running versions |
| PowerDNS behaviour | [`powerdns-api-discipline.md`](powerdns-api-discipline.md) — sources plus a live round-trip |
| Source locations in prose | reviewer opens the cited file at the cited tag |

`scripts/check-action-pins.sh` rejects two things: a SHA that does not resolve
upstream, and a floating tag used where a SHA belongs. On its first run it found
four floating tags in `release.yml` — a file nobody had touched, holding the GPG
signing key.

## 4. Floating tags are a separate offence

`actions/checkout@v6` is not an identifier at all; it is a mutable pointer. In a
workflow with access to a signing key, that is a supply-chain position. Pin the
SHA and put the version in a trailing comment:

```yaml
- uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
```

The comment is for humans and Dependabot; the SHA is what runs.

## 5. When the lookup is unavailable

If the network or the tool is unavailable, the correct move is to **say so and
stop**, not to supply a value that looks plausible. A blocked task is visible; a
fabricated identifier is not.

The hook degrades this way deliberately: with no authenticated `gh` it warns and
exits zero rather than blocking an offline commit, because CI runs the same
check and will catch it there. That is a considered trade, not an escape hatch —
the check still happens, just later.
