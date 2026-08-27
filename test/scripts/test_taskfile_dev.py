"""Development-container Taskfile structural contracts."""

import hashlib
import re
import subprocess
from pathlib import Path

import pytest

from test.scripts.test_taskfile import (
    COMPOSEFILE,
    TASKFILE,
    fake_git_env,
    run_dev_suffix,
    run_dev_suffix_result,
    run_recreate_guard,
    shell_variable,
    task_body,
    variable,
)


def test_dev_compose_commands_use_a_worktree_specific_project():
    """Container names alone cannot isolate podman-compose cleanup."""
    assert variable("DEV_PROJECT") == ("terraform-provider-powerdns-dev{{.DEV_SUFFIX}}")

    assert variable("DC") == (
        "DEV_SUFFIX={{.DEV_SUFFIX}} podman-compose -p {{.DEV_PROJECT}} "
        "-f deployments/compose/compose.dev.yml"
    )
    assert variable("EXEC") == (
        "DEV_SUFFIX={{.DEV_SUFFIX}} podman-compose -p {{.DEV_PROJECT}} "
        "-f deployments/compose/compose.dev.yml exec -T dev"
    )


def test_dev_suffix_distinguishes_equal_basenames_by_canonical_root(tmp_path):
    """Distinct full worktree paths cannot collide through their basename."""
    first = run_dev_suffix(
        tmp_path,
        root=tmp_path / "first" / ".worktrees" / "fix" / "containers" / "cache",
    )
    second = run_dev_suffix(
        tmp_path,
        root=tmp_path / "second" / ".worktrees" / "build" / "repo" / "cache",
    )

    assert first != second
    assert first.startswith("-cache-")
    assert second.startswith("-cache-")


def test_dev_suffix_is_stable_from_a_worktree_subdirectory(tmp_path):
    """The checkout root, not the caller's current directory, defines identity."""
    root = tmp_path / "repo" / ".worktrees" / "fix" / "containers" / "cache"
    from_root = run_dev_suffix(tmp_path, root=root)
    from_subdirectory = run_dev_suffix(tmp_path, root=root, cwd=root / "internal/api")

    assert from_subdirectory == from_root


def test_dev_suffix_stays_empty_for_the_main_checkout(tmp_path):
    """The primary checkout preserves the unsuffixed development names."""
    suffix = run_dev_suffix(tmp_path, root=tmp_path / "main", linked=False)

    assert suffix == ""


def test_dev_suffix_contains_only_safe_lowercase_name_characters(tmp_path):
    """Compose object names receive a readable safe basename and SHA-256 prefix."""
    suffix = run_dev_suffix(
        tmp_path,
        root=tmp_path / "repo" / ".worktrees" / "Feature Weird_CACHE!",
    )

    assert re.fullmatch(r"-feature-weird-cache-[0-9a-f]{12}", suffix)


def test_dev_suffix_bounds_the_readable_name_and_retains_the_root_hash(tmp_path):
    """Long branch leaves cannot produce oversized Compose object identities."""
    root = tmp_path / "repo" / ".worktrees" / ("Very-Long_" * 20)
    suffix = run_dev_suffix(tmp_path, root=root)
    expected_digest = hashlib.sha256(str(root.resolve()).encode()).hexdigest()[:12]

    assert len(suffix) <= 62
    assert re.fullmatch(r"-[a-z0-9-]{1,48}-[0-9a-f]{12}", suffix)
    assert suffix.endswith(f"-{expected_digest}")


def test_dev_suffix_fails_closed_when_sha256sum_exits_nonzero(tmp_path):
    """A failed hasher cannot collapse every worktree onto an empty digest."""
    result = run_dev_suffix_result(
        tmp_path,
        root=tmp_path / "repo" / ".worktrees" / "cache",
        sha256sum_body="exit 23",
    )

    assert result.returncode != 0
    assert result.stdout == ""


@pytest.mark.parametrize(
    "digest",
    [
        "",
        "a" * 63,
        "a" * 65,
        "A" * 64,
        "g" * 64,
    ],
    ids=["empty", "short", "long", "uppercase", "nonhex"],
)
def test_dev_suffix_rejects_malformed_sha256sum_output(tmp_path, digest):
    """Only one exact lowercase SHA-256 field is a valid worktree identity."""
    result = run_dev_suffix_result(
        tmp_path,
        root=tmp_path / "repo" / ".worktrees" / "cache",
        sha256sum_body=f"printf '%s  -\\n' '{digest}'",
    )

    assert result.returncode != 0
    assert result.stdout == ""


def test_task_dry_run_uses_the_canonical_worktree_suffix(tmp_path):
    """The real Task engine must not pre-expand suffix-script shell locals."""
    root = Path.cwd().resolve()
    expected_digest = hashlib.sha256(str(root).encode()).hexdigest()[:12]
    expected_suffix = f"-{root.name.lower()}-{expected_digest}"
    result = subprocess.run(
        ["task", "--dry", "--verbose", "up"],
        check=False,
        capture_output=True,
        text=True,
        env=fake_git_env(tmp_path, root=root),
    )
    output = result.stdout + result.stderr

    assert result.returncode == 0, output
    assert shell_variable("DEV_SUFFIX") == "scripts/dev-suffix.sh"
    assert f'result: "{expected_suffix}"' in output
    assert f"DEV_SUFFIX={expected_suffix} podman-compose" in output
    assert f"-p terraform-provider-powerdns-dev{expected_suffix}" in output


def test_suffix_script_is_checked_by_the_shell_lint_gate():
    """The host-side identity script remains inside the blocking shell gate."""
    assert "scripts/dev-suffix.sh" in task_body("lint:shell")


def test_dev_compose_image_tag_is_worktree_specific():
    """A concurrent checkout rebuild cannot retag another checkout's image."""
    assert "image: terraform-provider-powerdns-dev${DEV_SUFFIX:-}:local" in COMPOSEFILE
    assert "image: terraform-provider-powerdns-dev:local" not in COMPOSEFILE


def test_up_builds_without_implicitly_replacing_a_container():
    """Routine startup must remain non-destructive."""
    up = task_body("up")

    assert "up -d --build" in up
    assert "--force-recreate" not in up


def test_recreate_is_the_explicit_container_replacement_task():
    """Replacement builds first, targets one exact container, then starts it."""
    recreate = task_body("recreate")
    target = "terraform-provider-powerdns-dev{{.DEV_SUFFIX}}"

    assert "--force-recreate" not in TASKFILE
    assert f"target={target}" in recreate
    assert 'podman container exists "$target"' in recreate
    assert 'podman rm --force "$target"' in recreate
    assert recreate.index("{{.DC}} build") < recreate.index("podman container exists")
    assert recreate.index("podman rm --force") < recreate.index(
        "{{.DC}} up -d --no-build"
    )


@pytest.mark.parametrize(
    ("project", "source_factory"),
    [
        ("wrong-project", lambda _tmp_path: Path.cwd()),
        ("expected-project", lambda tmp_path: tmp_path / "other-worktree"),
    ],
    ids=["wrong-project", "wrong-bind-source"],
)
def test_recreate_refuses_a_container_not_owned_by_this_worktree(
    tmp_path, project, source_factory
):
    """A name collision must fail before the exact container can be removed."""
    source = source_factory(tmp_path)
    source.mkdir(parents=True, exist_ok=True)
    result, marker = run_recreate_guard(tmp_path, project=project, source=source)

    assert result.returncode != 0
    assert not marker.exists()


@pytest.mark.parametrize(
    "project_label",
    ["com.docker.compose.project", "io.podman.compose.project"],
)
def test_recreate_allows_the_exact_project_and_canonical_bind(tmp_path, project_label):
    """The ownership guard permits the container it was designed to replace."""
    result, marker = run_recreate_guard(
        tmp_path,
        project="expected-project",
        source=Path.cwd(),
        project_label=project_label,
    )

    assert result.returncode == 0, result.stderr
    assert marker.exists()


def test_dev_guard_fails_closed_when_container_go_differs_from_go_mod():
    """Every container-backed gate must reject a stale Go runtime."""
    guard = task_body("_dev-running")

    assert guard.count("- sh:") == 2
    assert "go env GOVERSION" in guard
    assert "go.mod" in guard
    assert "run: task recreate" in guard


def test_unit_gate_enables_race_shuffle_and_atomic_coverage():
    """The default unit gate exercises concurrency, order, and coverage."""
    test = task_body("test")
    assert "go test ./..." in test
    assert "-race" in test
    assert "-shuffle=on" in test
    assert "-covermode=atomic" in test


def test_aggregate_runs_explicit_go_vet():
    """The stdversion analyzer is blocking independently of go test."""
    assert "go vet ./..." in task_body("vet")
    assert "{task: vet}" in task_body("all")
