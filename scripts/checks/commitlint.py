"""Lint a commit message inside the dev container.

pre-commit passes the path to the message file. It is read here, on the host,
and piped into the container: commitlint's own --edit resolves the path from
git, and in a worktree that is an absolute path into the main checkout's .git
directory, which the container has never seen.

Run as: python -m scripts.checks.commitlint <commit-msg-file>
"""

from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path

from scripts.checks.paths import checked_path

COMPOSE_FILE = "deployments/compose/compose.dev.yml"


def dev_suffix(cwd: Path) -> str:
    """Return the container suffix identifying this checkout.

    Each worktree runs its own dev container so two sprints can be open at
    once; the suffix is how they are told apart. See Taskfile.yml.
    """
    return f"-{cwd.name}" if ".worktrees" in cwd.parts else ""


def main(argv: list[str], bases: tuple[Path, ...] | None = None) -> int:
    """Pipe the commit message named by the single argument into commitlint."""
    if len(argv) != 1:
        print(
            "usage: python -m scripts.checks.commitlint <commit-msg-file>",
            file=sys.stderr,
        )
        return 2

    try:
        message = checked_path(argv[0], bases).read_bytes()
    except (ValueError, OSError) as error:
        print(f"{error}", file=sys.stderr)
        return 2
    cwd = Path.cwd()
    result = subprocess.run(
        [
            "podman-compose",
            "-f",
            COMPOSE_FILE,
            "exec",
            "-T",
            "dev",
            "npx",
            "--no-install",
            "commitlint",
            "--config",
            ".commitlintrc.yaml",
        ],
        input=message,
        env={**os.environ, "DEV_SUFFIX": dev_suffix(cwd)},
        check=False,
    )
    return result.returncode


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
