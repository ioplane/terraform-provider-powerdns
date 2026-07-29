<!-- markdownlint-disable MD013 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=Naming+conventions&subtitle=Files%2C+branches%2C+commits%2C+versions&logo=abstract&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="Naming conventions" src="https://shieldcn.dev/header/graph.svg?title=Naming+conventions&subtitle=Files%2C+branches%2C+commits%2C+versions&logo=abstract&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>
<!-- markdownlint-enable MD013 -->

<div align="center">

[![status normative](https://shieldcn.dev/badge/status-normative-cf222e.svg?variant=secondary)](../README.md)
![the synthesis 5 sources](https://shieldcn.dev/badge/the_synthesis-5_sources-0969da.svg?variant=secondary)
![enforced see the table](https://shieldcn.dev/badge/enforced-see_the_table-3fb950.svg?variant=secondary)

</div>

# Naming conventions

One standard covering every name this project produces: files, identifiers,
branches, commits, versions and changelog sections.

It is a **synthesis** of five sources rather than a restatement of any one of
them:

| Source | What it contributes |
| --- | --- |
| [Harvard HMS file-naming conventions](https://datamanagement.hms.harvard.edu/plan-design/file-naming-conventions) | Machine-readable file names: no spaces, ISO dates, most significant token first |
| [IT Glue naming best practices](https://www.itglue.com/blog/naming-conventions-examples-formats-best-practices/) | Consistency as an operational property: one scheme, documented, enforced |
| [Semantic Versioning 2.0.0](https://semver.org/) | The version is a name, and it makes a promise |
| [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/) | The commit subject is a structured name, machine-parseable |
| [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/) | Change categories are a closed vocabulary, not free prose |

The synthesis is the point: the same tokens travel from a branch name through a
commit type into a changelog section and out into a version bump. If they do
not line up, the toolchain cannot check them and a human has to.

## 1. The chain

A change is named once and that name propagates:

```text
branch      feat/dnssec/cryptokey-resource
commit      feat(dnssec): add powerdns_zone_cryptokey
changelog   ### Added
version     MINOR
```

| Commit type | Changelog section | SemVer effect |
| --- | --- | --- |
| `feat` | `Added` | MINOR |
| `fix` | `Fixed` | PATCH |
| `feat!` / `BREAKING CHANGE:` | `Changed`, prefixed `BREAKING:` | MAJOR (MINOR while `0.x`) |
| `perf` | `Changed` | PATCH |
| `refactor`, `style`, `test`, `chore`, `build`, `ci` | none — no user-visible change | none |
| `docs` | `Changed` only if it corrects documented behaviour | none |
| security fix | `Security` | PATCH, or MINOR if the fix narrows accepted input |

`Deprecated` and `Removed` have no commit type of their own: deprecation is a
`feat` that adds a warning, removal is a `feat!`.

This table is the standard's core. Everything below serves it.

## 2. Universal rules for names

From Harvard HMS and IT Glue, applied to a code repository:

- **Alphanumeric plus `-` and `_` only.** No spaces, no
  `~ ! @ # $ % ^ & * ( ) : < > ? , [ ] { } ' " |`.
- **One separator per context** (§3); never mix within one identifier.
- **ISO 8601 dates** (`YYYY-MM-DD`) so names sort chronologically.
- **Most significant token first**, so lexical sort is useful:
  `AUDIT-01-…`, `0001-…`, `feat/dnssec/…`.
- **Concise but descriptive** — aim for ≤ 50 characters in file names.
- **No vague descriptors**: `final`, `new`, `latest`, `copy`, `v2`. Git and
  SemVer exist for that.
- **No undocumented abbreviations.** Coining one means adding it to §7 and to
  `.cspell.json` in the same commit.

IT Glue's contribution is the one people skip: a convention that is not
enforced is a preference. Every rule here that can be checked mechanically is
checked — see §8.

## 3. Case by context

| Context | Case | Example |
| --- | --- | --- |
| Go files | `snake_case.go` | `zone_metadata.go`, `transport_test.go` |
| Go packages | short, lowercase, no underscores | `dnsdist`, `testutil` |
| Go exported identifiers | `PascalCase` | `ZoneResource`, `APIError` |
| Go unexported identifiers | `camelCase` | `newHTTPClient`, `providerModel` |
| Markdown | `kebab-case.md` | `naming-conventions.md` |
| ADRs | `NNNN-kebab.md` | `0006-dnsdist-scope.md` |
| Design records | `DESIGN-NN-kebab.md` | `DESIGN-01-target-architecture.md` |
| Audit records | `AUDIT-NN-kebab.md` | `AUDIT-01-baseline.md` |
| Directories | `kebab-case` or one word | `deployments/`, `internal/api/` |
| YAML / config | `kebab-case.yml` | `compose.dev.yml` |
| Terraform files | `snake_case.tf` | `main.tf` |
| Environment variables | `SCREAMING_SNAKE_CASE` | `PDNS_SERVER_URL` |
| Container images | `lowercase-with-dashes` | `terraform-provider-powerdns-dev` |

## 4. Terraform-facing names — the public contract

These names **are** the provider's public API. Changing one is breaking
(see [`versioning.md`](versioning.md)).

**Product prefix.** Unprefixed means Authoritative; this is a rule, not an
accident of history:

| Product | Prefix | Example |
| --- | --- | --- |
| Authoritative | none | `powerdns_zone`, `powerdns_tsigkey` |
| Recursor | `recursor_` | `powerdns_recursor_zone` |
| dnsdist | `dnsdist_` | `powerdns_dnsdist_acl` |

**Type names:** `powerdns_<noun>`, `snake_case`, **singular**. A resource
manages one object; a data source returning many is plural
(`powerdns_zones`).

**Attributes:** `snake_case`, matching the PowerDNS field name where the API
name is not actively misleading. Where PowerDNS is itself inconsistent the API
name still wins — `soa_edit` and `soa_edit_api` are distinct fields and both
keep their names.

**Booleans describe an action and default to `false`**: `api_rectify`,
`nsec3narrow` — never `disable_*`.

**Identifiers:** `id` is computed. Typed references carry the type:
`zone_id`, `tsigkey_id`. Timestamps are RFC 3339 and suffixed `_at`.

**Deliberate divergence from the ecosystem, recorded rather than hidden.**
PowerDNS calls a secondary zone's sources `masters` and the kind `Slave`.
PowerDNS itself has moved to primary/secondary language, but its API still
sends and accepts the old names. The API name wins: a user reading the PowerDNS
documentation and a user reading ours must find the same word.

## 5. Git branches

| Kind | Pattern | Example |
| --- | --- | --- |
| Sprint | `sprint/<id>-<scope>` | `sprint/S2-auth-client` |
| Feature | `feat/<scope>/<name>` | `feat/dnssec/cryptokey-resource` |
| Fix | `fix/<scope>/<name>` | `fix/zone/ipv6-masters` |
| Chore | `chore/<scope>/<name>` | `chore/deps/framework-bump` |
| Build | `build/<scope>/<name>` | `build/repo/taskfile` |
| CI | `ci/<scope>/<name>` | `ci/actions/acceptance-lab` |
| Docs | `docs/<scope>/<name>` | `docs/standards/naming` |

`<type>` is a Conventional Commits type; `<scope>` is one of the scopes in
`.commitlintrc.yaml`. The branch name therefore predicts the commit type, which
predicts the changelog section, which predicts the version bump — §1.

## 6. Versions, tags, artefacts

- Versions follow SemVer 2.0.0; the source of truth is `VERSION`.
- Tags are `vX.Y.Z`, annotated and GPG-signed.
- Pre-releases: `vX.Y.Z-rc.N`, `-beta.N`, `-alpha.N`.
- Release archives, mandated verbatim by the Terraform Registry:
  `terraform-provider-powerdns_<version>_<os>_<arch>.zip`.
- Container images: `terraform-provider-powerdns-dev:<git-describe>`, and
  **always referenced by digest**, never by tag, in anything that runs.

## 7. Glossary of accepted abbreviations

| Abbreviation | Meaning |
| --- | --- |
| `pdns` | PowerDNS |
| `auth` | PowerDNS Authoritative Server |
| `rec` | PowerDNS Recursor |
| `rrset` | resource record set |
| `soa` | start of authority |
| `tsig` | transaction signature |
| `axfr` | full zone transfer |
| `acl` | access control list |
| `acc` | acceptance (test) |
| `crud` | create/read/update/delete |
| `lmdb` | Lightning Memory-Mapped Database backend |
| `gpgsql` | generic PostgreSQL backend |
| `oci` | Open Container Initiative |

Add entries here and to `.cspell.json` in the same commit that introduces them.

## 8. What is enforced mechanically

IT Glue's point, made operational. A rule nobody checks is a preference:

| Rule | Enforced by |
| --- | --- |
| Commit subject shape and type | `commitlint`, in the `commit-msg` hook and on MR titles |
| Scope is from the closed list | `.commitlintrc.yaml` `scope-enum` |
| Go identifier and file case | `golangci-lint` (`revive`, `gofumpt`) |
| Spelling and abbreviations | `cspell` against `.cspell.json` |
| Markdown file conventions | `markdownlint-cli2` |
| Changelog section vocabulary | nobody — read in review |
| Changelog: nothing added to a released section | `scripts/check-release.sh` |
| Version format, and `VERSION` agreeing with the tag | `scripts/check-release.sh` |
| Image and action pins by hash | `scripts/check-pins.sh` |

The remainder — branch names, document codes — is checked at review. Where a
rule proves worth enforcing, it moves into this table rather than staying an
exhortation.
