<!-- markdownlint-disable MD013 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=ADR+0001&subtitle=Gated-iterative+delivery&logo=checkmarx&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="ADR 0001" src="https://shieldcn.dev/header/graph.svg?title=ADR+0001&subtitle=Gated-iterative+delivery&logo=checkmarx&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>
<!-- markdownlint-enable MD013 -->

<div align="center">

![status accepted](https://shieldcn.dev/badge/status-accepted-3fb950.svg?variant=secondary)
![date 2026-07-28](https://shieldcn.dev/badge/date-2026--07--28-0969da.svg?variant=secondary)
[![adr 0001](https://shieldcn.dev/badge/adr-0001-6e7781.svg?variant=secondary)](../README.md)

</div>

# ADR 0001 — Gated-iterative delivery

- **Status:** accepted
- **Date:** 2026-07-28
- **Deciders:** PM, ARC, DEV

## Context

The provider has a hard external contract — schema, state, resource identity,
registry publication, SemVer — and a large surface: 12 resources, 16 data
sources, 6 actions and 5 functions across three products.

## Decision

Phase-gated macro-lifecycle, two-week sprints inside implementation,
trunk-based development, a per-item Definition of Done, evidence discipline.
Detail in [`../methodology.md`](../methodology.md).

## Rationale

A shipped breaking schema cannot be refactored away, so the contract is decided
at a gate before it is implemented. But each item needs feedback from a live
server, which rewards short iterations. Pure Waterfall under-serves the
discovery loop; pure Agile under-serves the contract.

## Consequences

- ARC signs a schema before DEV implements it. A schema changed after
  implementation is a design failure, not an iteration.
- Transport precedes clients and clients precede resources, which is a phase
  ordering rather than a preference.
