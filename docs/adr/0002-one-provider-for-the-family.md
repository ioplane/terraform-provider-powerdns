<!-- markdownlint-disable MD013 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=ADR+0002&subtitle=One+provider+for+the+whole+PowerDNS+family&logo=checkmarx&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="ADR 0002" src="https://shieldcn.dev/header/graph.svg?title=ADR+0002&subtitle=One+provider+for+the+whole+PowerDNS+family&logo=checkmarx&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>
<!-- markdownlint-enable MD013 -->

<div align="center">

![status accepted](https://shieldcn.dev/badge/status-accepted-3fb950.svg?variant=secondary)
![date 2026-07-28](https://shieldcn.dev/badge/date-2026--07--28-0969da.svg?variant=secondary)
[![adr 0002](https://shieldcn.dev/badge/adr-0002-6e7781.svg?variant=secondary)](../README.md)

</div>

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
