"""Repository contracts for the development Containerfile."""

import re
import shlex
from pathlib import Path

import pytest

CONTAINERFILE = Path("deployments/containers/Containerfile.dev").read_text(
    encoding="utf-8"
)
GO_IMAGE = re.search(r"^ARG GO_IMAGE=(\S+)$", CONTAINERFILE, re.MULTILINE)


def instructions(containerfile: str) -> list[str]:
    """Return logical Containerfile instructions, without standalone comments."""
    parsed = []
    current = []
    for line in containerfile.splitlines():
        stripped = line.strip()
        if not current and (not stripped or stripped.startswith("#")):
            continue
        current.append(line)
        if not line.rstrip().endswith("\\"):
            parsed.append("\n".join(current))
            current = []
    if current:
        parsed.append("\n".join(current))
    return parsed


def executable(instruction: str) -> str:
    """Remove comments before checking which shell commands can execute."""
    return "\n".join(
        line.split("#", maxsplit=1)[0] for line in instruction.splitlines()
    )


def go_tool_run(containerfile: str) -> str:
    """Return the single RUN that installs Go tools."""
    runs = [
        executable(instruction)
        for instruction in instructions(containerfile)
        if instruction.startswith("RUN ")
        and re.search(r"\bgo\s+install\b", executable(instruction))
    ]
    assert len(runs) == 1
    return runs[0]


def assert_module_cleanup_follows_tool_installs(containerfile: str) -> None:
    """Assert module cleanup executes after every install in the same RUN."""
    run = go_tool_run(containerfile)
    shell = run.removeprefix("RUN ").replace("\\\n", " ")
    try:
        lexer = shlex.shlex(shell, posix=True, punctuation_chars=";&|")
        lexer.whitespace_split = True
        lexer.commenters = ""
        tokens = list(lexer)
    except ValueError as error:
        raise AssertionError from error
    operators = {"&&", "||", ";", "&", "|", "|&", ";;", ";&", ";;&"}
    segments: list[list[str]] = []
    connectors: list[str] = []
    current: list[str] = []
    for token in tokens:
        if token in operators:
            assert current
            segments.append(current)
            connectors.append(token)
            current = []
        else:
            current.append(token)
    assert current
    segments.append(current)

    assert connectors == ["&&"] * (len(segments) - 1)
    installs = [
        index
        for index, segment in enumerate(segments)
        if any(
            segment[offset : offset + 2] == ["go", "install"]
            for offset in range(len(segment) - 1)
        )
    ]
    cleanups = [
        index
        for index, segment in enumerate(segments)
        if segment == ["go", "clean", "-modcache"]
    ]

    assert installs
    assert cleanups == [len(segments) - 1]
    assert cleanups[0] > installs[-1]


GO_TOOL_INSTALLATION = go_tool_run(CONTAINERFILE)


def test_go_module_cache_is_not_a_persistent_build_mount():
    """An incomplete shared module tree must not poison a later image build."""
    assert "type=cache,target=/go/pkg/mod" not in CONTAINERFILE


def test_go_tool_build_cache_mount_uses_the_declared_gocache():
    """Compiler objects belong in the cache mount, not the resulting OCI layer."""
    declared_gocache = re.search(r"\bGOCACHE=(/[^\s\\]+)", CONTAINERFILE)

    assert declared_gocache is not None
    assert f"type=cache,target={declared_gocache.group(1)}" in GO_TOOL_INSTALLATION
    assert "type=cache,target=/root/.cache/go-build" not in GO_TOOL_INSTALLATION


def test_go_tool_installation_cleans_downloaded_module_sources():
    """Downloaded tool sources must not be retained in the resulting OCI layer."""
    assert_module_cleanup_follows_tool_installs(CONTAINERFILE)


@pytest.mark.parametrize(
    "mutation",
    [
        lambda text: text.replace(" && go clean -modcache", " # go clean -modcache", 1),
        lambda text: text.replace(" && go clean -modcache", "", 1).replace(
            "    go install", "    go clean -modcache \\\n && go install", 1
        ),
        lambda text: (
            text.replace(" && go clean -modcache", "", 1) + "\nRUN go clean -modcache\n"
        ),
        lambda text: text.replace(
            " && go clean -modcache", " && echo go clean -modcache", 1
        ),
        lambda text: text.replace(
            " && go clean -modcache", " && sh -c 'go clean -modcache'", 1
        ),
        lambda text: text.replace(
            " && go clean -modcache", " && true || go clean -modcache", 1
        ),
    ],
    ids=[
        "comment",
        "before-installs",
        "separate-run",
        "echo",
        "shell-wrapper",
        "or-list",
    ],
)
def test_go_tool_cleanup_contract_rejects_non_executable_or_misordered_mutations(
    mutation,
):
    """Comments, early cleanup, and later layers cannot satisfy the contract."""
    with pytest.raises(AssertionError):
        assert_module_cleanup_follows_tool_installs(mutation(CONTAINERFILE))


def test_go_base_image_is_fully_qualified_and_matches_the_module_directive():
    """The image release channel and module language version must not drift."""
    module_version = re.search(
        r"^go (\d+\.\d+\.\d+)$",
        Path("go.mod").read_text(encoding="utf-8"),
        re.MULTILINE,
    )

    assert GO_IMAGE is not None
    assert module_version is not None
    major, minor, patch = module_version.group(1).split(".")
    image_version = f"{major}.{minor}" if patch == "0" else module_version.group(1)
    assert GO_IMAGE.group(1).startswith(
        f"docker.io/library/golang:{image_version}-trixie@sha256:"
    )

    assert "\ntoolchain " not in Path("go.mod").read_text(encoding="utf-8")


def test_workflow_go_images_match_the_complete_containerfile_pin():
    """A matching digest cannot hide an outdated Go tag in a workflow."""
    assert GO_IMAGE is not None

    workflow_images = []
    for workflow in Path(".github/workflows").glob("*.yml"):
        workflow_images.extend(
            line.split("#", maxsplit=1)[0].strip()
            for line in workflow.read_text(encoding="utf-8").splitlines()
            if re.search(r"#\s*pin:\s*GO_IMAGE\b", line)
        )

    assert workflow_images
    assert all(GO_IMAGE.group(1) in image for image in workflow_images)
