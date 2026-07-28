# ADR 0003 — terraform-plugin-framework, protocol 6, from scratch

- **Status:** accepted
- **Date:** 2026-07-28
- **Deciders:** ARC

## Context

Two plugin APIs exist: `terraform-plugin-sdk/v2` (protocol 5, maintenance) and
`terraform-plugin-framework` (protocol 6). Every existing PowerDNS provider of
any completeness is on SDKv2.

## Decision

`terraform-plugin-framework`, protocol 6, written from scratch. No fork, no
SDKv2, no multiplexer.

## Rationale

Protocol 6 and the framework are not merely newer; four capabilities this
provider needs exist only there:

| Capability | Needed for |
|---|---|
| Write-only attributes (TF 1.11) | TSIG secrets that must not persist |
| Ephemeral resources (TF 1.10) | DNSSEC private key material |
| Resource identity (TF 1.12) | Stable identity for zones and records |
| Actions (TF 1.14) | notify, axfr-retrieve, rectify, cache flush |

Without ephemeral resources and write-only attributes, a DNSSEC private key
lands in plain-text state. That alone settles it.

## Alternatives rejected

- **Fork an existing provider and migrate.** Evaluated in depth against a
  concrete candidate. The migration cost equals the from-scratch cost, because
  porting a resource between the two APIs is a rewrite; what a fork adds is
  coupling to a client that caps coverage and an obligation to somebody else's
  state.
- **SDKv2 for speed.** Rejected: no ephemeral resources, no actions, and a
  deprecated foundation for work that will outlive it.

## Consequences

- Terraform 1.11 is the floor; actions are gated at 1.14 through client
  capability so an older CLI loses the actions rather than the provider.
- No state-migration obligation to any existing provider. A user moving from
  one imports their resources.
