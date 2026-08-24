"""Canonical development-container identity for Python automation."""

from __future__ import annotations

from typing import TYPE_CHECKING

from scripts.automation.run import LOCAL, run

if TYPE_CHECKING:
    from pathlib import Path


def dev_suffix(repo_root: Path) -> str:
    """Return the canonical container suffix for ``repo_root``."""
    result = run(
        [str(repo_root / "scripts" / "dev-suffix.sh")],
        what="derive development container identity",
        timeout=LOCAL,
        cwd=repo_root,
        capture_output=True,
        text=True,
    )
    return result.stdout.strip()
