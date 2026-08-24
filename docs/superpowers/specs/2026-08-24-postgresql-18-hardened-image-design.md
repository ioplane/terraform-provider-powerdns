# PostgreSQL 18.6 hardened lab image design

**Status:** user-approved Alpine direction; revised version-aware Go mechanism
and replacement plan passed plan review; implementation remains stopped until
fresh exact SPEC then QUALITY approval.

**Owner:** `tfp-bqt.6.5`

**Blocks:** `tfp-bqt.6.1`, P10-06

## Problem

The disposable PowerDNS lab needs PostgreSQL 18.6, but the exact approved
Debian-based Docker Official Image does not pass the repository's executable
image gate. The historical Trivy 0.72.0 baseline reports 14 CRITICAL and 93
HIGH vulnerability entries, one HIGH private-key finding, and licence classifications that require
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
| PostgreSQL linux/amd64 child ref | `docker.io/library/postgres@sha256:63bdc97d67b5133bf0e5ebd500bec6d046fa851dc81340d838f0347e616107e8` |
| PostgreSQL linux/amd64 config/local ID | `sha256:b07129cc272f688c98f5b343138a0a52fa45b3d82f50d7a53ff441330624cd2e` |
| PostgreSQL linux/arm64/v8 child | `sha256:cbe15165195f7f2d63885b4d990fdec7b602248533cb05bd992284a45a58fed3` |
| PostgreSQL linux/arm64/v8 child ref | `docker.io/library/postgres@sha256:cbe15165195f7f2d63885b4d990fdec7b602248533cb05bd992284a45a58fed3` |
| PostgreSQL linux/arm64/v8 config/local ID | `sha256:6def1cb8d5ffa3443c527419cc13f395ab328c27bf90fcb1e80831aae4103bc3` |
| Go builder | `docker.io/library/golang:1.27.0-trixie@sha256:6212da3924947f4b6a939df02ea627c13f338f1a41d6c3fcb0dd9d076eef46c4` |
| Go linux/amd64 child | `sha256:31df75a0f51705bb15c74cacaeadb6596ae270cea9ec05138928de7e2d1f65e8` |
| Go linux/arm64/v8 child | `sha256:6237d172b66951ce3ebfe0156587ad41a9773772c061fb9a5e3d612f5fd22614` |
| `gosu` module | `github.com/tianon/gosu@v0.0.0-20250923190938-6456aaa0f3c8` |
| `gosu` release commit | `6456aaa0f3c854d199d0f037f068eb97515b7513` |
| `gosu` module checksum | `h1:HIpXk5mGBQGfOqcaBbRT4Vnss8NPICnMGlD5xTlPBdQ=` |
| `gosu` module-file checksum | `h1:SwhRwWsO6iqXZN9CpIaU9CnOrUqpWDINW16KaaSqnrU=` |
| Trivy scanner | `docker.io/aquasec/trivy:0.74.0@sha256:62b1e65e8869bc4b4c6aa4fa2b21595256c7c2f6018a9d9ad61caf87187c1969` |
| Trivy linux/amd64 child | `sha256:ee940acbf1f58ebadb42d01434ce4609530bf1b52536afbd1eee66cd7123c5c9` |
| Trivy linux/arm64 child | `sha256:55ad20f8a239a3e95427e60b8aaea38788550c18a3f1772976bebf732e6ae166` |

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

Add `deployments/containers/Containerfile.postgres-lab` with three stages.
Global `BUILDPLATFORM` and `TARGETPLATFORM` arguments are declared before any
`FROM`, so Buildah populates and expands them in stage selectors:

1. A digest-pinned Go 1.27.0 build-platform stage installs
   `github.com/tianon/gosu@v0.0.0-20250923190938-6456aaa0f3c8` with
   the verified module checksums through version-aware `go install` with
   `GOTOOLCHAIN=local`, `CGO_ENABLED=0`, explicit `GOOS=$TARGETOS` and
   `GOARCH=$TARGETARCH`, `-trimpath`, and the upstream-compatible
   `-ldflags=-d -w`. It copies the resulting native `$GOPATH/bin/gosu` or
   cross-compiled `$GOPATH/bin/${TARGETOS}_${TARGETARCH}/gosu` to the single
   `/out/gosu` handoff path. Building the downloaded module as the main module
   is forbidden because Go records its version as `(devel)`; the version-aware
   install must embed the exact pseudo-version. It does not use `-s`, so Go
   module metadata remains available to vulnerability scanners. The build
   stage parses the tab-delimited `go version -m` output with the native Go
   tool, without executing a foreign binary.
2. The exact PostgreSQL Alpine base receives only the rebuilt
   `/usr/local/bin/gosu`. No command executes in this target-platform stage.
3. A `scratch` final stage copies the complete modified filesystem. It
   restates the official runtime environment, entrypoint, command, port,
   volume and stop signal, then adds the complete required
   `org.opencontainers.image.*` annotations.

Only the compiler cache may use a persistent build-time cache mount. The module
cache is a fresh tmpfs for each build step, so a previously extracted or
poisoned module tree cannot feed `go install`; the exact download sums remain
mandatory. No cache, source checkout, package index, temporary key, builder
binary or original `gosu` layer is present in the final image.

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

The source config encodes the build-only `DOCKER_PG_LLVM_DEPS` package list as
`llvm21-dev`, two tab bytes, then `clang21`. The flattened image retains the
same two-token value normalized to one ASCII space. This is the sole permitted
environment-byte normalization; tests compare its parsed token sequence while
requiring every runtime-relevant environment value byte-for-byte.

## OCI identity and local selection

The external base and builder references retain readable versions plus exact
OCI index digests. The two unpublished local platform results use version
`18.6.0-lab.1` and distinct tags of the form:

```text
localhost/terraform-provider-powerdns-lab-postgres<DEV_SUFFIX>:18.6.0-lab.1-amd64
localhost/terraform-provider-powerdns-lab-postgres<DEV_SUFFIX>:18.6.0-lab.1-arm64
```

`DEV_SUFFIX` comes only from `scripts/dev-suffix.sh`; equal worktree leaf names
therefore cannot collide. Compose consumes only the native amd64 tag on the
current amd64 lab host. The build captures the full current commit SHA once,
addresses that exact Git object when reading its RFC 3339 commit timestamp and
Unix epoch, and rechecks that `HEAD` still equals the captured SHA immediately
before each platform build. Those values drive `revision`, `created` and
`podman build --timestamp`, making the image configuration and layer time
deterministic for the same inputs. Before the final evidence run, the committed
candidate is rebuilt so these annotations name the exact candidate rather than
its base commit.

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
The generated SPDX SBOM is the authoritative, lossless licence record. The OCI
`documentation` annotation is the stable default-branch URL of the audit that
identifies both SBOMs and the complete scanner inventory after merge. It is
intentionally not an immutable evidence identifier: `revision` names the exact
executable source, while report hashes in the merged audit provide immutable
evidence identities.

Compose may use the worktree-local tag only because `lab.py` serializes the
same-worktree build/up transaction, captures each single-platform build result
atomically through a unique `--iidfile`, inspects by that immutable ID, verifies
the tag resolves to it immediately before Compose, and validates the running
container ID afterward. No external image reference is allowed to use a tag
without a digest.

## Integration boundary

The PostgreSQL service uses a required local image variable; it does not gain a
Compose `build` block. `scripts/automation/lab.py` is the single owner of the
build and Compose environment:

- derive the suffix through the existing canonical adapter;
- obtain revision, RFC 3339 commit time and Unix epoch with bounded argv-form
  Git calls;
- acquire the single non-blocking, ownership-validated global runtime lock
  `/tmp/tfp-powerdns-lab-runtime.lock` before preflight or either build and hold
  it through Compose up plus running-ID verification; every worktree contends
  on this same lock because containers, volumes, network and host ports are
  globally fixed;
- require the global `/tmp/tfp-powerdns-lab-runtime.receipt.json` absent before
  up and the exact fixed network and pod names absent independently of labels;
  in that same pre-build check and again under the global lock immediately
  before lab Compose up, require the e2e receipt, all six exact e2e names and
  all four e2e project-labelled object sets empty;
  also require `/tmp/tfp-powerdns-postgresql-image-scan.receipt.json`, exact
  scanner names `tfp-powerdns-postgresql-scan-config`,
  `tfp-powerdns-postgresql-scan-amd64` and
  `tfp-powerdns-postgresql-scan-arm64`, and every
  container carrying `io.ioplane.powerdns.image-scan.transaction` absent before
  lab up or down;
  invoke podman-compose with `--in-pod=false` and prove no pod is created; stale
  state is a fail-closed ownership conflict, never something a new run reuses;
- require the complete sets of containers, volumes, networks and pods carrying
  either Compose project label for `terraform-provider-powerdns-lab` empty at
  preflight, not merely the expected fixed names;
- run bounded argv-form single-platform `podman build` with `--format=oci`,
  `--pull=always`, `--timestamp`, a unique validated `--iidfile`, the exact
  build arguments and the worktree-specific tag;
- capture the returned image ID from that iidfile, inspect only by immutable
  ID, recheck tag-to-ID equality, and only then invoke Compose up with an
  explicit environment;
- before Compose mutation, generate a unique transaction ID, atomically persist
  a mode-0600 provisional global receipt containing it plus canonical worktree,
  suffix, image ID and empty-preflight identities, and pass the ID through the
  Compose environment; all five containers, three named volumes and the exact
  default network carry the exact
  `io.ioplane.powerdns.lab.transaction=<32-lower-hex>` and Compose-project
  ownership labels; pods are disabled and excluded from the receipt;
- finalize that receipt after successful up with every fixed container, named
  volume and network ID, label, mount and volume creation identity; the complete
  project-labelled sets must equal exactly those receipted objects, with no
  additional name;
- on Compose error, health/API timeout or interruption caught by the driver,
  enumerate the created subset while still holding the global lock, accept only
  expected names with the provisional transaction/project labels, atomically
  record their identities, revalidate them immediately, and perform exact
  guarded rollback; if any object is foreign or changed, preserve the receipt
  and fail closed for separately authorized recovery;
- after successful up or rollback, release the global lock and remove only the
  same ownership-validated iidfile paths; the lock path may remain as an empty
  validated synchronization inode, while no iidfile residue remains;
- status, verify and down acquire the same global lock and load the receipt;
  only its canonical owning worktree may operate the fixture;
- before every down, require every recorded object present and byte-identical
  and require complete project-labelled set equality with no object outside the
  transaction. Lab down additionally requires the e2e receipt absent, both e2e
  fixed containers and volumes plus the exact e2e pod/network names absent, and
  complete e2e project-labelled container/volume/network/pod sets empty before
  any Compose mutation. After
  teardown prove all lab objects absent before removing only the exact owned
  lab receipt;
- the e2e fixture has a separate global mode-0600 receipt at
  `/tmp/tfp-powerdns-e2e-runtime.receipt.json`, managed by the lab owner wrapper
  while the same global lock is held. Its exact project is
  `terraform-provider-powerdns-e2e`; containers are `pdns-e2e-s3` and
  `pdns-e2e-forgejo`; volumes are
  `terraform-provider-powerdns-e2e_s3-data` and
  `terraform-provider-powerdns-e2e_forgejo-data`; the always-absent network and
  pod names are `terraform-provider-powerdns-e2e_default` and
  `pod_terraform-provider-powerdns-e2e`;
- before e2e up, require those four fixed objects, exact pod/network names,
  complete e2e project label sets and receipt absent. Atomically write a
  provisional receipt with exact fields `schema_version`, `state`,
  `transaction_id`, `canonical_worktree`, `lab_transaction_id`, `project`,
  `containers`, `volumes`, `networks` and `pods`. `schema_version` is integer
  1, `state` is `provisional` or `ready`, the transaction is 32 lowercase hex,
  and the owning lab transaction must equal the active lab receipt. Pass the ID
  as `PDNS_E2E_TRANSACTION_ID`, invoke every e2e Compose command with
  `--in-pod=false`, and label both services and named volumes exactly
  `io.ioplane.powerdns.e2e.transaction=${PDNS_E2E_TRANSACTION_ID}`;
- after health, finalize `containers` with exact name, immutable ID, labels and
  mounts, and `volumes` with exact name, creation time, labels and mountpoint.
  Because both services retain exact host networking, require `networks` and
  `pods` to be empty objects and both project-labelled sets empty;
- e2e up failure enumerates only the transaction-labelled expected subset,
  records and revalidates it, then performs guarded rollback; any foreign,
  replaced or extra project-labelled object preserves the provisional receipt
  and performs no teardown. E2e test validates the finalized e2e receipt before
  and after the child. E2e down validates byte identity and complete project
  set equality before zone deletion or Compose, proves all e2e objects absent,
  and only then removes its receipt. The lab receipt remains intact throughout;
- execute every lab-mutating consumer (`fixtures:record`, `testacc`, `e2e:up`,
  `e2e` and `e2e:down`) through a receipt-backed owner wrapper that acquires
  the same global lock, validates canonical worktree plus the complete object
  receipt, holds the lock for the bounded child argv and revalidates in a
  `finally` path after success, non-zero exit, timeout or interruption; the
  required acceptance, coverage and e2e workflows use the same wrapper around
  their direct host-runner argv rather than bypassing the lease;
- use exact local outer consumer budgets: 900 seconds for `fixtures:record`,
  7800 for `testacc`, 1800 for `e2e:up`, 58500 for `e2e`, and 900 for
  `e2e:down`; the suite value covers all 59 currently collected cases at their
  900-second per-case limit plus 5400 seconds of suite overhead. A non-zero
  pytest collection oracle must equal 59, so future parametrization fails the
  contract instead of silently invalidating the constant. Required workflow
  budgets are 3900 seconds for acceptance, 1800 for coverage, 1200 for e2e
  fixture up, 3000 for the e2e suite and 900 for e2e fixture down; workflow job
  ceilings retain explicit setup and teardown overhead above those budgets;
- on timeout, `KeyboardInterrupt`, SIGINT or SIGTERM, terminate the complete
  child process group while still holding the global lock, wait a bounded grace
  period, kill any survivor and reap the child before ownership revalidation.
  After clean child shutdown and successful revalidation, preserve the
  original child status or exception; ownership drift is the primary safety
  error and retains the original failure as context;
- e2e zone cleanup first reads each managed name. An existing zone is deleted
  only when DELETE returns exact HTTP 204 with an empty body; an already-absent
  zone is accepted only through its product-specific read response. Transport,
  malformed JSON and unexpected HTTP status/body errors are never suppressed.
  Before any e2e Compose teardown, re-read every managed name: eleven
  Authoritative/LMDB names must be HTTP 404 and the Recursor name must be HTTP
  422 whose parsed JSON value equals exactly
  `{"error":"Could not find domain 'internal.e2e.example'"}`. Any residue or
  observation failure stops teardown;
- required CI teardown remains `if: always()` but never uses `|| true`,
  `continue-on-error` or ignored status. The e2e workflow performs leased e2e
  down before lab down in one fail-fast ordered step, propagates both commands'
  built-in residue oracles, and never attempts broader lab teardown after an
  ownership, zone or e2e teardown refusal;
- workflow timeouts cover every sequential outer bound, not only the test
  child. Acceptance uses at least 120 minutes, coverage at least 85 minutes and
  e2e at least 140 minutes: 32 minutes for two platform builds plus Compose/API
  start, three minutes for the separate lab verification, the declared consumer
  bounds, 10 minutes for lab down and a final 10-minute runner reserve. The
  committed ceilings are 125, 90 and 150 minutes respectively and a structural
  arithmetic oracle rejects any smaller value;
- fail closed when the lock, Git identity, timestamp, iidfile, image ID, OCI
  labels or complete lifecycle receipt is absent or changed.

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
- missing or wrongly scoped global platform arguments, a persistent module
  cache, missing/invalid iidfile capture, global lock contention, same-time up
  from two worktrees, a foreign exact-name network, missing transaction label,
  partial-start rollback failure, unexpected pod creation, or replacement of
  any receipted container, volume or network before teardown;
- missing/corrupt/symlink e2e receipt; dirty e2e fixed-name or project-labelled
  preflight; omitted `--in-pod=false`; missing/wrong e2e transaction labels;
  partial-up rollback without exact identity; a replaced/extra e2e container,
  volume, network or pod; or broad e2e down before complete receipt/set equality;
- lab down with an active/corrupt/symlink e2e receipt or any fixed/project-
  labelled e2e container, volume, network or pod; every mismatch must make zero
  lab Compose calls;
- lab up with the same e2e receipt or fixed/project-labelled residue before the
  first build or at the repeated pre-Compose check; every mismatch must make
  zero build, retag or Compose calls;
- lab up/down or image scan with a scan receipt, exact scanner name or any
  scan-transaction-labelled container; scanner `run --rm`; missing CID capture;
  wrong name/label; ownership replacement before start/stop/remove; timeout or
  interruption that leaves a client, conmon/container, partial artifact,
  iidfile or receipt; and cleanup that touches an unrelated container;
- an extra differently named project-labelled container, volume, network or
  pod outside the receipt/required-empty set during pre-up, partial rollback,
  ready consumer validation or pre-down; a Task or required-workflow consumer
  command without the owner wrapper; workflow/Task argv or budget drift; a
  foreign-worktree consumer; releasing the runtime lock before its child exits;
  a missing/short/wrong consumer budget; a collected-case count mismatch; a
  surviving child process or grandchild after timeout/interruption; or skipping
  child-group reap or post-child revalidation on non-zero, timeout, SIGINT,
  SIGTERM or `KeyboardInterrupt`;
- suppression of an e2e zone-delete transport/unexpected HTTP error; accepting
  a successful DELETE other than HTTP 204 plus empty body; accepting malformed,
  extra-field or wrong product-specific absence JSON; skipping the exact
  post-delete zone absence oracle; mutating a zone before lease validation;
  fail-open CI teardown; ignored teardown status; or continuing into lab down
  after an e2e teardown refusal;
- drift in entrypoint, command, PostgreSQL environment, port, stop signal,
  declared volume, schema bind, tmpfs or health check.

After static GREEN, build separate worktree-local linux/amd64 and
linux/arm64/v8 tags and capture each image ID through the unique iidfile of its
single-platform command. The Containerfile declares `BUILDPLATFORM` and
`TARGETPLATFORM` globally, uses `FROM --platform=$BUILDPLATFORM` for the Go
compiler, and declares Buildah's `TARGETOS`/`TARGETARCH` arguments inside that
stage for cross-compilation. Target stages contain no `RUN`, so the current
amd64 host does not require QEMU or binfmt handlers. Do not use a multi-platform
`--manifest` build: an ordinary tag is ignored in that mode and `--iidfile` is
forbidden. A separate iidfile is valid and required for each one-platform build.
Compose selects only the native amd64 tag; the arm64 tag is a build-and-scan
portability artifact and is never executed on this host.

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
the embedded Go and module metadata. For both extracted binaries, the parsed
tab-delimited `go version -m` build settings must report `GOOS=linux` and the
exact corresponding `GOARCH=amd64` or `GOARCH=arm64`; a swapped or mismatched
binary fails closed. The arm64 contract is verified without execution through
those direct binary settings, OCI architecture, the build arguments, the
scanner package inventory and the final-layer filesystem hash.

Those filesystem and binary checks are one owned auxiliary evidence
transaction, not ad hoc Podman commands. `scripts.automation.lab
image-evidence` holds the global runtime lock, validates the ready lab receipt
and canonical worktree, and rejects active e2e or image-scan state. Inside that
lock it opens the evidence directory once with `O_DIRECTORY|O_NOFOLLOW` and
holds the descriptor for the transaction. `fstat` must prove the same canonical
immediate `/tmp` child, current UID, mode `0700` and directory device/inode.
Every leaf is opened relative to that descriptor with
`O_NOFOLLOW|O_CREAT|O_EXCL`; outputs are immutable versioned files rather than
path replacements. Before and after each output, the pathname-to-descriptor
device/inode check and exact phase allowlist/hash manifest must still match.
Swapping the pathname therefore cannot redirect the held directory descriptor.

The two registry child manifest digests are not local image IDs. Before source
traversal, the command journals the two fully qualified child references plus
their expected manifest and config digests, then boundedly pulls or resolves
each exact reference for its declared platform. Immutable inspection must prove
the local `RepoDigest` equals that exact child reference, OS/architecture match,
and the captured local image ID equals the child manifest's verified config
digest above. It records the manifest→config→local-ID mapping before use and
mounts only the captured config/local ID. A bare manifest digest, mutable tag,
missing or garbage-collected local image, wrong platform/config, or tag-derived
substitution fails closed; absence may trigger only a bounded pull of the exact
qualified child reference, followed by the complete mapping checks.

Before its first mount or container operation the command requires the exact
source/final image IDs unmounted, the verifier name
`tfp-powerdns-postgresql-evidence-go-version` absent, and every container with
label `io.ioplane.powerdns.image-evidence.transaction` absent. Coordination
uses the persistent append-only journal
`/tmp/tfp-powerdns-postgresql-image-evidence.receipt.jsonl`, analogous to the
never-unlinked global lock inode. Open it once with `O_NOFOLLOW`; create it only
with `O_CREAT|O_EXCL` mode `0600`, then hold that FD for the transaction.
`fstat` and pathname-to-FD checks require a regular single-link file, current
UID, mode `0600` and unchanged device/inode. Validate the complete canonical
JSON-lines hash chain and require every prior transaction terminal before
appending a schema-1 start record containing transaction, worktree, evidence
directory FD identity, Podman mode, exact source refs/manifest/config
expectations, final image IDs and empty source-local/mount/container maps.
Phase and identity changes append immutable sequence/previous-digest records to
the held FD and `fsync`; there is no receipt pathname replace or unlink. Every
mutation is bracketed by journal and pathname-to-FD validation. A swap cannot
redirect a write or delete its replacement: it fails closed and retains the
held journal and outputs. Complete success appends and flushes a terminal
`complete` record. The shared guard state machine accepts either an absent
journal as the virgin/pre-evidence state or a present, fully valid chain whose
latest transaction is terminal `complete`. Every other present state—active,
failed, truncated, replaced or invalid—fails closed. The persistent journal is
coordination evidence, not disposable runtime residue.

Only one exact image is mounted at a time. Before mutation, bounded `podman
info --format json` must yield a Boolean rootless value and a non-empty storage
driver; record both in the journal and revalidate them before cleanup. For a
validated rootless client, the complete mount, traversal, extraction and
unmount sequence runs in one process-group-aware `podman unshare` child. For a
validated rootful client, the identical helper runs as one direct bounded child
because rootful Podman rejects `podman unshare`. No failed-mode fallback is
permitted: missing or changed mode/driver, unshare in rootful mode, or direct
mount in rootless mode fails without trying the other branch. Neither branch
may expose a mount path to a later host-namespace command. The journal records
the intended image ID before the child and its returned mount identity after
the child. On timeout or interruption cleanup uses the same selected bounded
mode and queries only that exact image ID while retaining the lock. It unmounts
only a path absent at preflight and equal to the recorded or discovered path;
ambiguous, replaced or mismatched identity fails closed.

The Go metadata verifier is created, never invoked through `run --rm`, with the
exact transaction label and a read-only evidence-directory bind. Its CID is
captured in the journal and name/CID/label/bind are revalidated before attach-
start and before bounded stop, kill or remove. Normal completion uses the same
identity-guarded cleanup.

The command has one monotonic 1800-second outer deadline, a 1500-second work
cutoff and a 300-second cleanup reserve. Every info, mount, inspect, unmount,
create, attach, stop, kill and remove receives only positive time remaining
under that same outer deadline. Timeout, SIGINT, SIGTERM and
`KeyboardInterrupt` terminate, kill and reap the complete client process group
before cleanup. Safe abnormal cleanup proves no owned mount, verifier or
`conmon` survives but preserves journal and evidence for separately authorized
recovery. Only complete success may append the terminal journal record after
proving all four images unmounted, verifier absent and final hashes recorded.
Behavioural tests cover both rootless and rootful branches with no fallback;
timeout/interruption in every mount/verifier phase; replacement or label/bind/
path drift; active/invalid journal and stale container/mount preflight;
exhausted cleanup reserve; directory/journal swaps specifically between
validation and create/append/terminal handling; symlink, owner/mode,
extra-content; and unrelated-object preservation. Lab up/down, every owned lab
consumer and `image-scan` apply the same two-state guard: absent virgin journal
or present valid latest terminal `complete`; they reject any other present
journal, verifier or recorded mount residue. Tests prove absent-journal success
for each command. Task 5 post-evidence teardown is stricter and requires a
present valid latest `complete` transaction. A partial evidence run therefore
cannot be hidden by fixture teardown, another mutating test or a later scan.

Current pinned Trivy 0.74.0 scans the Containerfile with `trivy config`. It
scans both final platform images with vulnerability, secret and licence scanners, and also
enables `--image-config-scanners misconfig,secret` so the generated OCI image
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

The same image-security policy is permanent rather than one-off evidence.
`scripts.automation.lab image-scan` reuses the exact locked two-platform build
and immutable-ID export path. Before either build it holds the global lock and
requires both receipts, all fixed lab/e2e names and all project-labelled object
sets empty. It also accepts only an absent virgin evidence journal or a present
valid chain with latest transaction `complete`, and requires every evidence
verifier/mount residue absent. After
validating the requested output path is a fresh canonical
directory target and before creating that directory, iidfile, image or archive,
it atomically writes a mode-0600 scan receipt. The receipt contains schema 1,
transaction, canonical worktree/output, current phase, intended iidfile paths
and initially empty image/archive/container maps; every phase transition and
new immutable identity is atomically recorded before and after the corresponding
mutation. It scans both OCI archives with the pinned scanner and fails on an
empty input or any unapproved result. The required Security
workflow invokes that exact command on pull requests, default-branch pushes and
the scheduled run in addition to its repository filesystem scan. Static tests
bind the workflow argv, scanner tag and digest, both archive inputs, non-zero
package/test validation and failure propagation; mutations that remove an
archive, downgrade or float Trivy, scan only the source tree, suppress a result
or accept zero inputs must fail. The command has a 5400-second outer budget;
the Security job commits a 105-minute ceiling, leaving 15 minutes for checkout,
setup and report handling. A monotonic 5400-second whole-command deadline and a
4800-second work cutoff are captured before the first preflight; the final 600
seconds are reserved exclusively for ownership inspection, process-group reap
and scanner stop/kill/remove. Positive remaining work time is passed to every
build, export and scanner child through the existing process-group-aware bounded
runner; no work child starts after the work cutoff. The empty preflight already
requires the global scan receipt, the three exact scanner names and every
container carrying the scan transaction label absent. Create, never `run --rm`,
each exact scanner container with label
`io.ioplane.powerdns.image-scan.transaction=<transaction>`, capture its CID,
record it in the receipt and revalidate name/CID/label before start. Expiry
before a child prevents it from starting; expiry during attach terminates,
force-kills and reaps the client group. Every subsequent identity inspect,
stop, kill and remove receives only the positive remaining outer-deadline time;
cleanup may not reset or extend the deadline. It removes
only that exact scanner container before output cleanup. Normal exit uses the
same identity-guarded removal. If scanner ownership cannot be proved, preserve
the receipt and partial output and fail closed. Every non-zero, timeout or
interrupted build/export/scan preserves the receipt, output and recorded
identities even when owned scanner cleanup succeeds; lab up/down remain blocked
until separately authorized exact recovery. Only full scan success may remove
owned iidfiles and then the receipt, after proving all scanners absent and every
final report/hash recorded. If the 600-second cleanup reserve is exhausted,
preserve the receipt/evidence and fail closed without claiming a 5400-second
success.
Behavioural timeout and interruption tests prove no surviving child or owned
scanner after safe cleanup, prove the retained transaction receipt precedes the
first build/export mutation and blocks lab up/down, and prove unrelated
containers unchanged. Worst-case clock tests consume the complete 4800-second work budget
and every bounded cleanup phase without exceeding the 5400-second outer budget.
Lab up/down also reject a scan receipt or scanner residue. The workflow
header explicitly distinguishes this blocking image gate from alert-only source
reports, and a structural oracle rejects a smaller job or command budget.
The job reports the exact context `PostgreSQL lab image (Trivy)`. After that
context is green on the reviewed PR, stop for explicit user approval of the
material branch-protection mutation. Present the exact rule ID, strict flag,
the eleven current `{context, app.id}` pairs, the new check's observed app ID,
the proposed twelve-pair input and rollback. Only after approval and before
merge may `gh api graphql` append the new pair through one
`updateBranchProtectionRule(requiredStatusChecks: ...)` mutation while
preserving all existing app bindings. GraphQL must then report strict mode and
the exact twelve pairs. The pre-change rule ID and pairs form the rollback
receipt; if the PR is abandoned or cannot merge, restore those exact prior
eleven pairs and verify. Both Beads stay IN_PROGRESS through this external
policy transition.

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
