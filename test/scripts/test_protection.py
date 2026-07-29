"""Reading job names out of workflow YAML, including matrix expansion.

The parser is narrow by necessity — these checks run as `python3 -m` with no
virtualenv, so the standard library only — and a narrow parser earns its tests.
The first attempt at the matrix regexes counted nine spaces where the schema
puts eight, and the acceptance job's name came back with `${{ matrix.auth }}`
still in it, which would have reported both required contexts as missing.
"""

from __future__ import annotations

from pathlib import Path

import pytest
from scripts.checks.protection import declared_names, expand, job_names

MATRIX_WORKFLOW = """\
name: Acceptance

on:
  push:
    branches: [main]

jobs:
  acceptance:
    name: Lab acceptance (auth ${{ matrix.auth }} · two backends)
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        auth: ['5.1', '5.0']
    steps:
      - name: a step whose name is not a job name
        run: true
"""

BLOCK_MATRIX = """\
jobs:
  build:
    name: Build on ${{ matrix.os }}
    strategy:
      matrix:
        os:
          - ubuntu-latest
          - macos-latest
    steps:
      - run: true
"""

PLAIN = """\
jobs:
  first:
    name: Build & test
    steps:
      - name: not a job
        run: true
  second:
    name: Lint (Go · golangci-lint v2)
    steps:
      - run: true
"""


def test_a_matrix_name_is_expanded_to_one_context_per_value():
    """Both are separate required contexts; an unexpanded name matches neither."""
    assert job_names(MATRIX_WORKFLOW) == [
        "Lab acceptance (auth 5.1 · two backends)",
        "Lab acceptance (auth 5.0 · two backends)",
    ]


def test_a_block_style_matrix_is_read_too():
    """The schema allows both, and a workflow may be rewritten from one to the other."""
    assert job_names(BLOCK_MATRIX) == [
        "Build on ubuntu-latest",
        "Build on macos-latest",
    ]


def test_step_names_are_not_job_names():
    """A step's name never becomes a status context; counting it would mask drift."""
    assert job_names(PLAIN) == ["Build & test", "Lint (Go · golangci-lint v2)"]


def test_a_workflow_with_no_jobs_yields_nothing():
    """A reusable-workflow stub or a malformed file must not crash the check."""
    assert job_names("name: x\non:\n  push:\n") == []


def test_an_axis_the_name_does_not_use_does_not_multiply_it():
    """GitHub appends such values rather than substituting them.

    Multiplying here would invent contexts nobody required and report them as
    fine, which is the failure direction that matters.
    """
    assert expand("Fixed name", {"os": ["a", "b"]}) == ["Fixed name"]


def test_two_axes_in_one_name_produce_the_cross_product():
    """Not a shape this repository uses yet, and the arithmetic should be right."""
    names = expand(
        "x ${{ matrix.a }} y ${{ matrix.b }}", {"a": ["1", "2"], "b": ["p", "q"]}
    )
    assert names == ["x 1 y p", "x 1 y q", "x 2 y p", "x 2 y q"]


def test_a_referenced_axis_that_the_matrix_lacks_is_left_alone():
    """Silently dropping it would produce a name that resolves to nothing."""
    assert expand("x ${{ matrix.missing }}", {"auth": ["5.1"]}) == [
        "x ${{ matrix.missing }}"
    ]


@pytest.mark.parametrize(
    "context",
    [
        "Build & test",
        "Lint (Python · ruff · ty)",
        "Pins (digests · action SHAs · tool versions)",
    ],
)
def test_the_repositorys_own_required_contexts_are_declared(context):
    """These are required on main today; each must name a job that exists.

    Listed by hand on purpose. If a job is renamed, the rename has to be a
    deliberate edit here as well as in the workflow.
    """
    assert context in declared_names()


def test_both_acceptance_contexts_are_declared():
    """The matrix is the fragile case: its names change when the axis does."""
    declared = declared_names()
    for auth in ("5.1", "5.0"):
        assert any(
            name.startswith(f"Lab acceptance (auth {auth} ") for name in declared
        ), f"no job reports an acceptance context for auth {auth}"


def test_every_workflow_in_the_repository_parses_to_at_least_one_job():
    """A file this parser reads as empty is a file it is silently ignoring."""
    for path in sorted(Path(".github/workflows").glob("*.yml")):
        assert job_names(path.read_text(encoding="utf-8")), f"{path} yielded no jobs"
