"""Bounds on the paths and names these tools accept from an argument.

Every tool here takes something from `argv` and turns it into a filesystem
path or a git argument. That is ordinary for a developer's command line and
less ordinary in a repository whose commands are issued by an agent: a
mistyped or fabricated argument becomes a read outside the project, or a
worktree written somewhere nobody looks.

So the boundary is explicit. A path argument must resolve inside a directory
this repository owns, and a branch name must be a branch name — the shape
docs/standards/naming-conventions.md §4 already requires, which rejects
`../..` and `--upload-pack=…` as a side effect of rejecting everything that is
not a branch.
"""

from __future__ import annotations

import re
import subprocess
from pathlib import Path

# docs/standards/naming-conventions.md §4: kebab-case segments separated by
# slashes, with no segment starting with a dot or a dash.
BRANCH = re.compile(r"^[a-zA-Z0-9][a-zA-Z0-9._-]*(/[a-zA-Z0-9][a-zA-Z0-9._-]*)*$")


def git_directories() -> tuple[Path, ...]:
    """Return the repository root and the git directory, resolved.

    Both, not just the root: git hands the commit-msg hook a path under the
    git directory, and in a worktree that is inside the *main* checkout rather
    than under the tree being committed to.

    Falls back to the working directory where git answers nothing — inside the
    dev container `/app` is the repository's contents without its `.git`, and a
    guard that refuses every path there would be worse than one that bounds
    them to the tree it was invoked from.
    """
    found: list[Path] = []
    for argument in ("--show-toplevel", "--git-common-dir", "--git-dir"):
        result = subprocess.run(
            ["git", "rev-parse", argument],
            capture_output=True,
            text=True,
            check=False,
        )
        if result.returncode == 0 and result.stdout.strip():
            found.append(Path(result.stdout.strip()).resolve())
    return tuple(dict.fromkeys(found)) or (Path.cwd().resolve(),)


def inside(candidate: Path, bases: tuple[Path, ...]) -> bool:
    """Whether `candidate` resolves inside one of `bases`."""
    resolved = candidate.resolve()
    return any(resolved == base or base in resolved.parents for base in bases)


def checked_path(argument: str, bases: tuple[Path, ...] | None = None) -> Path:
    """Return `argument` as a path, refusing one outside the repository.

    Raises:
        ValueError: when the path resolves outside every base.
    """
    bases = git_directories() if bases is None else bases
    candidate = Path(argument)
    if not inside(candidate, bases):
        listed = ", ".join(str(base) for base in bases) or "(no repository found)"
        msg = f"{argument} resolves outside this repository; expected under {listed}"
        raise ValueError(msg)
    return candidate


def checked_branch(name: str) -> str:
    """Return `name` unchanged, refusing anything that is not a branch name.

    Raises:
        ValueError: when the name is not the shape the naming standard requires.
    """
    if not BRANCH.match(name):
        msg = (
            f"{name!r} is not a branch name — "
            "see docs/standards/naming-conventions.md §4"
        )
        raise ValueError(msg)
    return name
