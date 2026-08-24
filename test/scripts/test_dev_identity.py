"""Contracts for development-container identity consumers."""

from __future__ import annotations

import re
import shutil
import subprocess
from pathlib import Path
from typing import TYPE_CHECKING, cast

import pytest
from scripts.automation import dev_identity, e2e
from scripts.automation.run import LOCAL
from scripts.checks import commitlint

if TYPE_CHECKING:
    from collections.abc import Callable

REPO_ROOT = Path(__file__).resolve().parents[2]
PRE_COMMIT_CONFIG = REPO_ROOT / ".pre-commit-config.yaml"
DEV_SUFFIX_HELPER = REPO_ROOT / "scripts" / "dev-suffix.sh"
YAMLLINT_CONTROL_DIRECTIVE = re.compile(r"#\s*yamllint\b", re.IGNORECASE)
REPOSITORY_ITEM = re.compile(r"  - repo: \S.*")


def _git(cwd: Path, *arguments: str) -> subprocess.CompletedProcess[str]:
    """Run one fixed Git operation in a disposable test repository."""
    return subprocess.run(
        ["git", "-C", str(cwd), *arguments],
        check=True,
        capture_output=True,
        text=True,
    )


def _linked_checkout(
    tmp_path: Path,
    *,
    repository_name: str = "provider",
    branch_name: str = "fix/identity/same-leaf",
) -> tuple[Path, str]:
    """Create a real linked worktree with self-contained Git metadata."""
    repository = tmp_path / repository_name
    repository.mkdir(parents=True)
    _git(repository, "init")
    _git(repository, "config", "user.email", "tests@example.invalid")
    _git(repository, "config", "user.name", "Identity tests")

    helper = repository / "scripts" / "dev-suffix.sh"
    helper.parent.mkdir()
    shutil.copy2(DEV_SUFFIX_HELPER, helper)
    _git(repository, "add", "scripts/dev-suffix.sh")
    _git(repository, "commit", "-m", "test fixture")

    linked = tmp_path / ".worktrees" / repository_name / "same-leaf"
    linked.parent.mkdir(parents=True)
    _git(
        repository,
        "worktree",
        "add",
        "-b",
        branch_name,
        str(linked),
    )
    result = subprocess.run(
        [str(linked / "scripts" / "dev-suffix.sh")],
        cwd=linked,
        check=True,
        capture_output=True,
        text=True,
    )
    return linked, result.stdout.strip()


@pytest.fixture
def linked_checkout(tmp_path: Path) -> tuple[Path, str]:
    """Provide one canonical linked-worktree identity."""
    return _linked_checkout(tmp_path)


def _format_hook(config: str) -> str:
    """Return only the local Go-format hook YAML block."""
    return config.split("      - id: format\n", 1)[1].split("\n      - id:", 1)[0]


def _assert_format_hook_contract(config: str) -> None:
    """Require the exact effective format-hook contract."""
    assert YAMLLINT_CONTROL_DIRECTIVE.search(config) is None
    duplicate_check = subprocess.run(
        ["yamllint", "-d", "{rules: {key-duplicates: enable}}", "-"],
        input=config,
        text=True,
        capture_output=True,
        check=False,
    )
    assert duplicate_check.returncode == 0, (
        duplicate_check.stdout + duplicate_check.stderr
    )
    root_lines = [
        line
        for line in config.splitlines()
        if line and not line[0].isspace() and not line.startswith("#")
    ]
    assert root_lines == ["repos:"]
    prefix = config.split("      - id: format\n", 1)[0]
    prefix_lines = prefix.splitlines()
    repository_item_indexes = [
        index
        for index, line in enumerate(prefix_lines)
        if line.lstrip().startswith("-") and len(line) - len(line.lstrip()) < 6
    ]
    repository_items = [prefix_lines[index] for index in repository_item_indexes]
    assert repository_items
    assert all(REPOSITORY_ITEM.fullmatch(line) for line in repository_items)
    owners = [line.strip() for line in repository_items]
    assert owners[-1] == "- repo: local"
    local_owner_tail = [
        line
        for line in prefix_lines[repository_item_indexes[-1] + 1 :]
        if line.strip() and not line.lstrip().startswith("#")
    ]
    assert local_owner_tail
    assert local_owner_tail[0] == "    hooks:"
    hook = _format_hook(config)
    significant_lines = [line.strip() for line in hook.splitlines() if line.strip()]
    assert significant_lines == [
        "name: golangci-lint fmt",
        "language: system",
        "types: [go]",
        "exclude: '^vendor/'",
        "pass_filenames: false",
        "entry: task fmt",
    ]


def test_e2e_uses_the_canonical_suffix(linked_checkout, monkeypatch):
    """The e2e driver must select the same container that Task created."""
    linked, suffix = linked_checkout
    monkeypatch.setattr(e2e, "REPO_ROOT", linked)

    assert e2e.dev_container() == f"{e2e.DEV_CONTAINER_DEFAULT}{suffix}"


def test_commitlint_uses_the_canonical_suffix(linked_checkout):
    """Commitlint must not collapse equal-leaf worktrees to one container."""
    linked, suffix = linked_checkout

    assert commitlint.dev_suffix(linked) == suffix


def test_go_format_hook_uses_the_canonical_task_boundary():
    """Formatting must inherit Task's worktree-specific container identity."""
    config = PRE_COMMIT_CONFIG.read_text(encoding="utf-8")

    _assert_format_hook_contract(config)


def test_go_format_hook_rejects_a_non_executing_dry_run():
    """A dry-run hook must not satisfy the formatting contract."""
    config = PRE_COMMIT_CONFIG.read_text(encoding="utf-8")
    mutation = config.replace("entry: task fmt", "entry: task fmt --dry", 1)

    with pytest.raises(AssertionError):
        _assert_format_hook_contract(mutation)


def test_go_format_hook_rejects_a_multiline_dry_run_scalar():
    """A YAML continuation must not smuggle arguments into the formatter."""
    config = PRE_COMMIT_CONFIG.read_text(encoding="utf-8")
    mutation = config.replace(
        "entry: task fmt",
        "entry: task fmt\n          --dry",
        1,
    )

    with pytest.raises(AssertionError):
        _assert_format_hook_contract(mutation)


def test_go_format_hook_rejects_dry_run_arguments():
    """Pre-commit arguments must not modify the effective Task command."""
    config = PRE_COMMIT_CONFIG.read_text(encoding="utf-8")
    mutation = config.replace(
        "        entry: task fmt",
        "        args: [--dry]\n        entry: task fmt",
        1,
    )

    with pytest.raises(AssertionError):
        _assert_format_hook_contract(mutation)


def test_go_format_hook_rejects_a_global_go_exclusion():
    """Global filters must not skip every Go file with a successful status."""
    config = PRE_COMMIT_CONFIG.read_text(encoding="utf-8")
    mutation = "exclude: '\\.go$'\n" + config

    with pytest.raises(AssertionError):
        _assert_format_hook_contract(mutation)


def test_go_format_hook_rejects_a_remote_manifest_owner():
    """A remote manifest must not inject hidden format-hook defaults."""
    config = PRE_COMMIT_CONFIG.read_text(encoding="utf-8")
    mutation = config.replace(
        "  - repo: local\n",
        "  - repo: https://example.invalid/hooks\n    rev: v1.0.0\n",
        1,
    )

    with pytest.raises(AssertionError):
        _assert_format_hook_contract(mutation)


def test_go_format_hook_rejects_a_split_remote_manifest_owner():
    """An alternate YAML sequence form must not hide the remote owner."""
    config = PRE_COMMIT_CONFIG.read_text(encoding="utf-8")
    mutation = config.replace(
        "      - id: format\n",
        "  -\n"
        "    repo: https://example.invalid/hooks\n"
        "    rev: v1.0.0\n"
        "    hooks:\n"
        "      - id: format\n",
        1,
    )

    with pytest.raises(AssertionError):
        _assert_format_hook_contract(mutation)


def test_go_format_hook_rejects_a_continued_remote_manifest_owner():
    """A folded scalar must not turn the local owner into a remote repository."""
    config = PRE_COMMIT_CONFIG.read_text(encoding="utf-8")
    mutation = config.replace(
        "  - repo: local\n",
        "  - repo: local\n      https://example.invalid/hooks\n    rev: v1.0.0\n",
        1,
    )

    with pytest.raises(AssertionError):
        _assert_format_hook_contract(mutation)


def test_go_format_hook_rejects_a_duplicate_remote_owner():
    """A duplicate YAML key must not override the apparent local owner."""
    config = PRE_COMMIT_CONFIG.read_text(encoding="utf-8")
    mutation = config.replace(
        "  - repo: local\n",
        "  - repo: local\n    repo: https://example.invalid/hooks\n    rev: v1.0.0\n",
        1,
    )

    with pytest.raises(AssertionError):
        _assert_format_hook_contract(mutation)


def test_go_format_hook_rejects_a_suppressed_duplicate_remote_owner():
    """A yamllint directive must not hide an owner override."""
    config = PRE_COMMIT_CONFIG.read_text(encoding="utf-8")
    mutation = config.replace(
        "  - repo: local\n",
        "  - repo: local\n"
        "    # yamllint disable-line rule:key-duplicates\n"
        "    repo: https://example.invalid/hooks\n"
        "    rev: v1.0.0\n",
        1,
    )

    with pytest.raises(AssertionError):
        _assert_format_hook_contract(mutation)


def test_adapter_invokes_the_canonical_helper(monkeypatch, linked_checkout):
    """The adapter delegates all identity semantics to the shell helper."""
    linked, suffix = linked_checkout
    captured: dict[str, object] = {}

    def fake_run(
        argv: list[str], **options: object
    ) -> subprocess.CompletedProcess[str]:
        captured.update({"argv": argv, **options})
        return subprocess.CompletedProcess(argv, 0, stdout=f"{suffix}\n", stderr="")

    monkeypatch.setattr(dev_identity, "run", fake_run)

    assert dev_identity.dev_suffix(linked) == suffix
    assert captured == {
        "argv": [str(linked / "scripts" / "dev-suffix.sh")],
        "what": "derive development container identity",
        "timeout": LOCAL,
        "cwd": linked,
        "capture_output": True,
        "text": True,
    }


def test_adapter_propagates_helper_failure(monkeypatch, linked_checkout):
    """A broken identity helper must stop the consumer rather than fall back."""
    linked, _ = linked_checkout

    def fail(argv: list[str], **options: object) -> subprocess.CompletedProcess[str]:
        del options
        raise subprocess.CalledProcessError(64, argv)

    monkeypatch.setattr(dev_identity, "run", fail)

    with pytest.raises(subprocess.CalledProcessError):
        dev_identity.dev_suffix(linked)


def test_equal_leaf_worktrees_keep_distinct_container_names(tmp_path):
    """Left-cancellation preserves the helper's collision resistance."""
    first, first_suffix = _linked_checkout(
        tmp_path / "first",
        repository_name="provider",
        branch_name="fix/first/same-leaf",
    )
    second, second_suffix = _linked_checkout(
        tmp_path / "second",
        repository_name="provider",
        branch_name="fix/second/same-leaf",
    )

    assert first.name == second.name == "same-leaf"
    assert first_suffix != second_suffix
    assert e2e.dev_container(first) != e2e.dev_container(second)


def test_both_python_consumers_bind_the_same_adapter():
    """A second identity implementation would reopen cross-worktree drift."""
    assert e2e.dev_suffix is dev_identity.dev_suffix
    assert commitlint.dev_suffix is dev_identity.dev_suffix


def test_commitlint_places_adapter_identity_in_subprocess_environment(
    monkeypatch,
    linked_checkout,
    tmp_path,
):
    """The hook subprocess must receive the adapter result, not a basename."""
    linked, suffix = linked_checkout
    message = tmp_path / "COMMIT_EDITMSG"
    message.write_text("fix(e2e): test identity\n", encoding="utf-8")
    captured: dict[str, object] = {}

    def fake_suffix(repo_root: Path) -> str:
        assert repo_root == linked
        return suffix

    def fake_subprocess_run(
        argv: list[str],
        *,
        input: bytes,  # noqa: A002 - mirrors subprocess.run's keyword
        env: dict[str, str],
        check: bool,
    ) -> subprocess.CompletedProcess[bytes]:
        captured.update({"argv": argv, "input": input, "env": env, "check": check})
        return subprocess.CompletedProcess(argv, 0)

    monkeypatch.setattr(commitlint, "dev_suffix", fake_suffix)
    monkeypatch.setattr(commitlint.subprocess, "run", fake_subprocess_run)

    assert commitlint.main([str(message)], bases=(tmp_path,), repo_root=linked) == 0
    captured_env = cast("dict[str, str]", captured["env"])
    assert captured_env["DEV_SUFFIX"] == suffix


@pytest.mark.parametrize(
    "mutation",
    [
        lambda root: f"-{root.name}",
        lambda _root: "",
    ],
)
def test_basename_or_empty_identity_mutations_are_rejected(
    mutation: Callable[[Path], str],
    linked_checkout,
):
    """Known stale consumer substitutions disagree with the canonical helper."""
    linked, suffix = linked_checkout

    assert mutation(linked) != suffix
