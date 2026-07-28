# ADR 0006 — dnsdist is in scope, sized by what its API permits

- **Status:** accepted
- **Date:** 2026-07-28
- **Deciders:** PM, architect
- **Supersedes:** the identically numbered decision in the discontinued fork,
  which put dnsdist out of scope entirely

## Context

The earlier decision excluded dnsdist because its HTTP API writes almost
nothing. That reasoning conflated two different things: a **thin write
surface** and **out of scope**.

Three facts, all verified rather than assumed:

1. dnsdist authenticates with `X-API-Key` on a web server, exactly as
   Authoritative and Recursor do. The credential model is common to the family.
2. Its API exposes 10 operations. Two write:
   `PUT /api/v1/servers/localhost/config/allow-from` and
   `DELETE /api/v1/cache`.
3. Rules, pools, downstream servers and dynamic blocks are configured in Lua or
   YAML and are not reachable over HTTP at all.

## Decision

dnsdist is **in scope**, with a surface sized to what the API permits: one
resource, four data sources, one action. Nothing more, because nothing more
exists.

| Kind | Name | Operation |
|---|---|---|
| Resource | `powerdns_dnsdist_acl` | `PUT /config/allow-from` |
| Action | `powerdns_dnsdist_flush_cache` | `DELETE /api/v1/cache` |
| Data source | `powerdns_dnsdist_server` | `GET /servers/localhost` |
| Data source | `powerdns_dnsdist_statistics` | `GET /statistics` |
| Data source | `powerdns_dnsdist_pool` | `GET /pool` |
| Data source | `powerdns_dnsdist_rings` | `GET /rings` |

## Two findings that only appeared on a running server

Both were invisible in the documentation and would have produced a broken
resource.

**`setAPIWritable` is the gate, not `apiConfigDir`.** `isMethodAllowed()` in
`dnsdist-web.cc` checks `d_apiReadWrite` **before** it looks at the path, so
without `setAPIWritable(true, dir)` every `PUT` answers `405` regardless of
what `setWebserverConfig` contains. Setting `apiConfigDir` alone is not enough:

```console
# setWebserverConfig{apiConfigDir=…} only
PUT /api/v1/servers/localhost/config/allow-from   405

# setAPIWritable(true, "/var/lib/dnsdist")
PUT /api/v1/servers/localhost/config/allow-from   200
{"name": "allow-from", "type": "ConfigSetting", "value": ["10.0.0.0/8", "127.0.0.1/32"]}
```

**`DELETE /api/v1/cache` needs a packet cache to exist.** With no cache on the
pool it answers `404`, which reads as "the endpoint is missing" and is really
"the pool has no cache":

```console
# no packet cache
DELETE /api/v1/cache?pool=&name=example.com.   404

# newPacketCache(10000); getPool(""):setCache(pc)
DELETE /api/v1/cache?pool=&name=example.com.   200 {"count": "0", "status": "purged"}
```

The provider must therefore translate a `404` from the cache action into a
diagnostic naming the missing packet cache, and its documentation must state
the `setAPIWritable` requirement. Both are the same pattern as the LMDB and
`api_dir` requirements on the other two products.

## Consequences

- The lab grows a fifth service. dnsdist's two write operations cannot be
  exercised anywhere else.
- The provider covers the whole PowerDNS family, which nothing else in the
  ecosystem does.
- Supporting dnsdist will attract requests to manage rules and pools. The
  answer is in the resource documentation, with the numbers: 10 operations, two
  of them writes. That is a property of dnsdist, not of this provider.
- If dnsdist grows a real configuration API, the surface expands and this
  record is superseded rather than edited.
