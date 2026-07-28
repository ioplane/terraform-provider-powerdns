# Go 1.26 patterns, antipatterns, standards

Project Go standard, pinned to **Go 1.26.5** (dev container
`golang:1.26-trixie`). Aligned with the
[Go 1.26 release notes](https://go.dev/doc/go1.26).

Before writing Go, consult the current API through the `gopls` LSP and the
`context7` MCP. Do not rely on training-data recall for library signatures.

## 1. Language and stdlib features adopted

| Feature (Go 1.26) | Pattern here | Rationale |
|---|---|---|
| `new(expr)` accepting an expression | `new("Native")` for `*string` schema defaults | Drops the `v := …; &v` ceremony. Flagged by `modernize`. |
| `errors.AsType[T]()` | `if e, ok := errors.AsType[*APIError](err); ok {…}` | Type-safe matching on the PowerDNS error type without an out-parameter. |
| `reflect.Type.Fields()` / `.Methods()` iterators | schema and codegen tooling only | Never in CRUD hot paths. |
| `net/http.ClientConn` | connection reuse in the PowerDNS client if profiling justifies it | The provider is HTTP-bound; measure before adopting. |
| `net.Dialer.DialTCP` taking `netip.AddrPort` | address handling in the client | Avoids a string round-trip for parsed addresses. |
| Green Tea GC (default) | no code change | Lower GC overhead. |
| Heap address randomisation (default) | no code change | Mitigates address-prediction attacks. |

`new(expr)` is genuinely useful in a provider: schema defaults and optional
API-payload fields are pointer-typed everywhere.

## 2. Provider-specific conventions

| Area | Rule |
|---|---|
| Logging | `github.com/hashicorp/terraform-plugin-log/tflog` only. `depguard` denies `^log$`; `forbidigo` denies `fmt.Print*` outside `main.go` and `scripts/`. |
| Errors surfaced to Terraform | `resp.Diagnostics.AddError` / `AddAttributeError`. Never `panic`, never a bare returned string. |
| Internal errors | Package-level sentinels `var ErrXxx = errors.New("…")`; wrap with `fmt.Errorf("…: %w", err)` (`err113`, `errorlint`). |
| HTTP status handling | **Every** response has its status code examined before the body is decoded. Decoding first and hoping is the shape of defect D-08. |
| Context | `context.Context` is the first parameter of every API and CRUD helper; honour `ctx.Done()`; never store a context in a struct (`containedctx`). |
| Pointer receivers | Every method on a resource type uses a pointer receiver (`recvcheck`). |
| Framework types | Convert `types.String` / `types.Bool` at the boundary; never assume non-null — check `IsNull()` and `IsUnknown()`. |
| Randomness | `crypto/rand` for anything secret; `math/rand/v2` otherwise. `depguard` denies `math/rand`. |
| TLS | Minimum TLS 1.2, preferring 1.3; leave the Go 1.26 post-quantum hybrid defaults on. `InsecureSkipVerify` only behind the documented `insecure_https` provider flag, never a code default. |
| Secrets | API keys, TSIG keys and DNSSEC private keys are `Sensitive`. Terraform state is **not** encrypted — a value that need not persist should use a write-only attribute or an ephemeral resource instead. |

## 3. Antipatterns banned

| Antipattern | Replacement | Enforced by |
|---|---|---|
| `v := "x"; p := &v` | `p := new("x")` | `modernize` |
| `var e *E; errors.As(err, &e)` | `errors.AsType[*E](err)` | review |
| `json.NewDecoder(resp.Body).Decode(&v)` without checking `resp.StatusCode` | check status, then decode | review; this is D-08 |
| `log.Printf` / `fmt.Println` in library code | `tflog.Debug/Info/Error` | `depguard`, `forbidigo` |
| `panic(...)` in CRUD | diagnostic and early return | `gocritic` |
| `strings.Split(addr, ":")` to split host and port | `net.SplitHostPort` | review; this is D-01 |
| Naked returns in non-trivial functions | explicit returns | `nakedret max-func-lines: 0` |
| `math/rand` for keys or tokens | `crypto/rand` | `depguard`, `gosec` |
| `tls.Config{InsecureSkipVerify: true}` as a default | CA bundle via `RootCAs` | `gosec` |
| `interface{}` where a concrete type fits | concrete type, or a small interface | `iface`, `interfacebloat` |
| Response body left unclosed | `defer resp.Body.Close()` with the error handled | `bodyclose` |

None of these is hypothetical. `strings.Split` on a host-port string and
decoding a body without checking the status are both defects observed in
existing PowerDNS providers, the first of which produced an open upstream
issue. They are listed because they are the mistakes this domain invites.

## 4. Mandatory standards

| Area | Standard |
|---|---|
| Module path | `github.com/ioplane/terraform-provider-powerdns`. |
| Go directive | `go 1.26.5`. |
| Layout | `internal/provider`, `internal/resources/<area>`, `internal/client/pdns`. Framework resources are unexported under `internal/`. |
| Vendoring | None. `vendor/` is removed; the module cache and `go.sum` are the reproducibility mechanism. |
| Imports | `gofumpt` order; `goimports.local-prefixes` set to the module path. |
| Test names | `Test<Subject>_<Behaviour>`; acceptance `TestAcc<Resource>_<Case>`. |
| Test parallelism | `t.Parallel()` mandatory in unit tests; forbidden in acceptance tests that share a lab instance. Enforced by `paralleltest` with an exclusion for `test/acceptance/`. |
| Race detector | `-race` always, in `task test` and in CI. |
| Build tags | `//go:build acceptance` for lab-dependent tests. |
| Vulnerability gate | `govulncheck` and `osv-scanner`; an allow-listed advisory needs a documented reason. |

## 5. Tooling

| Tool | Version | Use |
|---|---|---|
| `go` | 1.26.5 | language and build |
| `gofmt` / `gofumpt` / `goimports` | bundled | format gate |
| `golangci-lint` | v2.12.2 | aggregate linter, allowlist mode |
| `gopls` | latest | LSP: references, rename, diagnostics |
| `govulncheck` | v1.6.0 | vulnerability gate |
| `osv-scanner` | v2.4.0 | vulnerability gate |
| `tfplugindocs` | v0.25.0 | registry documentation |
| `goreleaser` | v2.17.1 | signed release bundle |
| `gotestsum` | latest | JUnit reporting in CI |

## 6. Reading list

- [Go 1.26 release notes](https://go.dev/doc/go1.26) — authoritative.
- [Effective Go](https://go.dev/doc/effective_go) — baseline style.
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
  — the framework's own conventions win where this guide is silent.
