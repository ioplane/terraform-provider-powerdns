---
name: powerdns-facts
description: Use before stating any fact about PowerDNS behaviour — what an endpoint returns, what a field means, what a status code implies, how many operations exist. Use when writing a code comment, a schema description or a plan entry that asserts something about the API.
---

# Establishing a PowerDNS fact

Do not state a PowerDNS behaviour from documentation or memory. Measure it.

This is not a general caution. Eleven comments in this provider were written
from the documentation and later corrected by one request against the lab, and
two of the corrections would have caused data loss in production:

| Assumed | Actual |
| --- | --- |
| `/export` returns `text/plain` | JSON, `{"zone": "…"}` |
| a PATCH without `changetype` gets an opaque 422 | the server names the field |
| an unset metadata kind is 404 | 200 with an empty list |
| a TSIG key keeps the name it was given | canonicalised, `probe` → `probe.` |
| `setAPIWritable` gates every dnsdist write | only `PUT`; `DELETE /cache` is not gated |
| an unknown dnsdist pool returns an empty list | 404, empty body, no message |
| the dnsdist flush count is a number | a string, `"0"` |
| `keytype` is stored | derived from flags **and** how many keys the zone has |
| a ZSK has no DS | only once it is not the zone's only key |
| a TSIG `PUT` changes the key | it adds a second under the same id |
| cryptokey ids are per zone | a global counter |

## The order

1. **Ask the lab.** `task lab:up`, then `curl`. This is the only source that
   describes the version being targeted.
2. **Read the source** at the pinned tag, for *why* rather than *what*:
   `/opt/projects/repositories/pdns-upstream`, tags `auth-5.1.3`,
   `rec-5.4.4`, `dnsdist-2.1.0`.
3. **Cross-check the specification last**, and never trust it. It diverges
   from the implementation in ten documented ways; two are reported as
   PowerDNS/pdns#17807 and the rest are in `docs/plan.md`.

## Citing

A `file:line` without a revision is unfalsifiable — `ws-auth.cc:3361` is right
on master and wrong at `auth-5.1.3`, where the same registration is 3349.
Always name the revision. A hook warns when one is missing.

## Recording

A behaviour worth a comment is worth a fixture. `task fixtures:record`
captures it, `internal/testutil` replays it without a container, and the next
version of PowerDNS that changes it produces a diff somebody reads.

Never record a fixture from an endpoint that returns key material —
`GET /cryptokeys/{id}` or `GET /tsigkeys/{id}`. Fixtures are committed, and a
private key in git is removed by rewriting history rather than by deleting a
file. `TestFixturesCarryNoKeyMaterial` enforces this.
