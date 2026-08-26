"""Verify everything referenced by hash resolves, and that nothing floats.

Two classes are checked:

  * container images — must carry @sha256:<64 hex>, and it must exist
  * GitHub Actions   — must carry @<40 hex>, and it must resolve

A fabricated digest or SHA is syntactically valid: it parses, reads as correct,
survives review, and fails only when something tries to pull it. That is why
this is a gate and not a convention.

Run as: python -m scripts.checks.pins
"""

from __future__ import annotations

import json
import re
import shutil
import subprocess
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from collections.abc import Callable

from scripts.checks.report import Report

SEARCH_ROOTS = (Path("deployments"), Path(".github"))
CONTAINERFILE_DIR = Path("deployments/containers")

IMAGE_REF = re.compile(
    r"(?:docker\.io|ghcr\.io|quay\.io|codeberg\.org)"
    r"/[a-z0-9./_-]+:[a-zA-Z0-9._-]+(?:@sha256:[0-9a-f]{64})?"
)
ACTION_REF = re.compile(r"uses:\s*([A-Za-z0-9._-]+/[A-Za-z0-9._/-]+@\S+)")
DIGEST = re.compile(r"@sha256:[0-9a-f]{64}$")
COMMIT_SHA = re.compile(r"^[0-9a-f]{40}$")
TAG_BEFORE_DIGEST = re.compile(r":[^:@/]+@")

# Every manifest media type a registry might answer with. A digest is only
# retrievable if the client asks for the type it was published as, and asking
# for too few is how the same pin reads as present on one machine and missing
# on another.
MANIFEST_ACCEPT = (
    "application/vnd.oci.image.index.v1+json, "
    "application/vnd.docker.distribution.manifest.list.v2+json, "
    "application/vnd.oci.image.manifest.v1+json, "
    "application/vnd.docker.distribution.manifest.v2+json"
)

TIMEOUT = 20.0
UNAUTHORIZED = 401
FOUND = 200
NOT_FOUND = 404
ATTEMPTS = 3


def references(pattern: re.Pattern[str], roots: tuple[Path, ...]) -> list[str]:
    """Return every distinct match of `pattern` in the files under `roots`."""
    found: set[str] = set()
    for root in roots:
        if not root.exists():
            continue
        for path in sorted(root.rglob("*")):
            if not path.is_file():
                continue
            try:
                text = path.read_text(encoding="utf-8")
            except (UnicodeDecodeError, OSError):
                continue
            for match in pattern.finditer(text):
                found.add(match.group(1) if match.groups() else match.group(0))
    return sorted(found)


def _head(url: str, headers: dict[str, str]) -> tuple[int, dict[str, str]] | None:
    """Issue a HEAD request, returning its status and headers."""
    request = urllib.request.Request(url, method="HEAD", headers=headers)  # noqa: S310
    try:
        with urllib.request.urlopen(request, timeout=TIMEOUT) as response:  # noqa: S310  # nosemgrep
            return response.status, dict(response.headers)
    except urllib.error.HTTPError as error:
        return error.code, dict(error.headers)
    except (OSError, ValueError):
        return None


def _bearer(challenge: str, repo: str) -> str | None:
    """Fetch an anonymous pull token from the realm the registry names."""
    realm = re.search(r'realm="([^"]+)"', challenge)
    service = re.search(r'service="([^"]+)"', challenge)
    if realm is None:
        return None
    url = f"{realm.group(1)}?service={service.group(1) if service else ''}"
    url += f"&scope=repository:{repo}:pull"
    try:
        with urllib.request.urlopen(url, timeout=TIMEOUT) as response:  # noqa: S310  # nosemgrep
            payload = json.load(response)
    except (OSError, ValueError):
        return None
    token = payload.get("token") or payload.get("access_token")
    return str(token) if token else None


def manifest_status(ref: str) -> int:
    """Return the registry's HTTP status for a digest reference, or 0.

    Asked over HTTP rather than through a container client. skopeo consults
    local storage, mirrors and caches, so it happily resolved a digest that
    quay.io answers 404 for — the pin looked verified locally and failed in CI,
    and the checker sided with the machine that was wrong. The registry is the
    authority on whether a digest exists; ask it directly.
    """
    registry, _, rest = ref.partition("/")
    repo, _, digest = rest.partition("@")
    host = "registry-1.docker.io" if registry == "docker.io" else registry
    url = f"https://{host}/v2/{repo}/manifests/{digest}"

    answer = _head(url, {"Accept": MANIFEST_ACCEPT})
    if answer is None:
        return 0
    status, headers = answer

    # Anonymous pull: the registry says who to ask for a token and for what.
    if status == UNAUTHORIZED:
        challenge = headers.get("WWW-Authenticate") or headers.get("www-authenticate")
        if not challenge:
            return status
        token = _bearer(challenge, repo)
        if token is None:
            return status
        retry = _head(
            url, {"Accept": MANIFEST_ACCEPT, "Authorization": f"Bearer {token}"}
        )
        return retry[0] if retry else 0
    return status


def resolvable(ref: str) -> str:
    """Drop the tag from a reference carrying both a tag and a digest.

    A reference may carry both for human legibility. The digest is the part
    that pins, so the tag is dropped before asking.
    """
    return TAG_BEFORE_DIGEST.sub("@", ref)


def action_repo(action: str) -> str:
    """Return the repository an action reference belongs to.

    An action may live in a subdirectory — github/codeql-action/init,
    anchore/sbom-action/download-syft. The SHA belongs to the repository, so
    the first two segments are what the API is asked about; matching
    owner/repo greedily against the whole reference instead reads
    `codeql-action/init` as a repository and reports a valid pin as missing.
    """
    return "/".join(action.split("/")[:2])


def commit_exists(
    repo: str, sha: str, sleep: Callable[[float], None] = time.sleep
) -> tuple[bool, str]:
    """Ask GitHub whether a commit exists, distinguishing absent from unknown.

    "gh failed" and "GitHub says this commit does not exist" are different
    answers, and treating them alike is how a rate-limited run reports two
    dozen valid pins as fabricated. The first run of this check in CI did
    exactly that for one action out of twenty-four.
    """
    error = ""
    for attempt in range(1, ATTEMPTS + 1):
        result = subprocess.run(
            ["gh", "api", f"repos/{repo}/commits/{sha}", "--jq", ".sha"],
            capture_output=True,
            text=True,
            check=False,
        )
        if result.returncode == 0:
            return True, ""
        error = result.stderr.strip()
        if "Not Found" in error or "No commit found" in error:
            return False, error
        if attempt < ATTEMPTS:
            sleep(attempt)
    return False, error


def gh_available() -> bool:
    """Whether gh is installed and authenticated."""
    if shutil.which("gh") is None:
        return False
    return (
        subprocess.run(
            ["gh", "auth", "status"],
            capture_output=True,
            check=False,
        ).returncode
        == 0
    )


def unpinned_from(directory: Path) -> list[str]:
    """Return every FROM line in `directory` that carries neither digest nor ARG."""
    lines: list[str] = []
    if not directory.exists():
        return lines
    for path in sorted(directory.rglob("*")):
        if not path.is_file():
            continue
        try:
            text = path.read_text(encoding="utf-8")
        except (UnicodeDecodeError, OSError):
            continue
        for number, line in enumerate(text.splitlines(), start=1):
            floats = (
                line.startswith("FROM ")
                and re.fullmatch(
                    r"FROM scratch(?: AS [A-Za-z0-9._-]+)?", line, re.IGNORECASE
                )
                is None
                and not line.startswith("FROM $")
                and "@sha256:" not in line
                and "${" not in line
            )
            if floats:
                lines.append(f"{path}:{number}: {line}")
    return lines


def check_images(report: Report) -> int:
    """Check every image reference resolves; return how many did."""
    verified = 0
    for ref in references(IMAGE_REF, SEARCH_ROOTS):
        if not DIGEST.search(ref):
            report.fail(f"FLOATING  {ref}")
            continue
        status = manifest_status(resolvable(ref))
        if status == FOUND:
            report.ok(ref.split("@")[0])
            verified += 1
        elif status == NOT_FOUND:
            report.fail(f"NOT FOUND {ref}")
        else:
            report.fail(f"UNVERIFIED {ref} — registry answered {status or '000'}")
    return verified


def check_actions(report: Report) -> int:
    """Check every action pin resolves; return how many did."""
    verified = 0
    for ref in references(ACTION_REF, (Path(".github"),)):
        action, _, sha = ref.rpartition("@")
        if not COMMIT_SHA.match(sha):
            report.fail(f"FLOATING  {ref} — pin the commit SHA")
            continue
        exists, error = commit_exists(action_repo(action), sha)
        if exists:
            report.ok(f"{action:<34} {sha[:12]}")
            verified += 1
        elif "Not Found" in error or "No commit found" in error:
            report.fail(f"NOT FOUND {ref}")
        else:
            report.fail(f"UNVERIFIED {ref} — {error.splitlines()[0] if error else ''}")
    return verified


def main() -> int:
    """Check images, Containerfiles and action pins."""
    report = Report("check-pins")

    print("== container images ==")
    verified = check_images(report)

    print("\n== unpinned FROM in Containerfiles ==")
    floating = unpinned_from(CONTAINERFILE_DIR)
    for line in floating:
        report.fail(f"{line} does not carry a digest")
    if not floating:
        report.ok("every FROM is digest-pinned or uses a pinned ARG")

    print("\n== GitHub Actions ==")
    if not gh_available():
        print("skip      gh unavailable or unauthenticated; CI runs the same check")
        return report.summary(f"{verified} images verified; actions deferred to CI")

    verified += check_actions(report)
    return report.summary(f"{verified} references verified")


if __name__ == "__main__":
    raise SystemExit(main())
