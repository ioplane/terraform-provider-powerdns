"""Executable Taskfile contracts for the disposable local lab."""

import re

import pytest
from test.scripts.test_taskfile import (
    CI_WORKFLOW,
    DEV_CONTAINERFILE,
    TASKFILE,
    task_body_from,
    task_commands_from,
    workflow_commands,
    workflow_job,
    workflow_step,
)

LAB_AUTH = '--auth {{.AUTH | default "5.1"}}'
LAB_COMMANDS = {
    task: f"python3 -m scripts.automation.lab {task.removeprefix('lab:')} {LAB_AUTH}"
    for task in ("lab:up", "lab:down", "lab:status", "lab:verify")
}
HADOLINT_TASK_COMMAND = f"{{{{.EXEC}}}} hadolint {DEV_CONTAINERFILE}"
HADOLINT_CI_COMMAND = f"hadolint {DEV_CONTAINERFILE}"


def assert_python_lab_boundary(taskfile: str) -> None:
    """Require the small executable Python lifecycle used by local and CI E2E."""
    assert "  labctl:build:\n" not in taskfile
    for task, command in LAB_COMMANDS.items():
        body = task_body_from(taskfile, task)
        assert re.findall(r"(?m)^    ([a-z_]+):", body) == ["desc", "cmds"]
        assert task_commands_from(taskfile, task) == [command]


def assert_hadolint_parity(taskfile: str, workflow: str) -> None:
    """Require local and CI lint to cover the one executable Containerfile."""
    assert task_commands_from(taskfile, "lint:containers") == [HADOLINT_TASK_COMMAND]
    step = workflow_step(workflow_job(workflow, "lint-shell"), "hadolint")
    assert workflow_commands(step) == [HADOLINT_CI_COMMAND]


def test_lab_tasks_use_the_executable_python_boundary():
    """Do not point public tasks at a future or absent control plane."""
    assert_python_lab_boundary(TASKFILE)


@pytest.mark.parametrize(
    ("old", "new"),
    [
        ("python3 -m scripts.automation.lab up", "bin/pdns-lab up"),
        ("python3 -m scripts.automation.lab up", "go run ./cmd/pdns-lab up"),
        (LAB_COMMANDS["lab:up"], "sh -c " + LAB_COMMANDS["lab:up"]),
    ],
    ids=("absent-binary", "go-run", "shell-wrapper"),
)
def test_lab_task_contract_rejects_unavailable_or_wrapped_commands(old, new):
    """A similarly named command is not the tested lifecycle boundary."""
    mutated = TASKFILE.replace(old, new, 1)
    assert mutated != TASKFILE
    with pytest.raises(AssertionError):
        assert_python_lab_boundary(mutated)


def test_task_and_ci_hadolint_the_exact_containerfile():
    """The only executable Containerfile is linted in both environments."""
    assert_hadolint_parity(TASKFILE, CI_WORKFLOW)


@pytest.mark.parametrize(
    ("surface", "old", "new"),
    [
        ("task", DEV_CONTAINERFILE, "deployments/containers/Containerfile.*"),
        ("workflow", DEV_CONTAINERFILE, "deployments/containers/Containerfile.*"),
        ("task", HADOLINT_TASK_COMMAND, HADOLINT_TASK_COMMAND + " || true"),
        ("workflow", HADOLINT_CI_COMMAND, HADOLINT_CI_COMMAND + " || true"),
    ],
    ids=("task-wildcard", "ci-wildcard", "ignored-task", "ignored-ci"),
)
def test_hadolint_contract_rejects_bypasses(surface, old, new):
    """Wildcards and suppressed failures cannot satisfy the lint contract."""
    taskfile, workflow = TASKFILE, CI_WORKFLOW
    if surface == "task":
        taskfile = taskfile.replace(old, new, 1)
    else:
        workflow = workflow.replace(old, new, 1)
    with pytest.raises(AssertionError):
        assert_hadolint_parity(taskfile, workflow)
