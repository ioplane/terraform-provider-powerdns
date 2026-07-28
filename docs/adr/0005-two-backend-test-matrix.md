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
