# AUDIT-02 — Go 1.27 toolchain evidence

**Date:** 2026-08-21

**Bead:** `tfp-bqt.3`

**Status:** final gates passed; exact evidence candidate pending sequential review

## Authoritative release evidence

The [Go 1.27 release notes](https://go.dev/doc/go1.27), the
[language specification](https://go.dev/ref/spec), the
[module reference](https://go.dev/ref/mod), and the `cmd/go` testing help were
read before implementation. Go tag `go1.27.0` resolves to commit
`8af21751f066eced273ca3ce49506b366847c623`.

The migration-relevant changes are generic methods (with the interface-method
restriction), selector keys in struct literals, generalized generic-function
inference, default `stdversion` vet under `go test`, JSON v1 implemented over
v2 with v1 semantics preserved, bounded HTTP/1 response-body draining,
`SystemCertPool` environment handling on Darwin and Windows, Unicode 17, and
the macOS 13 minimum. The provider keeps the v1 JSON API and verifies behavior
rather than error text. CA, mTLS, TLS 1.2 and TLS 1.3 tests are hermetic; the
platform trust-store change is documented rather than simulated on Linux.

`go 1.27.0` is the minimum language version. Go 1.27 makes the matching
`toolchain go1.27.0` implicit: `go mod tidy -diff` removes an explicit line and
`go build` refuses the resulting non-tidy module. The project therefore omits
the redundant line and enforces the compiler through the digest-pinned OCI
image, inherited `GOTOOLCHAIN=local`, runtime `GOVERSION` parity, and workflow
pin checks. Image-installed build tools remain an OCI toolbox responsibility:
moving them into `go.mod` `tool` directives would
couple provider consumers to development-only modules and is deferred to the
development-image tooling boundary.

## OCI identity

Live Docker Hub resolution and an independent raw-manifest SHA-256 calculation
both returned:

`docker.io/library/golang:1.27-trixie@sha256:6212da3924947f4b6a939df02ea627c13f338f1a41d6c3fcb0dd9d076eef46c4`

The manifest is an OCI image index. Its Linux child manifests are
`sha256:31df75a0f51705bb15c74cacaeadb6596ae270cea9ec05138928de7e2d1f65e8`
for amd64 and
`sha256:6237d172b66951ce3ebfe0156587ad41a9773772c061fb9a5e3d612f5fd22614`
for arm64/v8.

## Module and analyzer inventory

GitHub GraphQL pagination completed with the following release/tag totals:

| Repository | Releases | Tags | Pages complete |
| --- | ---: | ---: | --- |
| `getkin/kin-openapi` | 151 | 151 | yes, 2/2 |
| `hashicorp/terraform-plugin-framework` | 58 | 59 | yes |
| `hashicorp/terraform-plugin-framework-validators` | 19 | 19 | yes |
| `hashicorp/terraform-plugin-go` | 44 | 44 | yes |
| `hashicorp/terraform-plugin-log` | 13 | 13 | yes |
| `hashicorp/terraform-plugin-testing` | 25 | 25 | yes |
| `golang/go` | 0 | 494 | yes, 5 tag pages |
| `golangci/golangci-lint` | 191 | 204 | yes, 2 release / 3 tag pages |
| `golang/vuln` | 10 | 18 | yes |

Selected tag commits are kin-openapi v0.147.0
`eda80e2676e9f577ceed2dd80e64f16083edb041`, framework v1.19.0
`c7ac25e86333d194946fb5e3fd1114e7d101fc23`, validators v0.19.0
`25a1378536d4975c1f8676989788a38e141c5e2e`, plugin-go v0.31.0
`09a1181b051c53a3700401895ae281afbc91f0fc`, plugin-log v0.11.0
`dbd9e7ec261db03160c961915409d39d55d23d79`, and plugin-testing v1.16.0
`54ba38bae695d587b38c9d54009668349a0f1f76`.

Kin-openapi v0.146.0 changes origin-location storage; v0.147.0 adds OAS 3.2
query/additional-operation support and fixes validation panics, ZIP body
decoding, and deep-object map deserialization. Plugin-log v0.11.0 raises its
minimum Go version to 1.25 without an API migration. The only direct module
updates are therefore kin-openapi v0.147.0 and plugin-log v0.11.0.

The Go-compatible analyzer pins are golangci-lint v2.13.1 at commit
`6d2288e072e6f9c9bca28180cae9ce58a049c912` and govulncheck v1.7.0 at commit
`617f44b718537dccdea1915395650e0529e3b72e`.

## RED/GREEN mapping

- Full-reference checker RED: three failures proved digest-only comparison,
  wrong-tag acceptance, and short/qualified-name mismatch.
- Go 1.26.7 language RED: generic methods failed to parse; Unicode 17 did not
  classify U+11DB4; ignored success bodies used two HTTP connections.
- Go 1.26.7 compatibility GREEN: request/response JSON v1, bounded error body,
  custom CA, invalid CA, mTLS, TLS 1.2 and TLS 1.3 tests passed.
- Go 1.27 focused GREEN: full-reference parsing, JSON, bounded drain, TLS,
  Unicode 17, generic methods, selector keys, and generalized inference pass.

## Build and final gates

Cold development-image build: 292.426 seconds. Warm build: 1.666 seconds.
The isolated Compose project
`terraform-provider-powerdns-dev-go127-cache-final` built without replacing an
existing container. Its Go runtime is 1.27.0; golangci-lint is 2.13.1,
govulncheck is 1.7.0, gopls is 0.23.0, and the Compose project label carries
the same unique suffix.

The final local OCI image has ID
`031859a098b36df474ddacd9e6d1f40327ea8426cb3c296e50a3d071ecda750d`,
digest
`sha256:a285733d76a7a66de39ed9fe191aba38cff5b7664673cd3266cbbcc47ac0275f`,
size 2,784,983,862 bytes, OCI image-manifest media type, and 15 layers. Its
labels are limited to the Buildah version and the existing OCI title,
description, source, licence, and vendor labels. A mounted immutable-rootfs
inspection found no `/go/pkg/mod`, `/tmp/go-cache`, or
`/root/.cache/go-build`. Image history contains one Go-tool installation RUN,
the `/tmp/go-cache` compiler-cache mount, and the trailing
`go clean -modcache`; it contains no module-cache mount.

`task all` passed in that Go 1.27 container: race, shuffle and atomic-coverage
unit tests; contract tests; explicit `go vet ./...`; golangci-lint with zero
issues; pin and workflow parity; container, workflow and shell lint; Semgrep
with zero findings; 163 Python contract tests; Terraform formatting; docs,
spelling and 106 badge checks; and `govulncheck ./...` with no vulnerabilities.
The separate OSV scan found zero affected packages. The release dry run built
13 cross-platform archives, verified every digest, and matched the registry
manifest. `go mod tidy -diff` and touched-file `gopls check` also passed.

The Auth 5.1 lab reported Authoritative 5.1.3 on both gpgsql and LMDB,
Recursor 5.4.4, and dnsdist 2.1.0. `task verify AUTH=5.1` passed the aggregate
gate, version check, and live suite: 35 acceptance tests passed and the one
documented `api_dir` negative case was intentionally skipped. The approved
`task lab:down AUTH=5.1` lifecycle then removed the five disposable lab
containers and three project volumes; read-only post-checks found no remaining
`pdns-lab` container or `terraform-provider-powerdns-lab` volume.

Concurrent host activity initially reduced free root filesystem capacity to
685 MB at 100% usage after the Auth 5.1 teardown, so the Auth 5.0 run stopped
fail-closed before creating any second-branch lab object. After capacity was
restored, a fresh preflight found 39,852,642,304 bytes free and an empty lab
namespace. The Auth 5.0 lab then reported Authoritative 5.0.6 on both gpgsql
and LMDB, Recursor 5.4.4, and dnsdist 2.1.0. `task verify AUTH=5.0` also passed
the aggregate gate, version check, and live suite: 35 acceptance tests passed
and the same documented `api_dir` negative case was intentionally skipped.
The approved teardown removed the five disposable lab containers and three
project volumes. Final post-checks found no lab container or volume and
38,584,352,768 bytes free at 88% usage.

## Independent-review remediation

The first quality review found five important gaps in the candidate: short
Docker Hub Go image names still passed the parity parser; cleanup detection
accepted text wrapped by `echo` or `sh -c`; concurrent worktrees shared the
same local development-image tag; explicit recreation checked only a container
name rather than ownership; and ADR 0004's Go 1.26 decision had no superseding
record. The execution documents also still described intermediate commits
despite the requirement that no intermediate Go 1.26.7 project state be
published. The user explicitly approved combining the cache and Go migrations
into one final atomic boundary.

Focused mutation RED was **6 failed, 37 passed**. The six failures proved short
`golang:` acceptance, both cleanup-command wrappers, the shared image tag, and
wrong-project and wrong-`/app`-bind paths reaching the fake removal command. A
follow-up exact-ARG mutation was independently RED at **1 failed, 6 passed**
because prefixed text could still contain a valid-looking reference. The
corrected focused suite is **45 passed**. The parser now requires a fully
qualified `docker.io/library/golang:` reference with a 64-character hexadecimal
digest and a valid reference boundary, and the Containerfile ARG must equal
that reference exactly. Cleanup is an exact shell segment after
the last `go install`. Compose project, container and image identities all
carry the worktree suffix. `task recreate` now requires both the expected
Compose project label and the canonical current-worktree `/app` bind before its
exact-name removal command.
[ADR 0010](../adr/0010-go-1.27-development-toolchain.md) supersedes only ADR
0004 decision point 1.

No development container, image, pod or volume was created, replaced, stopped
or removed during this remediation, and `task recreate` was not executed. The
earlier full gates and dual-branch acceptance evidence predate these fixes.
They remain historical evidence, not final post-review GREEN; the required
post-fix full repetition and independent approvals remain open.

After explicit approval, destructive validation was limited to the new
disposable project `terraform-provider-powerdns-dev-go127-recreate-review`.
Preflight found its exact container, pod and two cache volumes absent, with
38,556,917,760 bytes free. Cold `task up` passed in 281.181 seconds and created
container `37b4bfc3fdaeea5fbb177e6e0da8c66d076810ca1e02e1983ddb95b30e04bfa7`.
Both Compose labels matched the project, and `/app` was bound from the canonical
current worktree.

The worktree-specific image is
`fb4af4f276a7e7f95ed6ebe16cce858ae8101ea4521de841aeb3fc9c7353a757`,
digest
`sha256:72c4272d6095b45ccece726cd4ab9e9ea80e6f9ade16d85c6e4468b99e4fce47`,
2,784,983,862 bytes, and an OCI image manifest with the existing six labels
only. Its runtime reported Go 1.27.0 and `GOTOOLCHAIN=local`. Immutable-rootfs
inspection found no `/go/pkg/mod`, `/tmp/go-cache`, or
`/root/.cache/go-build`; history contains the `/tmp/go-cache` compiler mount
and trailing exact `go clean -modcache`, with no module-cache mount.

The explicitly authorized `task recreate` passed in 12.018 seconds. Its warm
build completed from cache before the ownership guard removed only the old
exact container; the toolbox sleep process required SIGKILL after its SIGTERM
timeout. New container
`c6bc40b7710c1d5cfbe95dc5ed2e3ab369e34c0a9a734d1b206745d1f2813ab4`
then ran with the same image, labels, canonical bind and cache volumes. Version
checks passed and the Python contract suite reported 171 passed.

The authorized `down -v` passed in 10.440 seconds and removed only that
disposable container, project pod and its two cache volumes. Post-flight found
all four exact targets absent, preserved the image, and reported
32,366,120,960 bytes free. Read-only inspection confirmed the pre-existing
Go 1.27 final verifier, legacy worktree container and Go 1.26.7 verifier still
exist with their prior IDs and images; no mutating command addressed them.

## Post-review remediation rerun

The final review fixes were validated in the explicitly approved disposable
project `terraform-provider-powerdns-dev-go127-recreate-review`. The first
post-fix aggregate exposed a timing-sensitive test assumption: Go 1.27 drains
an unread HTTP/1 body asynchronously, bounded by 256 KiB and 50 milliseconds,
so an immediate second request is not guaranteed to reuse the connection. The
test now waits beyond that bounded interval. The exact failing shuffle seed
then passed 100 focused race repetitions and ten complete transport-package
race repetitions.

After correcting one audit spelling failure found by the first aggregate,
`task all` passed completely: race, shuffle, atomic coverage, explicit vet,
golangci-lint with zero issues, 29 pin references, 11 protection contexts,
Semgrep with zero findings, 171 Python contract tests, documentation with 110
badges, and govulncheck with no vulnerabilities. The separate OSV scan found
zero affected packages among 62 Go and 27 Python packages. The release dry run
completed in 1 minute 37 seconds, produced 13 archives including both Darwin
architectures, verified every archive digest, and matched the registry
manifest.

The inspected final image is
`fb4af4f276a7e7f95ed6ebe16cce858ae8101ea4521de841aeb3fc9c7353a757`,
digest
`sha256:72c4272d6095b45ccece726cd4ab9e9ea80e6f9ade16d85c6e4468b99e4fce47`,
2,784,983,862 bytes, an OCI image manifest with 15 layers and the existing six
labels. Its history contains the `/tmp/go-cache` compiler mount and trailing
exact `go clean -modcache`, with no module-cache mount. A fresh immutable-rootfs
mount found `/go/pkg/mod`, `/tmp/go-cache`, and `/root/.cache/go-build` absent.

The Auth 5.1 rerun reported Authoritative 5.1.3 on PostgreSQL and LMDB,
Recursor 5.4.4, and dnsdist 2.1.0. `task verify AUTH=5.1` passed with 35
acceptance tests and the one documented `api_dir` negative case intentionally
skipped. The paired teardown removed the five lab containers, three volumes,
and lab pod/network; post-checks were empty. The Auth 5.0 rerun then reported
Authoritative 5.0.6 on both backends with the same Recursor and dnsdist
versions. Its full verify also passed 35 acceptance tests with the same single
intentional skip. Its paired teardown left the lab container, volume and pod
inventories empty. Final free space was 27,559,493,632 bytes.

The final disposable-verifier teardown removed its exact container, pod and
two cache volumes after all gates; its sleep process required SIGKILL after the
ten-second SIGTERM timeout. The inspected image remains present, all exact
runtime targets are absent, and free space recovered to 32,055,451,648 bytes.

The phase remains in progress pending independent review of this exact
post-gate candidate.

## Specification-process remediation rerun

Candidate `fa77bf273b06f33da0c715d05fc220913e92b985` was not approved by the
specification reviewer because its implementation plan deferred the evidence
commit until review, then required a new, unreviewed closure commit after that
review. The plan now defines a finite sequence: create the gated evidence
candidate first; review that exact HEAD sequentially for specification and
quality; create a docs-only closure commit while the Beads stay open; review
that exact closure HEAD sequentially with no later Git changes; only then close
the Beads externally and push. Any Critical or Important finding restarts the
full gate-and-review loop from a new evidence commit.

Although the correction changes documentation only, the required post-finding
gate repetition used the isolated project
`terraform-provider-powerdns-dev-go127-recreate-review`. `task all` passed
with race, shuffle and atomic coverage; explicit vet; zero golangci-lint and
Semgrep findings; 171 Python contract tests; 110 badge checks; and no
govulncheck vulnerabilities. OSV again found zero affected packages among 62
Go and 27 Python packages. The release dry run completed in 1 minute 34
seconds, generated 13 archives including both Darwin architectures, verified
all 13 archive digests, and matched the registry manifest.

Immutable inspection reconfirmed image
`fb4af4f276a7e7f95ed6ebe16cce858ae8101ea4521de841aeb3fc9c7353a757`,
digest
`sha256:72c4272d6095b45ccece726cd4ab9e9ea80e6f9ade16d85c6e4468b99e4fce47`,
2,784,983,862 bytes, OCI image manifest, 15 layers and the existing six
labels. Runtime parity reported Go 1.27.0 with `GOTOOLCHAIN=local`. History
uses `/tmp/go-cache` for compiler caching, runs exact trailing
`go clean -modcache`, and has no module-cache mount. The immutable rootfs has
none of `/go/pkg/mod`, `/tmp/go-cache`, or `/root/.cache/go-build`.

Auth 5.1 verification reported Authoritative 5.1.3 on PostgreSQL and LMDB,
Recursor 5.4.4, and dnsdist 2.1.0. The full `task verify AUTH=5.1` passed 35
acceptance tests with the one documented `api_dir` negative case intentionally
skipped. Its approved teardown exited zero; post-status failed closed with
`lab is not running`, and the lab container, volume and pod inventories were
empty. Auth 5.0 then reported Authoritative 5.0.6 on both backends with the
same Recursor and dnsdist versions. Its full verify also passed 35 acceptance
tests with the same single intentional skip. Its teardown and empty post-flight
passed, leaving 27,404,353,536 bytes free. The phase remains in progress until
the new evidence candidate and later closure HEAD each pass their sequential
specification and quality reviews.

After all gates, the final approved disposable-project teardown exited zero,
removing only container
`bd95125eff6a3ce1e2003b21e78f4ce9b8ff8882762d006fe3413e907a370dd2`,
its project pod and two cache volumes. The toolbox sleep process required
SIGKILL after its ten-second SIGTERM timeout. Post-flight found all exact
runtime targets absent, preserved image
`fb4af4f276a7e7f95ed6ebe16cce858ae8101ea4521de841aeb3fc9c7353a757`,
and reported 31,874,240,512 bytes free. Read-only comparison reconfirmed the
four pre-existing PowerDNS development containers at their prior IDs and
states; no command addressed them.

## Second quality-review remediation

The specification review approved candidate
`8cb86b63f891a4cc2e7159c9cf2d061f8f9b4c73`, but the subsequent quality
review found two Important gaps. Worktree identity used only `$PWD`'s basename,
so equal leaves could collide and subdirectory invocation could change the
target. The cleanup contract also discarded shell connectors, allowing
`&& true || go clean -modcache` to masquerade as a guaranteed cleanup.

Executable RED produced three suffix failures: two distinct `cache` roots both
resolved to `-cache`; the same checkout resolved to `-cache` at its root and
`-api` below it; and unsafe mixed-case punctuation passed through unchanged.
The main-checkout empty-suffix case already passed. The OR-list mutation was
separately RED because the existing assertion did not reject it. A long-name
RED then produced a 213-character suffix, proving that sanitization also needs
an explicit bound.

The focused correction derives a linked-worktree suffix from the sanitized,
lowercase, 48-character-bounded root basename plus 12 hexadecimal SHA-256
characters of the canonical full root. Fake-Git executable tests prove equal
base names at different full paths differ, subdirectories remain stable, the
main checkout keeps an empty suffix, unsafe characters cannot escape, and long
names retain the path digest within a 62-character suffix. The cleanup parser
now retains shell control operators, requires every connector in the Go-tool
RUN to be `&&`, and requires exact standalone `go clean -modcache` as the final
segment after all installs. On the unchanged Containerfile, the shell lexer
reports ten `&&` connectors and final segment
`['go', 'clean', '-modcache']`.

The derivation lives in executable POSIX `scripts/dev-suffix.sh`, so Task
captures its output without pre-expanding local shell variables. The blocking
shell gate now checks that script. Focused GREEN is 29 lifecycle and
Containerfile contracts, 179 repository Python tests, clean ruff formatting
and lint, clean ty, shellcheck, documentation lint, gopls diagnostics, and
`git diff --check`. Host `task --dry --verbose up` resolves the expected suffix
and project without executing Podman.

The approved automatic identity for this canonical worktree is suffix
`-go-module-cache-6eae7109dd0c`. It creates project and container
`terraform-provider-powerdns-dev-go-module-cache-6eae7109dd0c`, image
`terraform-provider-powerdns-dev-go-module-cache-6eae7109dd0c:local`, and
volumes
`terraform-provider-powerdns-dev-go-module-cache-6eae7109dd0c_go-mod-cache`
and
`terraform-provider-powerdns-dev-go-module-cache-6eae7109dd0c_go-build-cache`.
The approved lifecycle preflight found all four disposable targets absent and
40,929,329,152 bytes free. The first fully cached `task up` completed in
1.852 seconds and created container
`3bed189c714294abade404aa86cd70b8b00c43806738656978c16a747368be0d`.
Inspection proved the exact project labels, canonical `/app` bind, two scoped
cache volumes, image ID
`fb4af4f276a7e7f95ed6ebe16cce858ae8101ea4521de841aeb3fc9c7353a757`,
Go 1.27.0, `GOTOOLCHAIN=local`, `GOCACHE=/tmp/go-cache` and
`GOMODCACHE=/go/pkg/mod`. The transactional `task recreate` completed in
12.011 seconds: it built first, revalidated ownership, removed only that exact
container, and created replacement
`b960b1259859cc81315d609d2e2963d8c0dcfc7c3d6180f47736ec8c13b7c371`
from the same image. The old ID was absent; the new labels and bind matched.
Focused contracts passed 29/29 and the full Python suite passed 179/179.
The authorized `down -v` removed only the disposable container, pod and two
volumes; the image remained and all four pre-existing development-container
IDs and states were unchanged.

A fresh automatic `task up` then completed in 1.781 seconds for the final gate
run. `task all` passed: race/shuffle/atomic Go tests, vet, golangci-lint with
zero issues, all 179 Python tests, Semgrep with zero findings, 29 pin checks,
11 protection contexts, 110 badge checks, documentation and Terraform checks,
and `govulncheck` with no vulnerabilities. OSV reported zero affected packages
and zero vulnerabilities. The release dry run succeeded in 1 minute 38
seconds; all 13 archives matched their recorded digests and the registry
manifest matched. Immutable image inspection reported OCI image-manifest and
config media types, Linux/amd64, 15 layers, six labels, size 2,784,983,862
bytes and digest
`sha256:72c4272d6095b45ccece726cd4ab9e9ea80e6f9ade16d85c6e4468b99e4fce47`.
Its mounted rootfs contained none of `/go/pkg/mod`, `/tmp/go-cache` or
`/root/.cache/go-build`.

Fresh Auth 5.1 verification reported Authoritative 5.1.3 on PostgreSQL and
LMDB, Recursor 5.4.4 and dnsdist 2.1.0. The full gate and acceptance run passed
35 acceptance tests with one intentional `api_dir` capability skip. Its paired
down left lab containers, volumes and network empty. Fresh Auth 5.0 then
reported Authoritative 5.0.6 on both backends with the same Recursor and
dnsdist versions; its full run also passed 35 acceptance tests with the same
one intentional skip. Its paired down again left all lab targets empty, with
34,547,372,032 bytes free. After the full gates and focused documentation
checks passed, the approved final `down -v` removed the automatic verifier, its
pod and both cache volumes.
Post-flight checks found all four targets absent, the image preserved, all four
pre-existing development-container IDs and states unchanged, the lab namespace
empty, and 37,819,920,384 bytes free. The evidence candidate and sequential
specification/quality review loop remain open.

## Third specification-review remediation

Specification review of exact candidate
`e444770350b938e30d7dc63426c014b84c9b0da6` found one Important fail-open
path. POSIX `set -e` observes only the final command of a pipeline, so a failed
`sha256sum` could be hidden by successful `cut` and emit a shared empty-digest
suffix. RED proved the command-failure case and empty, short, long, uppercase
and non-hexadecimal output all returned success and emitted a suffix.

The corrected helper captures the complete output with `sha256sum` as the
checked final pipeline command, extracts the first field with shell parameter
expansion, requires exactly 64 lowercase hexadecimal characters and only then
truncates to 12 characters. Every failure returns nonzero and emits no suffix.
Focused GREEN is 35 lifecycle and Containerfile contracts. GNU coreutils
`sha256sum` is now an explicit host prerequisite.

The authorized automatic-identity lifecycle then passed again. A cached
`task up` completed in 1.792 seconds and created container `b4e96a3b5614`;
the ownership-guarded `task recreate` completed in 11.906 seconds, removed only
that exact container and created `a00a4b161667` on the same immutable image.
The project label, canonical `/app` bind, Go 1.27.0 runtime, local toolchain and
both exact cache volumes matched. Focused contracts passed 35 tests, and the
first exact `down -v` left the disposable container, pod and volumes absent.
A fresh cached `task up` completed in 1.776 seconds as `ea411879f340` for the
full gate repetition.

`task all` passed with race, shuffle and atomic coverage, explicit vet,
golangci-lint and Semgrep zero findings, 185 Python tests, 29 pins, 11
protection contexts, 110 badge checks and no govulncheck findings. OSV found
zero affected packages among 62 Go and 27 Python packages. The 1 minute 34
second release dry run generated 13 archives, verified every digest and matched
the registry manifest. Immutable image `fb4af4f276a7`, digest
`sha256:72c4272d6095b45ccece726cd4ab9e9ea80e6f9ade16d85c6e4468b99e4fce47`,
is an OCI manifest of 2,784,983,862 bytes with 15 layers and the same six
labels. Its mounted rootfs contained none of `/go/pkg/mod`, `/tmp/go-cache` or
`/root/.cache/go-build`, and was unmounted successfully.

Auth 5.1 used Authoritative 5.1.3 on PostgreSQL and LMDB, Recursor 5.4.4 and
dnsdist 2.1.0; Auth 5.0 substituted Authoritative 5.0.6. Each `task verify`
exited zero with 35 acceptance tests passing and the single documented
`api_dir` case intentionally skipped. Each exact lab teardown exited zero and
left its containers, volumes and network absent. The post-Auth-5.0 filesystem
had 32,513,314,816 bytes free.

After the full gates and focused documentation checks passed, the approved
automatic-project `down -v` exited zero; the inert sleep process required the
documented SIGTERM-timeout/SIGKILL fallback. The exact container, pod and two
cache volumes are absent, while image `fb4af4f276a7` is preserved. The lab is
empty. Pre-existing development containers `ae5ab9bfba65`, `7e7569a62c42`,
`37471b3b2732` and `3b0b830715be` retain their prior running, running, exited
and running states respectively. Final free space is 36,979,503,104 bytes.
The amended evidence candidate and sequential specification/quality review
remain open; neither Bead is complete.
