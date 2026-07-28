<!-- markdownlint-disable MD033 MD041 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=Changelog&subtitle=Keep+a+Changelog+1.1.0&logo=keepachangelog&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="Changelog" src="https://shieldcn.dev/header/graph.svg?title=Changelog&subtitle=Keep+a+Changelog+1.1.0&logo=keepachangelog&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>

<div align="center">

[![status normative](https://shieldcn.dev/badge/status-normative-cf222e.svg?variant=secondary)](../README.md)
[![keep a changelog 1.1.0](https://shieldcn.dev/badge/keep_a_changelog-1.1.0-0969da.svg?variant=secondary)](#)
[![enforced see the table](https://shieldcn.dev/badge/enforced-see_the_table-3fb950.svg?variant=secondary)](#)

</div>

# Changelog conventions

[`CHANGELOG.md`](../../CHANGELOG.md) follows
[Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/).

## Rules

- **Human-written and curated.** Not a `git log` dump — describe user impact.
- **`[Unreleased]` accumulates** at the top. Every merge request with a
  user-visible change adds an entry; the MR template checks it.
- **Closed vocabulary**, in this order: `Added`, `Changed`, `Deprecated`,
  `Removed`, `Fixed`, `Security`. No other section headings. The vocabulary is
  closed so that the commit-type mapping in
  [`naming-conventions.md`](naming-conventions.md) §1 can be checked.
- **Newest release first.** Headings are `## [X.Y.Z] — YYYY-MM-DD`, ISO date,
  em dash.
- **Breaking changes** go under `Changed`, prefixed `BREAKING:`, with a
  migration note.
- **Link the diff** at the bottom.

## Entries carry evidence

An entry that fixes a defect states what was wrong, not that something was
fixed:

```markdown
### Fixed

- `powerdns_zone` no longer produces a permanent diff when a master is written
  with uncompressed IPv6 zeros. PowerDNS stores the compressed form, so
  `:0000:` returned as `:0:` made every plan dirty. Masters are now compared as
  parsed addresses.
```

"Fixed a bug in zone masters" tells a reader nothing they can act on.

## Release cut

1. Rename `[Unreleased]` to `## [X.Y.Z] — YYYY-MM-DD`; recreate an empty
   `[Unreleased]`.
2. Update `VERSION`.
3. Commit `chore(release): X.Y.Z`.
4. Tag `vX.Y.Z`, annotated and GPG-signed.
5. The release workflow extracts the section between two `## [` headings and
   feeds it to goreleaser as the release notes.

Step 5 parses the file mechanically, so the heading format is exact.
`scripts/check-changelog.sh` verifies it before the tag.
