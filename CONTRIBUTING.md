# Contributing

Read [`AGENTS.md`](AGENTS.md) first — it is the canonical guide. This is the
short path.

## Setup

```sh
task up        # dev container; no host toolchain
task lab:up    # five services, three products, two authoritative backends
```

## The loop

1. `scripts/worktree.sh new <type>/<scope>/<name>` — `main` is never committed
   to directly.
2. Develop in the container. Use `gopls`, and `context7` for current library
   documentation.
3. Establish PowerDNS behaviour from the **sources plus a live round-trip**.
   The published OpenAPI is not sufficient; it diverges from the implementation
   in both directions.
4. `task all` before pushing; `task verify` if you touched a resource.
5. Update `CHANGELOG.md` and the task in `docs/plan.md` **in the same commit**.
6. Open a pull request titled as a Conventional Commit subject.

## The five that catch people out

- **Validate what the server will reject** — a configuration the API cannot
  accept fails at `plan`, not `apply`.
- **Two backends** — views and networks work only on LMDB; a test on one
  backend is half a test.
- **Status before body** — check `resp.StatusCode` before decoding.
- **No secrets in state** — write-only or ephemeral, never `Sensitive` and
  stored.
- **Never write a digest or SHA from memory** — look it up; `task lint:pins`
  will catch you.

## No AI attribution

Not in code, documentation, commits or pull-request text. Enforced by a commit
hook. The one exception is a third party whose own policy requires disclosure —
ask first.
