# PostgreSQL 18.6 hardened lab image design

**Status:** independently SPEC-approved; exact written-design user review
pending before implementation planning.

**Owner:** `tfp-bqt.6.5`

**Blocks:** `tfp-bqt.6.1`, P10-06

## Problem

The disposable PowerDNS lab needs PostgreSQL 18.6, but the exact approved
Debian-based Docker Official Image does not pass the repository's executable
image gate. Pinned Trivy 0.72.0 reports 14 CRITICAL and 93 HIGH vulnerability
entries, one HIGH private-key finding, and licence classifications that require
manual triage. Fifty-eight HIGH/CRITICAL entries have published fixes. Pulling
the tag again resolves to the same OCI index and cannot remediate the result.

This is not a production database image and does not store persistent data. It
is nevertheless executable supply-chain input to acceptance and end-to-end
tests. Silently exempting it would make the image gate decorative.

## Decision

Build a repository-owned, worktree-local hardened image from the exact
PostgreSQL 18.6 Alpine 3.24 OCI index. Preserve the official PostgreSQL
filesystem and entrypoint, but replace `gosu` 1.19 with the same released source
built by exact Go 1.27.0. Copy the resulting complete filesystem into a
`scratch` final stage so the old vulnerable binary is absent from every final
layer rather than merely hidden by a whiteout.

The image remains a lab implementation detail. A bounded argv-form
`podman build` builds it immediately before the fixture starts, assigns a
worktree-specific local tag, and never publishes it. Every external build
input is qualified and pinned; the built image is selected and revalidated by
its content-addressed local image ID during the run.

## Verified inputs

Fresh registry and GitHub queries on 2026-08-24 establish these inputs:

| Input | Exact identity |
| --- | --- |
| PostgreSQL base | `docker.io/library/postgres:18.6-alpine3.24@sha256:d3e1620b530c944afa6e887d22eb899824da68e19c52024bf98f5220c88a65b2` |
| PostgreSQL linux/amd64 child | `sha256:63bdc97d67b5133bf0e5ebd500bec6d046fa851dc81340d838f0347e616107e8` |
| PostgreSQL linux/arm64/v8 child | `sha256:cbe15165195f7f2d63885b4d990fdec7b602248533cb05bd992284a45a58fed3` |
| Go builder | `docker.io/library/golang:1.27.0-trixie@sha256:6212da3924947f4b6a939df02ea627c13f338f1a41d6c3fcb0dd9d076eef46c4` |
| Go linux/amd64 child | `sha256:31df75a0f51705bb15c74cacaeadb6596ae270cea9ec05138928de7e2d1f65e8` |
| Go linux/arm64/v8 child | `sha256:6237d172b66951ce3ebfe0156587ad41a9773772c061fb9a5e3d612f5fd22614` |
| `gosu` module | `github.com/tianon/gosu@v0.0.0-20250923190938-6456aaa0f3c8` |
| `gosu` release commit | `6456aaa0f3c854d199d0f037f068eb97515b7513` |
| `gosu` module checksum | `h1:HIpXk5mGBQGfOqcaBbRT4Vnss8NPICnMGlD5xTlPBdQ=` |
| `gosu` module-file checksum | `h1:SwhRwWsO6iqXZN9CpIaU9CnOrUqpWDINW16KaaSqnrU=` |

The upstream release tag is `1.19`, not the Go-semver tag `v1.19.0`.
The latter does not resolve through the Go module proxy. The verified
pseudo-version above resolves to the exact release commit and is the only
module identity permitted by this design.

The Docker Official Images catalog at commit
`37079fb336f572a88700cf32e5e1f6be54a04e8f` maps PostgreSQL 18.6 to Alpine
3.24. The PostgreSQL image index and both selected children use OCI media
types. The `gosu` repository is active, not a fork or archive; release 1.19 is
the newest of all 20 releases returned by exhaustive GraphQL pagination.

## Why this variant

The same pinned scanner finds no secrets and no OS-package HIGH/CRITICAL
findings in the exact Alpine 3.24 image. Its one CRITICAL and 21 HIGH entries
all describe the Go 1.24.6 standard library embedded in
`/usr/local/bin/gosu`; every one has a published fixed Go release. Rebuilding
the unchanged `gosu` 1.19 source with Go 1.27.0 removes that entire finding
set without changing PostgreSQL or its entrypoint contract.

The remaining `golang.org/x/sys` v0.1.0 report is an UNKNOWN-severity Windows
code-path advisory in a linux binary. It is retained for explicit reachability
triage rather than mislabeled as a zero-vulnerability result.

## Alternatives considered

### Upgrade packages in the Debian image

Rejected. The report contains 49 HIGH/CRITICAL entries without a published
fixed Debian package, including core runtime libraries. A partial `apt`
upgrade would still require broad exceptions, would depend on mutable package
repositories, and would preserve the embedded test private key unless handled
separately.

### Use the official Alpine image unchanged

Rejected. It removes the secret and most operating-system findings, but its
released `gosu` binary still carries 22 fixable HIGH/CRITICAL standard-library
findings. The approved decision did not authorize that exception.

### Wait for another official image digest

Safe but not selected. The user explicitly approved continuing with the
hardened-image path. A future official image that independently passes the
same gate can supersede this derived image in a separate dependency update.

### Delete `gosu` and run PostgreSQL as a fixed non-root user

Rejected for this change. It would require new tmpfs ownership options and
would bypass the official entrypoint's root-to-`postgres` transition. Replacing
the binary preserves behavior and is the smaller compatibility change.

## Image construction

Add `deployments/containers/Containerfile.postgres-lab` with three stages:

1. A digest-pinned Go 1.27.0 build-platform stage downloads
   `github.com/tianon/gosu@v0.0.0-20250923190938-6456aaa0f3c8` with
   the verified module checksums, then uses `go build -o /out/gosu` with
   `GOTOOLCHAIN=local`, `CGO_ENABLED=0`, explicit `GOOS=$TARGETOS` and
   `GOARCH=$TARGETARCH`, `-trimpath`, and the upstream-compatible
   `-ldflags=-d -w`. This avoids the architecture-dependent output location of
   cross-compiled `go install`. It does not use `-s`, so Go module metadata
   remains available to vulnerability scanners. The build stage verifies the
   module metadata with the native Go tool, without executing a foreign binary.
2. The exact PostgreSQL Alpine base receives only the rebuilt
   `/usr/local/bin/gosu`. No command executes in this target-platform stage.
3. A `scratch` final stage copies the complete modified filesystem. It
   restates the official runtime environment, entrypoint, command, port,
   volume and stop signal, then adds the complete required
   `org.opencontainers.image.*` annotations.

Only compiler and module caches may use build-time cache mounts. No cache,
source checkout, package index, temporary key, builder binary or original
`gosu` layer is present in the final image.

The final image keeps the official runtime contract:

- UID transition remains root to the official `postgres` account through
  `docker-entrypoint.sh` and rebuilt `gosu`;
- `PG_MAJOR=18`, `PG_VERSION=18.6`, and
  `PGDATA=/var/lib/postgresql/18/docker`;
- `/var/lib/postgresql` remains the declared image volume and is overlaid by
  the existing bounded lab tmpfs;
- `/docker-entrypoint-initdb.d` retains empty-cluster initialization;
- entrypoint `docker-entrypoint.sh`, command `postgres`, TCP 5432 and
  `STOPSIGNAL SIGINT` remain exact.

## OCI identity and local selection

The external base and builder references retain readable versions plus exact
OCI index digests. The local result uses version `18.6.0-lab.1` and a tag of
the form:

```text
localhost/terraform-provider-powerdns-lab-postgres<DEV_SUFFIX>:18.6.0-lab.1
```

`DEV_SUFFIX` comes only from `scripts/dev-suffix.sh`; equal worktree leaf names
therefore cannot collide. The build receives the full current commit SHA and
its RFC 3339 commit timestamp for `revision` and `created`. It also passes that
commit's Unix epoch to `podman build --timestamp`, making the image
configuration and layer time deterministic for the same inputs. Before the
final evidence run, the committed candidate is rebuilt so these annotations
name the exact candidate rather than its base commit.

The required standard annotations are `created`, `revision`, `version`,
`source`, `title`, `description`, `vendor`, and `documentation`. Standard
`base.name` and `base.digest` annotations are deliberately absent: OCI 1.1
defines them for an immediate base that shares layers, while the flattened
final image shares no layers with the PostgreSQL build stage. Namespaced
`io.ioplane.image.source-base.name` and `io.ioplane.image.source-base.digest`
annotations preserve the exact source provenance without making the false OCI
base-layer claim.

The optional standard `licenses` annotation is also deliberately absent. The
contained Alpine packages, PostgreSQL, entrypoint scripts and rebuilt `gosu`
have a conjunction of licences, including a scanner-reported `Public-Domain`
value that is not a standalone SPDX licence-expression identifier. Reducing
that inventory to `PostgreSQL AND Apache-2.0` would be materially incomplete.
The generated SPDX SBOM is the authoritative, lossless licence record; the OCI
`documentation` annotation points to the committed audit that identifies both
SBOMs and the complete scanner inventory.

Compose may use the worktree-local tag only because `lab.py` owns the build and
the lifecycle validates the exact resulting image ID immediately after build
and immediately before teardown. No external image reference is allowed to use
a tag without a digest.

## Integration boundary

The PostgreSQL service uses a required local image variable; it does not gain a
Compose `build` block. `scripts/automation/lab.py` is the single owner of the
build and Compose environment:

- derive the suffix through the existing canonical adapter;
- obtain revision, RFC 3339 commit time and Unix epoch with bounded argv-form
  Git calls;
- run bounded argv-form `podman build` with `--format=oci`, `--pull=always`,
  `--timestamp`, the exact build arguments and the worktree-specific tag;
- capture the returned image ID, inspect it, and only then invoke Compose up
  with an explicit environment;
- pass the identical environment to status, verify and down;
- fail closed when Git identity, timestamp, image ID or OCI labels are absent.

No new Python dependency or shell-string command is introduced. The approved
locked `uv`/Python boundary remains unchanged; the later P10-12 migration owns
moving that automation to Go.

## Verification contract

Tests are written before production edits and must reject:

- the Debian base, a floating Alpine tag, a wrong digest, registry ambiguity,
  or a non-OCI external manifest;
- omission of the rebuilt `gosu`, use of a prebuilt release binary, stripping
  Go module metadata, or copying the modified base directly without the
  `scratch` flattening stage;
- missing or malformed OCI annotations;
- a shared local image tag, basename-only identity, missing build metadata,
  unbounded Git/build/Compose calls, an unqualified build context, or Compose
  starting before the exact local image ID is verified;
- drift in entrypoint, command, PostgreSQL environment, port, stop signal,
  declared volume, schema bind, tmpfs or health check.

After static GREEN, build separate worktree-local linux/amd64 and
linux/arm64/v8 tags and capture each image ID. The Containerfile uses
`FROM --platform=$BUILDPLATFORM` for the Go compiler and Buildah's automatic
`TARGETOS`/`TARGETARCH` arguments for cross-compilation. Target stages contain
no `RUN`, so the current amd64 host does not require QEMU or binfmt handlers.
Do not use a multi-platform `--manifest` build: Buildah ignores an ordinary
tag in that mode and forbids `--iidfile`, which would weaken the required
per-platform identity capture. Compose selects only the native amd64 tag; the
arm64 tag is a build-and-scan portability artifact and is never executed on
this host.

For each platform, verify OCI manifest/config media types, architecture,
labels, history, layer count, entrypoint and absence of build/cache residue.
Compare normalized source-child and final filesystem metadata manifests over
every path: path, type, UID, GID, mode and symlink target must match, with only
the `gosu` content hash permitted to change. Assert independently that
`/var/lib/postgresql` is `70:70` mode `1777`, `/var/run/postgresql` is `70:70`
mode `3777`, every entrypoint script remains executable, and rebuilt
`/usr/local/bin/gosu` is `0:0` mode `0755`. After native initialization,
`PGDATA` must be `70:70` mode `0700`. The final filesystem must contain exactly
one flattened rootfs layer created by this build; the original vulnerable
`gosu` bytes must not be recoverable from any final layer.

The native amd64 runtime additionally executes `gosu --version` and validates
the embedded Go and module metadata. The arm64 contract is verified without
execution through OCI architecture, the build arguments, `go version -m`, the
scanner package inventory and the final-layer filesystem hash.

Pinned Trivy scans the Containerfile with `trivy config`. It scans both final
platform images with vulnerability, secret and licence scanners, and also
enables `--image-config-scanners misconfig` so the generated OCI image
configuration is actually evaluated. A filesystem `--scanners misconfig`
result alone is not image-configuration evidence. Every report records
non-zero package or test inputs as applicable, database timestamps and full
JSON output. Acceptance requires:

- zero secrets;
- zero HIGH or CRITICAL vulnerabilities;
- zero untriaged Containerfile or OCI image-configuration findings, with
  narrow exact-check exceptions documented for the preserved root-to-`gosu`
  entrypoint and the Compose-owned health check when either pinned scan reports
  them; duplicate check identifiers are reconciled once, and absence from one
  scan is never evidence that the other scan executed;
- explicit triage of every UNKNOWN vulnerability and unknown-category licence;
- no repository-incompatible licence;
- CycloneDX and SPDX SBOMs with non-zero package inventories.

The image is not published, so this change does not claim a registry referrer,
signature or hosted provenance. The audit must say so explicitly.

## Runtime and regression gates

Changing the executable PostgreSQL image invalidates all previous Task 4
runtime evidence. Repeat from clean preflight:

1. Authoritative 5.1 clean bootstrap, SQL schema/version/index/constraint/row
   oracles, full provider verify, full e2e lifecycle, ownership guards and zero
   residue;
2. the identical complete sequence on Authoritative 5.0;
3. `task all`, OSV, release dry-run, image/SBOM scans and documentation gates;
4. exact final OCI and filesystem inspection after the candidate rebuild;
5. sequential SPEC then QUALITY review of an immutable candidate, followed by
   the repository's reviewed closure and pull-request workflow.

Any executable edit after either compatibility run invalidates both runtime
runs. Any image-content edit invalidates both platform scans and SBOMs.

## Rollback

Rollback removes the local build definition, the lab automation build path and
the required local-image variable, then restores the previously verified
PostgreSQL 17 digest, documentation and observation constant in one revert.
It also moves the bounded tmpfs from the PostgreSQL 18 parent destination back
to PostgreSQL 17's exact declared-volume and `PGDATA` destination:

```text
/var/lib/postgresql/data:rw,nosuid,nodev,noexec,size=512m
```

Leaving the PostgreSQL 18 parent tmpfs in place is not a valid rollback because
the nested PostgreSQL 17 image volume would become an anonymous volume. The
fixture contains no reusable PostgreSQL data volume, so rollback does not
claim or attempt a data downgrade. Exact project ownership guards and
zero-residue teardown remain mandatory before either image is changed.
