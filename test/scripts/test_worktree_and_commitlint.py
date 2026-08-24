"""The two decisions in the worktree helper and the commitlint wrapper.

Both are mostly subprocess calls, which are the shell's job and not worth
mocking. What is worth testing is the arithmetic each does before calling out —
where a worktree lands, and which container the message is piped into — because
both were silent string manipulation in the shell versions.
"""

from __future__ import annotations

from pathlib import Path

import pytest
from scripts.automation.worktree import main as worktree_main
from scripts.automation.worktree import worktree_path


def test_the_worktree_lands_beside_the_repository_not_inside_it(monkeypatch):
    """Inside the checkout it would be walked by every linter and test runner."""
    monkeypatch.setattr(
        "scripts.automation.worktree.repo_root", lambda: Path("/repos/provider")
    )
    assert worktree_path("sprint/S1-x") == Path("/repos/.worktrees/sprint/S1-x")


def test_a_slashed_branch_keeps_its_shape_on_disk(monkeypatch):
    """The naming standard mandates slashes, so `git worktree add` needs the parent."""
    monkeypatch.setattr(
        "scripts.automation.worktree.repo_root", lambda: Path("/repos/provider")
    )
    path = worktree_path("feat/dnssec/cryptokey-resource")
    assert path.parent == Path("/repos/.worktrees/feat/dnssec")


def test_an_unknown_subcommand_is_a_usage_error():
    """Argparse decides this, and the exit status is what the caller reads."""
    with pytest.raises(SystemExit) as exit_info:
        worktree_main(["frobnicate"])
    assert exit_info.value.code == 2


def test_no_subcommand_is_a_usage_error():
    """Bare `worktree` used to print usage and exit 2; it still must."""
    with pytest.raises(SystemExit) as exit_info:
        worktree_main([])
    assert exit_info.value.code == 2
