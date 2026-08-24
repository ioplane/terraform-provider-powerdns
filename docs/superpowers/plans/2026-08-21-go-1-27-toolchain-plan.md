<!-- markdownlint-disable MD010 -->

# Go 1.27 Toolchain Implementation Plan

> **For agentic workers:** REQUIRED: Use
> `superpowers:subagent-driven-development` (if subagents are available) or
> `superpowers:executing-plans` to implement this plan. Steps use checkbox
> (`- [ ]`) syntax for tracking.

**Goal:** Move the provider's complete Go build and analysis surface to Go
1.27.0 while preserving protocol 6 behavior and proving the release-note
changes that touch this provider's JSON, HTTP, TLS, Unicode and analyzer paths.

**Architecture:** Keep the provider and API layout unchanged. Treat the cache
repair, worktree-owned development lifecycle, Go directive, immutable Go OCI
image, CI images, `golangci-lint` and `govulncheck` as one atomic compatibility
boundary. Add focused characterization tests before changing the toolchain,
retain direct module versions unless live upstream evidence proves a newer
stable release, and use the existing development container for every Go
command.

**Tech Stack:** Go 1.27.0, `terraform-plugin-framework` v1.19.0, protocol 6,
`golangci-lint` v2.13.1, `govulncheck` v1.7.0, gopls v0.23.0, Podman Compose,
Go `testing`, `httptest`, GitHub GraphQL API, Context7 and official Go
documentation.

---

## Scope and prerequisites

This plan implements Beads `tfp-bqt.2.1` and `tfp-bqt.3` as one explicitly
approved final boundary. The baseline development image on branch
`fix/containers/go-module-cache`, cut from `origin/main` at `9ef6fb0`, could not
build because the persistent module cache contained incomplete source trees.
That repair was a prerequisite for valid Go migration evidence, and runtime
parity belongs to the same development-image lifecycle. The user then required
that the unpublished branch expose only Go 1.27, so the staged design, cache
repair and originally separate implementation commits collapse into one atomic
implementation-and-evidence candidate commit before review. A later docs-only
closure commit records review outcomes without changing that implementation.
No intermediate Go 1.26.7 project state is published.

This boundary does not update PostgreSQL, PowerDNS, Terraform, OpenTofu,
Terragrunt or the remaining development-image tools. Those changes retain
their own Beads and pull-request boundaries.

The reconciled starting sequence was:

1. Recover capacity only under explicitly authorized, repository-scoped Podman
   storage work; keep `tfp-bqt.2` active until its own exit evidence is complete.
2. Carry the approved but unpublished design into the same final branch rather
   than publishing an intermediate documentation pull request.
3. Continue the existing isolated worktree and claim both implementation Beads:

   ```bash
   cd ../.worktrees/fix/containers/go-module-cache
   bd update tfp-bqt.2.1 --claim
   bd update tfp-bqt.3 --claim
   ```

4. Record the cache-build RED on the original baseline, then validate Go 1.27
   only in the fresh explicit Compose project:

   ```bash
   git status --short
   git rev-parse HEAD
   task DEV_SUFFIX=-go127-cache-final versions
   ```

The observed baseline was not cleanly buildable; that is the `tfp-bqt.2.1`
RED. A temporary Go 1.26.7 verifier was used only to diagnose a security gate
before the Go 1.27 override and is historical evidence, not project state.
Record actual commits and outputs in the Beads; never substitute expected
values for observed ones.

- [x] **Boundary approval: explicitly approve the combined
  `tfp-bqt.2.1` plus `tfp-bqt.3` execution boundary**

The user explicitly approved this boundary and required that no intermediate
Go 1.26.7 state be published.

## Task 1: Lock live release and dependency evidence

**Files:**

- Create: `docs/audit/AUDIT-02-go-1.27-toolchain.md`
- Modify: `docs/plan.md`

- [x] **Step 1: Query authoritative upstream state**

Use `gh api graphql --paginate` for each direct module repository and record
node totals, pagination completion, latest stable tag, peeled tag commit and
release-note range. The current direct modules and GraphQL-verified targets
are:

```bash
for repo in getkin/kin-openapi \
  hashicorp/terraform-plugin-framework \
  hashicorp/terraform-plugin-framework-validators \
  hashicorp/terraform-plugin-go \
  hashicorp/terraform-plugin-log \
  hashicorp/terraform-plugin-testing \
  golang/go golangci/golangci-lint golang/vuln
do
  owner=${repo%/*}
  name=${repo#*/}
  gh api graphql --paginate -F owner="$owner" -F name="$name" -f query='
    query($owner:String!, $name:String!, $endCursor:String) {
      repository(owner:$owner, name:$name) {
        releases(first:100, after:$endCursor) {
          totalCount
          nodes { tagName publishedAt url description }
          pageInfo { hasNextPage endCursor }
        }
      }
    }' | jq -s '{pages:length, total:.[0].data.repository.releases.totalCount,
      complete:(.[-1].data.repository.releases.pageInfo.hasNextPage == false),
      releases:[.[].data.repository.releases.nodes[]]}'

  gh api graphql --paginate -F owner="$owner" -F name="$name" -f query='
    query($owner:String!, $name:String!, $endCursor:String) {
      repository(owner:$owner, name:$name) {
        refs(refPrefix:"refs/tags/", first:100, after:$endCursor) {
          totalCount
          nodes {
            name
            target {
              ... on Commit { oid }
              ... on Tag { target { ... on Commit { oid } } }
            }
          }
          pageInfo { hasNextPage endCursor }
        }
      }
    }' | jq -s '{pages:length, total:.[0].data.repository.refs.totalCount,
      complete:(.[-1].data.repository.refs.pageInfo.hasNextPage == false),
      tags:[.[].data.repository.refs.nodes[]]}'
done
```

Expected totals for the first six repositories are `151/151`, `58/59`,
`19/19`, `44/44`, `13/13` and `25/25` for releases/tags in order. Record fresh
totals for the three Go tool repositories because they can change before
execution. Every final `hasNextPage` must be false. Select latest stable by
SemVer, not publication order.

- `github.com/getkin/kin-openapi` v0.145.0 to v0.147.0, tag commit
  `eda80e2676e9f577ceed2dd80e64f16083edb041`;
- `github.com/hashicorp/terraform-plugin-framework` v1.19.0, already
  current, tag commit `c7ac25e86333d194946fb5e3fd1114e7d101fc23`;
- `github.com/hashicorp/terraform-plugin-framework-validators` v0.19.0,
  already current, tag commit
  `25a1378536d4975c1f8676989788a38e141c5e2e`;
- `github.com/hashicorp/terraform-plugin-go` v0.31.0, already current, tag
  commit `09a1181b051c53a3700401895ae281afbc91f0fc`;
- `github.com/hashicorp/terraform-plugin-log` v0.10.0 to v0.11.0, tag commit
  `dbd9e7ec261db03160c961915409d39d55d23d79`;
- `github.com/hashicorp/terraform-plugin-testing` v1.16.0, already current,
  tag commit `54ba38bae695d587b38c9d54009668349a0f1f76`.

Also query the `golang/go`, `golangci/golangci-lint` and `golang/vuln`
repositories. Verify, rather than infer, these intended pins:

- Go tag `go1.27.0`, commit
  `8af21751f066eced273ca3ce49506b366847c623`
- `docker.io/library/golang:1.27-trixie` digest
  `sha256:6212da3924947f4b6a939df02ea627c13f338f1a41d6c3fcb0dd9d076eef46c4`
- `golangci-lint` v2.13.1
- `govulncheck` v1.7.0

Resolve the Go image from Docker Hub itself, not local image storage:

```bash
go_registry_token=$(curl -fsSL \
  'https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/golang:pull' \
  | jq -r .token)
curl -fsSI \
  -H "Authorization: Bearer ${go_registry_token}" \
  -H 'Accept: application/vnd.oci.image.index.v1+json' \
  https://registry-1.docker.io/v2/library/golang/manifests/1.27-trixie \
  | tr -d '\r' | rg -i '^docker-content-digest:'
```

Expected:

```text
docker-content-digest: sha256:6212da3924947f4b6a939df02ea627c13f338f1a41d6c3fcb0dd9d076eef46c4
```

- [x] **Step 2: Read current documentation**

Resolve the Go library through Context7, then query it for the Go 1.27
language and standard-library changes. Read the complete official
[Go 1.27 release notes](https://go.dev/doc/go1.27) and relevant normative
sections of the [Go specification](https://go.dev/ref/spec), including method
declarations, type parameter declarations, type inference, selector
expressions, method sets and assignability.

The audit must distinguish normative language rules from release-note runtime
behavior and cover:

- generic methods and the interface-method restriction;
- selector keys in struct literals;
- the default `stdversion` vet analyzer;
- `encoding/json` v1 behavior backed by v2;
- bounded automatic draining on HTTP/1 response-body close;
- `SystemCertPool` environment behavior on Darwin and Windows;
- Unicode 17;
- the macOS 13 minimum.

- [x] **Step 3: Write the evidence record**

Document exact sources, observed versions, release deltas, decisions and the
test mapping in `AUDIT-02`. Do not claim that a dependency is current without
the GraphQL result. The audit must record the complete `kin-openapi` totals of
151 releases and 151 tags across two GraphQL pages and the complete one-page
counts for every HashiCorp module. It must read the v0.146.0 and v0.147.0
validation changes and the `terraform-plugin-log` v0.11.0 minimum-Go change.
If another newer direct module appears before implementation, stop and add its
release delta and migration work to this plan before changing `go.mod`.

- [x] **Step 4: Mark execution in progress**

Change P10-03 in `docs/plan.md` from `[ ]` to `[~]` and append the exact Bead
identifier. Check documentation whitespace:

```bash
git diff --check
bd update tfp-bqt.3 --append-notes "Evidence audit completed; implementation started from $(git rev-parse HEAD)."
```

- [x] **Step 5: Stage the evidence for the final atomic commit**

```bash
git add docs/audit/AUDIT-02-go-1.27-toolchain.md docs/plan.md
# These paths enter the post-gate candidate commit before review.
```

## Task 2: Make the Go image tag and digest drift check exact

**Files:**

- Modify: `test/scripts/test_tool_versions.py`
- Modify: `scripts/checks/tool_versions.py`

- [x] **Step 1: Write the failing tests**

Replace the test fixture's digest-only expectation with full-reference
expectations. Add cases proving that:

- the same digest with the wrong Go tag is rejected;
- the same tag with the wrong digest is rejected;
- a short `golang:1.27-trixie` reference is rejected even when its digest
  matches;
- malformed, short, non-hex and trailing-suffix digests fail closed;
- ordinary non-image version pins retain their existing exact comparison.

Fixtures use exactly 64 hexadecimal digest characters. YAML quotes and a pin
comment remain valid delimiters, but neither can become part of the reference.

- [x] **Step 2: Run the focused test and observe RED**

```bash
DEV_SUFFIX=-go-1-27 podman-compose -f deployments/compose/compose.dev.yml \
  exec -T dev uv run --locked pytest test/scripts/test_tool_versions.py -q
```

Observed RED included short-name acceptance and trailing-reference boundary
mutations before the fail-closed parser correction.

- [x] **Step 3: Implement the smallest parser correction**

Keep the full image reference for `GO_IMAGE`, require the literal
`docker.io/library/golang:` registry path and continue returning plain version
strings for all other ARGs. Do not introduce a general OCI parser into this PR.

Use this focused implementation:

```python
GO_IMAGE_REF = re.compile(
    r"(?<![A-Za-z0-9._/-])"
    r"docker\.io/library/golang:[^\s@'\"]+@sha256:[0-9a-f]{64}"
    r"(?=$|[\s'\"])"
)


def normalise_go_image(value: str) -> str | None:
    """Return one canonical Docker Hub Go image reference from text."""
    match = GO_IMAGE_REF.search(value)
    if match is None:
        return None
    return match.group(0)
```

In `declared_versions`, assign `normalise_go_image(value)` for `GO_IMAGE` and
the existing value for all other names. In `satisfies`, compare
`normalise_go_image(code)` with the expected image when `value` begins with
`docker.io/library/golang:`; retain exact substring comparison for plain
versions.

- [x] **Step 4: Run focused and Python gates**

```bash
DEV_SUFFIX=-go-1-27 podman-compose -f deployments/compose/compose.dev.yml \
  exec -T dev uv run --locked pytest test/scripts/test_tool_versions.py -q
task py
```

Expected: all tests and the Python gate pass.

- [x] **Step 5: Stage the parser correction for the final atomic commit**

```bash
git add scripts/checks/tool_versions.py test/scripts/test_tool_versions.py
# These paths enter the post-gate candidate commit before review.
```

## Task 3: Add JSON compatibility characterizations

**Files:**

- Modify: `internal/api/transport/client_test.go`

- [x] **Step 1: Add exact request and response tests**

Add `io` to the test imports and add these tests. They deliberately use the v1
API and assert behavior, not error-message text:

```go
func TestDo_EncodesRequestJSON(t *testing.T) {
	t.Parallel()

	gotBody := make(chan string, 1)
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("ReadAll: %v", err)
		}
		gotBody <- string(raw)
		w.WriteHeader(http.StatusNoContent)
	}))

	body := struct {
		Name string `json:"name"`
		TTL  int    `json:"ttl"`
	}{Name: "example.com.", TTL: 300}
	if err := client.Do(context.Background(), "create zone", http.MethodPost, "/zones", body, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if got := <-gotBody; got != `{"name":"example.com.","ttl":300}` {
		t.Errorf("body = %q", got)
	}
}

func TestDo_PreservesJSONV1ResponseSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body []byte
		want string
	}{
		{"duplicate name keeps last", []byte(`{"name":"first","name":"last"}`), "last"},
		{"invalid UTF-8 is replaced", []byte{'{', '"', 'n', 'a', 'm', 'e', '"', ':', '"', 0xff, '"', '}'}, "\ufffd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(tt.body)
			}))
			var out struct{ Name string `json:"name"` }
			if err := client.Do(context.Background(), "get zone", http.MethodGet, "/zone", nil, &out); err != nil {
				t.Fatalf("Do: %v", err)
			}
			if out.Name != tt.want {
				t.Errorf("Name = %q, want %q", out.Name, tt.want)
			}
		})
	}
}

func TestDo_NoContentDoesNotDecode(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	out := struct{ Name string }{Name: "unchanged"}
	if err := client.Do(context.Background(), "delete zone", http.MethodDelete, "/zone", nil, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if out.Name != "unchanged" {
		t.Errorf("Name = %q, want unchanged", out.Name)
	}
}
```

- [x] **Step 2: Add the bounded error-body test**

```go
func TestDo_BoundsAnOversizedErrorBody(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(strings.Repeat("x", maxErrorBody+1024)))
	}))
	err := client.Do(context.Background(), "list zones", http.MethodGet, "/zones", nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T, want *APIError", err)
	}
	if len(apiErr.ServerMessage) != maxErrorBody {
		t.Errorf("message length = %d, want %d", len(apiErr.ServerMessage), maxErrorBody)
	}
}
```

- [x] **Step 3: Run the characterization tests on the pre-migration Go toolchain**

Run:

```bash
task test:run RUN='TestDo_(EncodesRequestJSON|PreservesJSONV1ResponseSemantics|NoContentDoesNotDecode|BoundsAnOversizedErrorBody)' PKG=./internal/api/transport
```

Expected: PASS. These tests lock the documented v1 compatibility surface; no
production change is authorized by this task.

- [x] **Step 4: Run the complete transport package**

```bash
DEV_SUFFIX=-go-1-27 podman-compose -f deployments/compose/compose.dev.yml \
  exec -T dev go test ./internal/api/transport -race -count=1 -v
```

Expected: PASS.

- [x] **Step 5: Stage the JSON tests for the final atomic commit**

```bash
git add internal/api/transport/client_test.go
# These paths enter the post-gate candidate commit before review.
```

## Task 4: Add TLS compatibility tests

**Files:**

- Create: `internal/api/transport/tls_test.go`

- [x] **Step 1: Add hermetic server-trust tests**

Use `httptest.NewTLSServer`. Encode `srv.Certificate().Raw` as a PEM
certificate and assert three explicit cases: the default client rejects it,
`CACertificate` trusts it, and `InsecureSkipVerify` trusts it only when true.
Also assert that `CACertificate: []byte("not PEM")` makes `New` return an error
matching `ErrInvalidConfig`. The complete helper is:

```go
func serverCertificatePEM(t *testing.T, srv *httptest.Server) []byte {
	t.Helper()
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
}

func doNoContent(t *testing.T, cfg Config) error {
	t.Helper()
	client, err := New(cfg)
	if err != nil {
		return err
	}
	return client.Do(context.Background(), "tls probe", http.MethodGet, "/", nil, nil)
}
```

Implement `TestNew_ServerTrust` as a table over default, custom CA and explicit
insecure mode, using `errors.Is` only for the invalid-configuration case.

```go
func TestNew_ServerTrust(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	ca := serverCertificatePEM(t, srv)

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"untrusted by default", Config{BaseURL: srv.URL, Attempts: 1}, true},
		{"supplied CA", Config{BaseURL: srv.URL, CACertificate: ca, Attempts: 1}, false},
		{"explicit insecure", Config{BaseURL: srv.URL, InsecureSkipVerify: true, Attempts: 1}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := doNoContent(t, tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}

func TestNew_InvalidCA(t *testing.T) {
	t.Parallel()
	_, err := New(Config{BaseURL: "https://example.invalid", CACertificate: []byte("not PEM")})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}
```

- [x] **Step 2: Add mTLS and protocol-floor tests**

Generate an Ed25519 test CA and client leaf entirely in memory with
`x509.CreateCertificate`; return a `tls.Certificate` plus a pool containing
the CA. Configure an unstarted TLS server with
`ClientAuth: tls.RequireAndVerifyClientCert`. Assert a handshake failure without
`ClientCert` and success with it. In a second table, configure the server with
`MinVersion == MaxVersion` for TLS 1.1, 1.2 and 1.3; expect 1.1 to fail and 1.2
and 1.3 to succeed. The certificate helper signature is fixed as:

```go
func newClientIdentity(t *testing.T) (tls.Certificate, *x509.CertPool)
```

It must use `crypto/rand`, `crypto/ed25519`, `math/big`, UTC-valid certificate
times, `IsCA: true` for the root and `ExtKeyUsageClientAuth` for the leaf. No
file, host trust store or environment variable is allowed.

Use this complete identity helper and test structure:

```go
func newClientIdentity(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	now := time.Now().UTC()
	caPublic, caPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey CA: %v", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, caPublic, caPrivate)
	if err != nil {
		t.Fatalf("CreateCertificate CA: %v", err)
	}
	leafPublic, leafPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey leaf: %v", err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caTemplate, leafPublic, caPrivate)
	if err != nil {
		t.Fatalf("CreateCertificate leaf: %v", err)
	}
	leafPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	keyPKCS8, err := x509.MarshalPKCS8PrivateKey(leafPrivate)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyPKCS8})
	identity, err := tls.X509KeyPair(leafPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}
	caCertificate, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("ParseCertificate CA: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(caCertificate)
	return identity, pool
}

func TestNew_MutualTLS(t *testing.T) {
	t.Parallel()
	identity, clientCAs := newClientIdentity(t)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	srv.TLS = &tls.Config{
		MinVersion: tls.VersionTLS12,
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  clientCAs,
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	without := Config{BaseURL: srv.URL, InsecureSkipVerify: true, Attempts: 1}
	if err := doNoContent(t, without); err == nil {
		t.Fatal("handshake without a client certificate succeeded")
	}
	with := without
	with.ClientCert = &identity
	if err := doNoContent(t, with); err != nil {
		t.Fatalf("handshake with a client certificate: %v", err)
	}
}
```

For `TestNew_TLSVersionFloor`, start one unstarted server per table row with
`MinVersion` and `MaxVersion` both set to the row's version, trust its generated
certificate through `serverCertificatePEM`, and assert errors for TLS 1.1 only.

```go
func TestNew_TLSVersionFloor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		version uint16
		wantErr bool
	}{
		{"TLS 1.1 rejected", tls.VersionTLS11, true},
		{"TLS 1.2 accepted", tls.VersionTLS12, false},
		{"TLS 1.3 accepted", tls.VersionTLS13, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))
			srv.TLS = &tls.Config{MinVersion: tt.version, MaxVersion: tt.version}
			srv.StartTLS()
			t.Cleanup(srv.Close)
			cfg := Config{
				BaseURL:       srv.URL,
				CACertificate: serverCertificatePEM(t, srv),
				Attempts:      1,
			}
			err := doNoContent(t, cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}
```

- [x] **Step 3: Run focused tests and fail closed**

```bash
task test:run RUN='TestNew_(ServerTrust|InvalidCA|MutualTLS|TLSVersionFloor)' PKG=./internal/api/transport
```

Expected: PASS. If any characterization fails, stop, append the failure to
`tfp-bqt.3`, amend this plan with an exact production path and RED/GREEN steps,
and repeat plan review before changing production code.

- [x] **Step 4: Stage the TLS tests for the final atomic commit**

```bash
git add internal/api/transport/tls_test.go
# These paths enter the post-gate candidate commit before review.
```

## Task 5: Update direct modules and the atomic Go toolchain boundary

**Files:**

- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `internal/api/transport/client_test.go`
- Modify: `internal/provider/normalise/normalise_test.go`
- Modify: `deployments/containers/Containerfile.dev`
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/security.yml`
- Modify: `.github/workflows/release.yml`
- Verify: `.github/workflows/acceptance.yml`
- Verify: `.github/workflows/coverage.yml`
- Verify: `.github/workflows/e2e.yml`

- [x] **Step 1: Update the two verified direct modules on the pre-migration toolchain**

```bash
task test:contract
task test
DEV_SUFFIX=-go-1-27 podman-compose -f deployments/compose/compose.dev.yml \
  exec -T dev go get github.com/getkin/kin-openapi@v0.147.0
DEV_SUFFIX=-go-1-27 podman-compose -f deployments/compose/compose.dev.yml \
  exec -T dev go get github.com/hashicorp/terraform-plugin-log@v0.11.0
DEV_SUFFIX=-go-1-27 podman-compose -f deployments/compose/compose.dev.yml \
  exec -T dev go mod tidy
git add go.mod go.sum
DEV_SUFFIX=-go-1-27 podman-compose -f deployments/compose/compose.dev.yml \
  exec -T dev go mod tidy
git diff --exit-code -- go.mod go.sum
task test:contract
task test
```

Expected: both baseline and updated suites pass and a second tidy has no
unstaged delta. Inspect and explain every indirect-module change.

- [x] **Step 2: Stage the attributable dependency delta**

```bash
# These paths enter the post-gate candidate commit before review.
```

At this intermediate diagnostic point the `go` directive remained on the
pre-migration toolchain. It is not a published project state.

- [x] **Step 3: Write but do not commit the version-sensitive tests**

Append this test to `client_test.go`:

```go
func TestDo_DrainsIgnoredSuccessBodyForConnectionReuse(t *testing.T) {
	t.Parallel()
	var remoteAddresses []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		remoteAddresses = append(remoteAddresses, r.RemoteAddr)
		mu.Unlock()
		_, _ = w.Write([]byte(strings.Repeat("x", 1024)))
	}))
	t.Cleanup(srv.Close)
	client, err := New(Config{BaseURL: srv.URL, Attempts: 1})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for range 2 {
		if err := client.Do(context.Background(), "probe", http.MethodGet, "/", nil, nil); err != nil {
			t.Fatalf("Do: %v", err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(remoteAddresses) != 2 || remoteAddresses[0] != remoteAddresses[1] {
		t.Fatalf("connections = %v, want one reused connection", remoteAddresses)
	}
}
```

Add `sync` to the imports. Do not stage or commit this test while it is RED.

Also add `unicode` to `normalise_test.go` and append this test:

```go
func TestUnicode17Tables(t *testing.T) {
	t.Parallel()
	const tolongSikiLetterA = '\U00011DB4'
	if !unicode.IsLetter(tolongSikiLetterA) {
		t.Fatal("U+11DB4 must be classified as a letter by Unicode 17")
	}
	if !normalise.DNSName("\U00011DB4.example", "\U00011DB4.example.") {
		t.Fatal("DNSName must preserve the new letter and apply the trailing-dot rule")
	}
}
```

U+11DB4 is TOLONG SIKI LETTER A, newly assigned with category `Lo` in Unicode
17. The first assertion distinguishes the standard-library tables; the second
guards provider byte preservation. Keep both tests uncommitted until GREEN.

- [x] **Step 4: Run the response-drain test before Go 1.27 and observe RED**

```bash
task test:run RUN='TestDo_DrainsIgnoredSuccessBodyForConnectionReuse|TestUnicode17Tables' PKG=./internal/...
```

Expected: `TestUnicode17Tables` fails because Go 1.26.5 treats U+11DB4 as
unassigned. The HTTP test may also fail with different remote addresses. Record
both exact results; do not fabricate the optional HTTP failure.

- [x] **Step 5: Update the exact toolchain and analyzers together**

Set:

```text
go 1.27.0
GO_IMAGE=docker.io/library/golang:1.27-trixie@sha256:6212da3924947f4b6a939df02ea627c13f338f1a41d6c3fcb0dd9d076eef46c4
GOLANGCI_LINT_VERSION=v2.13.1
GOVULNCHECK_VERSION=v1.7.0
```

Do not add a matching `toolchain go1.27.0` directive. Go 1.27 treats it as
implicit, `go mod tidy -diff` removes it, and `go build` rejects the resulting
non-tidy module. Compiler identity is enforced by the immutable OCI image,
`GOTOOLCHAIN=local`, runtime `GOVERSION` parity, and workflow parity checks.

Apply the complete Go image reference to both containerized image jobs in
`ci.yml`, the image job in `security.yml`, and the image job in `release.yml`.
Confirm with this exact query that every non-containerized compiling workflow
derives Go from `go.mod` through commit-SHA-pinned `actions/setup-go` and
therefore moves atomically without a second version literal:

```bash
rg -n 'setup-go|go-version-file|GO_IMAGE|golang:' .github/workflows/{acceptance,ci,coverage,e2e,release,security}.yml
```

Expected: `acceptance.yml`, `coverage.yml` and `e2e.yml` use
`go-version-file: go.mod`; no Go 1.26 image remains in any workflow. Do not
change any other tool ARG.

- [x] **Step 6: Rebuild and verify tool identities**

```bash
task up
DEV_SUFFIX=-go-1-27 podman-compose -f deployments/compose/compose.dev.yml exec -T dev go version
DEV_SUFFIX=-go-1-27 podman-compose -f deployments/compose/compose.dev.yml exec -T dev golangci-lint version
DEV_SUFFIX=-go-1-27 podman-compose -f deployments/compose/compose.dev.yml exec -T dev govulncheck -version
DEV_SUFFIX=-go-1-27 podman-compose -f deployments/compose/compose.dev.yml exec -T dev gopls version
```

Expected: Go 1.27.0, golangci-lint 2.13.1, govulncheck 1.7.0 and gopls
v0.23.0.

- [x] **Step 7: Re-run the response-drain test and observe GREEN**

```bash
task test:run RUN='TestDo_DrainsIgnoredSuccessBodyForConnectionReuse|TestUnicode17Tables' PKG=./internal/...
```

Expected: PASS on Go 1.27.0.

- [x] **Step 8: Resolve the module graph idempotently**

```bash
DEV_SUFFIX=-go-1-27 podman-compose -f deployments/compose/compose.dev.yml exec -T dev go mod tidy
git add go.mod go.sum
DEV_SUFFIX=-go-1-27 podman-compose -f deployments/compose/compose.dev.yml exec -T dev go mod tidy
git diff --exit-code -- go.mod go.sum
```

Expected: the second tidy creates no unstaged delta.

- [x] **Step 9: Run focused Go and supply-chain gates**

```bash
task build
task test
DEV_SUFFIX=-go-1-27 podman-compose -f deployments/compose/compose.dev.yml exec -T dev go vet ./...
task lint
task vulncheck
task osv-scan
task lint:tools
task lint:pins
```

Expected: every command passes; direct vet and `go test` both exercise the Go
1.27 `stdversion` analyzer.

- [x] **Step 10: Ask gopls for live semantic evidence**

```bash
DEV_SUFFIX=-go-1-27 podman-compose -f deployments/compose/compose.dev.yml \
  exec -T dev gopls check internal/api/transport/client.go
DEV_SUFFIX=-go-1-27 podman-compose -f deployments/compose/compose.dev.yml \
  exec -T dev gopls references internal/api/transport/client.go:35:6
```

Expected: no diagnostics; `transport.Config` references include provider
construction and tests.

- [x] **Step 11: Stage the now-green toolchain boundary**

```bash
git add go.mod go.sum internal/api/transport/client_test.go \
  internal/provider/normalise/normalise_test.go \
  deployments/containers/Containerfile.dev .github/workflows/ci.yml \
  .github/workflows/security.yml .github/workflows/release.yml
# These paths enter the post-gate candidate commit before review.
```

## Task 6: Update the active Go contract and release documentation

**Files:**

- Rename: `docs/standards/go-1.26-style.md` to
  `docs/standards/go-1.27-style.md`
- Modify: `AGENTS.md`
- Modify: `README.md`
- Modify: `docs/README.md`
- Modify: `docs/development.md`
- Modify: `docs/standards/versioning.md`
- Modify: `docs/standards/go-1.27-style.md`
- Modify: `CHANGELOG.md`
- Modify: `docs/plan.md`

- [x] **Step 1: Rename only the active standard**

Rename the single standard file and update active links. Do not rewrite
historical ADRs, released changelog entries or completed plan rows that
correctly describe Go 1.26 at that time.

- [x] **Step 2: Rewrite the active compatibility contract**

Update the exact Go and analyzer pins, image reference, badge and reading list.
Document generic-method/interface restrictions from the spec, continued JSON
v1 use, bounded HTTP close draining, hermetic CA tests, the macOS 13 minimum,
Unicode 17 and mandatory `stdversion` vet. Add an `[Unreleased]` Changed entry.

- [x] **Step 3: Keep public status active until final review**

Keep P10-03 `[~]`; add the implementation branch and Bead reference without
claiming success.

- [x] **Step 4: Run documentation and pin checks**

```bash
git diff --check
task docs:lint
task lint:tools
task lint:pins
rg -n '1\.26\.5|go-1\.26-style|golang:1\.26|v2\.12\.2|v1\.6\.0' \
  AGENTS.md README.md docs deployments .github scripts test go.mod
```

Expected: only inspected historical references remain.

- [x] **Step 5: Stage the documentation for the final atomic commit**

```bash
git add AGENTS.md README.md CHANGELOG.md docs
# These paths enter the post-gate candidate commit before review.
```

## Task 7: Run full release and live compatibility gates

**Files:**

- Modify: `docs/audit/AUDIT-02-go-1.27-toolchain.md`
- Modify: `docs/plan.md`

- [x] **Step 1: Run non-lab and release gates**

```bash
task all
task osv-scan
task release:dryrun
```

Expected: all pass. Record exact summaries, counts and durations. A Linux
cross-build of Darwin artifacts is not a claim that they ran on macOS.

- [x] **Step 2: Run both authoritative release branches**

```bash
task lab:up AUTH=5.1
task verify AUTH=5.1
task lab:down AUTH=5.1
task lab:up AUTH=5.0
task verify AUTH=5.0
task lab:down AUTH=5.0
```

These are only the disposable project lab objects created by the paired
`lab:up`. Quote the acceptance-test counts for both branches.

- [x] **Step 3: Record and stage gate evidence while status remains active**

Append exact outputs to `AUDIT-02`, keep P10-03 `[~]`, and create the post-gate
candidate commit before requesting review:

```bash
bd update tfp-bqt.3 --append-notes \
  "Full gates passed; exact counts and durations are recorded in AUDIT-02."
git add docs/audit/AUDIT-02-go-1.27-toolchain.md docs/plan.md
git commit
```

Steps 1 through 3 passed on the pre-review candidate. The independent review
found important ownership and parser gaps. After those fixes, the complete
post-fix sequence passed again in isolated project
`terraform-provider-powerdns-dev-go127-recreate-review`: `task all`, OSV,
release dry run, immutable image inspection and both Auth-branch `task verify`
runs. Specification review approved exact candidate
`8cb86b63f891a4cc2e7159c9cf2d061f8f9b4c73`; quality review then found two
Important gaps: basename-only worktree identities could collide, and cleanup
validation discarded `||` semantics. The focused remediation uses a bounded
sanitized basename plus canonical-root SHA-256 suffix, implemented by a POSIX
helper checked by shellcheck so Task cannot pre-expand its locals, and requires
an exact final cleanup in a pure AND-list. The user approved the newly derived
automatic disposable names recorded in `AUDIT-02`; exact lifecycle validation,
the full non-lab sequence, immutable OCI inspection and both Auth branches have
all passed. The new evidence candidate must now be created before Step 4.

Specification review of exact candidate `e444770350b938e30d7dc63426c014b84c9b0da6`
then found one Important fail-open hash pipeline. A failed or malformed
`sha256sum` could produce an empty or invalid worktree digest because POSIX
`set -e` follows the pipeline's final command. TDD now requires the helper to
check the hasher status, validate one exact 64-character lowercase hexadecimal
field and only then truncate it. GNU coreutils `sha256sum` is an explicit host
prerequisite. Steps 1 through 3 were reopened and passed: the exact automatic
lifecycle, Task 7 gates, OSV, release dry run, immutable OCI inspection and both
Auth branches are green again. The exact teardown left the authorized
automatic project absent, its image preserved and pre-existing objects
unchanged. The amended evidence candidate was committed as
`995309ba8edae941738f083a1a3c72c9e1b3851c`. Fresh specification review
approved that exact HEAD. Only then did fresh quality review approve the same
HEAD. No Git or Podman state changed between the reviews. Both Beads remained
`IN_PROGRESS`; approval notes were appended only after both approvals and
before the closure-docs commit.

Closure commit `330bdb5170017b27d29a3830e571a666bd05609d` then recorded those
outcomes with the repository-valid subject
`docs(docs): complete the go 1.27 toolchain migration`. Final specification
review did not approve that HEAD: the audit overstated Beads immutability after
the candidate approvals, and this plan recorded the rejected `docs(plan)`
subject rather than the successful command. Per Step 6, P10-03 and Steps 4
through 6 return to active state; Steps 1 through 3 must repeat in full before
a replacement candidate is created.

The documentation findings were corrected and Steps 1 through 3 repeated in
full. `task all`, OSV and the 13-archive release dry run exited zero; immutable
OCI and mounted-rootfs inspection remained clean. Auth 5.1.3 and Auth 5.0.6
each passed 35 acceptance tests with the one documented `api_dir` case
intentionally skipped, alongside Recursor 5.4.4 and dnsdist 2.1.0. Both paired
down commands left lab containers, volumes and network absent. The existing
isolated verifier stayed running and unchanged. P10-03 remains `[~]`; the
replacement evidence candidate now awaits a fresh Step 4 specification review.

- [ ] **Step 4: Review the exact post-gate candidate sequentially**

First dispatch a specification reviewer against the design, this plan and the
exact post-gate candidate HEAD. Only after specification approval, dispatch a
separate quality reviewer against the same `origin/main...HEAD`. Do not change
Git between these reviews. If either review reports a Critical or Important
finding, keep P10-03 `[~]`, fix it, run its focused test, repeat Task 7 Steps 1
through 3 in full, create a new evidence commit and restart Step 4 with
specification review. The candidate-review loop ends only when both reviewers
approve the same evidence HEAD.

- [ ] **Step 5: Create the closure-docs commit after candidate approval**

After both candidate reviewers approve the same HEAD, add their exact outcomes
to `AUDIT-02`, set P10-03 to `[x]`, and create one docs-only closure commit.
Keep `tfp-bqt.2.1` and `tfp-bqt.3` open: closing Beads is external lifecycle
state and must not create an unreviewed Git state.

```bash
git add docs/audit/AUDIT-02-go-1.27-toolchain.md docs/plan.md \
  docs/superpowers/plans/2026-08-21-go-1-27-toolchain-plan.md
git commit -m "docs(docs): complete the go 1.27 toolchain migration"
```

- [ ] **Step 6: Review the exact closure HEAD sequentially**

Dispatch specification review of the exact closure HEAD, then quality review
of that same HEAD only after specification approval. Make no Git changes after
the final approval. A Critical or Important finding at either closure review
returns the work to P10-03 `[~]`: fix the finding, repeat Task 7 Steps 1 through
3, create a new evidence commit, and restart Step 4. This rule makes every
review loop terminate at one exact, fully gated Git object.

- [ ] **Step 7: Close Beads and verify final repository state**

Only after both reviewers approve the exact closure HEAD, close
`tfp-bqt.2.1` and `tfp-bqt.3` externally, then verify that this changed no Git
content:

```bash
bd close tfp-bqt.2.1
bd close tfp-bqt.3
git status --short
git diff --check origin/main...HEAD
git diff --stat origin/main...HEAD
bd lint
bd dep cycles
git remote -v
```

Expected: clean worktree, no diff errors or Beads cycles, and the sole remote
is `origin` at `https://github.com/ioplane/terraform-provider-powerdns.git`.

- [ ] **Step 8: Create the pull-request body, push and open the PR**

Use `apply_patch` to create `/tmp/tfp-bqt-3-pr-body.md` with this structure,
replacing each bracketed value with the exact `AUDIT-02` result:

```markdown
## Summary

- move the complete Go build surface to Go 1.27.0
- update golangci-lint to 2.13.1 and govulncheck to 1.7.0
- update kin-openapi to 0.147.0 and terraform-plugin-log to 0.11.0

## Verification

- `task all`: [exact result]
- `task osv-scan`: [exact result]
- `task release:dryrun`: [exact result]
- PowerDNS Auth 5.1 acceptance: [N/N pass]
- PowerDNS Auth 5.0 acceptance: [N/N pass]

## Evidence

- Go release and module GraphQL evidence: `docs/audit/AUDIT-02-go-1.27-toolchain.md`
- Beads: `tfp-bqt.2.1`, `tfp-bqt.3`
```

Then run:

```bash
git push -u origin fix/containers/go-module-cache
gh pr create --repo ioplane/terraform-provider-powerdns \
  --base main --head fix/containers/go-module-cache \
  --title "fix(build): isolate the Go 1.27 toolchain" \
  --body-file /tmp/tfp-bqt-3-pr-body.md
```

Verify the returned PR URL and checks with `gh pr view --json url,headRefName,baseRefName,statusCheckRollup`.

<!-- markdownlint-enable MD010 -->
