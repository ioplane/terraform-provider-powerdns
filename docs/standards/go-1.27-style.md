<!-- markdownlint-disable MD013 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=Go+1.27+style&subtitle=Patterns%2C+antipatterns%2C+tooling&logo=go&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="Go 1.27 style" src="https://shieldcn.dev/header/graph.svg?title=Go+1.27+style&subtitle=Patterns%2C+antipatterns%2C+tooling&logo=go&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>
<!-- markdownlint-enable MD013 -->

<div align="center">

[![status normative](https://shieldcn.dev/badge/status-normative-cf222e.svg?variant=secondary)](../README.md)
![go 1.27.0](https://shieldcn.dev/badge/go-1.27.0-0969da.svg?variant=secondary)
![enforced see the table](https://shieldcn.dev/badge/enforced-see_the_table-3fb950.svg?variant=secondary)

</div>

# Go 1.27 patterns, antipatterns, standards

Project Go standard, pinned to **Go 1.27.0** in the digest-pinned dev container.
Aligned with the
[Go 1.27 release notes](https://go.dev/doc/go1.27) and the
[language specification](https://go.dev/ref/spec).

Before writing Go, consult the current API through the `gopls` LSP and the
`context7` MCP. Do not rely on training-data recall for library signatures.

## 1. Language and stdlib features adopted

| Feature (Go 1.27) | Pattern here | Rationale |
| --- | --- | --- |
| Generic methods | May declare their own type parameters; interface methods may not, and generic methods cannot implement interface methods. | Follows the normative method and method-set rules. |
| Generalized function inference | A generic function may infer type arguments when assigned or converted to a matching function type. | Removes redundant instantiation without weakening types. |
| Selector keys | A struct literal key may be any valid field selector for the struct type. | Covered by the Go 1.27 language contract test. |
| `stdversion` vet | Mandatory through both `go test` and explicit `go vet ./...`. | Rejects standard-library symbols newer than the module directive. |
| `encoding/json` v1 over v2 | Continue using the v1 API and lock duplicate-name and invalid-UTF-8 semantics. | Go 1.27 preserves v1 behavior while changing its implementation. |
| HTTP/1 bounded close drain | Ignored success bodies are closed and reuse is tested. | Prevents unnecessary connections without unbounded reads. |
| Unicode 17 | DNS normalization preserves newly assigned letters byte-for-byte. | Locks both the standard table and trailing-dot behavior. |

## 2. Provider-specific conventions

| Area | Rule |
| --- | --- |
| Logging | `github.com/hashicorp/terraform-plugin-log/tflog` only. `depguard` denies `^log$`; `forbidigo` denies `fmt.Print*` outside `main.go` and `scripts/`. |
| Errors surfaced to Terraform | `resp.Diagnostics.AddError` / `AddAttributeError`. Never `panic`, never a bare returned string. |
| Internal errors | Package-level sentinels `var ErrXxx = errors.New("…")`; wrap with `fmt.Errorf("…: %w", err)` (`err113`, `errorlint`). |
| HTTP status handling | **Every** response has its status code examined before the body is decoded. Decoding first and hoping is the shape of defect D-08. |
| Context | `context.Context` is the first parameter of every API and CRUD helper; honour `ctx.Done()`; never store a context in a struct (`containedctx`). |
| Pointer receivers | Every method on a resource type uses a pointer receiver (`recvcheck`). |
| Framework types | Convert `types.String` / `types.Bool` at the boundary; never assume non-null — check `IsNull()` and `IsUnknown()`. |
| Randomness | `crypto/rand` for anything secret; `math/rand/v2` otherwise. `depguard` denies `math/rand`. |
| TLS | Minimum TLS 1.2, preferring 1.3. Custom CA and mTLS tests are hermetic. `SystemCertPool` environment changes on Darwin and Windows are not simulated on Linux. `InsecureSkipVerify` only behind the documented provider flag. |
| Secrets | API keys, TSIG keys and DNSSEC private keys are `Sensitive`. Terraform state is **not** encrypted — a value that need not persist should use a write-only attribute or an ephemeral resource instead. |

## 3. Antipatterns banned

| Antipattern | Replacement | Enforced by |
| --- | --- | --- |
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
| --- | --- |
| Module path | `github.com/ioplane/terraform-provider-powerdns`. |
| Go directive | `go 1.27.0`. [ADR 0010](../adr/0010-go-1.27-development-toolchain.md) records why the matching implicit toolchain line is omitted and how exact execution is enforced. |
| Layout | `internal/provider` for the framework surface, `internal/api/{transport,auth,rec,dnsdist}` for the clients, `internal/testutil` for the contract layer. Everything unexported under `internal/`. |
| Vendoring | None. `vendor/` is removed; the module cache and `go.sum` are the reproducibility mechanism. |
| Imports | `gofumpt` order; `goimports.local-prefixes` set to the module path. |
| Test names | `Test<Subject>_<Behaviour>`; acceptance `TestAcc<Resource>_<Case>`. |
| Test parallelism | `t.Parallel()` mandatory in unit tests; forbidden in acceptance tests that share a lab instance. Enforced by `paralleltest`, relaxed for `_acc_test.go` — the suffix, not a directory: acceptance tests live beside the code they exercise. |
| Race detector | `-race` always, in `task test` and in CI. |
| Build tags | `//go:build acceptance` for lab-dependent tests. |
| Vulnerability gate | `govulncheck` and `osv-scanner`; an allow-listed advisory needs a documented reason. |

## 5. Tooling

| Tool | Version | Use |
| --- | --- | --- |
| `go` | 1.27.0 | language and build |
| `gofmt` / `gofumpt` / `goimports` | Go 1.27 / golangci-lint v2.13.1 formatters | format gate |
| `golangci-lint` | v2.13.1 | aggregate linter, allowlist mode |
| `gopls` | v0.23.0 | LSP: references, rename, diagnostics |
| `govulncheck` | v1.7.0 | vulnerability gate |
| `osv-scanner` | v2.4.0 | vulnerability gate |
| `tfplugindocs` | v0.25.0 | registry documentation |
| `goreleaser` | v2.17.1 | signed release bundle |
| `gotestsum` | v1.13.0 | JUnit reporting in CI |

## 6. Reading list

- [Go 1.27 release notes](https://go.dev/doc/go1.27) — release behavior.
- [Go language specification](https://go.dev/ref/spec) — normative language rules.
- [Effective Go](https://go.dev/doc/effective_go) — baseline style.
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
  — the framework's own conventions win where this guide is silent.
