"""Repository contracts for the development-container lifecycle."""

import hashlib
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
E2E_WORKFLOW = Path(".github/workflows/e2e.yml").read_text(encoding="utf-8")
RELEASE_WORKFLOW = Path(".github/workflows/release.yml").read_text(encoding="utf-8")
SECURITY_WORKFLOW = Path(".github/workflows/security.yml").read_text(encoding="utf-8")
RELEASE_SIGNING_KEY = Path(".github/release-signing-key.asc")

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


def test_required_ci_runs_the_complete_python_gate():
    """Required CI must exercise the same Python surface as local task py."""
    job = workflow_job(CI_WORKFLOW, "lint-py")
    assert (
        "uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0"
    ) in job
    assert 'go-version: "1.27.0"' in job
    assert "cache: false" in job
    assert workflow_commands(workflow_step(job, "Install Task")) == [
        "go install github.com/go-task/task/v3/cmd/task@v3.52.0 # pin: TASK_VERSION"
    ]
    assert workflow_commands(workflow_step(job, "ruff")) == [
        "uv run --locked ruff check scripts/ test/scripts/",
        "uv run --locked ruff format --check scripts/ test/scripts/",
    ]
    ty = workflow_step(job, "ty")
    assert "continue-on-error" not in ty
    assert workflow_commands(ty) == [
        "uv run --locked --group e2e ty check scripts/ test/scripts/"
    ]
    assert workflow_commands(workflow_step(job, "pytest")) == ["uv run --locked pytest"]


def test_required_ci_shellchecks_the_worktree_identity_script():
    """The required workflow must not omit a shell path covered by task all."""
    job = workflow_job(CI_WORKFLOW, "lint-shell")
    commands = workflow_commands(workflow_step(job, "shellcheck"))
    assert "shellcheck scripts/dev-suffix.sh" in commands


def test_local_e2e_commands_refuse_to_rewrite_the_lockfile():
    """Local evidence must use the exact locked environment CI uses."""
    for task in ("e2e:up", "e2e:down", "e2e:status", "e2e"):
        commands = task_commands_from(TASKFILE, task)
        assert commands
        assert all("uv run --locked" in command for command in commands)


def test_release_requires_exact_main_push_evidence():
    """A tag may publish only after every release-relevant workflow passed."""
    tags = RELEASE_WORKFLOW.split("    tags:\n", 1)[1].split("\n\npermissions:", 1)[0]
    assert re.findall(r"(?m)^      - '(.*)'$", tags) == [
        "v[0-9]+.[0-9]+.[0-9]+",
        "v[0-9]+.[0-9]+.[0-9]+-*",
        r"v[0-9]+.[0-9]+.[0-9]+\+*",
    ]
    gate = workflow_job(RELEASE_WORKFLOW, "gate")
    step = workflow_step(gate, "The gate and the lab were green for this commit")
    text = "\n".join(workflow_commands(step))
    assert "for wf in CI Acceptance End-to-end Security" in text
    assert ".head_branch" in text
    assert "main" in text
    assert ".event" in text
    assert "push" in text


def test_release_imports_the_tag_verification_key_before_the_gate():
    """Verify tags with a public key; private signing material belongs downstream."""
    gate = workflow_job(RELEASE_WORKFLOW, "gate")
    key_import = gate.index("gpg --batch --import .github/release-signing-key.asc")
    tag_check = gate.index("- name: Version, changelog, manifest, tag")
    assert key_import < tag_check
    assert "470479157AED6BD0ABA0DBD2436437EC9E89665F" in gate
    assert "GPG_PRIVATE_KEY" not in gate
    assert "PASSPHRASE" not in gate
    assert hashlib.sha256(RELEASE_SIGNING_KEY.read_bytes()).hexdigest() == (
        "864affa4945160b611588b1f02e9964b8be6e10d241a5cad3649d6f5e62f50db"
    )


def test_private_signing_material_is_scoped_to_the_release_environment():
    """Repository-level secrets let an arbitrary tag workflow bypass the gate."""
    gate = workflow_job(RELEASE_WORKFLOW, "gate")
    signer = workflow_job(RELEASE_WORKFLOW, "goreleaser")
    assert "GPG_PRIVATE_KEY" not in gate
    assert "PASSPHRASE" not in gate
    assert re.search(r"(?m)^    environment: release$", signer)
    assert "gpg_private_key: ${{ secrets.GPG_PRIVATE_KEY }}" in signer
    assert "passphrase: ${{ secrets.PASSPHRASE }}" in signer


def test_security_scanners_fail_after_writing_sarif():
    """SARIF publication must not turn a known High into a green job."""
    osv = workflow_step(workflow_job(SECURITY_WORKFLOW, "osv"), "osv-scanner")
    trivy = workflow_step(workflow_job(SECURITY_WORKFLOW, "trivy"), "trivy")
    osv_text = "\n".join(workflow_commands(osv))
    trivy_text = "\n".join(workflow_commands(trivy))
    assert "|| true" not in osv_text
    assert 'exit "$status"' in osv_text
    assert "--exit-code 1" in trivy_text
    assert 'exit "$status"' in trivy_text


def test_downloaded_tool_archives_are_verified_before_installation():
    """TLS and a versioned URL are not content-integrity controls."""
    workflows = (
        CI_WORKFLOW,
        E2E_WORKFLOW,
        Path(".github/workflows/acceptance.yml").read_text(encoding="utf-8"),
        Path(".github/workflows/coverage.yml").read_text(encoding="utf-8"),
    )
    for workflow in workflows:
        for archive in re.findall(r"(?m)^\s*curl .* -o (?P<path>/tmp/\S+)", workflow):
            suffix = workflow.split(f"-o {archive}", 1)[1].split("sudo ", 1)[0]
            assert "sha256sum -c -" in suffix


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
