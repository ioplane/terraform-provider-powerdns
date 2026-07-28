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
| `powerdns_zone_cryptokey` | `<zone>/<key_id>` | `example.com./3` |
| `powerdns_tsigkey` | canonical key name | `transfer.` |
| `powerdns_view_zone` | `<view>/<zone>` | `trusted/example.com.` |
| `powerdns_network` | the CIDR | `192.0.2.0/24` |
| `powerdns_autoprimary` | `<ip>/<nameserver>` | `192.0.2.53/ns1.example.com.` |
| `powerdns_recursor_zone` | canonical zone name | `example.com.` |
| `powerdns_recursor_acl` | the setting name | `allow-from` |
| `powerdns_dnsdist_acl` | `allow-from` | anything |

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
| `powerdns_zone_cryptokey` | One DNSSEC key | `zone`, `key_type`, and `algorithm`/`bits` when configured |
| `powerdns_tsigkey` | One TSIG key | `name`, `algorithm` — see below |
| `powerdns_view_zone` | A zone's membership of a view (LMDB only) | `view`, `zone` |
| `powerdns_network` | A subnet-to-view mapping (LMDB only) | `network` |
| `powerdns_autoprimary` | An autoprimary entry | `ip`, `nameserver`, `account` — every settable attribute |
| `powerdns_recursor_zone` | A Recursor zone | `name` |
| `powerdns_recursor_acl` | One of two Recursor netmask settings | — |
| `powerdns_dnsdist_acl` | dnsdist's `allow-from` | — |

### Data sources

| Type | Reads |
| --- | --- |
| `powerdns_zone` | One zone, including its RRSet count |
| `powerdns_zones` | Every zone, without records |
| `powerdns_record` | One RRSet |
| `powerdns_zone_metadata` | One metadata kind |
| `powerdns_zone_export` | A zone in presentation format |
| `powerdns_recursor_zone` | A Recursor zone |
| `powerdns_dnsdist_server` | dnsdist's version, ACL and downstream servers |

### Ephemeral resources

The only place the provider returns key material. Terraform discards an
ephemeral value rather than persisting it, and such a value may only be
consumed by another ephemeral or write-only attribute — which is what prevents
something downstream storing it.

| Type | Reads | Needs |
| --- | --- | --- |
| `powerdns_cryptokey_material` | A DNSSEC private key | Terraform 1.10 |
| `powerdns_tsigkey_secret` | A TSIG shared secret | Terraform 1.10 |

### Resource identity

Every resource with a stable natural key declares one, so Terraform can
recognise a remote object without parsing an id string.

| Resource | Identity |
| --- | --- |
| `powerdns_zone`, `powerdns_recursor_zone` | `zone_name` |
| `powerdns_record` | `zone_name`, `record_name`, `record_type` |
| `powerdns_zone_metadata` | `zone_name`, `kind` |
| `powerdns_zone_cryptokey` | `zone_name`, `key_id` |
| `powerdns_tsigkey` | `key_id` |
| `powerdns_view_zone` | `view`, `zone_name` |
| `powerdns_network` | `network` |
| `powerdns_autoprimary` | `ip`, `nameserver` |

**The scope is one PowerDNS installation.** Terraform asks that an identity
address at most one object across every instance of the provider; PowerDNS
offers no server identifier to compose in — `/servers/{id}` answers only for
`localhost` — and the endpoint URL cannot serve, because a server that moves
would change identity while remaining the same object. Within one installation
an identity is unique; across two, `example.com.` is ambiguous in exactly the
way the zone name itself is.

`powerdns_recursor_acl` and `powerdns_dnsdist_acl` have **no** identity. Their
natural key is the setting name, which is the same on every installation, so
any identity they declared would be false.

#### `nameservers` and import blocks

A zone created with `nameservers` cannot be imported by an import block without
planning a replacement. PowerDNS consumes the attribute once at creation and
never reports it, so the imported object has none while the configuration has
some. Import such a zone with `terraform import`, or omit `nameservers` and
manage the NS records with `powerdns_record`.

### Actions

Imperative operations, needing Terraform 1.14. They have no state: running one
twice does no harm, and none can be undone, which is why none is a resource.

| Type | Does |
| --- | --- |
| `powerdns_notify_zone` | Sends a NOTIFY to a zone's secondaries |
| `powerdns_axfr_retrieve` | Triggers a transfer from a zone's primary |
| `powerdns_rectify_zone` | Recomputes DNSSEC ordering and NSEC records |
| `powerdns_flush_cache` | Drops a name from a cache, on any of the three products |

### Functions

Pure and offline: no client, no request, no state.

| Function | Returns |
| --- | --- |
| `fqdn(name)` | The name with a trailing dot |
| `is_fqdn(name)` | Whether it already has one |
| `reverse_zone_name(cidr)` | The `in-addr.arpa` or `ip6.arpa` zone for a prefix |
| `ptr_name(address)` | The name an address's PTR sits at |
| `soa_serial(date, revision)` | A serial in `YYYYMMDDnn` form |

`reverse_zone_name` errors for a prefix off an octet boundary (IPv4) or a
nibble boundary (IPv6): such a prefix spans several reverse zones and has no
single name. `soa_serial` takes the date as an argument rather than reading the
clock, so a plan converges, and bounds the revision at 99 because the
convention has two digits for it.

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
| `powerdns_zone_cryptokey.key_type` | `csk` is compatible with `ksk` and `zsk` |
| `powerdns_network.network` | as a subnet, so an uncompressed IPv6 prefix is not a change |
| `powerdns_autoprimary.ip` | by address value |
| `powerdns_recursor_zone.servers` | as upstreams: a bare address equals the same address with `:53` |
| `powerdns_recursor_acl.netmasks`, `powerdns_dnsdist_acl.netmasks` | as subnets, ignoring order |

Every one of these is asserted in both directions: a respelling plans nothing,
and a genuine change plans a change. A comparison looser than the server's
would hide an edit, which is worse than a spurious diff because no plan shows
it.

### 4.2 Secrets do not reach state

DNSSEC private keys and TSIG secrets are never written to state. The mechanism
is that reconciliation reads the collection endpoints, which omit key material,
rather than the single-object endpoints, which include it.

`TestFixturesCarryNoKeyMaterial` enforces the same rule for recorded test
fixtures. `TestAccCryptoKey_NoPrivateKeyInState` enforces it for state: it
walks every attribute of every resource and fails on an attribute named for
key material *or* a value carrying a `Private-key-format` or PEM private-key
header.

The consequence is deliberate: **a generated DNSSEC private key cannot be read
back through `powerdns_zone_cryptokey`.** Nothing exposes it, because anything
that did would put it in a plan file as well as a state file.

Where the material genuinely has to leave PowerDNS — a signing appliance, a
secondary server's configuration — the ephemeral resources provide it. They
are the only callers of the endpoints that carry secrets, and what they return
is never persisted.

### 4.2.1 A TSIG secret is write-only or unreadable

`powerdns_tsigkey.secret_wo` is a write-only attribute: Terraform hands the
value to the provider during apply and stores it in neither the state file nor
the plan file. It needs Terraform 1.11 or later.

Leaving it unset asks PowerDNS to generate a secret, which then cannot be read
back through this provider at all. Both paths keep the secret off disk, which
is the guarantee; retrieving a generated secret is not offered because
anything that offered it would write it somewhere.

Because a write-only value is stored nowhere, a change to it cannot be detected
from state. Rotating a secret means replacing the resource.

### 4.2.2 Changing a TSIG algorithm replaces the key

Not a design choice. `PUT /tsigkeys/{id}` deletes the previous entry only when
the *name* changed, so changing only the algorithm leaves the old key in place
and adds a second under the same id — verified against auth-5.1.3, where three
PUTs produced three entries. Replacement is the only way to end up with one
key.

### 4.2.3 `csk` is not a third key type

PowerDNS stores DNSKEY flags and derives `keytype` from them together with how
many keys a zone holds. A zone's only key reads back as `csk` whatever it was
created as, and is renamed rather than replaced once a second key appears.

`csk` is therefore compared as compatible with both `ksk` and `zsk`. Without
that, adding a second key would replace the first — destroying the signing key
of a live zone and invalidating the DS its parent publishes.

### 4.3 What the provider does not manage, it does not touch

A resource owns exactly what it names. `powerdns_zone_metadata` owns one kind,
not the collection; `powerdns_record` owns one RRSet, not the zone's records.
Anything set by `pdnsutil`, by another tool, or by PowerDNS itself — such as
the `SOA-EDIT-API` every zone is created with — is left alone.

### 4.4 An absent object is an error for a data source, except where the API says otherwise

A data source reading something that is not there fails. Returning an empty
result would let a configuration proceed on values that do not exist and fail
somewhere further along, with no mention of what was missing.

The exception is `powerdns_zone_metadata`, and it comes from the API rather
than from taste: PowerDNS answers an unset kind with `200` and an empty list,
not `404`. Absence and emptiness are the same state there, so `values` is
empty and a configuration can branch on it.

### 4.5 Two metadata kinds are not addressable, and the provider says so

`SOA-EDIT-API` and `API-RECTIFY` appear in a zone's metadata collection and
answer `422 "Unsupported metadata kind"` when read or written by name, because
they exist as attributes of the zone object. Both are rejected before the
request with a diagnostic naming the attribute to use —
`powerdns_zone.soa_edit_api` and `powerdns_zone.api_rectify`.

The list is enumerated rather than derived: `NSEC3PARAM` and `PRESIGNED` are
also zone attributes and are addressable as metadata.

### 4.6 An impossible operation fails at plan or with a diagnostic that names the cause

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
