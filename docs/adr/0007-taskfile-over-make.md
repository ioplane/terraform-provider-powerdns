<!-- markdownlint-disable MD033 MD041 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=ADR+0007&subtitle=Task+is+the+command+interface&logo=checkmarx&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="ADR 0007" src="https://shieldcn.dev/header/graph.svg?title=ADR+0007&subtitle=Task+is+the+command+interface&logo=checkmarx&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>

<div align="center">

[![status accepted](https://shieldcn.dev/badge/status-accepted-3fb950.svg?variant=secondary)](#)
[![date 2026-07-28](https://shieldcn.dev/badge/date-2026--07--28-0969da.svg?variant=secondary)](#)
[![adr 0007](https://shieldcn.dev/badge/adr-0007-6e7781.svg?variant=secondary)](../README.md)

</div>

# ADR 0007 — Task is the command interface

- **Status:** accepted
- **Date:** 2026-07-28
- **Deciders:** ARC, OPS

## Decision

[Task](https://taskfile.dev) v3.52.0, pinned in the dev image. `Taskfile.yml`
is the only command entry point; there is no `Makefile`.

## Rationale

- **Namespaced tasks** — `lab:up`, `py:typecheck`, `tf:fmt:check` — group
  naturally, and `task --list` is a usable index. A `Makefile` achieves that
  only with a hand-maintained `help` target that goes stale.
- **`sources` and `generates`** give real incremental builds. A `.PHONY`-heavy
  Makefile abandons Make's own mechanism entirely.
- **`preconditions`** turn "the dev container is not running" into a sentence
  naming the fix rather than a podman-compose stack trace.
- **`requires: vars`** turns a missing argument into a message rather than a
  shell test inside the recipe.
- The rest of the repository is already YAML: compose, CI, linters,
  commitlint. One fewer syntax to hold.
- Make's resolution order between `GNUmakefile`, `makefile` and `Makefile` is
  implicit and silent, which has bitten this author before.

## What is given up

Make is everywhere; Task must be installed. Near zero here, because work
happens in the dev container where the toolchain is ours to define.

CI does not use Task for the Go and Python jobs — it invokes tools directly so
a pipeline failure points at the tool rather than at a wrapper. Task is used in
CI only for lab lifecycle, where orchestration is the point.
