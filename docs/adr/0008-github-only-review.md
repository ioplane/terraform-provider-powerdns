<!-- markdownlint-disable MD013 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=ADR+0008&subtitle=Review+happens+on+GitHub+only&logo=checkmarx&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="ADR 0008" src="https://shieldcn.dev/header/graph.svg?title=ADR+0008&subtitle=Review+happens+on+GitHub+only&logo=checkmarx&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>
<!-- markdownlint-enable MD013 -->

<div align="center">

![status accepted](https://shieldcn.dev/badge/status-accepted-3fb950.svg?variant=secondary)
![date 2026--07--29](https://shieldcn.dev/badge/date-2026--07--29-0969da.svg?variant=secondary)
[![adr 0008](https://shieldcn.dev/badge/adr-0008-6e7781.svg?variant=secondary)](../README.md)

</div>

# ADR 0008 — Review happens on GitHub only

- **Status:** accepted
- **Date:** 2026-07-29
- **Deciders:** PM, OPS

## Context

The repository lives at `ioplane/terraform-provider-powerdns` on GitHub and is
intended for the Terraform Registry, which publishes from GitHub. It also
carries a `.gitlab-ci.yml`, written in phase 0 on the reasoning that GitLab
owns the quality gate and GitHub owns release only ([ADR 0007's sibling
decision](0007-taskfile-over-make.md)).

No GitLab remote was ever configured. The pipeline has therefore never run
anywhere, and the quality gate has been `task all` on a developer's machine
throughout.

The available GitLab is a corporate instance. Mirroring an open-source provider
into it would put public code behind an internal login, add a second review
surface for the same change, and give the project two places where a merge can
happen.

## Decision

Reviews happen on GitHub. One pull request per sprint, squash-merged.

`.gitlab-ci.yml` stays in the repository, unexecuted, as the definition of the
gate for a mirror that may exist later. It is kept current — the job list
matches `task all` — but nothing runs it today, and the file says so.

## Consequences

- One review surface, one merge button.
- The quality gate is `task all` and `task verify`, run before the pull request
  and quoted in the commit body. That is weaker than a pipeline enforcing it,
  and is the accepted cost until a runner exists.
- GitHub Actions stays release-only, per phase 0. Adding the quality gate there
  would duplicate `.gitlab-ci.yml` into a third place.
- Revisit if the provider gains contributors: a gate nobody can bypass matters
  much more with more than one pair of hands.
