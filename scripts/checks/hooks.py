"""Assert the git hooks are installed and the attribution ban still bites.

Golden rule 6 in AGENTS.md forbids AI attribution in commits. That ban is only
real if the hook carrying it is installed and rejects a message that violates
it. A configuration file committed to the repository proves neither:
`.git/hooks` is not tracked, so a fresh clone starts unprotected.

The exhaustive cases live in test/scripts/test_no_ai_attribution.py, where they
can be extended without writing more of this. What stays here is the pair that
must hold wherever this runs — one message that must be refused and one that
must not — because the tests prove the classifier and this proves the gate.

Run as: python -m scripts.checks.hooks
"""

from __future__ import annotations

import subprocess
from pathlib import Path

from scripts.checks.no_ai_attribution import offences
from scripts.checks.report import Report

REQUIRED_HOOKS = ("commit-msg", "pre-commit")

REFUSED = "feat(x): a change\n\nCo-Authored-By: Claude <noreply@anthropic.com>\n"
ALLOWED = "feat(x): a change\n\nAn ordinary body naming the reason.\n"


def hooks_dir() -> Path:
    """Return the directory git actually reads hooks from.

    `--git-common-dir`, not `.git`: in a worktree `.git` is a file pointing at
    the main checkout, and the hooks live with the main checkout. Looking in
    `.git` reports every worktree as unprotected.
    """
    common = subprocess.run(
        ["git", "rev-parse", "--git-common-dir"],
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
    return Path(common) / "hooks"


def main() -> int:
    """Check both halves: installed, and observed to reject and to accept."""
    report = Report("check-hooks")

    directory = hooks_dir()
    for hook in REQUIRED_HOOKS:
        if (directory / hook).is_file():
            report.ok(f"{hook} hook is installed")
        else:
            report.fail(f"{hook} hook is not installed — run: task hooks")

    if offences(REFUSED):
        report.ok("the attribution ban refuses a message carrying an AI trailer")
    else:
        report.fail("the attribution ban accepted a message carrying an AI trailer")

    # A checker that rejects everything is equally broken, and only testing the
    # rejection would not notice.
    if offences(ALLOWED):
        report.fail("the attribution ban rejected an ordinary message")
    else:
        report.ok("the attribution ban accepts an ordinary message")

    return report.summary("hooks installed, and the attribution ban both ways")


if __name__ == "__main__":
    raise SystemExit(main())
