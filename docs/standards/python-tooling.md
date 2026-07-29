<!-- markdownlint-disable MD013 -->
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=Python+tooling&subtitle=uv%2C+ruff%2C+ty&logo=python&mode=dark&align=left&font=geist-mono&border=false" />
    <img alt="Python tooling" src="https://shieldcn.dev/header/graph.svg?title=Python+tooling&subtitle=uv%2C+ruff%2C+ty&logo=python&mode=light&align=left&font=geist-mono&border=false" />
  </picture>
</p>
<!-- markdownlint-enable MD013 -->

<div align="center">

[![status normative](https://shieldcn.dev/badge/status-normative-cf222e.svg?variant=secondary)](../README.md)
![ruff 0.16.0](https://shieldcn.dev/badge/ruff-0.16.0-0969da.svg?variant=secondary)
![enforced see the table](https://shieldcn.dev/badge/enforced-see_the_table-3fb950.svg?variant=secondary)

</div>

# Python tooling

The repository's automation is Python: the lab lifecycle, the end-to-end
fixture, the worktree helper, and every check the gate runs. It is held to the
same standard as the Go — an explicit allowlist, strict by default, one declared
configuration, everything pinned.

There is no Python package to publish here. `pyproject.toml` exists so the tools
have a configuration file rather than a growing list of command-line flags.

**The checks were shell until they grew branches nobody could test.** Nine
scripts under `scripts/`, 961 lines of bash, exercised only by running them
against the repository's own state — so a branch that had never executed was
indistinguishable from one that worked. Two of them shipped with exactly that
defect. They are now `scripts/checks/`, one module per check, imported by
`test/scripts/` and covered by 87 assertions, including the negative cases the
shell versions asserted by hand and the ones nobody could reach at all.

What is left in shell is the `terraform import` snippets under `examples/`,
which `tfplugindocs` renders verbatim into the registry pages. They are
documentation, not programs.

## The toolchain

| Tool | Version | Role |
| --- | --- | --- |
| [`uv`](https://docs.astral.sh/uv/) | 0.11.33 | Environment and tool installation. Not `pip`, not `venv` by hand. |
| [`ruff`](https://docs.astral.sh/ruff/) | 0.16.0 | Linter **and** formatter. Replaces flake8, isort, black, pyupgrade, bandit. |
| [`ty`](https://docs.astral.sh/ty/) | 0.0.64 | Type checker. |
| [`pytest`](https://docs.pytest.org/) | 9.1.1 | The checks' tests. `test/scripts/` for the gate's own checks, `test/e2e/` for the consumer path. |

Pinned exactly, mirrored between `Containerfile.dev` build arguments and
`compose.dev.yml`, per the dependency policy in
[`versioning.md`](versioning.md).

## Gates

```sh
task py:lint        # ruff check + ruff format --check
task py:fmt         # ruff format + ruff check --fix
task py:typecheck   # ty check
task py:test        # pytest over test/scripts/
task py             # all four
```

`task py` is part of `task all`, so the Python gate blocks a pull request in
exactly the way the Go gate does. Python that only ever runs on a developer's
machine is still code someone else has to read.

## Ruff configuration

`[tool.ruff.lint] select` is an allowlist of 42 rule families, not `ALL`.
`ALL` is unstable across releases — a new rule family appearing in a patch
version would break the gate for reasons unrelated to the change under review,
which is the same argument that makes `.golangci.yml` an allowlist.

Four suppressions, each with a reason:

| Rule | Why suppressed |
| --- | --- |
| `T201` (`print` found) | The scripts are operator-facing command-line tools; stdout is their output, not a debugging leftover. |
| `D203`, `D213` | Mutually exclusive with the Google docstring convention selected below. |
| `ISC001` | Conflicts with the formatter. |
| `S603`, `S607` in `scripts/automation/` and `scripts/checks/` | `subprocess` is the point of the automation. Every call site is a fixed argument list, with no shell and no interpolated input. |
| `S101`, `ANN001`, `ANN201`, `INP001`, `PLR2004` in `test/` | Each inverts in a test: `assert` is how pytest reports, fixtures are typed at the call site, a test directory must not be importable as a package, and the comparison value *is* the assertion. |

A suppression added without a reason in this table is a review finding.

## Type checking with a pre-1.0 tool

`ty` is version 0.0.64. That is early, and the standard says so rather than
pretending otherwise.

It is in the gate because its findings on this code base have been accurate and
because the automation manipulates loosely typed API payloads, where a type
checker earns its place. But a **ty-only failure is reviewed, not obeyed**: if
`ty` reports something `ruff` does not and the code is demonstrably correct,
the finding is recorded and the rule adjusted in `[tool.ty.rules]`, rather than
the code contorted to satisfy a pre-release checker.

If `ty` reaches a state where it blocks more than it catches, it comes out. That
decision would be an ADR, not a quiet edit.

## Conventions

| Area | Rule |
| --- | --- |
| Python version | 3.12 minimum (`requires-python = ">=3.12"`). |
| Annotations | Every function annotated, parameters and return. Enforced by `ANN`. |
| Docstrings | Google convention, every public function. Enforced by `D`. |
| Paths | `pathlib`, never `os.path`. Enforced by `PTH`. |
| Subprocess | Fixed argument lists, `check=True`, never `shell=True`. |
| Exceptions | Never a bare `except:`; catch the narrowest type that is correct. Enforced by `BLE`. |
| Errors to the operator | A message that says what to do next, not just what failed. |
| Line length | 88, matching the formatter default. |

### The exception-handling rule has teeth here

The lab automation polls services that are still starting. A server that
accepts a connection and closes it before replying raises a bare
`ConnectionResetError`, not the `urllib.error.URLError` one expects — so the
narrow catch has to be `OSError`, and the reason is written at the call site.
That comment exists because the first version of the code caught `URLError`
alone and crashed on the real startup path.

Catch narrowly, but catch what actually happens, and record why when the two
differ.

## Running the tools

Inside the dev container, through `uv`:

```sh
uv run ruff check scripts/ test/scripts/
uv run ruff format scripts/ test/scripts/
uv run --group e2e ty check scripts/ test/scripts/
uv run pytest
```

`ty` is run with `--group e2e` because `scripts/automation/e2e.py` imports
`boto3` and `tenacity`, which live in that group. Without it the imports
resolved only when a previous `task e2e` had left them in the virtualenv, so
whether the gate passed depended on what had been run before it.

`uv run` resolves the environment declared in `pyproject.toml`, so the versions
in the gate and the versions on a developer's machine are the same by
construction rather than by convention.
