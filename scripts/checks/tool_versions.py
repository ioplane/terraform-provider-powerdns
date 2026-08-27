"""Assert every tool version a workflow names matches the dev image's.

The gate runs inside a podman-compose dev container. A hosted runner cannot
cheaply reproduce that — building the image costs more per job than the job —
so the workflows install the same tools themselves. The toolchain now exists in
two places, and two places drift.

The drift is the whole risk. A linter at v2.13.1 locally and v2.14 in CI
disagrees about a finding, and the argument that follows is about which machine
is right rather than about the code. So Containerfile.dev holds the versions, a
workflow line that names one carries `# pin: <ARG>`, and this refuses the
mismatch.

Two directions are checked, because only one of them is the obvious one:

  * a marked line must contain its ARG's value  — catches a bumped version
  * every required ARG must be marked somewhere — catches a deleted marker,
    which otherwise turns the check off silently and reads as passing

Run as: python -m scripts.checks.tool_versions
"""

from __future__ import annotations

import re
from pathlib import Path

from scripts.checks.report import Report

CONTAINERFILE = Path("deployments/containers/Containerfile.dev")
# pyproject declares three of the same versions. It was missed once, and the
# drift only surfaced when a workflow installed a different podman-py.
PINNED_FILES = (Path(".github/workflows"), Path("pyproject.toml"))

ARG_LINE = re.compile(r"^ARG\s+([A-Z0-9_]+)=(.+)$")
PIN_MARKER = re.compile(r"#\s*pin:\s*([A-Z0-9_]+)")
GO_IMAGE_REF = re.compile(
    r"(?<![A-Za-z0-9._/-])"
    r"docker\.io/library/golang:[^\s@'\"]+@sha256:[0-9a-f]{64}"
    r"(?=$|[\s'\"])"
)
INVALID_GO_IMAGE = (
    "GO_IMAGE must be fully qualified and contain a tag and 64-hex sha256 digest"
)

# The tools CI is expected to install. An ARG absent from this list is one the
# dev image needs and CI does not. Task joined the list when the hosted Python
# gate began executing the real Taskfile expansion oracle.
#
# OpenTofu and Terragrunt joined the list when the end-to-end suite reached CI.
# Terragrunt's version is not a formality there: `run` arrived when the CLI
# contract froze in 1.0, the suite drives every command through it, and a 0.x
# binary forwards `run` to the engine, which has no such command.
REQUIRED = (
    "GO_IMAGE",
    "GOLANGCI_LINT_VERSION",
    "GOVULNCHECK_VERSION",
    "OSV_SCANNER_VERSION",
    "TFPLUGINDOCS_VERSION",
    "GORELEASER_VERSION",
    "SYFT_VERSION",
    "TASK_VERSION",
    "GOTESTSUM_VERSION",
    "TERRAFORM_VERSION",
    "OPENTOFU_VERSION",
    "TERRAGRUNT_VERSION",
    "TERRAFORM_LINUX_AMD64_SHA256",
    "OPENTOFU_LINUX_AMD64_SHA256",
    "TERRAGRUNT_LINUX_AMD64_SHA256",
    "SHELLCHECK_LINUX_AMD64_SHA256",
    "HADOLINT_LINUX_AMD64_SHA256",
    "NODE_MAJOR",
    "MARKDOWNLINT_VERSION",
    "CSPELL_VERSION",
    "COMMITLINT_VERSION",
    "COMMITLINT_CONFIG_VERSION",
    "UV_VERSION",
    "RUFF_VERSION",
    "TY_VERSION",
    "YAMLLINT_VERSION",
    "SEMGREP_VERSION",
    "SHELLCHECK_VERSION",
    "HADOLINT_VERSION",
    "ACTIONLINT_VERSION",
    "ZIZMOR_VERSION",
    "PODMAN_PY_VERSION",
)


def declared_versions(containerfile: str) -> dict[str, str]:
    """Read the ARG defaults out of a Containerfile.

    GO_IMAGE retains both its human-readable tag and immutable digest.
    """
    versions: dict[str, str] = {}
    for line in containerfile.splitlines():
        match = ARG_LINE.match(line)
        if not match:
            continue
        name, value = match.group(1), match.group(2)
        if name == "GO_IMAGE":
            image = normalise_go_image(value)
            if image is None or image != value:
                raise ValueError(INVALID_GO_IMAGE)
            versions[name] = image
        else:
            versions[name] = value
    return versions


def normalise_go_image(value: str) -> str | None:
    """Return one canonical Docker Hub Go image reference from text."""
    match = GO_IMAGE_REF.search(value)
    if match is None:
        return None
    return match.group(0)


def marked_pin(line: str) -> str | None:
    """Return the ARG name a line's `# pin:` marker names, if it has one."""
    match = PIN_MARKER.search(line)
    return match.group(1) if match else None


def satisfies(line: str, value: str) -> bool:
    """Whether the code on `line`, marker excluded, carries `value`.

    Matching against the line without its comment is deliberate: a comment
    naming the ARG must not be able to satisfy the check on its own.
    """
    code = line.split("#", 1)[0]
    if value.startswith("docker.io/library/golang:"):
        return normalise_go_image(code) == value
    return value in code


def pinned_lines(paths: tuple[Path, ...]) -> list[tuple[Path, int, str]]:
    """Return every line under `paths` carrying a pin marker."""
    hits: list[tuple[Path, int, str]] = []
    for root in paths:
        files = sorted(root.rglob("*")) if root.is_dir() else [root]
        for path in files:
            if not path.is_file():
                continue
            try:
                text = path.read_text(encoding="utf-8")
            except UnicodeDecodeError:
                continue
            for number, line in enumerate(text.splitlines(), start=1):
                if PIN_MARKER.search(line):
                    hits.append((path, number, line))
    return hits


def main() -> int:
    """Check both directions and report."""
    report = Report("check-tool-versions")
    expected = declared_versions(CONTAINERFILE.read_text(encoding="utf-8"))

    print(f"== workflow pins vs {CONTAINERFILE} ==")
    seen: set[str] = set()
    for path, number, line in pinned_lines(PINNED_FILES):
        name = marked_pin(line)
        if name is None:  # pragma: no cover - pinned_lines already filtered
            continue
        if name not in expected:
            report.fail(f"{path}:{number} names {name}, which {CONTAINERFILE} lacks")
        elif satisfies(line, expected[name]):
            report.ok(f"{name:<28} {expected[name]}")
            seen.add(name)
        else:
            report.fail(
                f"{path}:{number} expects {name}={expected[name]}, "
                f"line reads: {line.split('#', 1)[0].strip()}"
            )

    print("\n== every CI tool is pinned somewhere ==")
    for name in REQUIRED:
        if name not in expected:
            report.fail(f"{name} is required but {CONTAINERFILE} does not define it")
        elif name not in seen:
            report.fail(f"{name} is not referenced anywhere")
        else:
            report.ok(name)

    return report.summary("CI and the dev image agree on every pinned tool")


if __name__ == "__main__":
    raise SystemExit(main())
