"""Assert a built release is shaped the way the Registry ingests.

This exists because v0.1.0 was built, signed, published and then refused. The
Registry parses SHA256SUMS and treats every line as a file belonging to the
version; adding SBOMs to the release put thirteen lines in there that it will
not accept, and it rejected the whole submission with "missing files in request
body". Nothing in the release path noticed, because every check asked whether
the artefacts were correct and none asked whether they were the ones the
Registry expects.

Run as: python -m scripts.checks.release_artifacts [dist-dir]
"""

from __future__ import annotations

import hashlib
import sys
from pathlib import Path

from scripts.checks.paths import checked_path
from scripts.checks.report import Report

MANIFEST = Path("terraform-registry-manifest.json")
READ_SIZE = 1024 * 1024


def digest_of(path: Path) -> str:
    """Return the SHA-256 of a file, read in chunks."""
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(READ_SIZE):
            digest.update(chunk)
    return digest.hexdigest()


def parse_sums(text: str) -> tuple[list[tuple[str, str]], list[str]]:
    """Return the (digest, filename) pairs, and the lines that are neither.

    The malformed lines are returned rather than dropped. The Registry reads
    every line of this file, so a line this cannot parse is a line that will
    reach it unexamined — and silently discarding it would make the check pass
    on exactly the file it exists to reject.
    """
    pairs: list[tuple[str, str]] = []
    malformed: list[str] = []
    for line in text.splitlines():
        if not line.strip():
            continue
        parts = line.split()
        if len(parts) == 2:  # noqa: PLR2004 - a checksum line is a digest and a name
            pairs.append((parts[0], parts[1].lstrip("*")))
        else:
            malformed.append(line)
    return pairs, malformed


def check_listing(
    report: Report, sums: list[tuple[str, str]], malformed: list[str]
) -> None:
    """Every line must be an archive or the manifest.

    Exactly two shapes. Anything else — an SBOM, a signature, a checksum of a
    checksum — is a line the Registry cannot resolve to a file it accepts, and
    it refuses the whole submission over it.
    """
    print("== SHA256SUMS lists only what the Registry ingests ==")
    for line in malformed:
        report.fail(f"{line!r} is not a checksum line")
    for _, name in sums:
        if name.endswith("_manifest.json"):
            report.ok(f"manifest    {name}")
        elif name.endswith(".zip"):
            report.ok(f"archive     {name}")
        else:
            report.fail(f"{name} is neither an archive nor the manifest")


def check_archives(report: Report, sums: list[tuple[str, str]], dist: Path) -> None:
    """Every archive on disk must match its recorded digest.

    Only the archives live in dist. The manifest is checksummed under a renamed
    copy that exists in the uploaded release and never on disk, so it is checked
    against its source instead.
    """
    print("\n== every archive matches its recorded digest ==")
    archives = [(digest, name) for digest, name in sums if name.endswith(".zip")]
    if not archives:
        report.fail("SHA256SUMS records no archives")
        return
    wrong = 0
    for recorded, name in archives:
        path = dist / name
        if not path.is_file():
            report.fail(f"{name} is listed in SHA256SUMS and missing from {dist}")
            wrong += 1
        elif digest_of(path) != recorded:
            report.fail(f"{name} does not match its recorded digest")
            wrong += 1
    if wrong == 0:
        report.ok(f"all {len(archives)} archives verify against their digests")


def check_manifest(report: Report, sums: list[tuple[str, str]]) -> None:
    """The recorded manifest digest must be the repository's own manifest."""
    print("\n== the manifest is the one in the repository ==")
    recorded = [d for d, name in sums if name.endswith("_manifest.json")]
    if not recorded:
        report.fail(
            "SHA256SUMS records no manifest — "
            "the Registry needs one to learn the protocol"
        )
        return
    if not MANIFEST.is_file():
        report.fail(f"{MANIFEST} is missing from the repository")
        return
    if recorded[0] == digest_of(MANIFEST):
        report.ok(f"manifest      matches {MANIFEST}")
    else:
        report.fail("the recorded manifest digest is not the repository's manifest")
    if "protocol_versions" not in MANIFEST.read_text(encoding="utf-8"):
        report.fail("the manifest declares no protocol_versions")


def main(argv: list[str], bases: tuple[Path, ...] | None = None) -> int:
    """Check the built release in `dist` (or the directory named)."""
    try:
        dist = checked_path(argv[0] if argv else "dist", bases)
    except (ValueError, OSError) as error:
        print(f"{error}", file=sys.stderr)
        return 2
    found = sorted(dist.glob("*_SHA256SUMS")) if dist.is_dir() else []
    if not found:
        print(
            f"no *_SHA256SUMS in {dist}; build first (task release:dryrun)",
            file=sys.stderr,
        )
        return 2

    sums, malformed = parse_sums(found[0].read_text(encoding="utf-8"))
    report = Report("check-release-artifacts")
    check_listing(report, sums, malformed)
    check_archives(report, sums, dist)
    check_manifest(report, sums)
    print()
    return report.summary("the release is shaped for ingestion")


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
