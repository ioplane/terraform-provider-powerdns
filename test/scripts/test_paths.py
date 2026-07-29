"""The bounds on what an argument may become.

These exist because SonarCloud's agent-oriented rules pointed at every place
this repository turns `argv` into a path or a git argument, and it was right to:
the commands here are issued by an agent, so a fabricated argument is a realistic
input rather than a theoretical one.
"""

from __future__ import annotations

import re
from pathlib import Path

import pytest
from scripts.automation.worktree import worktree_path
from scripts.checks.paths import checked_branch, checked_path, inside


def test_a_path_inside_the_base_is_returned_unchanged(tmp_path):
    """The ordinary case: git hands the hook a file under the git directory."""
    target = tmp_path / "COMMIT_EDITMSG"
    target.write_text("feat(x): a change\n")
    assert checked_path(str(target), (tmp_path,)) == target


def test_traversal_out_of_the_base_is_refused(tmp_path):
    """`../..` is a perfectly good argument and must not be a usable one."""
    base = tmp_path / "repo"
    base.mkdir()
    with pytest.raises(ValueError, match="outside this repository"):
        checked_path(str(base / ".." / "elsewhere" / "file"), (base,))


def test_an_absolute_path_elsewhere_is_refused(tmp_path):
    """The failure mode is a read outside the project, however it is spelled."""
    with pytest.raises(ValueError, match="outside this repository"):
        checked_path("/etc/passwd", (tmp_path,))


def test_the_base_itself_is_inside_it(tmp_path):
    """`dist` given as the base directory is the release check's default."""
    assert inside(tmp_path, (tmp_path,))


def test_a_second_base_is_honoured(tmp_path):
    """In a worktree the commit-msg file is under the *main* checkout's git dir.

    Accepting only the tree being committed to would refuse every commit made
    from a worktree, which is how this repository does all of its work.
    """
    tree = tmp_path / "worktree"
    gitdir = tmp_path / "main" / ".git"
    gitdir.mkdir(parents=True)
    tree.mkdir()
    message = gitdir / "COMMIT_EDITMSG"
    message.write_text("feat(x): a change\n")
    assert checked_path(str(message), (tree, gitdir)) == message


@pytest.mark.parametrize(
    "name",
    [
        "sprint/S13-registry",
        "feat/dnssec/cryptokey-resource",
        "fix/zone/ipv6-masters",
        "main",
        "release-0.1.1",
    ],
)
def test_the_names_the_standard_allows_are_accepted(name):
    """Rejecting a legitimate branch name would block the workflow entirely."""
    assert checked_branch(name) == name


@pytest.mark.parametrize(
    "name",
    [
        "../../elsewhere",
        "--upload-pack=/bin/sh",
        "--force",
        "sprint/../..",
        "with space",
        "",
        "/absolute",
        ".hidden",
        "trailing/",
    ],
)
def test_anything_that_is_not_a_branch_name_is_refused(name):
    """`../..` would place the worktree outside `.worktrees`.

    A leading dash reaches git as an option rather than as a branch. Both are
    rejected as a side effect of requiring the shape the naming standard
    already mandates, which is the point: one rule, two benefits.
    """
    with pytest.raises(ValueError, match="not a branch name"):
        checked_branch(name)


def test_the_error_names_the_standard(tmp_path):
    """A refusal has to say what shape was expected, or it is just a wall."""
    with pytest.raises(ValueError, match=r"naming-conventions\.md"):
        checked_branch("--force")
    with pytest.raises(ValueError, match=re.escape(str(tmp_path))):
        checked_path("/etc/passwd", (tmp_path,))


def test_a_worktree_path_is_still_under_the_worktrees_directory(monkeypatch):
    """The guard's whole purpose, stated as the property it protects."""
    monkeypatch.setattr(
        "scripts.automation.worktree.repo_root", lambda: Path("/repos/provider")
    )
    assert worktree_path("sprint/S1-x") == Path("/repos/.worktrees/sprint/S1-x")
    with pytest.raises(ValueError, match="not a branch name"):
        worktree_path("../../escape")


def test_a_path_argument_is_resolved_before_it_is_compared(tmp_path):
    """A symlink out of the base is the same escape wearing a different name."""
    base = tmp_path / "repo"
    base.mkdir()
    outside = tmp_path / "outside"
    outside.mkdir()
    (outside / "file").write_text("x")
    link = base / "link"
    link.symlink_to(outside / "file")
    with pytest.raises(ValueError, match="outside this repository"):
        checked_path(str(link), (base,))


def test_the_default_bases_come_from_git_or_the_working_directory():
    """Called with no bases the guard asks git, and bounds to cwd if git is silent.

    Both answers contain the file this is invoked next to, which is the point:
    the default must be usable, not merely safe.
    """
    assert checked_path("pyproject.toml") == Path("pyproject.toml")
