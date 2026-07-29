"""Assert every required status check names a job that exists.

Branch protection lists required checks by their reported name. Rename a job,
or change a matrix dimension that appears in a job's name, and the old context
simply never arrives — GitHub waits for it forever, or, worse, the protection
rule is satisfied by nothing at all because the context was removed on the
GitHub side and nobody noticed the gate is now shorter than it was.

Nothing in the repository couples the two. The names live in workflow YAML and
the requirement lives in a repository setting, so this is the only place the
pair can be compared.

The audit of 2026-07-29 compared them by hand and found them consistent. Doing
it by hand is how it drifts: the comparison has to be somebody's job or it is
nobody's.

Run as: python -m scripts.checks.protection
"""

from __future__ import annotations

import itertools
import json
import re
import shutil
import subprocess
from pathlib import Path

from scripts.checks.report import Report

WORKFLOWS = Path(".github/workflows")
REPOSITORY = "ioplane/terraform-provider-powerdns"
BRANCH = "main"

MATRIX_REFERENCE = re.compile(r"\$\{\{\s*matrix\.([A-Za-z0-9_-]+)\s*\}\}")


def expand(name: str, matrix: dict[str, list[str]]) -> list[str]:
    """Return every name a job template produces under `matrix`.

    Only the dimensions the name actually references are expanded — a matrix
    axis that does not appear in the name does not multiply it, because GitHub
    disambiguates such jobs by appending the values rather than substituting
    them, and this check is about the names a person wrote.
    """
    keys = [key for key in MATRIX_REFERENCE.findall(name) if key in matrix]
    if not keys:
        return [name]
    names: list[str] = []
    for combination in itertools.product(*(matrix[key] for key in keys)):
        resolved = name
        for key, value in zip(keys, combination, strict=True):
            resolved = re.sub(
                r"\$\{\{\s*matrix\." + re.escape(key) + r"\s*\}\}", value, resolved
            )
        names.append(resolved)
    return names


JOB_ID = re.compile(r"^  ([A-Za-z0-9_-]+):\s*$")
JOB_NAME = re.compile(r"^    name:\s*(.+?)\s*$")
MATRIX_OPEN = re.compile(r"^      matrix:\s*$")
MATRIX_INLINE = re.compile(r"^        ([A-Za-z0-9_-]+):\s*\[(.*)\]\s*$")
MATRIX_BLOCK = re.compile(r"^        ([A-Za-z0-9_-]+):\s*$")
MATRIX_ITEM = re.compile(r"^          - (.+?)\s*$")


def unquote(value: str) -> str:
    """Strip one layer of YAML quoting."""
    return value.strip().strip("'\"")


def job_names(workflow: str) -> list[str]:
    """Return the reported names of every job in one workflow's YAML.

    Read with a narrow parser rather than a YAML library, because these checks
    are invoked as `python3 -m` with no virtualenv and so may use the standard
    library only. The shape needed is small and its indentation is fixed by the
    workflow schema: a job at two spaces, its `name:` at four, and the matrix
    axes at eight under `matrix:` at six.
    """
    names: list[str] = []
    in_jobs = False
    in_matrix = False
    current: str | None = None
    matrix: dict[str, list[str]] = {}
    axis: str | None = None

    def flush() -> None:
        if current is not None:
            names.extend(expand(current, matrix))

    for line in workflow.splitlines():
        if line.startswith("jobs:"):
            in_jobs = True
            continue
        if not in_jobs:
            continue

        job = JOB_ID.match(line)
        if job:
            flush()
            current, matrix, axis, in_matrix = None, {}, None, False
            continue

        name = JOB_NAME.match(line)
        if name:
            current = unquote(name.group(1))
            continue

        if MATRIX_OPEN.match(line):
            in_matrix = True
            continue
        if not in_matrix:
            continue

        inline = MATRIX_INLINE.match(line)
        if inline:
            matrix[inline.group(1)] = [
                unquote(value) for value in inline.group(2).split(",") if value.strip()
            ]
            axis = None
            continue

        item = MATRIX_ITEM.match(line)
        if item and axis:
            matrix[axis].append(unquote(item.group(1)))
            continue

        block = MATRIX_BLOCK.match(line)
        if block:
            axis = block.group(1)
            matrix[axis] = []
            continue

        # Anything at six spaces or less has left the matrix.
        if line.strip() and not line.startswith("        "):
            in_matrix = False

    flush()
    return names


def declared_names() -> set[str]:
    """Every job name declared across the repository's workflows."""
    found: set[str] = set()
    for path in sorted(WORKFLOWS.glob("*.yml")):
        found.update(job_names(path.read_text(encoding="utf-8")))
    return found


def required_contexts() -> list[str] | None:
    """The contexts branch protection requires, or None if it cannot be read."""
    if shutil.which("gh") is None:
        return None
    result = subprocess.run(
        [
            "gh",
            "api",
            f"repos/{REPOSITORY}/branches/{BRANCH}/protection",
            "--jq",
            ".required_status_checks.contexts",
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        return None
    try:
        contexts = json.loads(result.stdout or "null")
    except json.JSONDecodeError:
        return None
    if not isinstance(contexts, list):
        return None
    # Narrowed rather than cast: the API is documented to return strings, and a
    # payload that does not is a surprise worth failing on rather than
    # formatting into a message.
    return [context for context in contexts if isinstance(context, str)]


def main() -> int:
    """Compare the required contexts against the job names that exist."""
    report = Report("check-protection")
    declared = declared_names()

    contexts = required_contexts()
    if contexts is None:
        # Not a failure: a contributor without repository administration cannot
        # read the protection rule, and refusing to run for them would make the
        # gate depend on who is running it.
        report.warn(
            "branch protection is unreadable here — gh is missing, "
            "unauthenticated, or lacks administration scope"
        )
        return report.summary(
            f"{len(declared)} job names read; protection not compared"
        )

    for context in sorted(contexts):
        # Third-party checks post their own contexts and have no job in this
        # repository. They are required deliberately and are not drift.
        if context in declared:
            report.ok(f"{context}")
        elif "/" in context or context.islower():
            report.ok(f"{context} (external)")
        else:
            report.fail(f"{context} is required but no job reports that name")

    return report.summary(f"{len(contexts)} required contexts, all accounted for")


if __name__ == "__main__":
    raise SystemExit(main())
