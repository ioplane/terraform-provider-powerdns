<!-- markdownlint-disable MD013 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=ADR+0005&subtitle=Acceptance+runs+on+two+authoritative+backends&logo=checkmarx&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="ADR 0005" src="https://shieldcn.dev/header/graph.svg?title=ADR+0005&subtitle=Acceptance+runs+on+two+authoritative+backends&logo=checkmarx&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>
<!-- markdownlint-enable MD013 -->

<div align="center">

![status accepted](https://shieldcn.dev/badge/status-accepted-3fb950.svg?variant=secondary)
![date 2026-07-28](https://shieldcn.dev/badge/date-2026--07--28-0969da.svg?variant=secondary)
[![adr 0005](https://shieldcn.dev/badge/adr-0005-6e7781.svg?variant=secondary)](../README.md)

</div>

# ADR 0005 — Acceptance runs on two authoritative backends

- **Status:** accepted
- **Date:** 2026-07-28
- **Deciders:** ARC, QA

## Context

PowerDNS views and networks are unimplemented by the generic SQL backends. On
`gpgsql` a write returns `422` while a read returns `200` with an empty
collection; on LMDB both succeed. Verified on 5.1.3 against both.

## Decision

The acceptance matrix runs **two authoritative instances** — `gpgsql` on
PostgreSQL 17 and LMDB. Every resource declares which backends it supports and
its acceptance test runs on each. A resource expected to fail on a backend has
a negative test asserting the **diagnostic**, not merely the failure.

## Rationale

A read cannot distinguish "unsupported" from "not configured", so a
single-backend fixture cannot detect the difference. Whichever backend is
chosen, half the provider goes untested: LMDB-only leaves the failure path
untested, gpgsql-only leaves the resources untested.

## Consequences

- The lab is five services and CI runs two acceptance jobs.
- Backend support becomes an explicit, testable property of a resource rather
  than folklore.
