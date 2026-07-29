"""Everything about a release that can be checked before anything is built.

The Terraform Registry treats a published version as immutable. There is no
amending a release: a wrong one is corrected by publishing another version and
leaving the wrong one visible forever. So the expensive half of the release
path — building, signing, uploading — must not start until the cheap half has
agreed with itself.

Run as: python -m scripts.checks.release [version]
With no argument the version is read from VERSION.
"""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

from scripts.checks.report import Report

MANIFEST = Path("terraform-registry-manifest.json")
CHANGELOG = Path("CHANGELOG.md")
VERSION_FILE = Path("VERSION")
MAIN = Path("main.go")

SEMVER = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$")
RELEASED_HEADING = re.compile(r"^## \[([0-9]+\.[0-9]+\.[0-9]+)\]", re.MULTILINE)
PROTOCOL = re.compile(r'"([0-9]+\.[0-9]+)"')
LINK_DEFINITION = re.compile(r"^\[[^\]]+\]:\s*http")
PROTOCOL_FIVE = re.compile(r"ProtocolVersion:\s*5")


def git(*args: str) -> subprocess.CompletedProcess[str]:
    """Run git and return the completed process, never raising on exit status."""
    return subprocess.run(["git", *args], capture_output=True, text=True, check=False)


def tag_exists(tag: str) -> bool:
    """Whether `tag` resolves in this repository."""
    return git("rev-parse", "-q", "--verify", f"refs/tags/{tag}").returncode == 0


def changelog_section(changelog: str, version: str) -> list[str]:
    """Return the lines of one changelog section, excluding its heading.

    Stops at the next section heading and at the link definitions, which are a
    trailing block belonging to the document rather than to any release.
    """
    lines: list[str] = []
    capturing = False
    for line in changelog.splitlines():
        if line.startswith("## ["):
            if capturing:
                break
            capturing = line.startswith(f"## [{version}]")
            continue
        if capturing and LINK_DEFINITION.match(line):
            break
        if capturing:
            lines.append(line)
    return lines


def added_since(before: str, now: str, version: str) -> int:
    """Count lines a released section has gained since it was tagged.

    A released section describes what shipped. Writing into it after the tag
    claims the change went out when it did not, and the next release cut —
    which reads only [Unreleased] — silently drops it. Twelve entries had
    accumulated in [0.1.1] before a reviewer noticed.

    Additions are the fault; removals are how one is corrected, so only lines
    present now and absent at the tag count. A section that has had entries
    taken back out of it passes, which is the state this check was written in.
    """
    was = {line for line in changelog_section(before, version) if line.strip()}
    became = {line for line in changelog_section(now, version) if line.strip()}
    return len(became - was)


def served_protocol(main_go: str) -> str:
    """Return the protocol version the provider serves.

    terraform-plugin-framework serves protocol 6 unless ServeOpts asks for 5.
    """
    return "5.0" if PROTOCOL_FIVE.search(main_go) else "6.0"


def declared_protocols(manifest: str) -> str:
    """Return the protocol versions the registry manifest advertises."""
    return " ".join(PROTOCOL.findall(manifest))


def check_version(report: Report, version: str, file_version: str) -> None:
    """The release version and VERSION must be the same semantic version."""
    print("== version ==")
    if version == file_version:
        report.ok(f"VERSION and the release agree on {version}")
    else:
        # docs/standards/versioning.md calls VERSION the source of truth and
        # says the tag is cut from it. A tag that disagrees means one of the two
        # was edited and the other forgotten, and the Registry believes the tag.
        report.fail(f"releasing {version} but VERSION says {file_version}")
    if not SEMVER.match(version):
        report.fail(f"{version} is not a semantic version")


def check_tag(report: Report, version: str) -> None:
    """Re-tagging is the failure mode this guards.

    The Registry has already read the old tag; moving it changes what a
    signature covers without changing what anyone downloaded.
    """
    print("\n== the tag is new ==")
    tag = f"v{version}"
    if not tag_exists(tag):
        report.ok(f"{tag} does not exist yet")
        return
    head = git("rev-parse", "HEAD").stdout.strip()
    tagged = git("rev-parse", f"{tag}^{{commit}}").stdout.strip()
    if head == tagged:
        report.ok(f"{tag} already points at HEAD")
    else:
        report.fail(f"{tag} exists and points at {tagged[:12]}, not at HEAD")


def check_changelog(report: Report, changelog: str, version: str) -> None:
    """The release notes are extracted from this section.

    Without it the release ships either nothing or, worse, whatever
    [Unreleased] happened to hold.
    """
    print("\n== the changelog has a section for it ==")
    if f"## [{version}]" not in changelog:
        report.fail(f"CHANGELOG.md has no '## [{version}]' section")
        return
    body = [line for line in changelog_section(changelog, version) if line.strip()]
    if not body:
        report.fail(f"the '## [{version}]' section is empty")
    else:
        report.ok(f"changelog section present, {len(body)} non-blank lines")


def check_released_sections_are_closed(report: Report, changelog: str) -> None:
    """No released section may gain a line after its tag."""
    print("\n== nothing has been added to a released section ==")
    for released in RELEASED_HEADING.findall(changelog):
        tag = f"v{released}"
        if not tag_exists(tag):
            print(f"skip      [{released}] has no tag yet")
            continue
        at_tag = git("show", f"{tag}:CHANGELOG.md").stdout
        added = added_since(at_tag, changelog, released)
        if added == 0:
            report.ok(f"[{released}] has gained nothing since {tag}")
        else:
            report.fail(
                f"[{released}] has {added} line(s) added since {tag}; "
                "new entries belong under [Unreleased]"
            )


def check_manifest(report: Report) -> None:
    """The Registry advertises what the manifest says.

    If it says 5.0 and the binary speaks 6, every consumer's `terraform init`
    negotiates a protocol the plugin does not implement — and the version
    cannot be withdrawn.
    """
    print("\n== the manifest matches the protocol the provider serves ==")
    if not MANIFEST.is_file():
        report.fail(f"{MANIFEST} does not exist")
        return
    declared = declared_protocols(MANIFEST.read_text(encoding="utf-8"))
    served = served_protocol(MAIN.read_text(encoding="utf-8"))
    if declared == served:
        report.ok(f"manifest declares {declared}, and the provider serves it")
    else:
        report.fail(f"manifest declares '{declared}' but the provider serves {served}")


def check_clean_tree(report: Report) -> None:
    """Goreleaser stamps the build with `git describe`.

    A dirty tree carries -dirty into the stamp, and the archive would not
    correspond to any commit anyone can check out.
    """
    print("\n== the working tree is the thing being released ==")
    dirty = git("status", "--porcelain").stdout.strip()
    if dirty:
        report.fail("the working tree has uncommitted changes")
        for line in dirty.splitlines()[:10]:
            print(f"      {line}", file=sys.stderr)
    else:
        report.ok("clean")


def main(argv: list[str]) -> int:
    """Run every pre-build release assertion."""
    if not VERSION_FILE.is_file():
        print(
            "VERSION does not exist; run this from the repository root",
            file=sys.stderr,
        )
        return 2

    file_version = VERSION_FILE.read_text(encoding="utf-8").strip()
    version = (argv[0] if argv else file_version).removeprefix("v")
    changelog = CHANGELOG.read_text(encoding="utf-8")

    report = Report("check-release")
    check_version(report, version, file_version)
    check_tag(report, version)
    check_changelog(report, changelog, version)
    check_released_sections_are_closed(report, changelog)
    check_manifest(report)
    check_clean_tree(report)
    print()
    return report.summary(f"v{version} is releasable")


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
