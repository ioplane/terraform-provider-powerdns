<!-- markdownlint-disable MD013 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=Provider+contract&subtitle=What+is+promised%2C+and+for+how+long&logo=terraform&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="Provider contract" src="https://shieldcn.dev/header/graph.svg?title=Provider+contract&subtitle=What+is+promised%2C+and+for+how+long&logo=terraform&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>
<!-- markdownlint-enable MD013 -->

<div align="center">

![status normative](https://shieldcn.dev/badge/status-normative-cf222e.svg?variant=secondary)
[![semver 2.0.0](https://shieldcn.dev/badge/semver-2.0.0-0969da.svg?variant=secondary)](https://semver.org/spec/v2.0.0.html)
![stability pre--1.0](https://shieldcn.dev/badge/stability-pre--1.0-0969da.svg?variant=secondary)

</div>

# Provider contract

> **Normative.** What this provider promises to users, and what it does not.
> A change to anything on this page is a change to the contract and follows
> [`standards/versioning.md`](standards/versioning.md).

A Terraform provider's schema is not an implementation detail. It is written
into every user's state file, and renaming an attribute after release breaks
every configuration that used it. That is why the methodology
([`methodology.md`](methodology.md)) puts an architect's signature on the
schema before a resource is implemented, and why this page exists separately
from the plan: the plan records what was built, this records what was
promised.

## Contents

- [1. What is under contract](#1-what-is-under-contract)
- [2. Resource identity](#2-resource-identity)
- [3. Current surface](#3-current-surface)
- [4. Guarantees](#4-guarantees)
- [5. Non-goals](#5-non-goals)
- [6. Changing the contract](#6-changing-the-contract)

---

## 1. What is under contract

| Under contract | Not under contract |
| --- | --- |
| Resource and data source type names | Package layout under `internal/` |
| Attribute names, types and optionality | Diagnostic wording |
| Resource `id` format and import syntax | Log output at any level |
| Which attributes force replacement | Request ordering within one apply |
| Documented semantic comparisons | Retry timing and backoff |
| Provider argument names and their environment variables | Which HTTP endpoint implements an attribute |

The right-hand column is deliberately long. Everything in it can change in a
patch release, and nothing in it appears in a user's state file.

## 2. Resource identity

PowerDNS assigns no opaque identifiers. Every object is addressed by where it
sits, so every `id` here is composed from the attributes that locate it.

| Resource | `id` | Import |
| --- | --- | --- |
| `powerdns_zone` | canonical zone name | `example.com.` |
| `powerdns_record` | `<zone>/<name>/<type>` | `example.com./www.example.com./A` |
| `powerdns_zone_metadata` | `<zone>/<kind>` | `example.com./ALLOW-AXFR-FROM` |

The consequence is that changing any part of an id replaces the resource.
There is no rename: PowerDNS has no operation that would implement one.

## 3. Current surface

Pre-1.0. Everything here is implemented and covered by acceptance tests on
both authoritative backends; see [`plan.md`](plan.md) for what is still to
come.

### Resources

| Type | Manages | Replacement forced by |
| --- | --- | --- |
| `powerdns_zone` | A zone's own attributes | `name`, `nameservers` |
| `powerdns_record` | One RRSet | `zone`, `name`, `type` |
| `powerdns_zone_metadata` | One metadata kind | `zone`, `kind` |

### Data sources

| Type | Reads |
| --- | --- |
| `powerdns_zone` | One zone, including its RRSet count |
| `powerdns_zones` | Every zone, without records |

### Provider arguments

Each takes its value from the argument, then the environment variable, then
the default.

| Argument | Environment | Default |
| --- | --- | --- |
| `server_url` | `PDNS_SERVER_URL` | — |
| `api_key` | `PDNS_API_KEY` | — |
| `recursor_server_url` | `PDNS_RECURSOR_SERVER_URL` | — |
| `recursor_api_key` | `PDNS_RECURSOR_API_KEY` | falls back to `api_key` |
| `dnsdist_server_url` | `PDNS_DNSDIST_SERVER_URL` | — |
| `dnsdist_api_key` | `PDNS_DNSDIST_API_KEY` | falls back to `api_key` |
| `ca_certificate` | `PDNS_CA_CERTIFICATE` | system roots |
| `client_cert_file` | `PDNS_CLIENT_CERT_FILE` | — |
| `client_cert_key_file` | `PDNS_CLIENT_CERT_KEY_FILE` | — |
| `insecure_https` | `PDNS_INSECURE_HTTPS` | `false` |
| `timeout_seconds` | `PDNS_TIMEOUT_SECONDS` | `30` |
| `retry_attempts` | `PDNS_RETRY_ATTEMPTS` | `5` |

At least one of the three `*_server_url` arguments must be set. A resource
belonging to an unconfigured product fails with a diagnostic naming the
argument, not a nil dereference.

## 4. Guarantees

### 4.1 An apply converges

Applying an unchanged configuration twice plans nothing the second time. This
is not a platitude — PowerDNS rewrites much of what it is given, and each
rewrite is a permanent diff unless it is compared semantically.

The comparisons are part of the contract, because a user relies on them when
deciding how to write a value:

| Attribute | Compared |
| --- | --- |
| `powerdns_zone.kind` | case-insensitively — the server title-cases it |
| `powerdns_zone.name`, `catalog` | as a DNS name — case and trailing dot ignored |
| `powerdns_zone.masters` | by address value, ignoring order |
| `powerdns_record.name` | as a DNS name — the server lowercases it |
| `powerdns_record.values`, type `A`/`AAAA` | by address value, ignoring order |
| `powerdns_record.values`, other types | exactly |

Every one of these is asserted in both directions: a respelling plans nothing,
and a genuine change plans a change. A comparison looser than the server's
would hide an edit, which is worse than a spurious diff because no plan shows
it.

### 4.2 Secrets do not reach state

DNSSEC private keys and TSIG secrets are never written to state. The mechanism
is that reconciliation reads the collection endpoints, which omit key material,
rather than the single-object endpoints, which include it.

`TestFixturesCarryNoKeyMaterial` enforces the same rule for recorded test
fixtures, and phase 4 adds a test that reads the state file itself.

### 4.3 What the provider does not manage, it does not touch

A resource owns exactly what it names. `powerdns_zone_metadata` owns one kind,
not the collection; `powerdns_record` owns one RRSet, not the zone's records.
Anything set by `pdnsutil`, by another tool, or by PowerDNS itself — such as
the `SOA-EDIT-API` every zone is created with — is left alone.

### 4.4 An impossible operation fails at plan or with a diagnostic that names the cause

PowerDNS has four conditions under which an operation cannot succeed on a given
installation, and each surfaces as a bare `4xx`:

| Condition | Bare status | What the diagnostic says |
| --- | --- | --- |
| Views and networks on a relational backend | `422` | the LMDB requirement |
| Any Recursor write without `api_dir` | `422` | the `webservice.api_dir` setting |
| A dnsdist configuration write without `setAPIWritable` | `405` | the Lua call to add |
| A dnsdist cache flush with no packet cache | `404` | that the pool has no cache |

The transport classifies these once, so every resource reports the same
actionable message.

## 5. Non-goals

Stated so they are not repeatedly proposed.

| Not provided | Why |
| --- | --- |
| dnsdist rules, pools, downstreams, dynamic blocks | No HTTP write path exists. dnsdist's API has two writes: the ACL and a cache flush ([ADR 0006](adr/0006-dnsdist-scope.md)) |
| A resource per individual DNS record | PowerDNS has no per-record identity; two such resources on one name would silently overwrite each other |
| A resource owning a zone's whole metadata collection | It would delete `SOA-EDIT-API`, which PowerDNS assigns itself |
| Reading arbitrary Recursor settings by name | Only `allow-from` and `allow-notify-from` are registered; every other name answers `404` |
| Code generated from the PowerDNS OpenAPI document | It diverges from the implementation in nine documented ways ([`plan.md`](plan.md) §Phase 1) |

## 6. Changing the contract

Per [`standards/versioning.md`](standards/versioning.md):

| Change | Bump |
| --- | --- |
| New resource, data source or optional attribute | MINOR |
| New required attribute, removed attribute, changed type | MAJOR |
| Attribute becomes `Computed` where it was not | MAJOR — it stops being settable |
| A semantic comparison becomes looser | MAJOR — a previously visible diff disappears |
| A semantic comparison becomes stricter | MINOR — a previously hidden diff appears |
| Diagnostic wording, retry timing, log output | PATCH |

Before 1.0, MINOR carries what MAJOR will carry afterwards. That is the
freedom this phase exists to use: the schema is still cheap to correct, and
[`plan.md`](plan.md) records every correction made so far rather than
pretending the first attempt was right.
