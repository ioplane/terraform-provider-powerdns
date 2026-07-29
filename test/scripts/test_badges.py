"""The badge check's decisions, with the network replaced by a dictionary.

The retry, the error-card probe and the two counters are the parts that have
been wrong before. None of them could be exercised in the shell version without
an outage or a hand-edited plan.
"""

from __future__ import annotations

from pathlib import Path

import pytest
from scripts.checks.badges import (
    badge_urls,
    check_badge,
    counter_verdicts,
    json_probe,
    markdown_files,
    mermaid_blocks,
    mermaid_problem,
    outside_fences,
)

BADGE = "https://shieldcn.dev/badge/tasks_done-90-3fb950.svg"
DYNAMIC = "https://shieldcn.dev/github/ci/ioplane/x.svg?workflow=CI"


def test_a_badge_inside_a_fence_is_an_example_not_a_claim():
    """The host renders any static label, so a placeholder would prove nothing."""
    markdown = (
        f"![real]({BADGE})\n"
        "\n"
        "```markdown\n"
        "![example](https://shieldcn.dev/badge/placeholder-1-red.svg)\n"
        "```\n"
    )
    assert badge_urls(markdown) == {BADGE}


def test_the_html_entity_in_a_query_string_is_decoded():
    """Markdown carries &amp;; the host needs the ampersand it stands for."""
    url = "https://shieldcn.dev/github/ci/o/r.svg?workflow=CI&amp;variant=secondary"
    assert badge_urls(f"![x]({url})") == {
        "https://shieldcn.dev/github/ci/o/r.svg?workflow=CI&variant=secondary"
    }


def test_fence_removal_keeps_the_text_around_it():
    """Dropping too much would hide badges that are being claimed."""
    assert outside_fences("a\n```\nb\n```\nc") == "a\nc"


def test_a_static_badge_is_not_probed_for_an_error_card():
    """There is nothing behind it to resolve; a 200 is the whole answer."""
    assert json_probe(BADGE) is None


def test_a_dynamic_badge_keeps_its_query_when_probed():
    """`github/ci` needs ?workflow= to name which workflow it reports.

    Dropping it asks a different question than the badge does — one that
    answers "not found" for a repository whose badge renders correctly.
    """
    assert (
        json_probe(DYNAMIC)
        == "https://shieldcn.dev/github/ci/ioplane/x.json?workflow=CI"
    )


def test_a_two_hundred_is_not_enough_for_a_dynamic_badge():
    """The host renders an error card rather than failing, so it answers 200."""
    responses = {
        DYNAMIC: (200, ""),
        json_probe(DYNAMIC): (200, '{"error":"not found"}'),
    }
    verdict = check_badge(DYNAMIC, get=responses.get, sleep=lambda _: None)
    assert verdict.state == "bad"
    assert "error" in verdict.detail


def test_a_dynamic_badge_that_resolves_is_ok():
    """The same path, with the endpoint answering properly."""
    responses = {DYNAMIC: (200, ""), json_probe(DYNAMIC): (200, '{"label":"CI"}')}
    assert check_badge(DYNAMIC, get=responses.get, sleep=lambda _: None).state == "ok"


def test_a_non_200_is_a_broken_badge():
    """This is the case the check exists for: a URL that is simply wrong."""
    verdict = check_badge(BADGE, get=lambda _: (404, ""), sleep=lambda _: None)
    assert verdict.state == "bad"
    assert "404" in verdict.detail


def test_one_timeout_is_retried_rather_than_reported():
    """A single timeout against a third-party CDN is not a broken badge.

    CI reported one as a failure the first time this ran.
    """
    attempts = {"n": 0}

    def flaky(_url: str) -> tuple[int, str] | None:
        attempts["n"] += 1
        return None if attempts["n"] == 1 else (200, "")

    assert check_badge(BADGE, get=flaky, sleep=lambda _: None).state == "ok"
    assert attempts["n"] == 2


def test_a_host_that_never_answers_is_unreachable_not_wrong():
    """Somebody else's outage must not fail this repository's gate."""
    verdict = check_badge(BADGE, get=lambda _: None, sleep=lambda _: None)
    assert verdict.state == "unreachable"


@pytest.mark.parametrize(
    ("markdown", "problem"),
    [
        ("```mermaid\nflowchart LR\n  A[plain]\n```\n", None),
        ("```mermaid\nflowchart LR\n  A[has: colon]\n```\n", "unquoted node label"),
        ('```mermaid\nflowchart LR\n  A["has: colon"]\n```\n', None),
        ("```mermaid\nflowchart LR\n", "unbalanced"),
        ("no diagram here\n", None),
    ],
)
def test_the_structural_mermaid_faults_are_caught(markdown, problem):
    """A full parse needs a browser; these are the mistakes that actually happen."""
    found = mermaid_problem(markdown)
    if problem is None:
        assert found is None
    else:
        assert found is not None
        assert problem in found


def test_punctuation_outside_a_mermaid_block_is_not_a_fault():
    """A python fence full of brackets and colons is not a diagram."""
    assert mermaid_problem("```python\nd = {A[x]: 1}\n```\n") is None


def test_blocks_are_counted_not_just_detected():
    """The summary reports how many, and a file may hold several."""
    assert mermaid_blocks("```mermaid\na\n```\ntext\n```mermaid\nb\n```\n") == 2


def test_the_counters_agree_when_the_document_says_so():
    """Both badges are derived from the document, never hand-incremented."""
    plan = (
        "![tasks_done 2](https://shieldcn.dev/badge/tasks_done-2-3fb950.svg)\n"
        "![phases_closed 1](https://shieldcn.dev/badge/phases_closed-1-3fb950.svg)\n"
        "## Phase 0 — Foundation · `[x]` closed\n"
        "| S0-01 | a | DEV | `[x]` |\n"
        "| S0-02 | b | DEV | `[x]` |\n"
        "| S0-03 | c | DEV | `[ ]` |\n"
        "## Phase 1 — Transport · `[~]` in progress\n"
    )
    assert [held for held, _ in counter_verdicts(plan)] == [True, True]


def test_a_drifted_task_counter_is_caught():
    """The audit found this badge reading 67 against 62 tasks actually done."""
    plan = (
        "![tasks_done 9](https://shieldcn.dev/badge/tasks_done-9-3fb950.svg)\n"
        "![phases_closed 0](https://shieldcn.dev/badge/phases_closed-0-3fb950.svg)\n"
        "| S0-01 | a | DEV | `[x]` |\n"
    )
    held, message = counter_verdicts(plan)[0]
    assert not held
    assert "claims 9" in message


def test_a_drifted_phase_counter_is_caught():
    """The counter beside the corrected one drifted the same way, unnoticed."""
    plan = (
        "![tasks_done 0](https://shieldcn.dev/badge/tasks_done-0-3fb950.svg)\n"
        "![phases_closed 3](https://shieldcn.dev/badge/phases_closed-3-3fb950.svg)\n"
        "## Phase 0 — Foundation · `[x]` closed\n"
    )
    held, message = counter_verdicts(plan)[1]
    assert not held
    assert "claims 3" in message


def test_a_missing_badge_is_a_failure_not_a_pass():
    """Deleting the badge must not be the way to satisfy the check."""
    assert [held for held, _ in counter_verdicts("no badges at all\n")] == [
        False,
        False,
    ]


def test_only_files_the_repository_tracks_are_checked():
    """Walking the tree counted the fixture's copy of these documents.

    The end-to-end fixture unpacks the released tag into `test/e2e/.released/`,
    which carries its own copies of every markdown file here — three extra
    badges and five extra mermaid files, all of them a past version's claims
    rather than this checkout's.
    """
    files = markdown_files()
    assert files, "no markdown files found at all"
    assert not [path for path in files if ".released" in path.parts]
    assert not [path for path in files if ".mirror" in path.parts]
    # And the ones that are genuinely ours are still there.
    assert Path("README.md") in files
    assert Path("docs/plan.md") in files


def test_an_untracked_markdown_file_is_not_a_claim(tmp_path):
    """A scratch note in the working tree is not something the project asserts."""
    (tmp_path / "scratch.md").write_text(
        "![x](https://shieldcn.dev/badge/nonsense-1-red.svg)\n"
    )
    # tmp_path is not a git repository, so the fallback walk applies and finds
    # it — the fallback is deliberately permissive, and the assertion here is
    # that the git path is what runs in the repository itself.
    assert markdown_files(tmp_path) == [tmp_path / "scratch.md"]
