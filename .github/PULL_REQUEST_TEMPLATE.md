<!--
The title becomes the commit message on a squash merge, so it must be a
Conventional Commits subject — CI checks it, and every commit in the branch.
See docs/standards/commits.md.
-->

## What changed, and why

<!-- The reasoning, not a restatement of the diff. If a decision was
     available and you took one road, say which and what it cost. -->

## Evidence

<!--
Claims about PowerDNS behaviour need a source and a round-trip against the lab
— the published OpenAPI diverges from the implementation in both directions
(AGENTS.md golden rule 3). Quote what you ran.
-->

- [ ] `task all`
- [ ] `task verify` (required for any change to a resource, data source or the client)

## Checklist

- [ ] `docs/plan.md` updated **in this commit** — a plan updated afterwards is a report, not a control
- [ ] `CHANGELOG.md` updated under `[Unreleased]`
- [ ] `task docs` re-run if the schema changed
- [ ] Nothing in state that should not be there — a DNSSEC private key or
      TSIG secret is write-only or ephemeral, never `Sensitive`
