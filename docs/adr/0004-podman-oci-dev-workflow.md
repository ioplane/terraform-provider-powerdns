<!-- markdownlint-disable MD013 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=ADR+0004&subtitle=Containerised+development+on+Podman+and+OCI&logo=checkmarx&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="ADR 0004" src="https://shieldcn.dev/header/graph.svg?title=ADR+0004&subtitle=Containerised+development+on+Podman+and+OCI&logo=checkmarx&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>
<!-- markdownlint-enable MD013 -->

<div align="center">

![status accepted](https://shieldcn.dev/badge/status-accepted-3fb950.svg?variant=secondary)
![date 2026-07-28](https://shieldcn.dev/badge/date-2026--07--28-0969da.svg?variant=secondary)
[![adr 0004](https://shieldcn.dev/badge/adr-0004-6e7781.svg?variant=secondary)](../README.md)

</div>

# ADR 0004 — Containerised development on Podman and OCI

- **Status:** accepted
- **Date:** 2026-07-28
- **Deciders:** ARC, OPS

## Decision

1. Development happens **inside a container** built from `golang:1.26-trixie`,
   pinned by digest, with every tool baked in and pinned by build argument.
2. The image is defined in a **`Containerfile`** per the `Containerfile.5`
   specification, buildable by Buildah and `podman build` without an external
   frontend.
3. Orchestration uses the **Compose Specification** — no top-level `version:`
   key — run through `podman-compose`.
4. Images carry **OCI image-spec annotations**.
5. Scripted automation uses **`podman-py`** against the Podman REST API, with a
   CLI fallback so a diagnostic command does not fail when the API socket is
   absent.
6. Everything that runs is referenced **by digest**, never by tag.

## Rationale

Point 6 is the one that matters. `golang:1.26-trixie` is a moving reference; a
digest is not. A build that is reproducible only until the upstream tag moves is
not reproducible. `scripts/checks/pins.py` enforces it.

## Consequences

- The host needs Podman, podman-compose and Task. Nothing else.
- CI uses the same digest-pinned images, so "works locally" and "works in CI"
  mean the same thing.
- Rootless Podman cannot bind privileged ports; the lab listens high and
  publishes high. A property of the fixture, not a defect.
