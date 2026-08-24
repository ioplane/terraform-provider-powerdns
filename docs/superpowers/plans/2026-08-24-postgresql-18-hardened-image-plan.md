# PostgreSQL 18.6 hardened lab image implementation plan

> **For agentic workers:** REQUIRED: Use
> `superpowers:subagent-driven-development` (if subagents are available) or
> `superpowers:executing-plans` to implement this plan. Steps use checkbox
> (`- [ ]`) syntax for tracking.

**Goal:** Replace the blocked Debian PostgreSQL 18.6 lab image with a
repository-built, unpublished, worktree-local OCI image based on the exact
official Alpine 3.24 image and an exact Go 1.27.0 rebuild of `gosu` 1.19.

**Architecture:** Build one flattened OCI image per target platform from
digest-pinned PostgreSQL and Go inputs. The existing host-side lab driver owns
the bounded builds, local tag derivation, Compose environment and runtime image
identity check; Compose cannot fall back to a remote or shared tag. Preserve
the official PostgreSQL runtime filesystem/configuration and prove the result
with TDD, per-platform metadata/content comparison, SBOMs, pinned Trivy scans,
both Authoritative compatibility branches and complete e2e residue gates.

**Tech Stack:** PostgreSQL 18.6, Alpine 3.24, `gosu` 1.19, Go 1.27.0,
Containerfile 5, OCI Image Specification 1.1, Buildah/Podman, Compose,
podman-py 5.8.0, Trivy 0.74.0, CycloneDX, SPDX, pytest, uv, ruff, ty, hadolint,
Beads, gopls, GitHub GraphQL.

---

## Boundary, sources and immutable inputs

This plan implements blocker `tfp-bqt.6.5` inside the still-unpublished
PostgreSQL migration `tfp-bqt.6.1`. It changes only the disposable lab/e2e
PostgreSQL image. It does not move the provider, release artifacts or Go dev
container to Alpine; publish an image; migrate persistent data; update another
Compose image; rewrite the PowerDNS fixture; or perform the P10-12 Python-to-Go
automation migration.

The binding design is:

`docs/superpowers/specs/2026-08-24-postgresql-18-hardened-image-design.md`

The user explicitly approved retaining Alpine on 2026-08-24. The first design
passed independent SPEC review; plan review then proved that main-module
`go build` cannot embed the required version. The revised design uses
version-aware cross-`go install` and explicitly normalizes one build-only Env
value. Its current SHA-256 is
`b2f2e9f7ffae248a4a2348645e7d46b45eec3eb78eaa10ff9fb966536abed492`.
Independent plan review approved the complete revised mechanism, then final
SPEC review required a direct binary-architecture oracle and a consistent
fail-closed status. The following QUALITY pass required global platform
arguments, a fresh module cache, atomic iidfile capture with locking, a complete
teardown receipt, CI hadolint parity and merge-time OID reconciliation. All
corrections are included. The next QUALITY pass required one global lock for
the fixed runtime namespace, exact-name network preflight and a provisional
transaction receipt with guarded partial-start rollback. Those corrections are
included. SPEC then required disabling podman-compose pod mode and treating the
fixed pod name as an always-absent collision. That correction is also included
in the fresh exact review. The next QUALITY pass required full project-labelled
set equality before broad Compose teardown and receipt-backed global-lock
leases for every direct lab consumer. Those corrections are also included in
the fresh exact review. SPEC then corrected the set-equality order around down
and required extra-object mutations for all four project-labelled classes.
Those corrections are also included in the fresh exact SPEC then QUALITY
review. QUALITY then required exact per-consumer timeout budgets and ownership
revalidation on every child exit path. SPEC then required the actual 59-case
e2e collection budget, process-group termination and reap before interrupt-path
revalidation, and receipt-backed parity for every required CI consumer. The
following SPEC pass added the omitted zone-mutating `e2e:down` lease, strict
post-delete zone oracle and failure-propagating CI teardown. Those corrections
were included before the next exact SPEC approval. QUALITY then required the
current Trivy pin plus a permanent two-platform CI image scan, full workflow
lifecycle timeout arithmetic, and the reciprocal e2e absence guard before lab
up as well as down. Those corrections are included for a fresh exact SPEC then
QUALITY review before implementation. That exact QUALITY review then found
that Task 5 mounts and the Go metadata verifier were still outside a receipt,
global-lock and bounded-cleanup transaction. The current design and plan add
the owned `image-evidence` command, reciprocal lab guards, exact mount/CID
identity, a deadline reserve, mutation tests and post-flight absence proof.
The subsequent SPEC reviews required an explicit Task 4 GREEN implementation,
mode-selected rootless-unshare/rootful-direct traversal, and held-FD evidence
directory plus append-only journal identity. The final bootstrap correction
defines absent journal as the virgin safe state and present/latest `complete`
as the only other safe state; Task 5 post-evidence still requires the latter.
The subsequent QUALITY review required the source-child OCI manifest, config
digest and local Podman image ID to remain distinct and verified. The current
contract adds fully qualified child refs, exact config digests, bounded
acquisition, RepoDigest/platform mapping and mounts only captured config IDs.
The following QUALITY pass also required the new run/e2e failure-path tests to
have their own pre-production semantic RED and exact focused GREEN counts rather
than relying on the later aggregate Python gate. Its follow-up required the
same treatment for Taskfile/workflow and protection-context tests. All five
affected modules now have independent exact RED and GREEN counts before the
candidate. Those corrections are included.
Fresh sequential SPEC then QUALITY review is required before implementation.

Use these exact external inputs; re-resolve them before the production edit
and record raw responses in `AUDIT-03` and Beads:

- PostgreSQL index:
  `docker.io/library/postgres:18.6-alpine3.24@sha256:d3e1620b530c944afa6e887d22eb899824da68e19c52024bf98f5220c88a65b2`;
- PostgreSQL amd64 child:
  `sha256:63bdc97d67b5133bf0e5ebd500bec6d046fa851dc81340d838f0347e616107e8`;
- PostgreSQL amd64 child ref:
  `docker.io/library/postgres@sha256:63bdc97d67b5133bf0e5ebd500bec6d046fa851dc81340d838f0347e616107e8`;
- PostgreSQL amd64 config/local ID:
  `sha256:b07129cc272f688c98f5b343138a0a52fa45b3d82f50d7a53ff441330624cd2e`;
- PostgreSQL arm64/v8 child:
  `sha256:cbe15165195f7f2d63885b4d990fdec7b602248533cb05bd992284a45a58fed3`;
- PostgreSQL arm64/v8 child ref:
  `docker.io/library/postgres@sha256:cbe15165195f7f2d63885b4d990fdec7b602248533cb05bd992284a45a58fed3`;
- PostgreSQL arm64/v8 config/local ID:
  `sha256:6def1cb8d5ffa3443c527419cc13f395ab328c27bf90fcb1e80831aae4103bc3`;
- Go index:
  `docker.io/library/golang:1.27.0-trixie@sha256:6212da3924947f4b6a939df02ea627c13f338f1a41d6c3fcb0dd9d076eef46c4`;
- Go amd64 child:
  `sha256:31df75a0f51705bb15c74cacaeadb6596ae270cea9ec05138928de7e2d1f65e8`;
- Go arm64/v8 child:
  `sha256:6237d172b66951ce3ebfe0156587ad41a9773772c061fb9a5e3d612f5fd22614`;
- `gosu` release commit:
  `6456aaa0f3c854d199d0f037f068eb97515b7513`;
- `gosu` module:
  `github.com/tianon/gosu@v0.0.0-20250923190938-6456aaa0f3c8`;
- module sum: `h1:HIpXk5mGBQGfOqcaBbRT4Vnss8NPICnMGlD5xTlPBdQ=`;
- module-file sum: `h1:SwhRwWsO6iqXZN9CpIaU9CnOrUqpWDINW16KaaSqnrU=`;
- Trivy:
  `docker.io/aquasec/trivy:0.74.0@sha256:62b1e65e8869bc4b4c6aa4fa2b21595256c7c2f6018a9d9ad61caf87187c1969`;
- Trivy amd64 child:
  `sha256:ee940acbf1f58ebadb42d01434ce4609530bf1b52536afbd1eee66cd7123c5c9`;
- Trivy arm64 child:
  `sha256:55ad20f8a239a3e95427e60b8aaea38788550c18a3f1772976bebf732e6ae166`.

Consult current Context7 `/docker-library/postgres`,
`/podman-container-tools/buildah`, `/aquasecurity/trivy`, OCI 1.1 annotations,
the Go 1.27 specification/release notes and the exact `gosu` source before
writing commands. Exact identifiers come from live registry, sum.golang.org
and exhaustive `gh api graphql` evidence, never memory.

## File map

- Create `deployments/containers/Containerfile.postgres-lab`: pinned
  cross-platform `gosu` build, target rootfs replacement and one-layer scratch
  final image with the exact official runtime config.
- Create `test/scripts/test_postgres_lab_containerfile.py`: static
  Containerfile and mutation contracts.
- Create `test/scripts/test_postgres_lab_lifecycle.py`: bounded build,
  tag/identity, Compose-environment and lifecycle mutation contracts.
- Modify `scripts/automation/lab.py`: canonical local tags, Git metadata,
  bounded per-platform iidfile builds, global runtime lock, immutable image
  inspection, provisional/final transaction receipt, guarded rollback and safe
  down, plus reusable owned `image-evidence` and exact two-platform `image-scan`
  evidence commands.
- Modify `scripts/automation/run.py`: bounded process-group shutdown and reap
  on timeout, SIGINT, SIGTERM and `KeyboardInterrupt`.
- Modify `scripts/automation/e2e.py`: fail-closed managed-zone deletion and
  exact product-specific absence proof before e2e Compose teardown.
- Modify `deployments/compose/compose.lab.yml`: require the native local image
  and transaction variables plus exact ownership labels while retaining
  credentials, schema bind, tmpfs and health check.
- Modify `deployments/compose/compose.e2e.yml`: add only the owner-wrapper
  transaction labels needed for exact e2e receipt and partial rollback.
- Modify `test/scripts/test_lab.py`: replace the remote-image oracle with the
  required local variable and retain all storage/runtime observation tests.
- Modify `test/scripts/test_run.py`: require process-group termination, bounded
  reap and original-exception preservation for every interruption path.
- Create `test/scripts/test_e2e.py`: require exact deletion/absence responses,
  error propagation and zero teardown on transport, HTTP or residue failure.
- Modify `Taskfile.yml`: hadolint both Containerfiles; do not add another lab
  lifecycle owner.
- Modify `test/scripts/test_taskfile.py`: require both exact hadolint inputs.
- Modify `.github/workflows/ci.yml`: lint both Containerfiles in the required
  direct hadolint context and keep it in parity with the Task target.
- Modify `.github/workflows/acceptance.yml`, `.github/workflows/coverage.yml`
  and `.github/workflows/e2e.yml`: route every lab-mutating child through the
  same receipt-backed owner wrapper with exact argv and CI-specific budget, and
  set complete-lifecycle job ceilings from the checked arithmetic.
- Modify `.github/workflows/security.yml`: retain the filesystem report, update
  Trivy to the exact current pin, and invoke the blocking two-platform
  `image-scan` command on the same pull-request/push/schedule triggers.
- Modify `.cspell.json`: add only new technical terms used by implementation
  evidence.
- Modify `AGENTS.md`, `README.md`, `docs/development.md`, `CHANGELOG.md`,
  `docs/plan.md`, `docs/audit/AUDIT-03-postgresql-18-lab.md`, the original
  PostgreSQL plan and this plan: state the active test-only image contract and
  retain exact evidence/status.
- Update Beads `tfp-bqt.6.5` and `tfp-bqt.6.1`; neither closes before the
  reviewed pull request is merged.

## Task 0: Re-establish the approved boundary and baseline

**Files:**

- Read: `AGENTS.md`
- Read: `README.md`
- Read: the binding design and both PostgreSQL plans
- Modify: this plan
- Update: Beads `tfp-bqt.6.5`, `tfp-bqt.6.1`

- [ ] **Step 1: Verify Git and durable state without mutation**

  Run `git status --short`, `git log --oneline --decorate -5`,
  `git merge-base origin/main HEAD`, `git stash list` and `bd show` for both
  Beads. Require the retained recovery object
  `2489ed02fe781f387e957c1c2131ff3ef643e283` to exist; never drop it. Record
  every current tracked/untracked WIP path. Do not discard or restage existing
  PostgreSQL work accidentally.

- [ ] **Step 2: Reconcile the default branch through GraphQL**

  Fetch `origin/main`, then use `gh api graphql` to read repository identity,
  `defaultBranchRef.target.oid`, strict status-check policy and all required
  `{context, app.id}` pairs. Require the fetched OID and GraphQL OID to match.
  If main advanced,
  preserve exact WIP paths to a new immutable recovery object, rebase, re-read
  complete `AGENTS.md`/`README.md`, restore with `--index`, run documentation
  gates and restart plan review. Never update a reviewed candidate implicitly.

- [ ] **Step 3: Revalidate external identifiers**

  Hash raw PostgreSQL and Go tag manifests, select both platform descriptors,
  require the exact values above, then read both PostgreSQL child manifests and
  require their OCI media type plus exact config digests/local IDs above. Hash
  the raw Trivy tag manifest, require
  its exact OCI index and both platform descriptors above, and use GraphQL to
  prove v0.74.0 is the newest stable non-draft/non-prerelease release.
  Exhaustively paginate the `gosu` releases
  and resolve release `1.19` to the exact commit. Inside the Go 1.27 dev
  container run `go list -m -json` for the exact pseudo-version and require its
  `Version`, `Origin.Hash`, `Sum` and `GoModSum`. Record that `v1.19.0` fails to
  resolve; it is not a valid substitute.

- [ ] **Step 4: Run the focused baseline**

  Require the current dev container to match the canonical suffix and bind.
  Run `task py`, `task lint:pins`, both existing Compose renders with the
  current remote PostgreSQL value, `task docs:lint`, gopls checks over the
  non-empty Go file set, `bd lint`, `bd dep cycles` and both diff checks. Append
  exact versions/counts to both Beads and this plan.

## Task 1: Specify the hardened Containerfile before creating it

**Files:**

- Create: `test/scripts/test_postgres_lab_containerfile.py`
- Create later: `deployments/containers/Containerfile.postgres-lab`

- [ ] **Step 1: Write the exact failing static contract**

  Add constants for every immutable input and helpers that split a
  Containerfile into stages without discarding instruction order. Tests must
  require:

  - both external `ARG` values equal the full qualified tag@digest strings;
  - global `BUILDPLATFORM` and `TARGETPLATFORM` arguments appear before every
    `FROM`, the builder is `FROM --platform=${BUILDPLATFORM}`, and it declares
    `TARGETOS`/`TARGETARCH` inside the stage;
  - the exact pseudo-version and both module sums are present;
  - `CGO_ENABLED=0`, `GOTOOLCHAIN=local`, `GOOS`, `GOARCH`, `-trimpath`,
    `-ldflags=-d -w`, and version-aware `go install` are present;
  - the native/cross `$GOPATH/bin` result is normalized to `/out/gosu`;
  - neither main-module `go build -C` nor `-s` is present;
  - no `RUN` exists in the target PostgreSQL or scratch stages;
  - the replacement copy is exactly root-owned executable `0755`;
  - the final stage is `FROM scratch` and copies `/` from the modified target;
  - exact official `PATH`, `GOSU_VERSION`, `LANG`, `PG_MAJOR`, `PG_VERSION`,
    `PG_SHA256`, `PGDATA`, working directory, volume, port, entrypoint, command
    and stop signal are restated;
  - `DOCKER_PG_LLVM_DEPS` parses to exactly `llvm21-dev` and `clang21`, with
    the design-approved source tabs normalized to one ASCII space;
  - required truthful OCI labels and the two namespaced source-base labels are
    present, while standard `base.name`, `base.digest` and `licenses` are
    absent.

  The test initially fails because the new Containerfile is absent. An absent
  file is the required RED, not a collection error.

- [ ] **Step 2: Add bounded mutation proofs**

  In memory, reject: floating/wrong base or builder; `v1.19.0`; wrong module
  sum; missing or stage-local-only `BUILDPLATFORM`/`TARGETPLATFORM`; a persistent
  module-cache mount or seeded modified module tree; main-module `go build`;
  `CGO_ENABLED=1`; `-s`; target-stage `RUN`; direct final stage from PostgreSQL;
  omission of scratch flattening; shared/non-root gosu; missing runtime config;
  a standard base/licence annotation; omitted dynamic label; and an extra final
  layer-producing instruction.

- [ ] **Step 3: Run RED and record it**

  First run `pytest --collect-only -q` for only the new static module through
  the canonical dev container, record the positive integer as `STATIC_CASES`,
  then run the same module:

  ```sh
  DEV_SUFFIX=-postgresql-18-17d2b9851be8 \
    podman-compose \
      -p terraform-provider-powerdns-dev-postgresql-18-17d2b9851be8 \
      -f deployments/compose/compose.dev.yml \
      exec -T dev uv run --locked pytest --collect-only -q \
      test/scripts/test_postgres_lab_containerfile.py
  DEV_SUFFIX=-postgresql-18-17d2b9851be8 \
    podman-compose \
      -p terraform-provider-powerdns-dev-postgresql-18-17d2b9851be8 \
      -f deployments/compose/compose.dev.yml \
      exec -T dev uv run --locked pytest \
      test/scripts/test_postgres_lab_containerfile.py -q
  ```

  Expected: tests collect and fail only because
  `Containerfile.postgres-lab` does not exist. Append command, count and
  failure to both Beads and `AUDIT-03` before production edits.

## Task 2: Specify bounded build and Compose identity behavior

**Files:**

- Create: `test/scripts/test_postgres_lab_lifecycle.py`
- Modify later: `scripts/automation/lab.py`
- Modify later: `deployments/compose/compose.lab.yml`
- Modify later: `Taskfile.yml`
- Modify later: `test/scripts/test_taskfile.py`
- Modify later: `scripts/automation/run.py`
- Modify later: `test/scripts/test_run.py`
- Modify later: `scripts/automation/e2e.py`
- Create: `test/scripts/test_e2e.py`
- Modify later: `.github/workflows/acceptance.yml`
- Modify later: `.github/workflows/coverage.yml`
- Modify later: `.github/workflows/e2e.yml`
- Modify later: `.github/workflows/security.yml`
- Modify later: `test/scripts/test_protection.py`

- [ ] **Step 1: Write failing local-tag tests**

  Require exact tags derived only from `scripts/dev-suffix.sh`:

  ```text
  localhost/terraform-provider-powerdns-lab-postgres<suffix>:18.6.0-lab.1-amd64
  localhost/terraform-provider-powerdns-lab-postgres<suffix>:18.6.0-lab.1-arm64
  ```

  Test main checkout's empty suffix, linked-worktree suffix, unsafe helper
  output rejection and distinct tags for equal-basename worktrees. Reuse the
  canonical helper; do not reimplement its hashing.

- [ ] **Step 2: Write failing build-command tests**

  Monkeypatch `lab.run` and require exactly two sequential argv-form builds:

  ```text
  podman build --format=oci --pull=always --platform <platform>
    --timestamp <commit-epoch> --tag <platform-tag> --iidfile <unique-path>
    --build-arg CREATED=<commit-rfc3339>
    --build-arg REVISION=<full-commit>
    --file <absolute-Containerfile> <absolute-container-directory>
  ```

  Both calls use timeout `PULL`, the repository root as `cwd`, no shell,
  distinct platform/tag pairs, a distinct ownership-validated temporary
  `--iidfile`, and no `--manifest` or shared tag. Hold the exact non-blocking
  global `/tmp/tfp-powerdns-lab-runtime.lock` across final empty preflight, both
  builds, tag checks, Compose up and running-ID verification; all worktrees use
  this one lock because the runtime namespace and ports are fixed. Validate its
  owner, mode, inode and non-symlink identity before acquisition. Read and validate the
  64-hex ID from each iidfile, inspect by immutable ID, then require its mutable
  tag to resolve to that ID. Tests reject
  lock contention from the same and a different worktree,
  symlink/wrong-owner lock or iid paths, missing/empty/malformed
  iidfiles, retagging between build and inspection, and mutation from iidfile ID
  to tag-derived identity, with zero Compose calls. Also reject wrong platform,
  duplicate ID/tag, missing labels, absent/malformed Git metadata and any
  non-zero subprocess. Add a pure parser
  contract for the tab-delimited `go version -m` fields used later: require
  `build GOOS=linux`, exact expected `build GOARCH=amd64|arm64`, Go 1.27.0 and
  the exact module version/sum. Wrong, missing and swapped architecture fields
  must fail before any runtime claim.

- [ ] **Step 3: Write failing Compose-environment tests**

  Require the PostgreSQL service image to be exactly:

  ```yaml
  image: ${PDNS_LAB_POSTGRES_IMAGE:?run task lab:up}
  ```

  Before up, generate a unique transaction ID and atomically persist it with
  canonical worktree/suffix/native image/preflight data in mode-0600 global
  `/tmp/tfp-powerdns-lab-runtime.receipt.json`. `compose()` copies the current
  environment, sets `PDNS_LAB_POSTGRES_IMAGE` to the canonical native amd64 tag
  and `PDNS_LAB_TRANSACTION_ID` to that receipt value, and passes the map to the
  bounded runner with exact global option `--in-pod=false` before the Compose
  subcommand. Status/verify/down reload both values from the same receipt and
  use the same no-pod option. Mutations that omit it or select `true` fail.
  Require the transaction grammar to be exactly 32 lowercase hex and label
  every service, all three named volumes and the explicit default network with
  `io.ioplane.powerdns.lab.transaction=${PDNS_LAB_TRANSACTION_ID}` plus their
  exact Compose-project labels in the rendered model. Test missing
  helper/transaction output, an external image value,
  shared tag, absent environment, mutation to arm64, missing label on every
  object class and wrong transaction as fail-closed.

- [ ] **Step 4: Write failing running-image identity tests**

  Before any build or Compose call, require every fixed `pdns-lab-*` container,
  the three exact project volumes
  `terraform-provider-powerdns-lab_{lmdb-data,rec-api,dnsdist-api}`, exact pod
  name `pod_terraform-provider-powerdns-lab`, and exact network name
  `terraform-provider-powerdns-lab_default` absent regardless of labels.
  Enumerate the complete container, volume, network and pod sets carrying
  either Compose project label `terraform-provider-powerdns-lab` and require
  all four empty, so a differently named stale project object cannot be reused
  or later removed by broad Compose teardown.
  Any present object stops without
  building, retagging or invoking Compose; separate container/volume/pod/
  network tests, including foreign unlabelled exact-name pod/network objects,
  assert call order and zero mutating calls. Require the global receipt absent before up; a
  stale receipt also stops with zero build or Compose calls and requires
  separately authorized recovery. The same pre-build preflight requires the
  exact e2e receipt, all six fixed e2e names and all four complete e2e
  project-labelled sets empty. Re-run the complete lab and e2e empty preflight while
  holding the global lock immediately before Compose. After Compose up,
  inspect `pdns-lab-pg` through bounded podman-py and require
  its immutable `Image` ID to equal the just-built native image ID. After every
  successful up, atomically finalize the provisional global receipt with
  the exact IDs, labels and mounts of all five containers; name, creation time,
  labels and mountpoint of all three named volumes; and exact network ID plus
  labels. Require the complete project-labelled container/volume/network sets
  equal exactly the receipt and the pod set empty. Assert the exact pod name
  remains absent after up, rollback and down.
  Status and verify compare the running ID to the captured native
  ID and require its canonical schema bind source to equal this exact worktree's
  `test/lab/schema.pgsql.sql`. Immediately before every down, load the receipt
  and require every object byte-identical plus complete project-labelled set
  equality before invoking Compose. After down, require all four labelled sets
  empty, every fixed name absent, and only then unlink the same validated receipt
  inode. Mutations inject an extra differently named project-labelled container,
  volume, network and pod in turn during pre-up, partial rollback, ready/consumer
  validation and pre-down as applicable; every mismatch requires zero build,
  child, mutation or teardown calls. Tests
  replace each container class and each volume/network class in turn and
  require zero teardown calls. Also cover exact success, dirty preflight,
  missing/corrupt/symlink receipt, absent object, API failure, empty image ID,
  stale retag, foreign worktree image and wrong bind source. For Compose error,
  health/API timeout and caught interruption, enumerate the created subset under
  the global lock, require only expected names with exact transaction/project
  labels, record and immediately revalidate immutable identities, then perform
  guarded rollback and prove zero runtime/receipt/iidfile residue. A foreign or
  replaced subset preserves the provisional receipt and makes no teardown call.

  Add the parallel e2e transaction oracle, managed by `lab owned` under the
  same global lock. Bind project `terraform-provider-powerdns-e2e`, containers
  `pdns-e2e-s3`/`pdns-e2e-forgejo`, volumes
  `terraform-provider-powerdns-e2e_s3-data`/
  `terraform-provider-powerdns-e2e_forgejo-data`, network
  `terraform-provider-powerdns-e2e_default` and pod
  `pod_terraform-provider-powerdns-e2e`. Before e2e up, require all fixed names,
  all four e2e project-labelled sets and
  `/tmp/tfp-powerdns-e2e-runtime.receipt.json` absent. Atomically write the
  mode-0600 provisional receipt with exact fields `schema_version=1`,
  `state=provisional`, 32-lower-hex `transaction_id`, `canonical_worktree`,
  active `lab_transaction_id`, exact `project`, and initially empty
  `containers`, `volumes`, `networks`, `pods`. Pass the transaction as exact
  environment variable `PDNS_E2E_TRANSACTION_ID`, require `--in-pod=false`,
  and label both services and both volumes exactly
  `io.ioplane.powerdns.e2e.transaction=${PDNS_E2E_TRANSACTION_ID}`. On success,
  finalize exact container ID/label/mount identities, volume
  name/CreatedAt/label/mount identities; retain exact host networking and
  set `state=ready`; require receipt `networks={}` and `pods={}`, both matching
  project-labelled sets empty, and complete project-labelled set equality. E2e
  tests validate
  both lab and e2e receipts before and after the child. E2e down validates both
  receipts and full sets before any zone or Compose mutation, then proves all
  e2e objects absent before removing only the exact e2e receipt. Partial up
  enumerates only expected transaction-labelled objects and performs guarded
  exact rollback; a foreign, replaced or extra container/volume/network/pod
  preserves the receipt and makes zero teardown calls. Add mutations for dirty
  preflight, receipt corruption/symlink, omitted no-pod/transaction labels,
  every replaced or extra object class, partial failure and broad down before
  identity equality.

  Extend local lab down RED: while holding the global lock and before any lab
  Compose call, require the exact e2e receipt absent, all six e2e fixed names
  above absent and complete e2e project-labelled container/volume/network/pod
  sets empty. Also require the exact scan receipt, all three scanner names and
  complete scan-transaction-labelled container set empty. Active/corrupt/symlink
  receipt and each fixed or differently named residual object-class mutation
  must produce zero lab Compose/teardown calls.

  Add RED for the auxiliary `image-evidence` transaction used by Task 5. It
  holds the same global lock, validates the canonical ready lab receipt and
  rejects e2e/scan state. Under that lock it opens the evidence directory once
  with `O_DIRECTORY|O_NOFOLLOW`: require the same canonical immediate `/tmp`
  child, current UID, mode `0700`, device/inode, exact phase contents and
  unchanged hashes. Hold that FD; create immutable versioned leaves only with
  `dir_fd` plus `O_NOFOLLOW|O_CREAT|O_EXCL`, and compare pathname-to-FD identity
  around each output. Before its first Podman mutation it requires all exact
  source/final image IDs unmounted, exact verifier
  `tfp-powerdns-postgresql-evidence-go-version` and every container labelled
  `io.ioplane.powerdns.image-evidence.transaction` absent.

  Use persistent append-only journal
  `/tmp/tfp-powerdns-postgresql-image-evidence.receipt.jsonl`; never replace or
  unlink it. Open once with `O_NOFOLLOW`, or create only with
  `O_CREAT|O_EXCL` mode `0600`, and hold its FD. Require regular single-link
  type, current UID, mode, device/inode and pathname-to-FD identity. Validate
  the full canonical JSON-lines sequence/previous-digest chain and require all
  prior transactions terminal. Before the first output/mount append and
  `fsync` the schema-1 start record with both exact qualified child references,
  manifest/config expectations, final IDs and empty source-local/mount/
  container maps; phase/identity changes append and flush immutable chained
  records. Only fully clean success appends a terminal
  `complete`. The shared guard accepts either an absent virgin/pre-evidence
  journal or a present valid chain with latest `complete`; every other present
  state blocks lab up/down, owned consumers and image-scan. No update or removal
  targets a pathname after a separate check.

  RED must distinguish registry manifest digest, config digest and local image
  ID. For each platform, boundedly pull or resolve only the fully qualified
  child reference above with explicit platform, inspect immutably, and require
  exact `RepoDigest`, OS/architecture and local ID equal to the verified config
  digest. Journal the manifest→config→local-ID mapping before traversal and
  mount only that captured local ID. Reject a bare manifest as image ID,
  missing/garbage-collected source without the exact bounded acquisition,
  mutable tag, wrong `RepoDigest`, platform, config or local ID, and any
  substitution after capture. Require zero mount on each mismatch.

  Boundedly parse `podman info --format json`, require Boolean rootless and a
  non-empty storage driver, journal both and revalidate before cleanup. Permit
  one exact mount at a time. Rootless mode runs the complete mount, walk,
  extraction and unmount in one bounded process-group-aware `podman unshare`
  child. Rootful mode runs the identical helper as one direct bounded child,
  because rootful Podman rejects unshare. Reject missing/changed mode or driver
  and any failed-branch fallback; test both branches. Unmount only the recorded image/path pair proved
  absent at preflight. Create
  the labelled verifier, capture its CID and revalidate exact name/CID/label/
  read-only bind before attach-start and bounded stop/kill/remove; forbid
  `run --rm`.

  Bind `image-evidence` to one monotonic 1800-second deadline with a 1500-second
  work cutoff and 300-second cleanup reserve. Every mount/inspect/unmount and
  verifier create/attach/stop/kill/remove receives only positive remaining
  outer time through the process-group-aware runner. Timeout, SIGINT, SIGTERM
  and `KeyboardInterrupt` terminate, kill and reap the client group before
  identity-guarded cleanup. Safe abnormal cleanup proves no owned mount,
  verifier or `conmon` survives but preserves journal/evidence; ownership drift
  removes nothing. Only full success appends the terminal journal record after
  all exact images are unmounted, verifier/labelled-container sets are empty
  and report hashes are recorded. Mutate every phase for timeout/interruption,
  replace CID/label/bind/path, inject active/invalid journal or stale container/
  mount, simulate rootless and rootful mode plus wrong-mode fallback, swap the
  directory/journal specifically between validation and create/append/terminal
  handling, inject symlink/chmod/chown/extra contents, exhaust cleanup reserve
  and add an unrelated object. Require no false GREEN and no unrelated
  mutation. Extend lab up/down, every owned lab consumer and `image-scan`
  reciprocal guards with the exact two-state oracle: absent virgin journal or
  present valid chain with latest `complete`. Any other present evidence
  journal, verifier, labelled container or recorded mount residue produces zero
  build/Compose/teardown/consumer/scan calls. RED proves absent-journal success
  independently for up, down, every named owned consumer and image-scan. Task 5
  post-evidence teardown separately requires present valid latest `complete`.

- [ ] **Step 5: Extend Taskfile contract RED**

  Require `lint:containers` to contain exactly one scalar hadolint command with
  both `Containerfile.dev` and `Containerfile.postgres-lab`. Reject omission,
  wildcard-only discovery, ignored failure, extra command and host hadolint.
  Require the direct `.github/workflows/ci.yml` hadolint command to name the
  same two exact files, and reject parity drift on either side. Run focused RED;
  expected failures name the absent functions, Compose variable and missing
  hadolint input, not syntax/collection errors.

  Replace `_lab-running` as an ownership boundary: each of `fixtures:record`,
  `testacc`, `e2e:up`, `e2e` and `e2e:down` must execute its exact existing argv
  through a `scripts.automation.lab owned` wrapper with exact outer timeouts
  900, 7800, 1800, 58500 and 900 seconds respectively. The e2e suite budget
  equals 59 currently
  collected cases times their 900-second per-case timeout plus 5400 seconds for
  suite overhead. A focused collection oracle executes pytest collection,
  requires a non-zero result and exactly 59 cases, and fails if future
  parametrization changes that count without revisiting the budget. The
  wrapper acquires the
  global runtime lock, requires a ready receipt owned by this canonical
  worktree, validates complete object/set identity, holds the lock through the
  child process, and on timeout, SIGINT, SIGTERM or `KeyboardInterrupt`
  terminates the complete child process group, waits a bounded grace period,
  kills any survivor and reaps the child before revalidating in `finally`.
  Only after clean shutdown and revalidation may it return the exact status or
  re-raise the original timeout/interruption. A revalidation failure becomes
  the primary safety error while retaining the original child failure as
  context. Static and
  behavioural mutations remove the wrapper from each target, use a foreign
  worktree receipt, release before child completion, use shell-string argv,
  omit/shorten/swap timeouts, suppress a non-zero child, or skip final
  revalidation on non-zero, timeout and interruption. Dedicated subprocess
  mutations leave a grandchild alive to perform a late ownership mutation after
  SIGINT, SIGTERM and `KeyboardInterrupt`; every mutation must fail closed and
  prove the child group was reaped before the lock or revalidation boundary.

  The required workflows use the same wrapper around their direct host-runner
  argv: acceptance uses `acceptance-ci` with 3900 seconds, coverage uses
  `coverage-ci` with 1800 seconds, e2e fixture up uses `e2e-up-ci` with 1200
  seconds, the e2e suite uses `e2e-ci` with 3000 seconds, and e2e fixture down
  uses `e2e-down-ci` with 900 seconds. Split the coverage report from the
  mutating Go test so no shell wrapper is needed. Add the complete sequential
  lifecycle arithmetic oracle: lab up is bounded by 1200 seconds of platform
  builds plus 600 seconds of Compose and 120 seconds of API wait; lab down is
  bounded by 600 seconds; the separate read-only lab verification receives 180
  seconds and each workflow retains another 600-second runner reserve.
  Acceptance therefore requires at least 120 minutes and commits 125; coverage
  requires at least 85 and commits 90; e2e requires at least 140 and commits
  150. Contract
  mutations bypass the wrapper, alter argv, omit/swap/shorten a budget, add a
  shell string, or reduce a job ceiling below its required overhead; require
  workflow/Task parity and zero child execution on every mismatch.

  Add RED workflow parity for permanent executable-image security: the
  Security job retains the repository filesystem scan, pins exact Trivy 0.74.0
  by qualified tag and digest, installs the already pinned uv action, and
  invokes this exact command for both exact platform archives:

  ```sh
  uv run --locked python -m scripts.automation.lab image-scan \
    --output "$RUNNER_TEMP/postgres-lab-security"
  ```

  Require a canonical fresh output directory, non-zero config-test and package
  counts and
  propagate scanner failure. Mutations retain only the filesystem scan, omit
  either platform, float or downgrade the scanner, suppress failure or accept
  an empty report. The subcommand holds the global lock and requires both
  receipts, every fixed lab/e2e name and every project-labelled object set
  empty before building. It also accepts only an absent virgin evidence journal
  or a present valid chain with latest `complete`, and requires every evidence
  verifier/mount residue absent; dirty-namespace/evidence mutations require zero build/export/scan
  calls. It additionally rejects global scan receipt
  `/tmp/tfp-powerdns-postgresql-image-scan.receipt.json`, exact container names
  `tfp-powerdns-postgresql-scan-config`,
  `tfp-powerdns-postgresql-scan-amd64` and
  `tfp-powerdns-postgresql-scan-arm64`, or any container carrying label
  `io.ioplane.powerdns.image-scan.transaction`. Bind the subcommand to a
  5400-second outer budget and the
  Security job to 105 minutes, leaving 15 minutes for checkout, setup and report
  handling. Capture one monotonic 5400-second outer deadline plus a 4800-second
  work cutoff before preflight; reserve the final 600 seconds only for ownership
  inspection, process-group reap and scanner stop/kill/remove. Pass the exact
  positive remaining work time to every build, export and scanner subprocess
  through the process-group-aware bounded runner and start no work child after
  the cutoff. After the complete empty preflight and fresh canonical output-path
  validation, but before creating output/iidfile/image/archive state, atomically
  write a mode-0600 receipt with schema 1, transaction, canonical
  worktree/output, phase, intended iidfiles and empty image/archive/container
  maps. Atomically record every phase transition and immutable identity before
  and after its mutation. RED advances the clock before and
  during each phase and injects a surviving grandchild. Require a mode-0600
  provisional scan receipt with transaction/worktree/output/container identity;
  exact create then attach-start commands, never `run --rm`; transaction labels;
  CID capture and revalidation; and identity-guarded stop/kill/remove after
  success, timeout or interruption. Require no later child, terminate/kill/reap
  the client then give every identity inspect/stop/kill/remove only the positive
  remaining outer-deadline time; no cleanup phase resets the clock. Clean only
  owned scanner cleanup. On any non-zero, timeout or interruption preserve the
  receipt, output and recorded iidfile/image/archive identities and require lab
  up/down to reject them; only full success may remove owned iidfiles and the
  receipt after scanner absence and final report hashes are recorded. Leave
  unrelated containers byte-identical. Missing/replaced/wrong-label scanner
  mutations preserve the receipt/output and perform no removal. Build-timeout
  and export-timeout mutations prove receipt creation precedes the first call
  and blocks lab up/down pending separately authorized recovery. Reject
  any smaller or purely
  structural budget. Worst-case clock mutation consumes 4800 seconds of work
  and each bounded cleanup phase, proving return by 5400 or a retained
  receipt/evidence failure. Require the Security workflow header to distinguish
  this blocking image gate from its alert-only source reports. Require its job
  name exactly `PostgreSQL lab image (Trivy)` and add it to the static
  declared-job protection test; name drift must fail before any external rule
  mutation.

  Write RED behavior for `e2e:down` before changing its driver. Read each name
  first. An existing zone is deleted only with exact HTTP 204 plus an empty
  body; an already-absent zone is accepted only through its product-specific
  read response. Transport, malformed JSON and unexpected HTTP status/body
  pairs propagate. Before e2e Compose teardown, re-read all managed names and
  require HTTP 404 for the 11 Authoritative/LMDB zones plus HTTP 422 whose
  parsed JSON value is exactly
  `{"error":"Could not find domain 'internal.e2e.example'"}` for the Recursor
  zone. Mutations suppress an error, accept a wrong success status/non-empty
  body, accept malformed/extra-field/wrong absence JSON, skip the post-delete
  oracle, leave one zone present or call Compose before the oracle; every case
  requires zero e2e teardown calls and no false success message.

  Required workflow teardown retains `if: always()` but rejects `|| true`,
  `continue-on-error`, ignored status and skipped residue postconditions. The
  e2e workflow keeps leased e2e down before lab down in one fail-fast ordered
  step, so a lease, zone, e2e-object or residue refusal prevents broader lab
  teardown. Acceptance and coverage propagate the receipt-backed lab down
  result. Mutation tests require a non-zero workflow result for every ownership,
  teardown or residue failure.

  Before any production edit, collect and run each affected module separately:
  `test_postgres_lab_lifecycle.py`, `test_run.py` and the newly created
  `test_e2e.py`, plus affected existing `test_taskfile.py` and
  `test_protection.py`. Record positive exact `LIFECYCLE_CASES`, `RUN_CASES`,
  `E2E_DRIVER_CASES`, `TASKFILE_CASES` and `PROTECTION_CASES`. Each execution
  must preserve its collection count, have zero collection errors/skips/xfails,
  at least one failure and only its planned missing lifecycle/lease,
  process-group interruption, zone teardown, Task/workflow/hadolint parity or
  Security-job/protection-context behavior. Fix test mistakes until failures
  are semantic. Do not include the static Containerfile module in this RED.

  ```sh
  DEV_SUFFIX=-postgresql-18-17d2b9851be8 \
    podman-compose \
      -p terraform-provider-powerdns-dev-postgresql-18-17d2b9851be8 \
      -f deployments/compose/compose.dev.yml \
      exec -T dev uv run --locked pytest --collect-only -q \
      test/scripts/test_postgres_lab_lifecycle.py
  DEV_SUFFIX=-postgresql-18-17d2b9851be8 \
    podman-compose \
      -p terraform-provider-powerdns-dev-postgresql-18-17d2b9851be8 \
      -f deployments/compose/compose.dev.yml \
      exec -T dev uv run --locked pytest -q \
      test/scripts/test_postgres_lab_lifecycle.py
  # Repeat both exact commands, independently, for:
  #   test/scripts/test_run.py
  #   test/scripts/test_e2e.py
  #   test/scripts/test_taskfile.py
  #   test/scripts/test_protection.py
  ```

## Task 3: Implement the minimum reproducible Containerfile

**Files:**

- Create: `deployments/containers/Containerfile.postgres-lab`
- Test: `test/scripts/test_postgres_lab_containerfile.py`

- [ ] **Step 1: Add pinned build arguments and builder stage**

  Implement the following structure with exact values from this plan:

  ```Dockerfile
  ARG GO_IMAGE=docker.io/library/golang:1.27.0-trixie@sha256:6212da3924947f4b6a939df02ea627c13f338f1a41d6c3fcb0dd9d076eef46c4
  ARG POSTGRES_IMAGE=docker.io/library/postgres:18.6-alpine3.24@sha256:d3e1620b530c944afa6e887d22eb899824da68e19c52024bf98f5220c88a65b2
  ARG BUILDPLATFORM
  ARG TARGETPLATFORM

  FROM --platform=${BUILDPLATFORM} ${GO_IMAGE} AS gosu-builder
  ARG TARGETOS
  ARG TARGETARCH
  ARG GOSU_VERSION=v0.0.0-20250923190938-6456aaa0f3c8
  ENV GOTOOLCHAIN=local CGO_ENABLED=0 GOCACHE=/tmp/go-cache
  RUN --mount=type=tmpfs,target=/go/pkg/mod \
      --mount=type=cache,target=/tmp/go-cache \
      download="$(go mod download -json github.com/tianon/gosu@${GOSU_VERSION})" \
      && printf '%s\n' "$download" | grep -F '"Sum": "h1:HIpXk5mGBQGfOqcaBbRT4Vnss8NPICnMGlD5xTlPBdQ="' \
      && printf '%s\n' "$download" | grep -F '"GoModSum": "h1:SwhRwWsO6iqXZN9CpIaU9CnOrUqpWDINW16KaaSqnrU="' \
      && GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
         go install -trimpath -ldflags='-d -w' \
         github.com/tianon/gosu@${GOSU_VERSION} \
      && host_os="$(go env GOHOSTOS)" \
      && host_arch="$(go env GOHOSTARCH)" \
      && if [ "${TARGETOS}" = "$host_os" ] && [ "${TARGETARCH}" = "$host_arch" ]; then \
           built="$(go env GOPATH)/bin/gosu"; \
         else \
           built="$(go env GOPATH)/bin/${TARGETOS}_${TARGETARCH}/gosu"; \
         fi \
      && install -D -m 0755 "$built" /out/gosu \
      && go version -m /out/gosu | \
         awk -v version="${GOSU_VERSION}" \
         '$1 == "mod" && $2 == "github.com/tianon/gosu" && $3 == version { found = 1 } END { exit !found }'
  ```

  Verify actual `go mod download -json` output in the pinned builder before
  relying on the exact checksum fields. If its layout differs, fix both code
  and test; never weaken either checksum check. The module-cache mount must be
  a fresh tmpfs, never a persistent cache. Mutation tests replace it with a
  cache mount and seed a modified extracted module tree; the static contract
  must reject the mount before that tree can feed compilation.

- [ ] **Step 2: Add target replacement and flattened final stage**

  Use `FROM --platform=${TARGETPLATFORM} ${POSTGRES_IMAGE} AS postgres-rootfs`,
  then only:

  ```Dockerfile
  COPY --from=gosu-builder --chown=0:0 --chmod=0755 /out/gosu /usr/local/bin/gosu
  ```

  The final `FROM scratch` copies `/` once and restates the exact inspected
  official configuration:

  ```Dockerfile
  ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
      GOSU_VERSION=1.19 \
      LANG=en_US.utf8 \
      PG_MAJOR=18 \
      PG_VERSION=18.6 \
      PG_SHA256=555610c24d53e4316da5b7d3fc25c279d96856d5e0e23ee308c328c5fa881d9f \
      DOCKER_PG_LLVM_DEPS="llvm21-dev clang21" \
      PGDATA=/var/lib/postgresql/18/docker
  WORKDIR /
  VOLUME /var/lib/postgresql
  EXPOSE 5432
  ENTRYPOINT ["docker-entrypoint.sh"]
  CMD ["postgres"]
  STOPSIGNAL SIGINT
  ```

  Inspect the resulting Env array rather than trusting Containerfile text.
  Every entry is byte-identical to the source config except the explicitly
  normalized `DOCKER_PG_LLVM_DEPS=llvm21-dev clang21`; split that value and
  require exactly the two source tokens.

- [ ] **Step 3: Add truthful OCI labels**

  Accept build args `CREATED` and `REVISION`. Set `created`, `revision`,
  `version=18.6.0-lab.1`, `source`, `documentation`, `title`, `description` and
  `vendor`, plus the two `io.ioplane.image.source-base.*` labels. Do not add
  standard `base.*` or `licenses`. Documentation points to `AUDIT-03` at the
  stable default-branch URL. The executable `revision` label remains immutable;
  final report/SBOM hashes land in that audit through the later reviewed docs
  commit.

  ```Dockerfile
  ARG CREATED
  ARG REVISION
  LABEL org.opencontainers.image.created="${CREATED}" \
        org.opencontainers.image.revision="${REVISION}" \
        org.opencontainers.image.version="18.6.0-lab.1" \
        org.opencontainers.image.source="https://github.com/ioplane/terraform-provider-powerdns" \
        org.opencontainers.image.documentation="https://github.com/ioplane/terraform-provider-powerdns/blob/main/docs/audit/AUDIT-03-postgresql-18-lab.md" \
        org.opencontainers.image.title="terraform-provider-powerdns PostgreSQL lab" \
        org.opencontainers.image.description="Disposable PostgreSQL 18.6 fixture for provider acceptance and e2e tests" \
        org.opencontainers.image.vendor="ioplane" \
        io.ioplane.image.source-base.name="docker.io/library/postgres:18.6-alpine3.24" \
        io.ioplane.image.source-base.digest="sha256:d3e1620b530c944afa6e887d22eb899824da68e19c52024bf98f5220c88a65b2"
  ```

- [ ] **Step 4: Run static GREEN and mutation matrix**

  Run `pytest --collect-only -q` and pytest for only
  `test/scripts/test_postgres_lab_containerfile.py` through the canonical dev
  container. Require the collection count equals the recorded positive
  `STATIC_CASES` and the result is exactly `STATIC_CASES passed`; the lifecycle
  module remains RED by design until Task 4. Then run `task lint:pins` and
  hadolint directly against the new file. Expected: both external references
  add exactly two verified pin inputs and no new hadolint error.

## Task 4: Integrate worktree-local builds into the lab lifecycle

**Files:**

- Modify: `scripts/automation/lab.py`
- Modify: `scripts/automation/run.py`
- Modify: `scripts/automation/e2e.py`
- Modify: `deployments/compose/compose.lab.yml`
- Modify: `deployments/compose/compose.e2e.yml`
- Modify: `test/scripts/test_lab.py`
- Modify: `test/scripts/test_postgres_lab_lifecycle.py`
- Modify: `test/scripts/test_run.py`
- Create: `test/scripts/test_e2e.py`
- Modify: `Taskfile.yml`
- Modify: `test/scripts/test_taskfile.py`
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/acceptance.yml`
- Modify: `.github/workflows/coverage.yml`
- Modify: `.github/workflows/e2e.yml`
- Modify: `.github/workflows/security.yml`
- Modify: `test/scripts/test_protection.py`

- [ ] **Step 1: Implement canonical metadata and tag helpers**

  Add frozen records for Git build metadata and built image identity. Obtain
  suffix via bounded argv `scripts/dev-suffix.sh`. Capture the full revision
  once, then obtain the committer RFC3339 timestamp and epoch from that exact
  captured object through bounded argv `git show -s --format=... <revision>`
  calls with `cwd=REPO_ROOT`. Validate full lowercase 40-hex revision, integer
  epoch, parseable timezone timestamp and suffix grammar. Construct exactly the
  two tags from Task 2. Immediately before each build, re-read `HEAD` and fail
  unless it still equals the captured revision. Tests mutate `HEAD` between
  metadata capture and build and require zero build calls.

- [ ] **Step 2: Implement two bounded builds and inspections**

  Build amd64 then arm64/v8 with the exact command contract from Task 2. Use
  the container directory as the qualified context because the Containerfile
  copies no repository files. Acquire and validate the single global runtime lock,
  use one unique iidfile per platform, normalize each returned ID to 64
  lowercase hex, inspect by that ID, then require the tag resolves to it.
  Validate target architecture and all dynamic/static labels and return the
  frozen identity. Hold the lock until Compose up and running-ID verification
  complete, then release it without unlinking its synchronization inode; unlink
  only the exact validated iidfiles.
  Never infer success from the build return code or mutable tag alone, and
  require zero iidfile residue after both success and bounded failure.

- [ ] **Step 3: Integrate Compose environment and runtime identity**

  `cmd_up` proves all fixed containers, three named volumes, the exact fixed pod
  and default-network names absent through bounded Podman API calls while
  holding the global lock. It also proves the e2e receipt, all six fixed e2e
  names and every e2e project-labelled object set empty before either build,
  and proves the global scan receipt, all three exact scanner names and complete
  scan-labelled container set empty,
  then builds both images and repeats that entire lab/e2e preflight immediately
  before Compose. Every Compose invocation uses
  `--in-pod=false`. It atomically writes the
  provisional transaction receipt, passes native tag and transaction ID through
  the copied environment, waits for PostgreSQL plus all PowerDNS APIs, then
  requires
  `pdns-lab-pg` to run the captured native image ID and exact canonical schema
  bind and exact ownership labels, and proves the fixed pod still absent.
  Require the complete project-labelled sets equal only these transaction
  objects, then finalize the receipt after every object is healthy and verified. `compose()`
  passes the receipt environment for every command. Status/verify acquire the
  global lock and use the receipt plus current
  immutable ID and worktree bind. Down acquires the same lock, validates every
  receipted object class immediately before calling Compose, and additionally
  requires `/tmp/tfp-powerdns-e2e-runtime.receipt.json`, both fixed e2e
  containers, both fixed e2e volumes, the exact e2e pod/network names and all
  four complete e2e project-labelled sets absent. It also requires the scan
  receipt, all three exact scanner containers and every scan-labelled container
  absent. It returns non-zero without
  teardown on any mismatch, then proves exact absence before removing the owned
  receipt. A failed/partial up records and validates its exact
  transaction-labelled subset and rolls it back under the same lock; if safe
  rollback cannot be proved, retain the provisional receipt and fail closed.

  For `e2e:up`, the owner wrapper proves the full e2e namespace and exact
  `/tmp/tfp-powerdns-e2e-runtime.receipt.json` absent, writes its mode-0600
  provisional receipt with `schema_version=1`, `state=provisional`, the
  32-lowercase-hex `transaction_id`, canonical worktree, active
  `lab_transaction_id`, exact project and initially empty object maps. It passes
  the ID as `PDNS_E2E_TRANSACTION_ID` to an `--in-pod=false` e2e Compose
  invocation and labels both services and named volumes exactly
  `io.ioplane.powerdns.e2e.transaction=${PDNS_E2E_TRANSACTION_ID}`. It finalizes
  exact object identities only after health. On partial failure it performs only
  transaction-labelled, identity-guarded rollback. For `e2e` it requires both complete receipts and
  revalidates the e2e object sets after the child. For `e2e:down` it requires
  both receipts and full e2e set equality before the child, then requires every
  receipted/fixed/project-labelled e2e object absent before removing only the
  e2e receipt. Add the transaction label to both e2e services and named volumes;
  preserve their images, host networking, mounts, health checks and data.

  Implement the bounded `owned` subcommand used by all five direct lab
  consumers and the required workflow variants. It acquires the same global
  lock, rejects a receipt owned by a
  different canonical worktree, verifies ready state and complete receipt/set
  equality, maps the named consumer to its exact timeout, and runs only the
  received argv with no shell. The subprocess runs in a new session. Timeout,
  SIGINT, SIGTERM and `KeyboardInterrupt` terminate that complete process group,
  wait a bounded grace period, force-kill survivors and reap the child before a
  `try/finally` revalidates ownership while still locked. If revalidation
  passes, return or raise the original child result unchanged; on ownership
  drift, raise the safety error with the original result chained as context.

  Replace `drop_managed_zones()` fail-open suppression with bounded exact
  response classification. Read first; delete only an existing zone and
  require exact HTTP 204 plus empty body. Then re-read all 12 names and require
  exact 11×HTTP-404 plus one Recursor HTTP-422 whose parsed JSON object is
  exactly `{"error":"Could not find domain 'internal.e2e.example'"}` before
  invoking e2e Compose down. Transport, malformed JSON, unexpected HTTP/body
  and residual-zone failures propagate without printing success or touching
  the e2e runtime.

  Implement the full `image-evidence` command specified by Task 2 before any
  final evidence run. Reuse the global lock, lab receipt validation, bounded
  process-group runner, held directory FD and append-only journal helpers; add
  the journal/verifier/mount reciprocal guards to lab up/down, every owned
  consumer and `image-scan`. The rootfs helper must run the entire mount/
  traversal/extraction/unmount sequence inside one mode-selected child:
  `podman unshare` only for validated rootless mode, direct only for validated
  rootful mode, and never a fallback. It returns only validated metadata/output
  identities. The
  parent retains the lock and the single 1800/1500/300 deadline, revalidates
  held directory and append-only journal identity at every phase, and performs
  only bounded identity-guarded cleanup. Implement exact per-platform source
  acquisition and manifest→config→local-ID validation before traversal; use no
  mutable name or bare manifest digest as an image ID. Implement verifier
  create/CID capture/attach and
  guarded stop/kill/remove without `run --rm`. No Task 5 command may be the
  first execution of this production path.

- [ ] **Step 4: Replace the image and add transaction ownership labels**

  Replace the remote PostgreSQL image with the required variable and add only
  the exact transaction label to all services, named volumes and an explicit
  default network. Preserve exact credentials, port, schema bind,
  `/var/lib/postgresql` tmpfs, health check and dependency ordering. No Compose
  `build`, `platform`, reusable data volume or fallback value is allowed.

- [ ] **Step 5: Update hadolint aggregate and run focused GREEN**

  Add the exact second Containerfile path to both the existing in-container
  Task hadolint command and the direct hadolint invocation in
  `.github/workflows/ci.yml`. Route the acceptance, coverage and e2e workflow
  children through the exact owned-consumer variants specified in Task 2;
  set job ceilings to acceptance 125, coverage 90 and e2e 150 minutes so every
  checked lifecycle/consumer budget plus the required reserve fits. Keep
  `if: always()` teardown, but
  remove every fail-open suppression and keep leased e2e down before lab down
  in one ordered fail-fast step. Extend the workflow/Task parity oracle so
  removal or drift on any surface fails.

  Update `.github/workflows/security.yml` to the exact Trivy 0.74.0 pin while
  retaining its filesystem scan. Add pinned uv setup and the exact required
  blocking `image-scan` command from Task 2 and workflow mutation/parity tests.
  The subcommand uses the same locked build,
  immutable ID, OCI export and scanner validation helpers used by Tasks 5 and
  6; it accepts only a validated fresh output directory, proves both runtime
  namespaces empty, applies one monotonic 5400-second deadline to every phase
  with a 4800-second work cutoff and 600-second cleanup reserve through the
  bounded process-group runner, writes the provisional scan receipt before
  creating the output directory/iidfiles/images/archives, atomically updates its
  phase and identity maps around every mutation,
  creates each exact transaction-labelled scanner container and captures its
  CID before attach-start. On timeout it reaps the client group, identity-guards
  exact scanner stop/kill/removal using only the remaining outer-deadline time.
  Any abnormal exit preserves receipt/output/recorded identities and blocks lab
  up/down; only full success removes owned iidfiles and the receipt after
  proving scanner absence and recording final report hashes. Ownership drift
  preserves the receipt/evidence and removes nothing.
  Rewrite the workflow header so alert-only source reports and the blocking
  executable-image gate are described truthfully. Name the job exactly
  `PostgreSQL lab image (Trivy)` and extend the declared-context test. Recollect
  only
  each of `test/scripts/test_postgres_lab_lifecycle.py`,
  `test/scripts/test_run.py`, `test/scripts/test_e2e.py`,
  `test/scripts/test_taskfile.py` and `test/scripts/test_protection.py`
  separately. Require their exact recorded positive `LIFECYCLE_CASES`,
  `RUN_CASES`, `E2E_DRIVER_CASES`, `TASKFILE_CASES` and `PROTECTION_CASES`, then
  require exactly that many passes for each with zero failures, skips, xfails
  or collection drift.
  The focused GREEN must include every `image-evidence` success, rootless and
  rootful mode/no-fallback, source manifest/config/local-ID acquisition and
  mismatch, journal/directory replacement, timeout/interruption, reciprocal-lab
  guard and residue mutation from Task 2; absence or xfail/skip is failure.
  Run the separate static module and require exactly `STATIC_CASES passed`.
  Then run `task py`,
  `task lint:containers`, `task lint:pins`, both Compose renders with an
  explicit canonical native value, `task docs:lint`, gopls over every Go file
  and both diff checks. Record exact counts and tool versions.

- [ ] **Step 6: Commit the executable candidate before final evidence**

  Fetch and require the GraphQL default OID still equals the reviewed base.
  Append pre-evidence results to both Beads, stage every intended executable,
  test and preliminary documentation path, inspect base-to-index plus unstaged
  diffs, and commit with hooks/no bypass as
  `build(lab): harden the postgresql image`. Append the exact commit SHA to
  both Beads. Tasks 5–7 must build this committed candidate so OCI `revision`
  and `created` describe the executable source being tested, never its parent.
  Any later executable edit requires a new hook-enforced executable candidate
  before all final evidence is repeated.

## Task 5: Build and prove both OCI platform artifacts

**Files:**

- Modify: `docs/audit/AUDIT-03-postgresql-18-lab.md`
- Modify: this plan
- Update: both Beads

- [ ] **Step 1: Establish exact local-image preflight**

  Resolve automatic suffix and exact amd64/arm64 tags. Inventory existing
  tags, image IDs, containers and disk. If either exact tag already exists,
  record its ID; building may retag it, but do not remove the prior image or
  any unrelated object. Require no fixed lab/e2e runtime object before the
  lifecycle begins. Create the retained evidence directory with
  `mktemp -d /tmp/tfp-postgresql18-hardened.XXXXXX`, resolve it with `realpath`,
  require it to be an immediate `/tmp` child with the exact prefix, mode
  `0700`, no symlink and no pre-existing contents, and record its canonical
  path in both Beads. Preserve it through review; do not delete it as part of
  this plan.

- [ ] **Step 2: Run the production build path**

  Require clean executable paths and confirm HEAD is the exact candidate from
  Task 4 Step 6. Run `task lab:up AUTH=5.1`; retain cold and immediate warm build durations.
  Capture both local tags, IDs, manifest/config media types, OS/architecture,
  diff IDs, history, labels, size and layer count. Require one final rootfs
  layer per platform and exact tag-to-ID equality. Require no QEMU/binfmt
  handler and no foreign binary execution in build history. Capture the exact
  IDs, project labels, mounts and ownership fields for every fixed lab
  container, named volume and network created by this lifecycle so the fixture
  can later be torn down only if its identity remains unchanged. Prove the
  fixed pod name remains absent.

- [ ] **Step 3: Compare normalized source and final filesystems**

  Invoke the already focused-GREEN shared `scripts.automation.lab
  image-evidence` command against the
  retained directory, both exact fully qualified source-child references and
  captured final image IDs. The command boundedly acquires each source child,
  proves exact RepoDigest/platform and manifest→config→local-ID mapping, records
  both captured source config IDs, and never passes a bare manifest digest to a
  local image operation. The
  command must execute the receipt/global-lock/deadline contract from Task 2;
  direct `podman image mount`, unbounded subprocesses and `run --rm` are
  forbidden here. It revalidates the held evidence-directory FD and append-only
  journal inside the lock, then uses a single mode-selected child per exact
  image: bounded `podman unshare` for validated rootless mode or the equivalent
  bounded direct helper for validated rootful mode, with no fallback. It
  processes each captured source local ID and final image ID one at a time and produces
  sorted manifests containing path, type, numeric UID,
  numeric GID, mode and symlink target, plus content SHA-256 for every regular
  file. Require exact path/metadata equality and exact content equality except
  `/usr/local/bin/gosu`. Explicitly require:

  - `/var/lib/postgresql` `70:70` mode `1777`;
  - `/var/run/postgresql` `70:70` mode `3777`;
  - all entrypoint scripts executable;
  - final `gosu` `0:0` mode `0755` and content hash different from source;
  - no original `gosu` hash in the final layer/archive;
  - no module/compiler cache, Go source, package index or temporary build file.

- [ ] **Step 4: Verify both binaries and execute the native runtime contract**

  In the live amd64 container run `gosu --version` and inspect `PGDATA` after
  initialization. Require `image-evidence` to extract the already-hashed `gosu`
  binary from each exact platform image into separate amd64/arm64 paths in the
  retained evidence directory. It runs `go version -m` against both files from
  the exact pinned native Go builder image through the exact captured,
  transaction-labelled verifier container with only that directory mounted
  read-only. Parse the tab-delimited fields and require Go
  1.27.0 plus the exact module pseudo-version and sum for each binary, together
  with `build GOOS=linux` and the platform-corresponding
  `build GOARCH=amd64|arm64`. Prove the same verifier rejects each binary under
  the swapped expected architecture before recording GREEN. Require `gosu`
  1.19, `PGDATA` `70:70` mode `0700`, PostgreSQL
  `server_version_num=180006`, seven tables, 19 indexes and four cascade foreign
  keys. Do not execute arm64 on the amd64 host.

- [ ] **Step 5: Tear down the Task 5 fixture with exact ownership guards**

  First require the evidence journal's latest transaction valid and terminal
  `complete`, all four source/final image IDs
  unmounted, the exact verifier and every evidence-transaction-labelled
  container absent, and the final evidence hashes present. An active, failed,
  truncated, replaced or invalid journal or any mount/container residue is a
  fail-closed recovery condition; do not
  hide it by tearing down the lab.
  Immediately before teardown, re-read every captured container, named volume
  and network identity and require byte-for-byte equality with Step 2; prove
  the fixed pod remains absent.
  Stop fail closed on absence, replacement, label drift or mount drift. Run the
  exact approved `task lab:down AUTH=5.1`, then prove all captured objects and
  all fixed lab/e2e runtime names absent while both local platform image IDs and
  the retained evidence directory remain present and unchanged. Re-prove the
  evidence journal still valid/terminal with identical inode and hash, and
  prove verifier, labelled-container set and exact image-mount set absent.
  Record the
  post-flight inventory and disk capacity in both Beads. Task 7 must therefore
  start from a genuinely empty runtime namespace rather than reusing this
  evidence fixture.

## Task 6: Produce scanner, SBOM and licence evidence

**Files:**

- Modify: `docs/audit/AUDIT-03-postgresql-18-lab.md`
- Modify: this plan
- Update: both Beads

- [ ] **Step 1: Revalidate the retained evidence directory**

  Re-resolve the Task 5 directory and require the same canonical immediate
  `/tmp` child, owner and mode. Its only permitted pre-existing files are the
  two platform `gosu` binaries and metadata reports recorded by exact SHA-256
  in Task 5. Fail closed on any extra path, symlink or changed hash.

- [ ] **Step 2: Export exact platform images**

  Use bounded `podman save --format oci-archive` for each captured image ID,
  not a mutable tag, and hash each archive. Re-inspect the archive manifest and
  require the captured platform/config ID. Parse the OCI archive `index.json`,
  require exactly one platform manifest descriptor, hash the exact referenced
  manifest blob, and require its `sha256` to equal the descriptor digest.
  Record that observed per-platform local manifest digest separately from the
  archive hash and config/image ID; do not represent it as a registry or
  published digest. A tag-to-ID check immediately before export prevents a
  concurrent retag false green.

- [ ] **Step 3: Scan the Containerfile and OCI image configs**

  Run the exact Trivy 0.74.0 image through the receipt-backed create/start/
  identity-guarded-remove scanner lifecycle from Task 4
  with only read-only source and evidence/cache mounts. Capture
  `trivy version --format json`. Run `trivy config --format json` on the exact
  Containerfile and, for both archives, `trivy image --input` with
  `--scanners vuln,secret,license`, `--image-config-scanners misconfig,secret`,
  `--list-all-pkgs`, `--license-full` and JSON output. Record non-zero config
  test and package counts plus DB UpdatedAt/NextUpdate/DownloadedAt.
  Run the shared `image-scan` command used by the Security workflow and require
  its two archive identities and normalized policy result to match these
  retained Task 6 reports. This is the executable proof for the permanent
  workflow path, not a second weaker scanner implementation.

- [ ] **Step 4: Generate and validate both SBOM formats**

  From each same archive/cache generate CycloneDX JSON and SPDX JSON. Require
  valid JSON, exact artifact/platform identity, non-zero components/packages,
  PostgreSQL 18.6, `gosu` exact pseudo-version, Go 1.27.0 and expected Alpine
  package inventory. Hash every report and SBOM.

- [ ] **Step 5: Triage every finding without a false zero**

  Acceptance is zero secrets and zero HIGH/CRITICAL vulnerabilities. Reconcile
  duplicate misconfiguration IDs across Containerfile/config reports; require
  zero untriaged findings and document exact rule ID, severity, input and
  rationale for only the preserved root-to-`gosu` entrypoint and Compose-owned
  healthcheck rules if reported. Explicitly record the remaining UNKNOWN
  Windows-only `x/sys` advisory and prove its package/platform reachability
  result. Normalize all licence names/categories, map them to SBOM packages,
  and require no repository-incompatible licence. The standard OCI `licenses`
  label remains absent because the full conjunction is not losslessly
  expressible from the scanner's `Public-Domain` value.

  Any secret, HIGH/CRITICAL, unexplained UNKNOWN, empty input or incompatible
  licence stops the plan and keeps both Beads IN_PROGRESS. Do not create an
  exception without new explicit user approval.

## Task 7: Repeat the complete PostgreSQL compatibility matrix

**Files:**

- Modify: `docs/audit/AUDIT-03-postgresql-18-lab.md`
- Modify: both PostgreSQL plans
- Update: both Beads

- [ ] **Step 1: Restart from exact empty preflight**

  The image/lab executable changes invalidate all earlier Task 4 evidence.
  Require fixed lab/e2e containers, volumes, pods/networks, managed zones,
  remote-state keys and generated paths absent. Capture unrelated Podman
  inventory with IDs, labels, mount sources and creation timestamps. Stop for
  scoped cleanup authority if any owned target is dirty.

- [ ] **Step 2: Run full Auth 5.1 lifecycle**

  Run lab up/status/verify, direct PostgreSQL version/schema/index/FK/row
  oracles, `task verify AUTH=5.1`, then the complete consumer e2e up/status/test
  lifecycle. Require 35 acceptance PASS plus the one intentional Recursor
  `api_dir` SKIP, all e2e scenarios, exact S3 keys, expected zone absence
  semantics and zero SQL rows. Capture complete lab/e2e ownership identities
  after up and compare byte-for-byte immediately before each fixed-name down.

- [ ] **Step 3: Tear down Auth 5.1 exactly**

  Remove only exact generated paths after canonical/non-symlink/untracked/
  gitignored validation and explicit scoped approval when material. Run guarded
  e2e down, then guarded lab down. Require all captured runtime objects, state,
  zones, rows and paths absent. Record unrelated inventory deltas without
  touching them.

- [ ] **Step 4: Repeat the identical full matrix on Auth 5.0**

  Start from a new empty preflight. Repeat every Step 2 and Step 3 oracle with
  Authoritative 5.0.6; do not reuse 5.1 evidence. End with exact empty lab/e2e
  state and retained local platform images only.

- [ ] **Step 5: Run all non-lab gates**

  Run `task all`, `task osv-scan`, `task release:dryrun`, both platform
  image/config/rootfs inspections, all scanner/SBOM checks, `task docs:lint`,
  `bd lint`, `bd dep cycles`, gopls over a non-empty Go file set, `go mod tidy
  -diff` inside the canonical dev container and both diff checks. Record exact
  counts, elapsed times and zero-finding input counts.

- [ ] **Step 6: Invalidate stale evidence after any executable edit**

  Any post-Step-4 code, test, Containerfile, Compose, Taskfile, lock or workflow
  edit repeats Tasks 5–7 in full. Documentation-only evidence corrections run
  docs/Beads/diff gates and receive exact candidate review; they do not invent
  runtime results.

## Task 8: Candidate, sequential reviews and pull-request closure

**Files:**

- Modify: `CHANGELOG.md`
- Modify: `AGENTS.md`
- Modify: `README.md`
- Modify: `docs/development.md`
- Modify: `docs/plan.md`
- Modify: `docs/audit/AUDIT-03-postgresql-18-lab.md`
- Modify: both PostgreSQL plans
- Update: Beads `tfp-bqt.6.5`, `tfp-bqt.6.1`

- [ ] **Step 1: Reconcile current documentation and rollback**

  State explicitly that Alpine is only the unpublished disposable lab/e2e
  PostgreSQL image. Document local builds, no-publish/no-signature boundary,
  exact scanner/SBOM evidence and rollback to PostgreSQL 17 plus tmpfs at
  `/var/lib/postgresql/data:rw,nosuid,nodev,noexec,size=512m`. Preserve released
  changelog/ADR history. Keep P10-06 and P10-15 active through reviews.

- [ ] **Step 2: Create an immutable evidence candidate**

  Re-fetch and recheck GraphQL default OID. If it changed, rebase and repeat all
  gates before committing. Append all available pre-commit evidence to both
  Beads and keep them IN_PROGRESS. Only documentation/evidence paths may differ
  from the Task 4 executable candidate; any executable difference returns to
  Task 4 Step 6 and repeats Tasks 5–7. Stage the intended evidence paths,
  inspect base-to-index and working-tree diffs, and commit with hooks/no bypass
  as `docs(docs): record the hardened postgresql image evidence`. Then append
  the exact evidence-candidate SHA to both Beads before review; make no Git or
  Podman change between sequential reviews. The OCI revision remains the exact
  executable candidate whose content is byte-identical in this docs commit.

- [ ] **Step 3: Run sequential candidate SPEC then QUALITY reviews**

  Review exact `origin/main...HEAD`. No Git, Beads or Podman mutation occurs
  between reviews. After both approvals append their exact results to Beads
  before the closure commit. Any Critical/Important finding resets active plan
  items, performs TDD remediation, repeats Tasks 5–7, creates a new candidate
  and restarts SPEC then QUALITY. Only two exact approvals permit closure docs.

- [ ] **Step 4: Create and review the docs-only closure commit**

  Mark evidenced P10 rows and plan steps complete while both Beads remain
  IN_PROGRESS. Commit only authorized closure docs with hooks. Run sequential
  SPEC then QUALITY on the exact closure HEAD; no later Git change is allowed
  before push.

- [ ] **Step 5: Push, open PR and verify all remote state**

  Recheck GraphQL default OID equals the reviewed base. Require the only push
  target to be canonical non-fork `origin` at the exact URL
  `https://github.com/ioplane/terraform-provider-powerdns.git`, corroborate
  `isFork=false` through GraphQL, and push the current branch only to `origin`.
  Open a Conventional Commit PR and exhaustively paginate checks, reviews,
  comments, threads and replies. Require all strict contexts green and no
  unresolved error-tier finding. Never force-push a reviewed head without
  restarting reviews.

  After the exact `PostgreSQL lab image (Trivy)` PR check is green, use
  `gh api graphql` to re-read the branch-protection rule ID, strict flag and
  exact existing eleven `{context, app.id}` pairs, then read the new check's app
  ID from its check suite. Introspect the live mutation schema and present the
  exact eleven-pair receipt, proposed GitHub-Actions-bound twelfth pair, complete
  mutation input and rollback to the user. Stop until the user explicitly
  approves this material external-policy change. After approval, append only
  the new pair through one
  `updateBranchProtectionRule(requiredStatusChecks: ...)` mutation, preserving
  strict mode and every prior app binding. Re-query and require the exact twelve
  pairs. Record the pre-change rule ID/pairs as a rollback receipt in both
  Beads; if the PR is abandoned or cannot merge, restore those exact prior
  eleven pairs and verify them. Do not close either Bead at this policy
  transition.

- [ ] **Step 6: Merge before closing Beads**

  Immediately before merge, fetch again and use GraphQL to require the current
  default-branch OID equals the reviewed base, PR `baseRefOid` equals that same
  OID, and PR `headRefOid` equals the exact reviewed closure HEAD. Any drift
  restarts rebase, affected and full evidence, executable/evidence candidate,
  closure commits, and sequential reviews; never update a reviewed head in
  place. Squash-merge only after these OIDs and all required checks/reviews are
  exact. Use GraphQL to prove the squash commit is the default-branch head or
  ancestor and the PR is MERGED.
  Then close `tfp-bqt.6.5` and `tfp-bqt.6.1`, verify dependency state and update
  external Beads only; do not create an unreviewed post-merge Git commit. The
  retained recovery stash is not dropped without a separate explicit cleanup
  decision.

## Acceptance summary

The plan is complete only when both external inputs remain exact, both local
platform artifacts are OCI and content/metadata-correct, the old `gosu` is
absent, all scanner/SBOM/licence inputs and findings are accounted for, both
Auth branches pass fresh lab/e2e matrices with zero residue, all repository
gates and sequential reviews pass, the PR is squash-merged, and both Beads are
closed only after default-branch proof.
