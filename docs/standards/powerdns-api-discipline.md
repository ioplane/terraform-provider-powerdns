<!-- markdownlint-disable MD033 MD041 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=API+discipline&subtitle=How+to+establish+a+PowerDNS+fact&logo=powerdns&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="API discipline" src="https://shieldcn.dev/header/graph.svg?title=API+discipline&subtitle=How+to+establish+a+PowerDNS+fact&logo=powerdns&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>

<div align="center">

[![status normative](https://shieldcn.dev/badge/status-normative-cf222e.svg?variant=secondary)](../README.md)
[![sources over spec](https://shieldcn.dev/badge/sources-over_spec-0969da.svg?variant=secondary)](#)
[![enforced see the table](https://shieldcn.dev/badge/enforced-see_the_table-3fb950.svg?variant=secondary)](#)

</div>

# PowerDNS API discipline

Project-specific rules for making and checking claims about PowerDNS behaviour.
This exists because the obvious source — the published OpenAPI specification —
is demonstrably not reliable on its own.

## 1. The order of authority

| Rank | Source | Use |
|---|---|---|
| 1 | **`PowerDNS/pdns` sources** at the pinned tag | What the server actually does. Handler registration in `pdns/ws-auth.cc`, `pdns/recursordist/ws-recursor.cc`, `pdns/dnsdistdist/dnsdist-web.cc`. |
| 2 | **A live round-trip against the lab** | What it does in the deployment we target, on the backend we target. |
| 3 | **PowerDNS documentation** via the `context7` MCP | Intent, feature-support matrices per backend. |
| 4 | **The OpenAPI specification** | A starting index. Never the last word. |

Where 1 and 4 disagree, 1 wins and the disagreement is recorded.

## 2. Why the specification is not sufficient

Verified against `auth-5.1.3`, both from source and against a running server:

| Divergence | Specification | Reality |
|---|---|---|
| `GET /servers/{id}/config/{name}` | documented | no handler registered; `404` |
| `POST /zones/{id}/cryptokeys/{key_id}` | absent | registered at `ws-auth.cc:3349`; `400`, not `404` |
| `/api`, `/api/v1`, `/api/docs`, `/metrics` | absent | all registered; all `200` |

The document served by a live `GET /api/docs` is byte-identical to the one in
the repository at that tag, so this is not a packaging artefact — the
specification itself is both wider and narrower than the implementation.

Consequence: **do not generate a client from the specification**, and do not
treat "it is in the spec" as evidence that an endpoint exists.

## 3. A `200` is not proof of support

`GET /views` and `GET /networks` return `200` with an empty collection on a
backend that cannot store them. Reading the collection therefore cannot
distinguish "unsupported" from "not yet configured"; only a write can.

Any claim of the form "PowerDNS supports X" must be backed by a **write** that
succeeded, on the **specific backend** in question.

## 4. Backend and configuration preconditions

Known preconditions, all verified. Extend this table rather than rediscovering
its entries.

| Capability | Precondition | Symptom when unmet |
|---|---|---|
| views, networks | backend is LMDB | `422 Failed to add … to view` |
| recursor zone write | `webservice.api_dir` set | `422 Config Option "api-config-dir" must be set` |
| recursor config write | `webservice.api_dir` set **and** name ∈ {`allow-from`, `allow-notify-from`} | `422`, or `404` for any other name |
| DNSSEC cryptokeys | none — works on gpgsql | — |
| TSIG keys | none — works on gpgsql | — |
| autoprimaries | none — works on gpgsql | — |

The last three matter as much as the first: they establish that the absence of
DNSSEC, TSIG and autoprimary support in the provider is **not** explained by a
backend limitation.

## 5. Pinned reference points

| Component | Tag | Image |
|---|---|---|
| Authoritative | `auth-5.1.3` | `powerdns/pdns-auth-51:5.1.3` |
| Recursor | `rec-5.4.4` | `powerdns/pdns-recursor-54:5.4.4` |
| dnsdist | `dnsdist-2.1.0` | — |

When any of these moves, the affected claims are re-verified rather than
assumed still correct. `auth-5.2.0-alpha0` exists and is deliberately excluded.

## 6. How to cite

In a package comment or a commit body, name the source precisely:

```go
// PowerDNS rejects a view write on any backend other than LMDB: the gpgsql
// schema has no views table (schema.pgsql.sql, auth-5.1.3) and the handler
// returns 422 "Failed to add <zone> to view <view>". Verified against
// pdns-auth-51:5.1.3 on both backends.
```

An uncorroborated assumption is labelled as such and kept non-load-bearing.
Filling in a behaviour by analogy with a similar endpoint is not permitted —
`/views` and `/networks` differ in exactly this way, one having no `DELETE`
at all.

## 7. Known API shapes that surprise

| Shape | Note |
|---|---|
| No `DELETE` for a network | Removal is `PUT` with an empty `view`. The entry stays listed with an empty view — `terraform destroy` cannot fully remove it. |
| `PATCH /zones/{id}` does RRsets | Records are not their own endpoint; every record change is a zone patch with a `changetype`. |
| Server id is always `localhost` | Hard-coded in the handler registration. Not a limitation of the client. |
| `soa_edit` and `soa_edit_api` are different fields | Both exist on the zone object; neither implies the other. |
