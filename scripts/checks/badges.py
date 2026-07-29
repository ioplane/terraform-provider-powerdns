"""Verify every badge URL resolves and every mermaid block is well-formed.

A badge pointing at a non-existent endpoint renders as a broken image on the
front page of the project. Nobody catches that in review; everybody sees it
afterwards. Same argument as scripts/checks/pins.py: a rule nobody checks is a
preference.

Run as: python -m scripts.checks.badges
"""

from __future__ import annotations

import re
import sys
import time
import urllib.error
import urllib.request
from collections.abc import Callable
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass
from pathlib import Path

from scripts.checks.report import Report

HOST = "https://shieldcn.dev"
BADGE_URL = re.compile(r'https://shieldcn\.dev/[^")\s]+')
FENCE = re.compile(r"^```")
MERMAID_OPEN = re.compile(r"^```mermaid")
# A node label containing a colon, bracket or slash must be quoted, or mermaid
# terminates the node at the punctuation and renders something else.
UNQUOTED_LABEL = re.compile(r'^\s*[A-Za-z0-9_]+\[[^"]*[:/()][^"]*\]')

PLAN = Path("docs/plan.md")
TASK_BADGE = re.compile(r"badge/tasks_done-(\d+)-")
TASK_ROW = re.compile(r"^\| S\d+-\d+ \|.*`\[x\]`", re.MULTILINE)
PHASE_BADGE = re.compile(r"badge/phases_closed-(\d+)-")
PHASE_HEADING = re.compile(r"^## Phase \d+ .*`\[x\]`", re.MULTILINE)

HTTP_OK = 200
ATTEMPTS = 3
TIMEOUT = 15.0

Fetch = Callable[[str], "tuple[int, str] | None"]


@dataclass(frozen=True)
class Verdict:
    """What one badge URL turned out to be."""

    url: str
    state: str  # "ok", "bad" or "unreachable"
    detail: str = ""

    @property
    def path(self) -> str:
        """The URL without the host, which is the same on every line."""
        return self.url.removeprefix(HOST)


def outside_fences(markdown: str) -> str:
    """Return `markdown` with fenced blocks removed.

    Fenced blocks hold templates and examples, and the badge host renders any
    static label, so a placeholder would pass and prove nothing.
    """
    kept: list[str] = []
    inside = False
    for line in markdown.splitlines():
        if FENCE.match(line):
            inside = not inside
            continue
        if not inside:
            kept.append(line)
    return "\n".join(kept)


def badge_urls(markdown: str) -> set[str]:
    """Return the badge URLs `markdown` references outside fenced blocks."""
    return {
        url.replace("&amp;", "&") for url in BADGE_URL.findall(outside_fences(markdown))
    }


def json_probe(url: str) -> str | None:
    """Return the .json form of a dynamic badge URL, or None if it is static.

    A 200 is not enough for these. The host renders an error card rather than
    failing, so `github/ci` for a repository with no GitHub Actions CI answers
    200 and displays "not found".

    The query string is kept: `github/ci` needs ?workflow=… to name which
    workflow it reports, and dropping it asks a different question than the
    badge does — one that answers "not found" for a repository whose badge
    renders correctly.
    """
    if "/github/" not in url:
        return None
    base, _, query = url.partition("?")
    return base.removesuffix(".svg") + ".json" + (f"?{query}" if query else "")


def fetch(url: str) -> tuple[int, str] | None:
    """GET `url`, returning its status and body, or None if it did not answer."""
    if not url.startswith("https://"):
        msg = f"refusing to open a non-HTTPS URL: {url}"
        raise ValueError(msg)
    try:
        with urllib.request.urlopen(url, timeout=TIMEOUT) as response:  # noqa: S310  # nosemgrep
            return response.status, response.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as error:
        return error.code, ""
    except (OSError, ValueError):
        return None


def check_badge(
    url: str,
    get: Fetch = fetch,
    sleep: Callable[[float], None] = time.sleep,
) -> Verdict:
    """Decide what one badge URL is, retrying a host that does not answer.

    Three attempts. A single timeout against a third-party CDN is not a broken
    badge, and CI reported one as a failure the first time this ran.
    """
    answer = None
    for attempt in range(1, ATTEMPTS + 1):
        answer = get(url)
        if answer is not None:
            break
        if attempt < ATTEMPTS:
            sleep(attempt)

    if answer is None:
        # Unreachable is not the same claim as wrong, and this check exists to
        # catch a badge URL that is wrong. Unlike pins.py — where an
        # unverifiable pin is a supply-chain claim the repository must not make
        # — an unreachable badge host is somebody else's outage. Reported
        # loudly, counted, and not fatal.
        return Verdict(url, "unreachable", "the host did not answer in 3 attempts")

    status, _ = answer
    if status != HTTP_OK:
        return Verdict(url, "bad", f"HTTP {status}")

    probe = json_probe(url)
    if probe is not None:
        resolved = get(probe)
        if resolved is not None and '"error"' in resolved[1]:
            return Verdict(url, "bad", "endpoint resolves to an error")

    return Verdict(url, "ok")


def mermaid_problem(markdown: str) -> str | None:
    """Return why a file's mermaid blocks are malformed, or None.

    Structural checks only: a full parse would need a browser. These catch the
    mistakes that actually happen — unbalanced fences, and labels whose
    punctuation ends the node early.
    """
    lines = markdown.splitlines()
    if not any(MERMAID_OPEN.match(line) for line in lines):
        return None
    if sum(1 for line in lines if FENCE.match(line)) % 2 != 0:
        return "unbalanced code fences"

    inside = False
    for line in lines:
        if MERMAID_OPEN.match(line):
            inside = True
            continue
        if FENCE.match(line):
            inside = False
            continue
        if inside and UNQUOTED_LABEL.match(line):
            return "unquoted node label containing punctuation"
    return None


def mermaid_blocks(markdown: str) -> int:
    """Count the mermaid blocks in a file."""
    return sum(1 for line in markdown.splitlines() if MERMAID_OPEN.match(line))


def counter_verdicts(plan: str) -> list[tuple[bool, str]]:
    """Check the plan's two counters against what the document actually says.

    The audit of 2026-07-29 found the task badge reading 67 against 62 tasks
    marked done, because it was hand-incremented every sprint. Recomputing it
    once fixes the number; only a check stops it happening again. The phase
    counter then drifted beside it, which is what happens when a check is
    written for the instance rather than for the class.
    """
    results: list[tuple[bool, str]] = []
    for name, badge, item, noun in (
        ("tasks_done", TASK_BADGE, TASK_ROW, "the tables show"),
        ("phases_closed", PHASE_BADGE, PHASE_HEADING, "the headings show"),
    ):
        claimed = badge.search(plan)
        actual = len(item.findall(plan))
        if claimed is None:
            results.append((False, f"docs/plan.md has no {name} badge"))
        elif int(claimed.group(1)) != actual:
            results.append((False, f"badge claims {claimed.group(1)}, {noun} {actual}"))
        else:
            results.append((True, f"{actual} {name}, badge agrees"))
    return results


def markdown_files(root: Path = Path()) -> list[Path]:
    """Every markdown file in the tree, excluding what is not ours."""
    skip = {".git", "node_modules", ".venv", ".worktrees"}
    return sorted(
        path
        for path in root.rglob("*.md")
        if not skip.intersection(path.parts) and path.is_file()
    )


def main() -> int:
    """Check the badges, the mermaid blocks and the plan's counters."""
    report = Report("check-badges")
    files = markdown_files()

    print("== badges ==")
    urls = sorted(
        {url for file in files for url in badge_urls(file.read_text("utf-8"))}
    )
    resolved = unreachable = 0
    if not urls:
        print("no badges found")
    else:
        # Concurrent because there are a hundred of them against one host and
        # they do not depend on each other; printed in URL order regardless, so
        # a diff between two runs is a real change.
        with ThreadPoolExecutor(max_workers=8) as pool:
            verdicts = list(pool.map(check_badge, urls))
        for verdict in verdicts:
            if verdict.state == "ok":
                report.ok(verdict.path)
                resolved += 1
            elif verdict.state == "unreachable":
                unreachable += 1
                print(
                    f"UNREACHABLE {verdict.path}  ({verdict.detail})",
                    file=sys.stderr,
                )
            else:
                report.fail(f"{verdict.path}  ({verdict.detail})")

    print("\n== mermaid blocks ==")
    verified = 0
    for file in files:
        text = file.read_text("utf-8")
        blocks = mermaid_blocks(text)
        if blocks == 0:
            continue
        problem = mermaid_problem(text)
        if problem:
            report.fail(f"{file}: {problem}")
        else:
            report.ok(f"{file} ({blocks} block(s))")
            verified += 1

    print("\n== the plan's counters ==")
    if PLAN.is_file():
        for held, message in counter_verdicts(PLAN.read_text("utf-8")):
            (report.ok if held else report.fail)(message)

    return report.summary(
        f"{resolved} badges verified, {unreachable} unreachable, "
        f"{verified} files with mermaid verified, counters agree"
    )


if __name__ == "__main__":
    raise SystemExit(main())
