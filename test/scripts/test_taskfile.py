"""Repository contracts for the development-container lifecycle."""

import json
import os
import re
import subprocess
from ast import literal_eval
from pathlib import Path

TASKFILE = Path("Taskfile.yml").read_text(encoding="utf-8")
COMPOSEFILE = Path("deployments/compose/compose.dev.yml").read_text(encoding="utf-8")
DEV_SUFFIX_SCRIPT = Path("scripts/dev-suffix.sh").resolve()
CI_WORKFLOW = Path(".github/workflows/ci.yml").read_text(encoding="utf-8")

DEV_CONTAINERFILE = "deployments/containers/Containerfile.dev"


def task_body(name: str) -> str:
    """Return one top-level Task task body."""
    match = re.search(
        rf"(?ms)^  {re.escape(name)}:\n(?P<body>.*?)(?=^  [\w][\w:-]*:\n|\Z)",
        TASKFILE,
    )
    assert match is not None
    return match.group("body")


def task_body_from(taskfile: str, name: str) -> str:
    """Return one task body from arbitrary Taskfile text."""
    matches = list(
        re.finditer(
            rf"(?ms)^  {re.escape(name)}:\n(?P<body>.*?)(?=^  [\w][\w:-]*:\n|\Z)",
            taskfile,
        )
    )
    assert len(matches) == 1
    return matches[0].group("body")


def normalized(value: str) -> str:
    """Collapse YAML folding and indentation without changing token order."""
    return " ".join(value.split())


def task_commands_from(taskfile: str, name: str) -> list[str]:
    """Read scalar and folded commands from one repository-style task."""
    body = task_body_from(taskfile, name)
    inline = re.search(r"(?m)^    cmds: (?P<value>\[.*\])$", body)
    if inline is not None:
        commands = literal_eval(inline.group("value"))
        assert isinstance(commands, list)
        assert all(isinstance(command, str) for command in commands)
        return [normalized(command) for command in commands]

    block = re.search(r"(?m)^    cmds:\n(?P<value>(?: {6}.*\n| {8}.*\n)+)", body)
    assert block is not None
    lines = block.group("value").splitlines()
    commands: list[str] = []
    index = 0
    while index < len(lines):
        line = lines[index]
        if line.startswith("      #"):
            index += 1
            continue
        assert line.startswith("      - ")
        item = line[8:]
        if item == ">-":
            index += 1
            folded = []
            while index < len(lines) and lines[index].startswith("        "):
                folded.append(lines[index].strip())
                index += 1
            commands.append(normalized(" ".join(folded)))
            continue
        if item.startswith("cmd: "):
            value = item.removeprefix("cmd: ")
            command = literal_eval(value) if value.startswith(("'", '"')) else value
            commands.append(normalized(command))
            index += 1
            while index < len(lines) and lines[index].startswith("        "):
                index += 1
            continue
        commands.append(normalized(literal_eval(item)))
        index += 1
    return commands


def workflow_job(workflow: str, name: str) -> str:
    """Return one top-level workflow job by its YAML key."""
    matches = list(
        re.finditer(
            rf"(?ms)^  {re.escape(name)}:\n(?P<body>.*?)(?=^  [A-Za-z0-9_-]+:\n|\Z)",
            workflow,
        )
    )
    assert len(matches) == 1
    return matches[0].group("body")


def workflow_step(job: str, name: str) -> str:
    """Return one named step from a workflow job."""
    match = re.search(
        rf"(?ms)^      - name: {re.escape(name)}\n"
        rf"(?P<body>.*?)(?=^      - (?:name:|uses:)|\Z)",
        job,
    )
    assert match is not None
    return match.group("body")


def workflow_commands(step: str) -> list[str]:
    """Return shell commands, joining only explicit backslash continuations."""
    literal = re.search(r"(?m)^        run: \|\n(?P<command>(?: {10}.*\n)+)", step)
    if literal is not None:
        commands = []
        continued = ""
        for raw_line in literal.group("command").splitlines():
            line = raw_line.strip()
            if not line:
                continue
            if line.endswith("\\"):
                continued += line[:-1].rstrip() + " "
                continue
            commands.append(normalized(continued + line))
            continued = ""
        assert not continued
        return commands
    scalar = re.search(r"(?m)^        run: (?P<command>[^|].*)$", step)
    assert scalar is not None
    return [normalized(scalar.group("command"))]


def variable(name: str) -> str:
    """Return one scalar Task variable."""
    match = re.search(rf"(?m)^  {re.escape(name)}: (?P<value>.+)$", TASKFILE)
    assert match is not None
    return match.group("value")


def shell_variable(name: str) -> str:
    """Return a scalar or folded shell command from one Task variable."""
    match = re.search(
        rf"(?m)^  {re.escape(name)}:\n(?P<body>(?:    [^\n]*\n)+)", TASKFILE
    )
    assert match is not None
    body = match.group("body")
    scalar = re.search(r"(?m)^    sh: (?P<command>[^>|].*)$", body)
    if scalar is not None:
        command = scalar.group("command")
        return literal_eval(command) if command.startswith(("'", '"')) else command
    folded = re.search(r"(?m)^    sh: >-\n(?P<command>(?:      .*\n)+)", body)
    assert folded is not None
    return " ".join(line.strip() for line in folded.group("command").splitlines())


def fake_git_env(
    tmp_path: Path,
    *,
    root: Path,
    linked: bool = True,
    sha256sum_body: str | None = None,
) -> dict[str, str]:
    """Return an environment describing one fake Git checkout topology."""
    invocation = len(list(tmp_path.glob("fake-bin-*")))
    fake_bin = tmp_path / f"fake-bin-{invocation}"
    fake_bin.mkdir()
    git = fake_bin / "git"
    git.write_text(
        """#!/bin/sh
case "$*" in
  "rev-parse --show-toplevel") printf '%s\\n' "$FAKE_GIT_ROOT" ;;
  "rev-parse --absolute-git-dir") printf '%s\\n' "$FAKE_GIT_DIR" ;;
  "rev-parse --path-format=absolute --git-common-dir")
    printf '%s\\n' "$FAKE_GIT_COMMON_DIR" ;;
  *) exit 64 ;;
esac
""",
        encoding="utf-8",
    )
    git.chmod(0o755)
    if sha256sum_body is not None:
        sha256sum = fake_bin / "sha256sum"
        sha256sum.write_text(
            f"#!/bin/sh\n{sha256sum_body}\n",
            encoding="utf-8",
        )
        sha256sum.chmod(0o755)
    root.mkdir(parents=True, exist_ok=True)
    common_dir = tmp_path / "git-common"
    git_dir = tmp_path / f"git-dir-{invocation}" if linked else common_dir
    common_dir.mkdir(exist_ok=True)
    git_dir.mkdir(exist_ok=True)
    return {
        **os.environ,
        "PATH": f"{fake_bin}:{os.environ['PATH']}",
        "FAKE_GIT_ROOT": str(root),
        "FAKE_GIT_DIR": str(git_dir),
        "FAKE_GIT_COMMON_DIR": str(common_dir),
    }


def run_dev_suffix(
    tmp_path: Path,
    *,
    root: Path,
    cwd: Path | None = None,
    linked: bool = True,
    sha256sum_body: str | None = None,
) -> str:
    """Evaluate DEV_SUFFIX with a fake Git checkout topology."""
    result = run_dev_suffix_result(
        tmp_path,
        root=root,
        cwd=cwd,
        linked=linked,
        sha256sum_body=sha256sum_body,
    )
    assert result.returncode == 0, result.stderr
    return result.stdout.strip()


def run_dev_suffix_result(
    tmp_path: Path,
    *,
    root: Path,
    cwd: Path | None = None,
    linked: bool = True,
    sha256sum_body: str | None = None,
) -> subprocess.CompletedProcess[str]:
    """Evaluate DEV_SUFFIX and retain failures for fail-closed assertions."""
    workdir = cwd or root
    workdir.mkdir(parents=True, exist_ok=True)
    return subprocess.run(
        [str(DEV_SUFFIX_SCRIPT)],
        cwd=workdir,
        env=fake_git_env(
            tmp_path,
            root=root,
            linked=linked,
            sha256sum_body=sha256sum_body,
        ),
        check=False,
        capture_output=True,
        text=True,
    )


def folded_command(body: str) -> str:
    """Return the one folded shell command in a task body."""
    match = re.search(r"(?m)^      - >-\n(?P<command>(?: {10}.*\n)+)", body)
    assert match is not None
    return " ".join(line.strip() for line in match.group("command").splitlines())


def run_recreate_guard(
    tmp_path: Path,
    *,
    project: str,
    source: Path,
    project_label: str = "com.docker.compose.project",
) -> tuple[subprocess.CompletedProcess[str], Path]:
    """Execute only recreate's ownership block against a fake Podman CLI."""
    marker = tmp_path / "removed"
    fake_bin = tmp_path / "bin"
    fake_bin.mkdir()
    podman = fake_bin / "podman"
    podman.write_text(
        """#!/bin/sh
case "$1:$2" in
  container:exists) exit 0 ;;
  inspect:*) printf '%s\\n' "$INSPECT_JSON" ;;
  rm:--force) : > "$RM_MARKER" ;;
  *) exit 64 ;;
esac
""",
        encoding="utf-8",
    )
    podman.chmod(0o755)
    inspect = [
        {
            "Config": {"Labels": {project_label: project}},
            "Mounts": [{"Type": "bind", "Source": str(source), "Destination": "/app"}],
        }
    ]
    command = folded_command(task_body("recreate"))
    command = command.replace("{{.DEV_PROJECT}}", "expected-project")
    command = command.replace("{{.DEV_SUFFIX}}", "-fixture")
    env = {
        **os.environ,
        "PATH": f"{fake_bin}:{os.environ['PATH']}",
        "INSPECT_JSON": json.dumps(inspect),
        "RM_MARKER": str(marker),
    }
    result = subprocess.run(
        ["/bin/sh", "-c", command],
        cwd=Path.cwd(),
        env=env,
        check=False,
        capture_output=True,
        text=True,
    )
    return result, marker
