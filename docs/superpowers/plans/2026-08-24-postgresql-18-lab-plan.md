# PostgreSQL 18 Disposable Lab Implementation Plan

> **For agentic workers:** REQUIRED: Use
> `superpowers:subagent-driven-development` (if subagents are available) or
> `superpowers:executing-plans` to implement this plan. Steps use checkbox
> (`- [ ]`) syntax for tracking.

**Goal:** Move only the disposable PowerDNS gpgsql lab database from the
digest-pinned PostgreSQL 17 image to the independently verified PostgreSQL
18.6 OCI index, and prove clean initialization, schema compatibility, provider
behavior, end-to-end behavior, and zero residue on both Authoritative branches.

**Architecture:** Keep the five-service Compose topology and its existing
credentials, port, healthcheck, schema bind, and `service_healthy` ordering.
The database remains disposable and Compose declares no PostgreSQL data volume.
The official PostgreSQL 18 image itself declares
`VOLUME /var/lib/postgresql`; Compose deliberately overlays that destination
with a bounded tmpfs, so the engine creates no anonymous data volume to leak.
Every compatibility run creates a new in-memory cluster and processes the
PowerDNS schema through `/docker-entrypoint-initdb.d`. Extend the existing lab
driver with one observed PostgreSQL contract: the running server must report
18.6 and the public schema must contain exactly the seven pinned gpgsql tables.

**Tech Stack:** PostgreSQL 18.6, Docker Official Image `postgres`, OCI Image
Specification, Compose Specification, Podman, podman-compose, podman-py 5.8.0,
PowerDNS Authoritative 5.1.3 and 5.0.6, pytest, uv, ruff, ty, Go 1.27.0,
gopls 0.23.0, Terraform, OpenTofu, Terragrunt, Beads.

---

## Boundary and sources

This plan implements only `tfp-bqt.6.1`. It does not update PowerDNS,
SeaweedFS, Forgejo, workflow images, development tools, port bindings, or
container security policy. It does not perform or claim a persistent-data
major upgrade. The separate Python-to-Go automation migration remains
`tfp-bqt.12`; this change adds the minimum PostgreSQL observation to the
existing lab lifecycle owner rather than creating a second implementation.

At the planning baseline, `scripts/automation/e2e.py` and
`scripts/checks/commitlint.py` derived basename-only dev-container identities
and could not address the canonical hashed linked-worktree container. That
cross-worktree defect was isolated as Bead `tfp-bqt.13`, reviewed separately,
and squash-merged by PR #35 as
`2028f31173cc04ca4494a00dc399ae94295dcc8e`. This branch is rebased onto that
reviewed fix. No e2e or commit-hook automation change is part of the PostgreSQL
18 boundary.

The implementation uses these retrieved sources rather than remembered facts:

- PostgreSQL 18.6 release notes:
  <https://www.postgresql.org/docs/18/release-18-6.html>
- PostgreSQL 18 major migration notes:
  <https://www.postgresql.org/docs/18/release-18.html>
- Docker Official Image entrypoint and PostgreSQL 18 directory contract at the
  commit mapped by Official Images to 18.6/trixie:
  <https://github.com/docker-library/postgres/blob/e00e1bd34ec5c8a8e7ad89b273b3d42efaf6d5bc/docker-entrypoint.sh>
- Docker Official Image major-upgrade discussion:
  <https://github.com/docker-library/postgres/issues/37>
- Compose Specification tmpfs contract:
  <https://github.com/compose-spec/compose-spec/blob/bd6ccc6581199b0103837b7f1529f1ea875d7362/05-services.md#tmpfs>
- PowerDNS gpgsql schema at `auth-5.1.3`, read with `git show` from
  `/opt/projects/repositories/pdns-upstream` after verifying the tag OID:
  `modules/gpgsqlbackend/schema.pgsql.sql`
- Go 1.27 language specification and release notes:
  <https://go.dev/ref/spec> and <https://go.dev/doc/go1.27>

Live evidence captured before implementation:

- exhaustive `gh api graphql` pagination read all 692 `postgres/postgres`
  tags and resolved `REL_18_6` to commit
  `724edf9bde9d356724ad384a2e196edc3c9f80f7`;
- `skopeo inspect --raw docker://docker.io/library/postgres:18.6` hashed to
  `sha256:06cad38a5d9f5d24b4d83d86def30795d5e4b757fedbf5281172b576dedcd941`;
- the manifest is an OCI index with linux/amd64 child
  `sha256:cd78ca58eb75f929698e117a589488ccb2bd45107247fe02400b50ff6c418324`
  and linux/arm64/v8 child
  `sha256:772ab753f714afefc07b096906b4961e2bb576938c7d007beaa9b62d80680c48`;
- the selected amd64 image declares `PG_VERSION=18.6-1.pgdg13+2`,
  `PGDATA=/var/lib/postgresql/18/docker`, and anonymous image volume
  `/var/lib/postgresql`;
- Docker Official Images maps 18.6/trixie to source commit
  `e00e1bd34ec5c8a8e7ad89b273b3d42efaf6d5bc`;
- GraphQL peels PowerDNS tag object
  `e1faaa3ee0e269d7a193338a796fda9a57f4647f` to commit
  `30653d6b8e997d17d9f9ef834e179870b810931a`;
- both baseline Compose combinations render, `task py` passes 201 tests,
  `task lint:pins` verifies 29 references, and gopls checks 81 Go files.

## Files and responsibilities

- Create `test/scripts/test_lab.py`: unit and mutation contracts for the exact
  image pin and PostgreSQL observation/failure classification.
- Modify `scripts/automation/lab.py`: observe PostgreSQL version and exact
  public-table inventory through podman-py `Container.exec_run`; include it in
  `status` and `verify`.
- Modify `Taskfile.yml`: run every host-side lab driver command through the
  repository's locked uv environment rather than the host Python environment.
- Modify `test/scripts/test_taskfile.py`: require the four lab tasks to keep
  the locked-driver command boundary.
- Modify `deployments/compose/compose.lab.yml`: replace only the PostgreSQL
  image reference and overlay the image-declared data volume with bounded
  tmpfs.
- Create `docs/audit/AUDIT-03-postgresql-18-lab.md`: retain exact source,
  registry, render, lifecycle, SQL, acceptance, e2e, residue, and review
  evidence.
- Modify `AGENTS.md`, `README.md`, `docs/development.md`: update the active lab
  version contract and explain disposable PostgreSQL 18 initialization.
- Modify `docs/plan.md`: keep P10-06 `[ ]` through Task 0; set it to `[~]` only
  after Task 1 records the required RED; keep it `[~]` through final sequential
  reviews; then set it to `[x]` in the closure commit. P10-14 remains `[x]` as
  merged prerequisite history.
- Modify `CHANGELOG.md`: add the PostgreSQL 18 fixture migration under the
  current `[Unreleased]` `Changed` section; never rewrite a released section.
- Modify `.cspell.json`: admit only the PostgreSQL and container CLI terms used
  by the new source and evidence documentation.
- Modify this plan: check steps only after their exact evidence exists.

### Task 0: Satisfy the isolated e2e worktree prerequisite

**Files:**

- Read: `scripts/automation/e2e.py`
- Read: `scripts/checks/commitlint.py`
- Read: `scripts/dev-suffix.sh`
- Update: Bead `tfp-bqt.6.1`

- [x] **Step 1: Require the separate e2e identity Bead and pull request**

  Record blocking Bead `tfp-bqt.13` and its merged PR in `tfp-bqt.6.1`. The fix
  must have its own reviewed plan, RED proving the basename/hash mismatch in
  both e2e and commitlint, GREEN proving both consumers resolve Task's canonical
  helper, full gates, and immutable sequential reviews. Do not copy its
  implementation into this branch.

- [x] **Step 2: Rebase this unpublished branch onto the merged prerequisite**

  Leave the three PostgreSQL plan/status files uncommitted while the isolated
  prerequisite is implemented in its own worktree. After `tfp-bqt.13` merges,
  save exactly the tracked and untracked paths with:

  ```sh
  git stash push --include-untracked \
    -m postgresql-18-plan-before-e2e-prerequisite-rebase -- \
    .cspell.json docs/plan.md \
    docs/superpowers/plans/2026-08-24-postgresql-18-lab-plan.md
  stash_oid=$(git rev-parse --verify 'refs/stash^{commit}')
  expected_paths=$(printf '%s\n' \
    .cspell.json docs/plan.md \
    docs/superpowers/plans/2026-08-24-postgresql-18-lab-plan.md |
    LC_ALL=C sort)
  actual_paths=$(git stash show --include-untracked --name-only "$stash_oid" |
    LC_ALL=C sort)
  test "$actual_paths" = "$expected_paths"
  test -z "$(git status --short)"
  ```

  Record the immutable `stash_oid` in `tfp-bqt.6.1`; retain that stash commit as
  a recovery object rather than dropping it. Fetch `origin/main`, verify the
  prerequisite squash commit and required checks with `gh api graphql`, and
  rebase the branch onto that exact base. Immediately re-read complete
  `AGENTS.md` and `README.md` from the new base before restoring any file.

  Apply the same immutable object with `git stash apply --index "$stash_oid"`,
  verify the exact three-path set and their base-to-working-tree diff, and
  re-run the plan review and documentation gate. Then stage only those three
  reviewed files and commit them with hooks as
  `docs(docs): plan the postgresql 18 lab migration`. Re-read complete
  `AGENTS.md` and `README.md` again only if the apply reports a conflict. Any
  stash, rebase, apply, or content mismatch stops the plan; never bypass hooks
  or resolve a conflict by assumption.

  Execution evidence: PR #35 passed 11/11 required checks and was
  squash-merged as `2028f31173cc04ca4494a00dc399ae94295dcc8e`. The exact
  three-path recovery object is
  `2489ed02fe781f387e957c1c2131ff3ef643e283`. Rebase reached the squash with a
  clean worktree. Applying the retained object restored all three paths and
  reported one content conflict in `docs/plan.md`: the snapshot carried the
  former active P10-14 state while merged `main` carried its completed state.
  Per the fail-closed rule, no resolution was attempted until the user
  explicitly approved retaining merged P10-02/P10-03, recording P10-14 as PR
  #35 merged, and keeping P10-06 `[ ]` with Task 0 active. The restored path set
  again contains exactly the three expected files; the stash remains retained.

  The first renewed SPEC review rejected the restored plan with four Important
  gaps. The durable Bead now has a superseding post-resolution note; the e2e
  lifecycle now proves a clean preflight, exact ownership and remote/local
  residue; the exact PostgreSQL image now has a pinned Trivy image scan backed
  by current Context7 CLI/reporting documentation; and Bead closure now follows
  required checks plus a GraphQL-verified squash merge. The corrected plan must
  receive fresh sequential SPEC then QUALITY approval before this step is
  checked or committed.

  That renewed SPEC approved, but the subsequent QUALITY review rejected the
  plan because its schema oracle omitted `tsigkeys_pkey`, fixed-name lab
  teardown lacked the same ownership guard as e2e, and e2e ran only on Auth
  5.1 despite the two-branch goal. The oracle now requires all 19 indexes,
  every lab teardown revalidates its complete captured identity, and Steps 2–5
  repeat on Auth 5.0. This change invalidates the earlier SPEC approval; both
  reviews restart sequentially on the corrected plan.

  The next SPEC approved, but QUALITY found that `Container.exec_run` had no
  client deadline, late executable edits did not repeat the complete e2e/SQL
  runtime matrix, and host `task lab:*` actually imported podman-py 5.7.0 while
  the locked contract is 5.8.0. The plan now requires
  `PodmanClient(timeout=LOCAL)` plus a timeout oracle, moves all four lab tasks
  to `uv run --locked`, records the executing version, and repeats all of Task
  4 after any late executable edit. Both reviews restart again.

  The following SPEC approved, but QUALITY found the existing
  `container_states()` Podman client still unbounded and the Taskfile oracle
  vulnerable to `status` skips or ignored command failures. The corrected
  contract passes `timeout=LOCAL` to every `PodmanClient` construction in
  `lab.py`, tests both call paths, and requires each lab task to have exactly
  one failure-propagating scalar command with no skip/cache/wrapper controls.
  Both reviews restart again.

  The final corrected plan received fresh sequential SPEC then QUALITY
  approval with zero Critical or Important findings. No Git, Beads, or Podman
  state changed between those two approvals. Both reviewers confirmed exact
  base `2028f31173cc04ca4494a00dc399ae94295dcc8e`, the retained stash and
  three-path restore, all previously rejected oracles, the locked runtime,
  complete dual-Auth lifecycle, image scan, and post-merge Bead ordering.

- [x] **Step 3: Re-establish the baseline**

  Recreate only this worktree's dev container if its image/runtime identity
  changed. Run both Compose renders, `task py`, `task lint:pins`, gopls over the
  non-empty Go file set, and the e2e identity focused test delivered by the
  prerequisite. Update exact baseline counts in this plan and Beads before
  Task 1.

  Baseline evidence on the merged prerequisite: automatic suffix
  `-postgresql-18-17d2b9851be8` resolves to running container
  `ffb1019c94402c986ac8c54075c4d5a4252622d5f8eac8d47c1ed6ab4dbac2b4`
  with exact Compose project label and canonical worktree bind. It reports Go
  1.27.0, `GOTOOLCHAIN=local`, `GOCACHE=/tmp/go-cache`, and
  `GOMODCACHE=/go/pkg/mod`; no recreation was required. Both Auth 5.1 and 5.0
  Compose renders completed with 101 lines and the expected baseline
  PostgreSQL 17 pin. The host locked environment reports podman-py 5.8.0;
  `task py` passed 201 tests with ruff/format/ty clean; the linked-worktree
  identity focus passed 23 tests; `lint:pins` verified 29 references; and gopls
  v0.23.0 checked a non-empty set of 81 Go files without diagnostics.

### Task 1: Lock the executable contract before changing the image

**Files:**

- Create: `test/scripts/test_lab.py`
- Read: `deployments/compose/compose.lab.yml`
- Read: `scripts/automation/lab.py`

- [ ] **Step 1: Add the failing exact-image test**

  Read `compose.lab.yml`, isolate the `postgres` service block, and assert its
  image is exactly:

  ```text
  docker.io/library/postgres:18.6@sha256:06cad38a5d9f5d24b4d83d86def30795d5e4b757fedbf5281172b576dedcd941
  ```

  The test must also reject these four bounded mutations: a 17 tag, a floating
  18.6 tag, the right digest with the wrong tag, and an unqualified registry.

- [ ] **Step 2: Add the failing topology contract**

  Assert the PostgreSQL service preserves `POSTGRES_USER`,
  `POSTGRES_PASSWORD`, `POSTGRES_DB`, `15432:5432`, the read-only SELinux-aware
  schema bind, and `pg_isready -U pdns -d pdns`. Assert Compose declares no
  PostgreSQL data volume, declares exactly one service tmpfs at
  `/var/lib/postgresql:rw,nosuid,nodev,noexec,size=512m`, and retains
  `auth-pg` `condition: service_healthy`. The tmpfs intentionally overlays the
  anonymous volume destination declared by the image config.

- [ ] **Step 3: Run RED**

  Run:

  ```sh
  podman exec -w /app \
    "terraform-provider-powerdns-dev$(scripts/dev-suffix.sh)" \
    uv run --locked pytest test/scripts/test_lab.py -vv
  ```

  Expected: exactly two assertions fail: the image assertion reports the
  current `postgres:17` pin and the storage assertion reports the missing
  `/var/lib/postgresql` tmpfs. All preserved-topology assertions pass. A
  collection error or any third failure is not the required RED.

- [ ] **Step 4: Prove the test rejects bounded mutations**

  In-memory mutations must reject the four invalid references from Step 1 and
  a synthetic Compose PostgreSQL data volume at `/var/lib/postgresql`. Do not
  edit the production Compose file for mutation proof. Mutations must also
  reject a missing tmpfs, a wrong destination, and a persistent `volume` type.

- [ ] **Step 5: Record RED in Beads and AUDIT-03**

  Append the exact command, tool versions, collected-test count, failure count,
  and expected mismatch to `tfp-bqt.6.1`. Set P10-06 to `[~]` in the same
  working tree before any production edit.

### Task 2: Make lab verification observe PostgreSQL itself

**Files:**

- Modify: `test/scripts/test_lab.py`
- Modify: `test/scripts/test_taskfile.py`
- Modify: `scripts/automation/lab.py`
- Modify: `Taskfile.yml`

- [ ] **Step 1: Add failing PostgreSQL observation tests**

  Specify a frozen `PostgresObservation` with `version_num`, display `version`,
  and sorted `tables`. Test exact success, wrong numeric version, missing table,
  extra table, malformed JSON, non-zero `psql` exit, absent container, and
  Podman API failure. Require every `PodmanClient` construction in `lab.py`,
  including the existing `container_states()` path and the new observation
  path, to receive `timeout=LOCAL`. A behavioural fake factory must exercise
  both callers and record the exact constructor keyword; removing the timeout
  from either call must fail. Simulate the requests timeout path (an `OSError`
  subclass) and require an unavailable observation rather than a hang or
  traceback.

  Add a fail-closed Taskfile structure contract for each of `lab:up`,
  `lab:down`, `lab:status`, and `lab:verify`. Each task may contain only its
  description and a `cmds` list with exactly one scalar command, and that
  command must be exactly `uv run --locked python -m scripts.automation.lab
  ...`. Reject command mappings/wrappers, extra commands, `status`,
  sources/generates caching, preconditions, `ignore_error`, or any other
  control that can skip the driver or suppress its exit status. Mutations back
  to host `python3`, unlocked uv, a missing module invocation, `status:
  ["true"]`, command-level `ignore_error: true`, an extra command, and a
  wrapped command must all fail. Expected tables are:

  ```python
  (
      "comments",
      "cryptokeys",
      "domainmetadata",
      "domains",
      "records",
      "supermasters",
      "tsigkeys",
  )
  ```

- [ ] **Step 2: Run the PostgreSQL observation RED and inspect the reason**

  Run the new observation and Taskfile tests. Expected: import or attribute
  failures because the observation contract does not exist, plus exact
  Taskfile assertion failures because the four tasks still use host `python3`.
  Fix test mistakes until the failures are semantic.

- [ ] **Step 3: Implement the minimum observation**

  Use Context7-confirmed `PodmanClient(timeout=LOCAL)` for every client in
  `lab.py` and podman-py `Container.exec_run` with an argv list and
  `psql --no-psqlrc --tuples-only --no-align --set ON_ERROR_STOP=1 --username
  pdns --dbname pdns`. Execute one query returning JSON with
  `current_setting('server_version_num')::integer`,
  `current_setting('server_version')`, and the sorted `pg_tables` names for
  schema `public`. Decode bytes explicitly, require exit status zero, validate
  the JSON boundary, and turn Podman/API/query failures into an unavailable
  observation rather than a traceback. Compare the numeric identity to
  `180006`; retain the human-readable value for status output.

  Change only the four Taskfile lab commands to the exact locked uv form from
  Step 1. Before any live lab run, execute `uv run --locked python -c 'import
  podman; print(podman.__version__)'` through the same host-side command path
  and require `5.8.0`; retain this runtime identity in AUDIT-03 and the Bead.

- [ ] **Step 4: Integrate with status and verify**

  `lab status` prints the PostgreSQL container state, observed version, and
  table count. `lab verify` fails unless `server_version_num` is exactly
  `180006`, the display version starts with `18.6`, and the table tuple is
  exactly the seven-table contract, then prints one PostgreSQL `OK` line beside
  the four PowerDNS services.

- [ ] **Step 5: Run focused GREEN**

  Run the new lab module and focused Taskfile contracts, then `task py`.
  Expected: all collected tests pass; ruff, format check, and ty report no
  findings.

### Task 3: Apply the isolated image migration and document the active contract

**Files:**

- Modify: `deployments/compose/compose.lab.yml`
- Modify: `AGENTS.md`
- Modify: `README.md`
- Modify: `docs/development.md`
- Modify: `docs/plan.md`
- Modify: `CHANGELOG.md`
- Modify: `.cspell.json`
- Create: `docs/audit/AUDIT-03-postgresql-18-lab.md`

- [ ] **Step 1: Replace the PostgreSQL image and make its storage explicitly ephemeral**

  Use the verified 18.6 tag plus OCI index digest. Add the exact bounded tmpfs
  from Task 1 at the image's `/var/lib/postgresql` volume destination. Do not
  set `PGDATA`, add a Compose data volume, change the healthcheck or credentials,
  or bump any unrelated image. The official image's `Config.Volumes` remains
  upstream-owned while the runtime mount is explicitly ephemeral.

- [ ] **Step 2: Run focused GREEN and both renders**

  Run the new test module, `task lint:pins`, and:

  ```sh
  podman-compose -f deployments/compose/compose.lab.yml config
  podman-compose -f deployments/compose/compose.lab.yml \
    -f deployments/compose/compose.lab-auth-50.yml config
  ```

  Expected: the only changed rendered image is PostgreSQL; both renders exit
  zero; the pin gate resolves all references.

- [ ] **Step 3: Re-run source and OCI identity checks**

  Hash the raw tag manifest again, inspect media types and platform children,
  and verify the Official Images metadata maps 18.6/trixie to Docker source
  commit `e00e1bd34ec5c8a8e7ad89b273b3d42efaf6d5bc`. Resolve the peeled
  `PowerDNS/pdns` `auth-5.1.3` tag OID with exhaustive `gh api graphql`
  pagination, then compare `test/lab/schema.pgsql.sql` with:

  ```sh
  cmp test/lab/schema.pgsql.sql \
    <(git -C /opt/projects/repositories/pdns-upstream \
      show auth-5.1.3:modules/gpgsqlbackend/schema.pgsql.sql)
  ```

  Record exact commands and results in AUDIT-03. Do not compare against the
  mutable checkout worktree.

- [ ] **Step 4: Update current documentation**

  Update active PostgreSQL references to 18.6 and explain that the fixture
  starts a clean cluster. Do not rewrite the released changelog history or the
  historical context of ADR 0005. Add a current `[Unreleased]` `Changed`
  entry, P10-06 `[~]`, source links, rollback, and evidence status.

- [ ] **Step 5: Run documentation and Go navigation checks**

  Run `task docs:lint` and `git diff --check`. Then run the following inside the
  canonical current-worktree dev container, after proving `rg --files -g
  '*.go'` returns a non-zero count:

  ```sh
  container="terraform-provider-powerdns-dev$(scripts/dev-suffix.sh)"
  podman exec -w /app "$container" go mod tidy -diff
  mapfile -t gofiles < <(rg --files -g '*.go')
  test "${#gofiles[@]}" -gt 0
  podman exec -w /app "$container" gopls check "${gofiles[@]}"
  ```

  No Go source should change; the Go 1.27 specification remains a checked
  project source, not an excuse to mix language work into this PR.

### Task 4: Prove two clean PostgreSQL 18 bootstraps and provider compatibility

**Files:**

- Modify: `docs/audit/AUDIT-03-postgresql-18-lab.md`
- Modify: this plan
- Update: Bead `tfp-bqt.6.1`

- [ ] **Step 1: Establish exact disposable preflight**

  Record disk space and prove the five `pdns-lab-*` containers, the three lab
  named volumes, and the lab network/pod are absent. Snapshot the volume list so
  the run can prove PostgreSQL created no anonymous volume. Inspect existing
  unrelated Podman objects read-only; never use broad prune or removal.

  In the same preflight, require both fixed e2e containers
  (`pdns-e2e-s3`, `pdns-e2e-forgejo`), both project volumes
  (`terraform-provider-powerdns-e2e_s3-data` and
  `terraform-provider-powerdns-e2e_forgejo-data`), and every pod or network
  labelled for Compose project `terraform-provider-powerdns-e2e` to be absent.
  Require these generated paths to be absent before the run:

  ```text
  test/e2e/.home
  test/e2e/.mirror
  test/e2e/.released
  test/e2e/.tls
  test/e2e/.token
  test/e2e/live*/.terraform.lock.hcl
  test/e2e/live*/.terragrunt-cache
  ```

  A dirty preflight is a blocker, not permission to reuse or remove another
  run's fixture. Record exact IDs, names, labels, mount paths, and creation
  times for the unrelated preflight inventory so the post-run comparison is
  executable.

- [ ] **Step 2: Run the Authoritative 5.1 bootstrap**

  Run `task lab:up AUTH=5.1`, `task lab:status AUTH=5.1`, and
  `task lab:verify AUTH=5.1`. Inspect the image config and require its exact
  `Config.Volumes["/var/lib/postgresql"]` declaration. Inspect `pdns-lab-pg`
  and require exactly one `tmpfs` mount at `/var/lib/postgresql`, no volume
  mount at that destination, `/var/lib/postgresql/data`, or the declared
  `PGDATA=/var/lib/postgresql/18/docker` subdirectory, and no new anonymous
  volume in the snapshot delta. Inspect logs from container creation through
  `database system is ready to accept connections`; fail on SQL errors or
  warnings attributable to `10-schema.sql`.

  After all five services are healthy, capture the immutable container IDs,
  both Compose project labels, exact image IDs, and canonical bind sources for
  `pdns-lab-pg`, `pdns-lab-auth-pg`, `pdns-lab-auth-lmdb`, `pdns-lab-rec`, and
  `pdns-lab-dnsdist`. Capture immutable IDs plus project labels for every lab
  pod/network. For each of the three named lab volumes capture its name,
  `CreatedAt`, labels, and mount path. Require every object to belong to the
  exact Compose project `terraform-provider-powerdns-lab`; the captured set is
  the teardown authorization boundary for this branch run.

- [ ] **Step 3: Run direct SQL and full 5.1 verification**

  Through `psql --username pdns --dbname pdns --set ON_ERROR_STOP=1` inside
  `pdns-lab-pg`, assert `server_version_num=180006` and the exact seven tables
  from Task 2. Query `pg_catalog.pg_indexes` and require these 19 sorted names:

  ```text
  catalog_idx
  comments_domain_id_idx
  comments_name_type_idx
  comments_order_idx
  comments_pkey
  cryptokeys_pkey
  domain_id
  domainidindex
  domainidmetaindex
  domainmetadata_pkey
  domains_pkey
  name_index
  namealgoindex
  nametype_index
  rec_name_index
  recordorder
  records_pkey
  supermasters_pkey
  tsigkeys_pkey
  ```

  Query `pg_catalog.pg_constraint` joined to `pg_catalog.pg_class` and require
  these four `(table, constraint, referenced table, delete action)` rows:

  ```text
  comments|domain_exists|domains|c
  cryptokeys|cryptokeys_domain_id_fkey|domains|c
  domainmetadata|domainmetadata_domain_id_fkey|domains|c
  records|domain_exists|domains|c
  ```

  Before tests, and again after `task verify AUTH=5.1`, run one `UNION ALL`
  count query and require zero rows in each of `comments`, `cryptokeys`,
  `domainmetadata`, `domains`, `records`, `supermasters`, and `tsigkeys`.

- [ ] **Step 4: Run the consumer e2e lifecycle on 5.1**

  While the clean 5.1 lab is live, first query all 12 names from
  `scripts.automation.e2e.MANAGED_ZONES` through their declared API and require
  HTTP 404 for every one; re-run the seven-table SQL query and require zero
  rows. Reconfirm that every fixed e2e object and generated path from Step 1 is
  absent. If any name, state object, container, volume, pod, network, or local
  generated path exists, stop for a separate scoped cleanup decision.

  Run `task e2e:up`, `task e2e:status`, and `task e2e`. Capture Terraform and
  OpenTofu scenario counts. Before teardown, enumerate the exact sorted S3
  object-key set under bucket `e2e-state`, and capture immutable IDs plus both
  Compose project labels for `pdns-e2e-s3`, `pdns-e2e-forgejo`, and any
  project pod/network. For each of the two named volumes capture its name,
  `CreatedAt`, labels, and mount path because Podman volumes have no immutable
  ID. Require the object set to be exactly the state keys declared by the live
  Terragrunt units and require every captured runtime object to belong to
  Compose project `terraform-provider-powerdns-e2e`.

  Immediately before `task e2e:down`, re-read and compare every captured
  identity and label. Stop on any change; never tear down by fixed name alone.
  Then run exact `task e2e:down`. While the lab APIs remain live, require all
  12 managed zones to return 404, the seven-table SQL residue query to return
  zero for every table, the two e2e containers and volumes to be absent, and
  every captured project pod/network to be absent. The removal of the exact
  captured S3 volume is the remote-state deletion proof; do not call an
  unavailable endpoint proof of deletion.

  Inventory the generated local paths from Step 1 after the run. Before their
  exact removal, require each path to resolve below this worktree's canonical
  `test/e2e` directory, reject symlinks and any path outside that root, print
  the complete allowlist, and obtain explicit approval for that scoped
  deletion. After removal, require the entire Step 1 generated-path set to be
  absent and compare the unrelated Podman inventory byte-for-byte with the
  preflight snapshot.

- [ ] **Step 5: Tear down 5.1 exactly**

  Immediately before teardown, re-read every lab identity, project label,
  image ID, canonical bind source, volume `CreatedAt`/labels/mount path, and
  pod/network ID captured in Step 2. Require byte-identical equality and stop
  if any object is absent, replaced, relabelled, or remounted. Only then run
  `task lab:down AUTH=5.1`; prove all captured lab containers, the three named
  lab volumes, and captured lab network/pod are absent. Compare the volume
  snapshot and prove no anonymous PostgreSQL volume was created; preserve every
  unrelated object.

- [ ] **Step 6: Repeat from empty state on Authoritative 5.0**

  Repeat Steps 2 through 5 with `AUTH=5.0`, including a second clean e2e
  preflight, scenario run, ownership-guarded e2e teardown, exact local fixture
  cleanup approval, SQL/zone/state residue proof, lab identity capture, and
  guarded lab teardown. This second clean bootstrap proves that the PostgreSQL
  18 init path is repeatable and that both supported Authoritative branches
  use the same database and consumer contract.

### Task 5: Run complete gates and create the evidence candidate

**Files:**

- Modify: `docs/audit/AUDIT-03-postgresql-18-lab.md`
- Modify: this plan
- Update: Bead `tfp-bqt.6.1`

- [ ] **Step 1: Run the non-lab aggregate from a clean worktree state**

  Run `task all`, `task osv-scan`, and `task release:dryrun`. Record tool input
  counts and exact results; a zero-finding scanner is not evidence until its
  package/file input is shown.

- [ ] **Step 2: Scan and re-inspect the final PostgreSQL image**

  Scan the exact executable target reference, not the repository filesystem,
  with the repository's independently pinned scanner image:

  ```text
  docker.io/aquasec/trivy:0.72.0@sha256:cffe3f5161a47a6823fbd23d985795b3ed72a4c806da4c4df16266c02accdd6f
  ```

  Use `trivy image` against
  `docker.io/library/postgres:18.6@sha256:06cad38a5d9f5d24b4d83d86def30795d5e4b757fedbf5281172b576dedcd941`
  with `--scanners vuln,secret,misconfig,license`, `--list-all-pkgs`,
  `--license-full`, and JSON output. Keep its cache and report only in an exact
  `mktemp -d` directory mounted into the ephemeral, exact-name scanner
  container. Capture `trivy version --format json` from the same cache so the
  scanner version and vulnerability database `UpdatedAt`, `NextUpdate`, and
  `DownloadedAt` are retained with the evidence; validate the temporary path
  before removing it.

  Record report creation time, artifact name/type, image ID and repo digest,
  OS, reported package-entry count, and finding counts grouped by scanner and
  severity. Any secret or HIGH/CRITICAL vulnerability or misconfiguration is
  blocking until every finding is tied to its identifier, installed/fixed
  version and an explicit remediation or user-approved exception. Record the
  full licence inventory; an unknown or repository-incompatible licence is
  likewise blocking. A zero-finding result without non-zero package inputs and
  database timestamps is not acceptable evidence.

  Record the exact local image ID, repo digest, OCI media types, platform,
  labels, layers, size, declared `PGDATA`, declared image
  `/var/lib/postgresql` volume, runtime tmpfs override, and package version. Do
  not claim signatures, provenance, or referrers that were not observed.

- [ ] **Step 3: Re-run final Auth 5.1 and 5.0 matrices if any executable file changed after Task 4**

  Any code, test, Compose, Taskfile, lockfile, or workflow edit after Task 4
  invalidates its runtime evidence. Repeat all of Task 4 Steps 1–6 before
  proceeding: both clean bootstraps, direct version/table/index/constraint/row
  oracles, both full e2e lifecycles, exact ownership guards, scoped local
  fixture cleanup, and every runtime/zone/state/volume residue proof.

- [ ] **Step 4: Finalize evidence without closing the Bead**

  Append exact outputs to AUDIT-03 and the Bead, keep `tfp-bqt.6.1`
  `IN_PROGRESS`, keep P10-06 `[~]`, and check only plan items whose evidence is
  present. Run `bd lint`, `bd dep cycles`, documentation gates, both diff
  checks, and `git status --short`. Stage only the intended candidate files,
  then self-review both `git diff origin/main` and
  `git diff --cached origin/main`; neither may omit an implementation change.

- [ ] **Step 5: Commit the evidence candidate with hooks enabled**

  Use the repository-valid subject:

  ```text
  build(lab): upgrade the disposable database to postgresql 18
  ```

  The body records why this is a clean disposable bootstrap, the exact OCI
  identity, both Auth acceptance counts, e2e counts, and rollback. Do not
  bypass hooks and do not close the Bead yet. After the commit, verify the
  immutable `origin/main...HEAD` range contains exactly the reviewed candidate
  and no unstaged or untracked file remains.

### Task 6: Sequential review, closure, and pull request

**Files:**

- Modify only after candidate approval: `docs/plan.md`
- Modify only after candidate approval: `docs/audit/AUDIT-03-postgresql-18-lab.md`
- Modify only after candidate approval: this plan
- Update only after final approval: Bead `tfp-bqt.6.1`

- [ ] **Step 1: Obtain SPEC review of the exact candidate HEAD**

  Reviewer checks every requirement, source, OCI identity, topology invariant,
  PostgreSQL observation, clean-bootstrap claim, Auth matrix, e2e evidence,
  residue proof, scope boundary, and documentation. No concurrent quality
  review and no Git mutation during review.

- [ ] **Step 2: Obtain QUALITY review of the same immutable HEAD**

  Start only after SPEC approves. Reviewer checks failure handling, parser and
  mutation strength, Podman API behavior, false-green paths, security, OCI and
  Compose correctness, and test quality. Any Critical or Important finding
  resets P10-06 to `[~]`, repeats affected focused tests and Tasks 4–5, creates
  a new candidate, and restarts SPEC then QUALITY.

- [ ] **Step 3: Create a docs-only closure commit**

  After both candidate reviews approve, record exact reviewed HEADs, set
  P10-06 `[x]`, check Tasks 5–6 through this step, keep the Bead open, run
  `task docs:lint` and diff checks, and commit with hooks:

  ```text
  docs(docs): complete the postgresql 18 lab migration
  ```

  This commit may set P10-06 `[x]` because it becomes public only if the pull
  request merges. It must not close `tfp-bqt.6.1`; the durable task remains
  `IN_PROGRESS` through reviews, push, required checks, and merge.

- [ ] **Step 4: Review the exact closure HEAD sequentially**

  Run final SPEC then final QUALITY against the same immutable closure HEAD.
  No Git or Beads mutation occurs between those reviews. A blocking finding
  reopens the full evidence loop rather than being patched after the gates.

- [ ] **Step 5: Push and open the pull request without closing Beads**

  Reconfirm that `origin` is exactly
  `https://github.com/ioplane/terraform-provider-powerdns.git` and that
  `gh repo view` reports `ioplane/terraform-provider-powerdns` as a non-fork
  with default branch `main`. Push the branch only to that `origin`. Open a PR
  titled with the candidate Conventional Commit subject, describe the isolated
  boundary, exact OCI digest, no-persistent-volume reasoning, Auth/e2e
  evidence, and rollback. Re-query PR fields and checks with
  `gh api graphql`; do not merge until required checks and review policy are
  green. Keep `tfp-bqt.6.1` `IN_PROGRESS`.

- [ ] **Step 6: Verify required checks and merge into the unchanged base**

  Exhaustively paginate the pull request's commits, top-level comments,
  reviews, review threads and replies with `gh api graphql`; reconcile all
  counts and address every request. Re-query the branch-protection rule and
  require every exact required context to be successful. Immediately before
  merge, fetch `origin/main` and require its OID to equal the candidate's
  recorded base; any base drift restarts rebase, Tasks 4–5, candidate and
  closure reviews rather than mutating the reviewed range. Obtain the required
  merge authorization, squash-merge the pull request, and use GraphQL to prove
  the resulting squash commit is the default branch head and the pull request
  is `MERGED`.

- [ ] **Step 7: Close Beads only after the verified merge**

  Append the exact PR number, squash OID, reconciled discussion/check counts,
  and default-branch OID to `tfp-bqt.6.1`; only then close it. Leave parent
  `tfp-bqt.6` open for `.6.2`. Run `bd show`, `bd lint`, `bd dep cycles`, and a
  read-only default-branch status/log check. The merged implementation plan
  cannot truthfully pre-check its own future push and merge; GitHub and this
  final Bead note are the durable evidence for Steps 5–7.

## Rollback

Rollback is one separate revert pull request restoring the prior
`postgres:17@sha256:a426e44bac0b759c95894d68e1a0ac03ecc20b619f498a91aae373bf06d8508d`
reference and the matching active documentation/verification constant. The
rollback tmpfs destination moves to PostgreSQL 17's independently inspected
`PGDATA=/var/lib/postgresql/data`, so it also creates no anonymous data volume.
Rollback recreates a clean 17 cluster from the same pinned PowerDNS schema. It
must never attach PostgreSQL 18 data to PostgreSQL 17 or claim that reverting
YAML downgrades stored data.
