# Dev-container identity consumers implementation plan

> **For agentic workers:** REQUIRED: Use
> `superpowers:subagent-driven-development` when independent reviewers are
> available, otherwise `superpowers:executing-plans`. Follow TDD and keep the
> Bead and `docs/plan.md` current with the implementation.

**Goal:** Make the e2e driver and both container-using local hooks address the
same canonical hashed development container that Task already creates for a
linked worktree.

**Architecture:** `scripts/dev-suffix.sh` remains the only implementation of
worktree identity. A small typed Python adapter executes that helper through
the existing bounded `scripts.automation.run.run` boundary. The two
basename-only Python consumers call the adapter. The Go-format hook delegates
to existing `task fmt`, whose Task variables already use the canonical helper.
The broad Python-to-Go automation migration remains `tfp-bqt.12`; this
prerequisite neither duplicates the hash algorithm in Python nor changes
Compose, PowerDNS, PostgreSQL, or Go code.

**Tech stack:** POSIX shell, Python 3.12, uv, ruff 0.16.0, ty 0.0.64, pytest
9.1.1, Podman, podman-compose, Go 1.27.0 and gopls 0.23.0 for repository
baseline validation, Beads, GitHub GraphQL.

---

## Boundary and verified sources

This plan implements only `tfp-bqt.13`, public control P10-14. PostgreSQL 18
Bead `tfp-bqt.6.1` depends on it and stays blocked until this pull request is
squash-merged. No PostgreSQL plan/status file from the sibling blocked worktree
is copied into this branch.

Sources used before implementation:

- complete `AGENTS.md`, `README.md`, Python tooling, naming, changelog, commit,
  and methodology standards from base
  `e65b4c78455ab0659b5b462b7fd4b507a207186a`;
- Context7 Python documentation: argv-list execution, `cwd`, captured text
  output, and `check=True` failure semantics;
- Go 1.27 specification and release notes at <https://go.dev/ref/spec> and
  <https://go.dev/doc/go1.27>; no Go language change is in scope;
- `scripts/dev-suffix.sh`, whose successful, malformed-hash, failed-hash,
  equal-basename, subdirectory, and main-checkout contracts are already in
  `test/scripts/test_taskfile.py`;
- exhaustive source search showing two stale basename consumers and one direct
  Compose invocation that bypasses identity:
  `scripts/automation/e2e.py::dev_container` and
  `scripts/checks/commitlint.py::dev_suffix`, plus the Go-format entry in
  `.pre-commit-config.yaml`;
- `gh api graphql` and local Git proving `origin/main` and this worktree start
  at the same exact base.

The mathematical contract is composition, not a second hash implementation.
For canonical suffix function `H`, container prefix `C`, and roots `x` and `y`,
`H(x) != H(y)` implies `C || H(x) != C || H(y)` by left-cancellation in the
free monoid of strings. Tests therefore prove `H` distinguishes equal-basename
roots, both Python consumers compose with that exact `H`, and the hook calls
the Task command that already consumes `H`.

## Files and responsibilities

- Create `scripts/automation/dev_identity.py`: the one Python adapter to the
  canonical shell helper.
- Modify `scripts/automation/e2e.py`: accept a repository root and compose the
  development container name from the adapter result.
- Modify `scripts/checks/commitlint.py`: use the same adapter rather than a
  basename calculation.
- Modify `.pre-commit-config.yaml`: delegate Go formatting to canonical
  `task fmt` instead of calling Compose directly without the suffix.
- Create `test/scripts/test_dev_identity.py`: adapter invocation, real linked
  worktree, collision, composition, and failure contracts.
- Modify `test/scripts/test_worktree_and_commitlint.py`: remove obsolete
  basename expectations and retain the hook-facing contract.
- Modify `docs/plan.md`: own P10-14 and keep P10-06 blocked until merge.
- Modify `CHANGELOG.md`: add one current `[Unreleased]` `Fixed` entry.
- Modify this plan and Bead `tfp-bqt.13`: record executable evidence.

### Task 1: Establish the exact baseline and RED

**Files:**

- Read: `scripts/dev-suffix.sh`
- Read: `scripts/automation/e2e.py`
- Read: `scripts/checks/commitlint.py`
- Create: `test/scripts/test_dev_identity.py`
- Modify: `test/scripts/test_worktree_and_commitlint.py`

- [x] **Step 1: Start only this worktree's development container**

  Prove the exact project/container/image/volume names derived from
  `scripts/dev-suffix.sh` are absent or owned by this canonical worktree path.
  Run `task up`, `task versions`, and inspect the running container. Require
  Go 1.27.0, gopls 0.23.0, the exact project label, and `/app` bound from this
  worktree. Do not recreate, replace, or remove another worktree's object.

- [x] **Step 2: Re-establish source and tool baselines**

  Run `task py`, `task lint:shell`, and `task lint:pins`. On the host, require
  `rg --files -g '*.go'` to return a non-empty exact path set, then pass that
  set as arguments to `gopls check` inside the current worktree's
  Go 1.27 container. The image deliberately has no `rg`; do not turn file
  discovery into an undocumented image dependency. Query `origin/main` and
  branch protection with paginated `gh api graphql`; record exact base and
  required checks in the Bead.

- [x] **Step 3: Add three direct mismatch tests first**

  Create a self-contained temporary Git repository and real linked worktree
  inside pytest's temporary directory, including the committed canonical helper.
  Patch current `e2e.REPO_ROOT` to that linked root and call the existing
  zero-argument `e2e.dev_container()`; assert it equals the fixed container
  prefix plus the helper's exact stdout. Assert
  `commitlint.dev_suffix(linked_root)` equals the same stdout. Assert the
  Go-format pre-commit entry is exactly `task fmt`, not a direct
  `podman-compose` invocation. Do not call the future e2e signature during RED.
  Use `uv run --locked --group e2e` because importing `e2e.py` requires its
  declared dependency group.

- [x] **Step 4: Run the semantic RED**

  Run only the three new tests. Expected: exactly three assertion failures:
  e2e and commitlint compare basename-only results with the longer canonical
  hashed identity, and the format hook still contains direct podman-compose.
  A `TypeError`, import, collection, fixture, missing-dependency, Git setup, or
  unrelated failure is not RED. Record exact expected and actual values in
  `tfp-bqt.13`.

- [x] **Step 5: Add the remaining executable contracts**

  Specify that the adapter invokes exactly
  `<repo_root>/scripts/dev-suffix.sh` as an argv list through `run`, with
  explicit `what`, `LOCAL`, `cwd=repo_root`, `capture_output=True`, and
  `text=True`; its default checked behavior must propagate helper failure.
  Create two temporary real linked worktrees with equal leaf names and
  require different suffixes and different e2e container names. Assert both
  Python consumer modules bind the same adapter function and the format hook
  binds `task fmt`. These tests lock the left-cancellation composition contract
  rather than reimplementing SHA-256 in Python. Temporary repositories keep
  their `.git` data inside the container-visible pytest directory; never invoke
  the helper against `/app`, whose linked-worktree Git metadata directory
  exists only on the host.

  Add a behavioral commitlint test that passes an explicit temporary repository
  root to `main`, captures the podman-compose subprocess call, and requires its
  `DEV_SUFFIX` environment value to equal the adapter result. A mutation that
  bypasses the adapter inside `main` must fail even if the module still imports
  the canonical function.

### Task 2: Implement the single identity boundary

**Files:**

- Create: `scripts/automation/dev_identity.py`
- Modify: `scripts/automation/e2e.py`
- Modify: `scripts/checks/commitlint.py`
- Modify: `.pre-commit-config.yaml`
- Modify: tests from Task 1

- [x] **Step 1: Implement only the bounded adapter**

  Add a fully annotated public `dev_suffix(repo_root: Path) -> str`. Invoke the
  canonical helper through `run` with the Task 1 argument contract and return
  stripped text. Do not use a shell, `subprocess` directly, basename logic,
  hashing, sanitisation, environment mutation, fallback identity, or broad
  exception handling.

- [x] **Step 2: Wire e2e through the adapter**

  Change `dev_container` to accept `repo_root: Path | None = None`; resolve
  `None` to `REPO_ROOT` at call time, then return the constant container prefix
  plus `dev_suffix(root)`. Runtime resolution deliberately preserves the RED
  test's patched zero-argument path and avoids Python's definition-time
  default binding. Add a second GREEN assertion for the explicit-root call.
  Keep every fixture, container, network, zone, credential, and scenario
  unchanged.

- [x] **Step 3: Wire commitlint through the adapter**

  Import the same `dev_suffix`; define the repository root from the module's
  resolved location; let `main` accept that root as an explicit test seam; and
  use the adapter result when setting `DEV_SUFFIX` for podman-compose. Preserve
  checked message-path handling, stdin bytes, commitlint argv, and return-code
  propagation. The env-capture test from Task 1 must fail if `main` restores a
  basename calculation or omits the suffix.

- [x] **Step 4: Route the Go-format hook through Task**

  Replace only its direct podman-compose entry with `task fmt`. Keep its ID,
  name, language, Go file type filter, exclusion, and filename policy. Do not
  introduce another Python wrapper or duplicate Task's container identity.

- [x] **Step 5: Run focused GREEN and mutation proof**

  Run the entire identity test module and
  `test/scripts/test_worktree_and_commitlint.py` through the e2e uv group.
  Require all tests pass. Demonstrate that bounded in-memory substitutions of
  either Python consumer back to `repo_root.name`, or the format entry back to
  direct podman-compose, fail the direct and collision contracts; do not edit
  production files for mutation proof.

- [x] **Step 6: Run the complete Python, shell, and format-hook gates**

  Run `task py` and `task lint:shell`. Require ruff check, ruff format check,
  ty, every `test/scripts` assertion, shellcheck, and the canonical suffix
  tests to pass. Run `pre-commit run format --all-files` with this worktree's
  hashed dev container running and require it invokes `task fmt` successfully
  without changing Go files. No ignore, suppression, dependency, or tool
  version changes are allowed in this fix.

### Task 3: Update the owning contract and evidence

**Files:**

- Modify: `docs/plan.md`
- Modify: `CHANGELOG.md`
- Modify: this plan
- Update: Bead `tfp-bqt.13`

- [x] **Step 1: Record the active public control**

  Keep P10-14 `[~]` through implementation and candidate review. Make P10-06
  depend on P10-14 and explicitly remain blocked. Do not copy the PostgreSQL
  implementation plan from its worktree.

- [x] **Step 2: Add the current changelog correction**

  Under `[Unreleased]` `Fixed`, state that linked-worktree e2e, commitlint, and
  Go formatting now select the canonical hashed development container instead
  of a basename-only or suffix-free path. Do not edit any released section.

- [x] **Step 3: Record exact evidence in Beads and this plan**

  Append RED/GREEN counts, exact suffix/container, tool versions, mutation
  result, source/base identifiers, and scope exclusions. Run `bd lint` and
  `bd dep cycles`; keep `tfp-bqt.13` `IN_PROGRESS`.

- [x] **Step 4: Run documentation and diff gates**

  Run `task docs:lint`, both `git diff --check` and staged diff check, and
  `git status --short`. The plan counters and badge must agree.

### Task 4: Prove the real e2e lifecycle and zero runtime residue

**Files:**

- Modify only evidence: this plan and Bead `tfp-bqt.13`

- [x] **Step 1: Establish an exact disposable preflight**

  Record disk capacity; this worktree's dev-container identity; absence of the
  two globally named e2e containers, their project volumes, and all managed
  test zones. Require the globally named five lab containers, three named lab
  volumes, lab network, and lab pod to be absent before `task lab:up`; if any
  exists, stop rather than reuse or remove another worktree's fixture. Inventory the
  complete Podman volume name set before startup. The pinned PostgreSQL 17 image
  declares `/var/lib/postgresql/data` as a volume, so after startup resolve that
  destination through the exact captured database container, require its source
  is one newly created anonymous name absent from the preflight set, and record
  its `CreatedAt`, labels, and mount path as part of this run's ownership proof.
  Do not assume podman-compose owns or removes the anonymous volume. Inventory the
  intentionally retained gitignored `.mirror`, `.released`, `.home`, `.token`,
  and `.tls` local caches separately; this identity fix does not alter their
  lifecycle. Preserve unrelated Podman objects.

- [x] **Step 2: Start and verify the lab, then start e2e**

  Run `task lab:up AUTH=5.1`, capture the exact created lab object identities,
  then run
  `task lab:status` and `task lab:verify`. Run `task e2e:up`, capture the exact
  created e2e object identities, and run `task e2e:status`. Prove the e2e driver
  selected this worktree's canonical hashed dev container and built both
  provider versions in its mirror. Use immutable IDs for containers, pods, and
  networks. For every volume, record and later compare its exact name,
  `CreatedAt`, labels, and mount path because Podman volumes have no immutable
  object ID.

- [x] **Step 3: Run the full consumer suite**

  Run `task e2e`. Record exact collected, passed, skipped, and failed scenario
  counts for Terraform and OpenTofu. Any failure is investigated before
  teardown; a subset is not acceptance.

- [x] **Step 4: Tear down only authorized fixtures**

  First require the e2e fixture names still resolve to the exact identities
  captured by this run; any mismatch stops without deletion. Run
  `task e2e:down`; while the lab APIs are still running, verify its runtime
  containers, volumes, remote state, and managed zones are absent. Re-inventory
  the intentionally retained local gitignored caches without claiming they were
  removed.

  Next require the lab container, pod, network, and named-volume identities
  still match; containers, pods, and networks use immutable IDs, while every
  volume matches by name, `CreatedAt`, labels, and mount path. Run
  `task lab:down AUTH=5.1`. Require the named lab objects absent and only the
  exact recorded PostgreSQL anonymous volume remaining beyond the complete
  preflight volume set. Podman-compose 1.5.0 removes only project-labelled
  volumes, so stop and obtain explicit destructive approval before removing
  that one exact volume by name. Revalidate its recorded identity immediately
  before removal, then require it absent and prove the complete preflight volume
  name set is unchanged. Without that approval, report the lifecycle BLOCKED
  rather than claiming zero residue. Do not use prune, broad Podman deletion, or
  new cache-cleanup behavior.

- [x] **Step 5: Re-run focused tests after lifecycle**

  Run the complete identity tests and `task py` again. A lifecycle-only pass
  cannot substitute for deterministic contracts.

### Task 5: Full gates, candidate, and sequential reviews

**Files:**

- Modify only evidence: this plan and Bead `tfp-bqt.13`

- [x] **Step 1: Run the complete non-lab gates**

  Fetch `origin/main` and require it still equals this branch's recorded base
  and is the sole merge base. If it advanced, stop before candidate creation,
  preserve all tracked and untracked work to an immutable verified stash OID,
  rebase, re-read complete `AGENTS.md`/`README.md`, restore, and repeat the plan
  and all evidence; never review a stale-base candidate. If unchanged, run
  `task all`, `task osv-scan`, and `task release:dryrun` in this worktree's
  canonical dev container. Record scanner input counts, findings, archive
  counts, and exact outcomes. Re-run e2e if any executable file changes later.

- [x] **Step 2: Self-review and create the evidence candidate**

  Stage only intended files; run staged diff check; review exact
  `origin/main` to index and working-tree diffs; require no unrelated change.
  Commit with hooks enabled using:

  ```text
  fix(e2e): restore worktree container identity
  ```

  The successful commit-msg hook is live evidence that commitlint selected the
  hashed current-worktree container; the explicit all-files format-hook run is
  the corresponding evidence for Go formatting. Do not bypass hooks or close
  the Bead.

- [ ] **Step 3: Obtain sequential candidate review**

  Obtain SPEC review of exact candidate HEAD, then QUALITY review of the same
  immutable HEAD. Review source-of-truth delegation, failure propagation,
  algebraic collision contract, false-green mutation paths, e2e lifecycle,
  docs, and scope. Any Critical or Important finding resets P10-14 to `[~]`,
  repeats affected focused plus full gates, creates a new candidate, and
  restarts SPEC then QUALITY. No Git, Beads, or Podman state changes between
  the two reviews; read-only checks are allowed.

- [ ] **Step 4: Create and review a docs-only closure**

  After both candidate approvals, fetch the default branch through GraphQL and
  Git and require both `defaultBranchRef.target.oid` and `origin/main` still
  equal the exact base used for candidate review. A PR does not exist yet, so
  `baseRefOid` is not available at this point. Any drift restarts the verified stash/rebase,
  affected and full gates, candidate creation, and sequential SPEC then QUALITY
  cycle before closure. Record exact reviewed HEADs, set P10-14
  `[x]`, keep P10-06 blocked until merge, and keep the Bead open. Run docs and
  diff gates and commit with hooks as:

  ```text
  docs(docs): complete the worktree identity fix
  ```

  Obtain final SPEC then final QUALITY against the exact immutable closure
  HEAD. No Git, Beads, or Podman mutation occurs between those reviews.

- [ ] **Step 5: Push, verify, and merge the pull request**

  Reconfirm `origin` is the canonical non-fork
  `ioplane/terraform-provider-powerdns`, push this branch only there, and open a
  PR titled with the candidate subject. Use paginated `gh api graphql` to
  re-read PR fields, comments, reviews, threads, replies, commits, and every
  required check. Immediately before merge, require the live GraphQL
  `baseRefOid` and fetched `origin/main` still equal the exact base used by the
  final closure reviews. Any drift restarts the verified stash/rebase, affected
  and full gates, new candidate and closure, and both sequential review cycles.
  Squash-merge only when all policy gates are green; verify the exact squash
  commit is on `origin/main`.

- [ ] **Step 6: Close the prerequisite and unblock PostgreSQL**

  Append the exact PR, squash commit, checks, and final reviewed HEAD to
  `tfp-bqt.13`, close it, and update `tfp-bqt.6.1` with the prerequisite
  evidence. Run `bd show`, `bd lint`, and `bd dep cycles`. Leave worktree,
  branch, containers, images, and volumes in place unless separately authorized
  for destructive cleanup.

## Rollback

Rollback is a separate revert PR restoring only the two basename-only consumer
functions, the direct format-hook entry, and their tests/docs. It does not
modify `scripts/dev-suffix.sh`, Task, Compose, PostgreSQL, PowerDNS, or Go.
Before reverting, prove why the canonical helper cannot be used; otherwise the
revert intentionally restores cross-worktree collisions and hook/e2e lookup
failures.

## Execution evidence

- Baseline base and live GraphQL default-branch OID are both
  `e65b4c78455ab0659b5b462b7fd4b507a207186a`; the one branch-protection page
  has 11 strict required GitHub Actions checks.
- Canonical suffix is `-dev-container-identity-5082b0029485`; preflight found
  no target container, cache volume, or pod. Warm `task up` created only project
  `terraform-provider-powerdns-dev-dev-container-identity-5082b0029485`.
- Running container `038b9d779483709d3fcfb469e2d2599446ad0de06e2e83b8d2c488a27aaab62a`
  uses image `fb4af4f276a7e7f95ed6ebe16cce858ae8101ea4521de841aeb3fc9c7353a757`,
  carries the exact Compose project label, and binds this canonical worktree at
  `/app` plus only its two named Go cache volumes.
- Tool baseline: Go 1.27.0, gopls 0.23.0, Terraform 1.15.8, OpenTofu 1.12.5,
  Terragrunt 1.1.1, golangci-lint 2.13.1, uv 0.11.33, ruff 0.16.0, ty 0.0.64.
  Host `rg` found 81 Go files and container `gopls check` passed all 81.
- `task py` passed ruff, formatting, ty, and 185 pytest assertions;
  `task lint:shell` passed; `task lint:pins` verified 29 references.
- One discarded diagnostic attempted `rg` inside the dev image and found that
  binary intentionally absent; it did not run gopls and is not counted as
  evidence. The executable plan now discovers paths on the host and runs gopls
  in the Go 1.27 container; that corrected command is the evidence above.
- Independent plan SPEC and QUALITY reviews approved this execution contract
  on 2026-08-24 with no Critical or Important findings. The final plan includes
  runtime-bound Python defaults, subprocess environment capture, all-files hook
  proof, exact fixture ownership, PostgreSQL anonymous-volume handling, and
  repeated default-branch OID checks.
- Semantic RED ran in the exact hashed development container: the first three
  tests failed 3/3 with basename-only e2e and commitlint values and the direct
  Compose format entry. The expanded tests-before-code run failed 8 and passed
  6, exposing the missing adapter, missing explicit-root seams, and all three
  stale consumers; an earlier command without `DEV_SUFFIX` only failed container
  lookup and is explicitly not counted as RED.
- Minimal GREEN delegates to `scripts/dev-suffix.sh` through the bounded runner,
  resolves the e2e default at call time, passes the adapter suffix to commitlint,
  and routes formatting through `task fmt`. Focused identity/worktree tests pass
  14/14; known basename, empty-suffix, and direct-Compose mutations are rejected.
- `task py` is green with Ruff, format, ty, and 192 pytest assertions;
  `task lint:shell` is green. `pre-commit run format --all-files` selected this
  worktree's hashed container through `task fmt`, passed, and changed no Go file.
- `task docs:lint` is green with zero Markdown/cspell findings, 110 verified
  badges, and agreeing counters. Git diff whitespace checks, Beads lint, and
  dependency-cycle checks are green; `tfp-bqt.13` remains `IN_PROGRESS`.
- Runtime preflight found all global lab/e2e fixtures absent, 59 pre-existing
  Podman volumes, and 17,979,277,312 bytes free. Auth 5.1 started exact versions
  5.1.3/5.1.3/5.4.4/2.1.0 and passed `lab:verify`; five container IDs, pod
  `5f163fc961f9`, network `aeec31d9a3fb`, three named volumes, and anonymous
  PostgreSQL volume `0180e727aa88` were recorded before e2e startup.
- `task e2e:up` built provider 0.1.1 from its tag and 0.1.2 from this worktree
  through the hashed Go 1.27 container. Both fixture containers were directly
  inspected running and healthy. `task e2e` passed all 59 Terraform/OpenTofu
  scenarios with no skip or failure. A separate `tfp-bqt.14` records the
  pre-existing false `absent` status when the Podman API socket is unavailable;
  direct immutable container IDs are the lifecycle evidence for this change.
- Exact e2e teardown removed its two containers, two volumes, remote state, and
  managed zones while lab APIs remained running; retained local caches are
  `.mirror`, `.released`, `.tls`, and `.token`. The five unchanged lab IDs were
  then torn down once; named volumes, pod, and network are absent. Only exact
  volume `0180e727aa884d4514b1c032a879c7fc22f52d7d945fb4949bb8addafff49c1f`
  remains beyond the preflight set, with unchanged empty labels, CreatedAt, and
  mount path. Its exact removal awaits explicit destructive approval, so Task 4
  Step 4 and the complete lifecycle remain fail-closed `IN_PROGRESS`.
- Post-lifecycle deterministic checks are green: focused identity/worktree tests
  pass 14/14 and full `task py` again passes all linters, type checking, and 192
  pytest assertions.
- Before the authorized removal, concurrent external cleanup removed the first
  anonymous target plus 34 unrelated preflight volumes. No command in this
  worktree performed that cleanup, so the first 59-name equality claim is
  rejected. A bounded repeat began from an exact 25-volume baseline, verified
  the same Auth 5.1 versions, created only three named lab volumes plus anonymous
  target `4560e73e70a9`, and rechecked all identities before one lab teardown.
  Context7 and local Podman help confirmed non-force removal refuses an in-use
  volume. The target's name, CreatedAt, empty labels, mount path, and zero users
  were revalidated immediately before the approved exact `podman volume rm`.
  The command exited zero; the target and every fixture object are absent, and
  the complete current volume set exactly equals the 25-name repeat preflight.
- Immediately before the full non-lab gates, `git fetch origin main`, the local
  `HEAD`, `origin/main`, merge base, and the paginated GraphQL
  `defaultBranchRef.target.oid` all resolved to
  `e65b4c78455ab0659b5b462b7fd4b507a207186a`; branch protection remained strict
  with 11 required contexts. `task all` exited zero: race/shuffle/atomic Go
  tests, contracts, vet, golangci-lint, 29 pins, tool parity, 11 protection
  checks, action/shell/container lint, Semgrep with zero findings, Ruff, format,
  ty, 192 pytest assertions, Terraform formatting, docs with 110 verified
  badges, and `govulncheck` all passed. `task osv-scan` scanned 62 Go and 27
  Python packages and reported zero affected packages and zero vulnerabilities.
  `task release:dryrun` succeeded after 1m37s; all 13 platform archives matched
  SHA256SUMS and the release manifest matched the repository manifest. A first
  standalone gopls diagnostic omitted the exported suffix, looked up the
  container without a suffix, and is excluded; the corrected fail-closed command
  exported the exact suffix and `gopls check` passed all 81 tracked Go files.
- Candidate self-review found exactly nine intended files, 750 insertions and
  40 deletions, with no unstaged or untracked path and clean staged whitespace.
  The first commit attempt was rejected before object creation because one body
  line exceeded the repository's 72-character limit. The corrected
  `fix(e2e): restore worktree container identity` attempt passed every enabled
  hook without bypass, including live commitlint through the canonical hashed
  development container. The Bead remains `IN_PROGRESS` for sequential review.
- Candidate `92254a28271964e344f41c2ea41f57f10af84c4b` was rejected by SPEC
  before QUALITY started. The format-hook oracle used substring membership, so
  an in-memory `entry: task fmt --dry` mutation still passed despite bypassing
  formatting. This Important resets complete non-lab gates and candidate
  creation; the replacement must require the exact entry and explicitly reject
  the dry-run mutation before a new immutable SPEC then QUALITY cycle.
- Remediation TDD reproduced the review finding exactly: the dry-run mutation
  produced one failure and ten passes because the substring oracle did not
  raise. The corrected oracle extracts every `entry:` from the format hook and
  requires the exact singleton `task fmt`; focused tests pass 11/11, the full
  Python gate passes 193 assertions, and the real all-files format hook passes.
  A fresh Git fetch and GraphQL query again resolved `origin/main`, merge base,
  and `defaultBranchRef.target.oid` to
  `e65b4c78455ab0659b5b462b7fd4b507a207186a`. The repeated `task all` exited
  zero with 193 pytest assertions, zero golangci-lint or Semgrep findings, 29
  verified pins, 11 protection checks, 110 badges, and no `govulncheck`
  vulnerability. OSV scanned 62 Go and 27 Python packages with zero affected
  packages. The release dry-run completed in 18 seconds and verified all 13
  archive digests and the repository manifest.
- Replacement self-review covers the same nine intended files with 795
  insertions and 40 deletions, no scope expansion, a clean staged whitespace
  check, and no unstaged or untracked path. The hook-enforced replacement
  amendment passed commitlint and every enabled repository hook without bypass;
  `tfp-bqt.13` remains `IN_PROGRESS` for a fresh sequential review cycle.
- Replacement `292f0dc433c4343c4a5b8ca3b59a48b99352fdc1` passed SPEC but was
  rejected by QUALITY. A valid YAML continuation can turn the exact-looking
  line into `task fmt --dry`; the line-oriented oracle ignored continuation
  content and remained false-green. This Important again resets the full gates
  and candidate. The next TDD loop must reject both same-line and multiline
  dry-run scalars without adding another Python dependency.
- The multiline continuation RED failed exactly one test while eleven passed.
  The dependency-free correction requires the exact singleton entry and makes
  it the last significant line of the isolated hook block, so both same-line
  and continued dry-run arguments are rejected. Focused tests pass 12/12, the
  Python gate passes 194 assertions, and the live all-files format hook passes.
  Git, merge base, and GraphQL still resolve the base to
  `e65b4c78455ab0659b5b462b7fd4b507a207186a`. The third complete `task all`
  repetition exited zero with 194 pytest assertions, zero golangci-lint and
  Semgrep findings, 29 pins, 11 protection checks, 110 badges, and no known Go
  vulnerability. OSV again found zero affected packages among 62 Go and 27
  Python packages. The 19-second release dry-run verified 13/13 archive digests
  and the repository manifest.
- Final replacement self-review covers only the same nine intended files with
  833 insertions and 40 deletions, clean staged and range whitespace checks,
  and no unstaged or untracked path. The amendment passed every enabled hook,
  including commitlint in the canonical hashed container, without bypass. The
  Bead remains open until a fresh immutable SPEC then QUALITY sequence.
- Candidate `d4ee0709fd2f1eb56441e58ee59cb25ce159459d` passed SPEC but
  QUALITY found that pre-commit's standard `args: [--dry]` field could precede
  the final exact entry and still change the effective command. The candidate
  is rejected and full gates reset. Rather than enumerate bypass fields, the
  replacement oracle must require the complete significant format-hook block
  exactly and reject an explicit `args` mutation.
- The pre-commit `args` mutation RED failed exactly one test while twelve
  passed. The corrected dependency-free oracle now requires the complete six
  significant format-hook fields in their one approved form, so entry suffixes,
  YAML continuation, `args`, aliases, and extra command modifiers all fail the
  same closed contract. Focused tests pass 13/13, the Python gate passes 195
  assertions, and the live all-files hook passes. Git and GraphQL still resolve
  the exact base to `e65b4c78455ab0659b5b462b7fd4b507a207186a`. The fourth
  full `task all` repetition exited zero with 195 pytest assertions, zero
  golangci-lint and Semgrep findings, 29 pins, 11 protection checks, 110 badges,
  and no Go vulnerability. OSV found zero affected packages among 62 Go and 27
  Python packages; the 19-second release dry-run verified all 13 archive digests
  and the repository manifest.
- Final self-review again covers only the same nine intended files, with clean
  staged and range whitespace checks and no unstaged or untracked path. The
  amendment passed all enabled hooks, including live hashed-container
  commitlint, without bypass. The Bead remains open for sequential review.
- Candidate `e23a773ec30c9676d6eeb680f1b2b79b218c85a2` passed SPEC but
  QUALITY showed that a root-level pre-commit `exclude` can remove every Go
  file before the exact hook runs, yielding a successful skip. The candidate
  is rejected and gates reset. The next oracle must reject every global
  execution modifier by requiring `repos` as the only root key, in addition to
  the exact local format-hook block.
- The global-exclude mutation RED failed exactly one test while thirteen
  passed. The replacement checks both execution layers: `repos:` is the only
  permitted root key and the format hook is the exact six-line block. Focused
  tests pass 14/14, the Python gate passes 196 assertions, and the live
  all-files hook passes. Git, merge base, and GraphQL still resolve to
  `e65b4c78455ab0659b5b462b7fd4b507a207186a`. The fifth full `task all`
  repetition exited zero with 196 pytest assertions, zero golangci-lint and
  Semgrep findings, 29 pins, 11 protection checks, 110 badges, and no Go
  vulnerability. OSV again found zero affected packages among 62 Go and 27
  Python packages. The 18-second release dry-run verified all 13 archive digests
  and the repository manifest.
- Final self-review remains limited to the same nine intended files; staged and
  range whitespace checks are clean and no unstaged or untracked path exists.
  All hooks passed without bypass, including commitlint through the canonical
  hashed container. The Bead remains open for sequential review.
- Candidate `7ede67b75328ab87bf614a758e181a52bd48c44d` passed SPEC but
  QUALITY showed that moving the exact hook under a remote repository permits
  manifest fields such as `args`, `files`, and `stages` to change execution when
  not overridden locally. The candidate and gates reset. The next oracle must
  bind the exact format hook to its nearest owning `repo: local` block and
  reject a remote-owner mutation.
- The remote-owner mutation RED failed exactly one test while fourteen passed.
  The corrected oracle binds the format block to the nearest preceding
  `repo: local`; focused tests pass 15/15 and the Python gate passes 197
  assertions. Ruff first exposed a comprehension formatting conflict; the
  final two-line setup satisfies check and formatter without suppression. The
  first full-gate attempt then stopped only on a newly coined evidence word and
  is excluded. After replacing that word, the complete
  `task all` rerun exited zero with 197 pytest assertions, zero golangci-lint
  and Semgrep findings, 29 pins, 11 protection checks, 110 badges, and no Go
  vulnerability. Git and GraphQL retain base
  `e65b4c78455ab0659b5b462b7fd4b507a207186a`; OSV found zero affected among
  62 Go and 27 Python packages, and the 19-second release dry-run verified all
  13 archive digests and the repository manifest.
- Final self-review remains limited to the same nine intended files, with clean
  staged and range whitespace checks and no unstaged or untracked path. Every
  hook passed without bypass, including live hashed-container commitlint. The
  Bead remains open for sequential review.
- Candidate `c7db38e87c91830281b6f003d0fc552fb9049c63` was rejected by SPEC
  before QUALITY. A duplicate `repo` key accepted by the pre-commit loader can
  override the apparent local owner while the textual nearest-owner oracle
  stays green. The candidate and gates reset. The replacement must run the
  pinned `yamllint` duplicate-key rule before the owner and exact-hook checks,
  with a duplicate-owner mutation as RED and no new Python dependency.
- The duplicate-owner mutation reproduced the false-green contract exactly:
  one test failed with `DID NOT RAISE` while fifteen passed. The minimal guard
  runs pinned yamllint 1.38.0 on standard input with only `key-duplicates`
  enabled before checking the local owner and exact hook block. Context7
  corroborates the standard-input and duplicate-key configuration semantics.
  Focused identity and commitlint tests pass 20/20; `task py` passes Ruff,
  format, ty, and 198 pytest assertions; the live all-files format hook passes.
  Git, merge base, and paginated GraphQL still resolve the strict 11-context
  base to `e65b4c78455ab0659b5b462b7fd4b507a207186a`, and `gopls check`
  passes all 81 tracked Go files under Go 1.27.0. The fresh `task all` exits
  zero with 198 pytest assertions, zero golangci-lint or Semgrep findings, 29
  verified pins, 11 protection checks, 110 badges, and no Go vulnerability.
  OSV reports zero affected packages among 62 Go and 27 Python packages; the
  20-second release dry-run verifies all 13 archive digests and the repository
  manifest.
- Replacement self-review remains limited to the intended nine-file change:
  998 insertions and 40 deletions from the exact base, clean staged and range
  whitespace checks, and no unrelated path. The hook-enforced amendment passed
  commitlint and every enabled repository hook without bypass. The Bead stays
  open for a fresh immutable SPEC then QUALITY sequence.
- Candidate `770b0ad94724fc942dad5efee2851472f9beee73` was rejected by SPEC
  before QUALITY. A `# yamllint disable-line rule:key-duplicates` directive can
  suppress the scanner finding while pre-commit resolves the duplicate remote
  owner. The candidate and complete gates reset. The replacement must reject
  yamllint control directives before duplicate-key scanning and cover the exact
  suppression mutation. This review also corrected the preceding self-review
  count from the intermediate 993 insertions to the candidate's exact 998.
- The suppression mutation RED failed one test while sixteen passed. The
  fail-closed oracle now rejects every yamllint control marker before invoking
  the duplicate-key scanner, so file, block, and line suppression cannot alter
  the result; focused identity and commitlint tests pass 21/21. `task py`
  passes Ruff, format, ty, and 199 pytest assertions, and the live all-files
  format hook passes. Git, merge base, and paginated GraphQL still resolve the
  strict 11-context base to
  `e65b4c78455ab0659b5b462b7fd4b507a207186a`; `gopls check` passes all
  81 tracked Go files under Go 1.27.0. The repeated `task all` exits zero with
  199 pytest assertions, zero golangci-lint or Semgrep findings, 29 pins, 110
  badges, and no Go vulnerability. OSV again reports zero affected among 62 Go
  and 27 Python packages; the 19-second release dry-run verifies all 13 archive
  digests and the repository manifest.
- Replacement self-review covers only the same intended nine paths. Staged and
  range whitespace checks are clean, no unrelated or untracked path exists,
  and all enabled hooks pass without bypass. The immutable candidate's exact
  range statistic is recorded in Beads after commit, avoiding a self-referential
  plan count that a later evidence-only amendment would invalidate.
- Candidate `b9069b739966137301a508aac65ba13be7b38bc9` passed SPEC but was
  rejected by QUALITY. YAML permits a sequence dash and its `repo` key on
  separate lines; that form moved the exact format hook under a remote manifest
  without triggering the compact-owner oracle. The candidate and full gates
  reset. The next TDD loop must reject this split-owner mutation and require one
  canonical fail-closed repository-item grammar before resolving the owner.
- The split-owner mutation RED failed one test while seventeen passed. The
  replacement oracle enumerates every repository-level sequence item before the
  format hook and requires the canonical two-space `- repo:` mapping form;
  split, flow, alias, and alternate-indentation items fail before owner lookup.
  Focused identity and commitlint tests pass 22/22; `task py` passes Ruff,
  format, ty, and 200 pytest assertions; the live all-files format hook passes.
  Git, merge base, and paginated GraphQL still resolve the strict 11-context
  base to `e65b4c78455ab0659b5b462b7fd4b507a207186a`; `gopls check`
  passes all 81 Go files under Go 1.27.0. The repeated `task all` exits zero
  with 200 pytest assertions, zero golangci-lint or Semgrep findings, 29 pins,
  110 badges, and no Go vulnerability. OSV reports zero affected among 62 Go
  and 27 Python packages; the 18-second release dry-run verifies all 13 archive
  digests and the repository manifest.
- Final self-review remains limited to the intended nine paths. The canonical
  repo-item grammar is checked before local ownership and the exact hook block;
  every earlier command, root filter, manifest-owner, duplicate-key, suppression,
  and split-sequence mutation remains covered. Staged and range whitespace
  checks are clean, no unrelated path exists, and all hooks pass without bypass.
- Candidate `3d8dcac38af4a973d8e75a06ccc08c15cbf1ced5` was rejected by SPEC
  before QUALITY. A YAML plain-scalar continuation after the apparent local
  owner changes the semantic repository into a remote manifest while the
  line-oriented canonical-item oracle stays green. Task 5 gates and candidate
  creation reset open. The exact continuation mutation RED failed one test
  while eighteen passed; the minimal fail-closed guard requires `hooks:` to be
  the first significant mapping field immediately after the canonical local
  owner. Focused identity and commitlint tests now pass 23/23. Full gates and a
  replacement immutable candidate remain open.
- Plain-scalar remediation gates are GREEN. `task py` passes Ruff, format, ty,
  and 201 pytest assertions; the real all-files format hook passes. Fresh Git,
  merge-base, and paginated GraphQL still resolve strict 11-context `main` to
  `e65b4c78455ab0659b5b462b7fd4b507a207186a`; Go 1.27.0 `gopls check`
  passes all 81 tracked Go files. The complete `task all` rerun exits zero with
  201 pytest assertions, zero golangci-lint and Semgrep findings, 29 verified
  pins, 110 badges, and no Go vulnerability. OSV finds zero affected packages
  among 62 Go and 27 Python packages. The 19-second release dry-run verifies
  all 13 archive digests and the repository manifest. Replacement candidate
  creation remains open.
- Replacement self-review covers exactly the same nine intended paths. The
  base-to-index and complete base-to-working-tree diffs contain no unrelated
  change; staged, working-tree, and range whitespace checks are clean. The
  plain-scalar continuation fails before semantic ownership can escape
  `repo: local`, and all prior false-green mutations remain covered. Every
  enabled commit hook passed without bypass. The Bead stays open for a fresh
  immutable SPEC then QUALITY sequence.
