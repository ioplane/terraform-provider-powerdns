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

from scripts.automation.dev_identity import dev_suffix
from scripts.checks.paths import checked_path

COMPOSE_FILE = "deployments/compose/compose.dev.yml"
REPO_ROOT = Path(__file__).resolve().parents[2]


def main(
    argv: list[str],
    bases: tuple[Path, ...] | None = None,
    repo_root: Path | None = None,
) -> int:
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
    root = REPO_ROOT if repo_root is None else repo_root
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
        env={**os.environ, "DEV_SUFFIX": dev_suffix(root)},
        check=False,
    )
    return result.returncode


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
