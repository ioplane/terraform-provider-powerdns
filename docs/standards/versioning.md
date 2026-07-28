<!-- markdownlint-disable MD013 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=Versioning&subtitle=SemVer+2.0.0&logo=semver&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="Versioning" src="https://shieldcn.dev/header/graph.svg?title=Versioning&subtitle=SemVer+2.0.0&logo=semver&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>
<!-- markdownlint-enable MD013 -->

<div align="center">

[![status normative](https://shieldcn.dev/badge/status-normative-cf222e.svg?variant=secondary)](../README.md)
![semver 2.0.0](https://shieldcn.dev/badge/semver-2.0.0-0969da.svg?variant=secondary)
![enforced see the table](https://shieldcn.dev/badge/enforced-see_the_table-3fb950.svg?variant=secondary)

</div>

# Versioning

[Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html). The binary,
its schema and its published documentation share one version. The source of
truth is [`VERSION`](../../VERSION); the tag `vX.Y.Z` is cut from it.

## What "the public API" means for a provider

A change is **breaking** if an unmodified, previously valid configuration or
state would newly error, silently change meaning, or force replacement:

- removing or renaming a resource, data source, action, function or attribute;
- making an optional attribute required, or removing a default;
- narrowing accepted values or changing a type;
- changing an attribute from updatable to `RequiresReplace`;
- changing the shape of `id`, of resource identity, or of imported state;
- raising the minimum Terraform version or the protocol version.

Non-breaking: adding a resource, an optional attribute, a computed output, an
action or a function; widening accepted input; fixing a bug so that documented
behaviour is restored.

## Validation that rejects what used to be accepted

Adding a validator that rejects a value previously accepted is formally
breaking — `plan` newly errors on an unmodified configuration.

The rule here: **such a change is MINOR when the rejected configuration could
not previously have applied successfully.** It still gets a `BREAKING:`
changelog entry, because the user-visible symptom moves from a failed apply to
a failed plan. Where the configuration *could* have applied, the change is
MAJOR and waits for one.

This case is common in this provider by design — a large part of its value is
refusing at plan time what the PowerDNS API would refuse at apply time.

## Rules

| Component | Bumps when |
| --- | --- |
| MAJOR | Breaking change, **after 1.0.0** |
| MINOR | Backward-compatible feature; **also breaking changes while `0.x`** (SemVer §4) |
| PATCH | Backward-compatible bug fix |

## Pre-1.0

While `0.x` the schema may change between minors. SemVer §4 permits anything;
this project constrains it to: breaking changes bump MINOR and carry a
`BREAKING:` entry with a migration note. Patch releases never break.

1.0.0 is cut when the resource surface for all three products is complete and
has survived one PowerDNS minor upgrade without a schema change.

## State and identity obligations

Any change to stored state shape ships a `ResourceWithUpgradeState`. Any change
to a resource's identity ships a `ResourceWithUpgradeIdentity`. Identity is a
stronger promise than state: it must denote at most one remote object per
provider and must not change over that object's life.

## Dependency policy — latest, pinned by hash

Track newest releases; pin exactly; pin by content hash wherever the ecosystem
offers one:

| Kind | Pinned by | Verified by |
| --- | --- | --- |
| Go modules | exact version | `go.sum` |
| Container base images | `sha256:` digest | `scripts/check-pins.sh` |
| GitHub Actions | commit SHA | `scripts/check-pins.sh` |
| Go tools in the image | exact version tag | build reproducibility |
| Python tools | exact version in `pyproject.toml` | `uv.lock` |

A tag is a mutable reference. `golang:1.26-trixie` may point at a different
image tomorrow; `golang:1.26-trixie@sha256:…` may not. Anything that runs in CI
or in the dev image is referenced by digest.

Dependabot proposes weekly bumps; each arrives as a `build(deps)` commit and
re-pins the hash.
