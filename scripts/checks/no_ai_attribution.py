"""Enforce AGENTS.md golden rule 6: no AI attribution in commit messages.

The rule bans a *claim of authorship*, not the letters. It once fired on
`.claude/hooks/guard-main-branch.py` — a path in this repository, named by the
tool that reads it, in a message describing what the file does. Refusing that
would mean the repository could never describe its own contents.

So paths under the tool directory are stripped before matching. What remains is
prose, and prose claiming a machine wrote the change is what the rule is about.

Run as: python -m scripts.checks.no_ai_attribution <commit-msg-file>
"""

from __future__ import annotations

import re
import sys
from typing import TYPE_CHECKING

from scripts.checks.paths import checked_path

if TYPE_CHECKING:
    from pathlib import Path

# An assertion of authorship, in any of the shapes it usually takes.
CLAIM = re.compile(
    r"""
      co-authored-by:\s*(claude|chatgpt|copilot|ai\b)
    | generated\s+(with|by)\s+(claude|chatgpt|copilot|an?\s+ai)
    | written\s+by\s+(claude|chatgpt|copilot|an?\s+ai)
    | assisted\s+by\s+(claude|chatgpt|copilot|an?\s+ai)
    | \bchatgpt\b
    | \bcopilot\b
    | \bgpt-[0-9]
    | \banthropic\b
    | \bopenai\b
    """,
    re.IGNORECASE | re.VERBOSE,
)

# A bare "claude" is ambiguous: it names the tool directory this repository
# carries. The character classes are spelled out rather than using \w, which
# would also exclude an underscore and quietly narrow the rule.
BARE_NAME = re.compile(r"(^|[^./a-zA-Z0-9])claude([^./a-zA-Z0-9]|$)", re.IGNORECASE)

# Paths under the tool directory, which are contents rather than claims.
TOOL_PATH = re.compile(r"\S*\.claude/\S*")


def prose(message: str) -> str:
    """Return `message` with paths under the tool directory removed."""
    return TOOL_PATH.sub("", message)


def offences(message: str) -> list[tuple[int, str]]:
    """Return the numbered lines of `message` that claim AI authorship."""
    found: list[tuple[int, str]] = []
    for number, line in enumerate(prose(message).splitlines(), start=1):
        if CLAIM.search(line) or BARE_NAME.search(line):
            found.append((number, line))
    return found


def main(argv: list[str], bases: tuple[Path, ...] | None = None) -> int:
    """Check the commit message named by the single argument."""
    if len(argv) != 1:
        print(
            "usage: python -m scripts.checks.no_ai_attribution <commit-msg-file>",
            file=sys.stderr,
        )
        return 2

    try:
        message = checked_path(argv[0], bases)
    except ValueError as error:
        print(f"{error}", file=sys.stderr)
        return 2

    found = offences(message.read_text(encoding="utf-8"))
    if not found:
        return 0

    print(
        "commit message claims AI authorship; see AGENTS.md golden rule 6",
        file=sys.stderr,
    )
    for number, line in found:
        print(f"{number}:{line}", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
