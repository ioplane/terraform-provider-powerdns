#!/usr/bin/env python3
"""Warn before a file:line citation without a revision reaches a file.

docs/standards/verified-identifiers.md forbids writing an exact identifier from
memory. It was broken once in a way that mattered: `ws-auth.cc:3361` was written
with no revision, which is correct on master and wrong at the tag this project
pins, where the same registration is line 3349.

A line number without a revision is not wrong so much as unfalsifiable — a
reader cannot tell which of the two it meant. This warns rather than blocks,
because the fix is to add a revision, not to delete the citation.

Self-contained by necessity: the hook runs from whatever directory the agent is
in, with no virtualenv. Standard library only, and no imports from scripts/.
"""

from __future__ import annotations

import json
import re
import sys

ALLOW = 0

# A PowerDNS source citation: a .cc or .hh file with a line number.
CITATION = re.compile(r"[a-zA-Z0-9_./-]+\.(?:cc|hh):[0-9]+")

# Satisfied when a revision is named nearby: a tag, a commit, or the word.
REVISION = re.compile(
    r"at (tag|commit|revision)|auth-5\.|rec-5\.|dnsdist-2\.|master [0-9a-f]{7,}",
    re.IGNORECASE,
)

MESSAGE = """\
Warning: a source citation without a revision.

{citations}

Line numbers move. `ws-auth.cc:3361` is right on master and wrong at tag
auth-5.1.3, where the same registration is line 3349 — a reader cannot tell
which was meant. Name the revision:

  ws-auth.cc:3349 at tag auth-5.1.3
  ws-auth.cc:3361 on master a74d89a8

Verify it rather than recalling it:
  git -C /opt/projects/repositories/pdns-upstream show <ref>:<path> | grep -n ...
"""


def unrevisioned(content: str) -> list[str]:
    """Return the citations in `content` that name no revision."""
    if not content.strip():
        return []
    if REVISION.search(content):
        return []
    return sorted(set(CITATION.findall(content)))


def main() -> int:
    """Read the edit and warn if it cites source without a revision."""
    try:
        payload = json.load(sys.stdin)
    except (json.JSONDecodeError, ValueError):
        return ALLOW

    tool_input = payload.get("tool_input") or {}
    content = f"{tool_input.get('content') or ''}\n{tool_input.get('new_string') or ''}"

    citations = unrevisioned(content)
    if citations:
        listed = "\n".join(f"  {citation}" for citation in citations)
        print(MESSAGE.format(citations=listed), file=sys.stderr)
    return ALLOW


if __name__ == "__main__":
    raise SystemExit(main())
