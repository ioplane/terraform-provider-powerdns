# ADR 0002 — One provider for the whole PowerDNS family

- **Status:** accepted
- **Date:** 2026-07-28
- **Deciders:** PM, ARC

## Context

PowerDNS ships three server products: Authoritative, Recursor and dnsdist. They
could be one provider, three providers, or one provider covering a subset.

## Decision

One provider, `ioplane/powerdns`, covering all three. Each product has its own
endpoint and its own API key in the provider block; each has a resource-name
prefix, with Authoritative unprefixed.

## Rationale

- **One credential model.** All three authenticate with `X-API-Key` on a web
  server. Verified in the sources of each.
- **Deployed together.** An operator running PowerDNS typically runs more than
  one of them, often all three.
- **One installation.** Three providers to configure for one vendor's stack is
  friction with no compensating benefit.

Against: HashiCorp's "one problem domain" principle could be read as making
authoritative serving, recursion and load balancing three domains. The reading
taken here is that the domain is *the PowerDNS deployment*, which is how the
operator experiences it.

## Consequences

- The provider block carries three endpoints and three keys. Sharing one key
  across products is a limitation, not a convenience, and is not the default.
- The resource-name prefix scheme becomes part of the public contract
  (`naming-conventions.md` §4) and cannot change without a major version.
- The lab must run all three.
