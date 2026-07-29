"""Worktree helper.

`main` is never committed to directly (AGENTS.md), so every piece of work
starts with the same four commands. This is those four.

Run as: python -m scripts.automation.worktree {new,rm,ls} [branch]
"""

from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path

BASE_REMOTE = "origin"

EPILOG = """\
Branch names follow docs/standards/naming-conventions.md §4, e.g.
  feat/dnssec/cryptokey-resource
  fix/zone/ipv6-masters
  sprint/S3-test-harness
"""


def repo_root() -> Path:
    """Return the top of the current checkout."""
    return Path(
        subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            capture_output=True,
            text=True,
            check=True,
        ).stdout.strip()
    )


def worktree_path(branch: str) -> Path:
    """Return where the worktree for `branch` lives.

    Beside the repository rather than inside it: a worktree under the checkout
    would be walked by every linter, test runner and `git add -A` in it.
    """
    return repo_root().parent / ".worktrees" / branch


def cmd_new(branch: str) -> int:
    """Create a worktree for `branch`, cut from the remote's main."""
    path = worktree_path(branch)
    subprocess.run(["git", "fetch", BASE_REMOTE, "main"], check=True)
    path.parent.mkdir(parents=True, exist_ok=True)
    subprocess.run(
        ["git", "worktree", "add", "-b", branch, str(path), f"{BASE_REMOTE}/main"],
        check=True,
    )
    print()
    print(f"worktree: {path}")
    print(f"cd {path} && task up && task shell")
    return 0


def cmd_rm(branch: str) -> int:
    """Remove the worktree for `branch` and delete the branch."""
    subprocess.run(
        ["git", "worktree", "remove", str(worktree_path(branch))], check=True
    )
    # The branch may already be gone — deleted by `gh pr merge --delete-branch`,
    # or never created because the worktree was made by hand. Not an error.
    subprocess.run(
        ["git", "branch", "-D", branch],
        check=False,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    print(f"removed {branch}")
    return 0


def cmd_ls() -> int:
    """List the worktrees."""
    subprocess.run(["git", "worktree", "list"], check=True)
    return 0


def main(argv: list[str]) -> int:
    """Dispatch the requested subcommand."""
    parser = argparse.ArgumentParser(
        description=__doc__,
        epilog=EPILOG,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    sub = parser.add_subparsers(dest="command", required=True)
    new = sub.add_parser("new", help="create a worktree cut from origin/main")
    new.add_argument("branch")
    remove = sub.add_parser("rm", help="remove a worktree and delete the branch")
    remove.add_argument("branch")
    sub.add_parser("ls", help="list worktrees")

    args = parser.parse_args(argv)
    if args.command == "new":
        return cmd_new(args.branch)
    if args.command == "rm":
        return cmd_rm(args.branch)
    return cmd_ls()


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
