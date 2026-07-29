#!/usr/bin/env python3
"""Refuse a commit or push on main.

AGENTS.md has said "main is never committed to directly" since phase 0. Phases
0 to 4 were committed straight to main anyway, fourteen times, because a rule
that lives only in a document is a rule that gets forgotten at the moment it
applies.

This is that rule as a mechanism. It reads the tool call the agent is about to
make and blocks it when the branch is main.

Exit 0 allows; exit 2 blocks and returns the message on stderr to the model.

Self-contained by necessity: the hook runs from whatever directory the agent
is in, with no virtualenv and no guarantee this repository's packages are
importable. Standard library only, and no imports from scripts/.
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

ALLOW = 0
BLOCK = 2

# Only these two are interesting. Anything else on main is fine.
GUARDED = ("git commit", "git push")

# A merge landing on main through gh is the sanctioned path, not a direct
# commit. So is a fast-forward pull.
SANCTIONED = ("gh pr merge", "git pull", "--ff-only")

MESSAGE = """\
Blocked: this is main, and AGENTS.md says main is never committed to directly.

One worktree per sprint, one pull request per sprint:

  task worktree:new BRANCH=sprint/<phase>-<name>
  cd ../.worktrees/sprint/<phase>-<name>
  # work, task all, task verify
  gh pr create --fill
  gh pr merge --squash --delete-branch
  task worktree:rm BRANCH=sprint/<phase>-<name>

If this really has to land on main — a merge through gh, or a fast-forward
pull — those are already allowed and this would not have fired.
"""


def current_branch(cwd: Path) -> str | None:
    """Return the checked-out branch in `cwd`, or None if it is not a checkout."""
    result = subprocess.run(
        ["git", "rev-parse", "--abbrev-ref", "HEAD"],
        capture_output=True,
        text=True,
        check=False,
        cwd=cwd,
    )
    return result.stdout.strip() if result.returncode == 0 else None


def blocks(command: str, branch: str | None) -> bool:
    """Whether this command on this branch must be refused."""
    if branch != "main":
        return False
    if not any(guarded in command for guarded in GUARDED):
        return False
    return not any(allowed in command for allowed in SANCTIONED)


def main() -> int:
    """Read the tool call and decide."""
    try:
        payload = json.load(sys.stdin)
    except (json.JSONDecodeError, ValueError):
        return ALLOW

    command = (payload.get("tool_input") or {}).get("command") or ""
    if not command:
        return ALLOW
    if not any(guarded in command for guarded in GUARDED):
        return ALLOW

    cwd = payload.get("cwd") or ""
    if not cwd or not Path(cwd).is_dir():
        return ALLOW

    if blocks(command, current_branch(Path(cwd))):
        print(MESSAGE, file=sys.stderr)
        return BLOCK
    return ALLOW


if __name__ == "__main__":
    raise SystemExit(main())
