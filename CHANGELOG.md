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

### Notes

- ADR 0006 records two dnsdist findings that are absent from its documentation
  and were discovered while standing up the lab: `setAPIWritable`, not
  `apiConfigDir`, gates every write, and `DELETE /api/v1/cache` answers `404`
  when the pool has no packet cache.

[Unreleased]: https://github.com/ioplane/terraform-provider-powerdns/commits/main
