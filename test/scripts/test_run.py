"""The deadline on every command the fixture drivers issue.

Written after a stalled image pull consumed an entire sixty-minute end-to-end
job and left nothing behind: the step was killed at the job ceiling, GitHub
reported "cancelled" rather than a failure, and no log for the step was ever
written. The re-run took two minutes. The hang was identified by inference,
which is the part worth preventing.
"""

from __future__ import annotations

import subprocess

import pytest
from scripts.automation.run import COMMAND, LOCAL, PULL, DeadlineError, run


def test_a_command_that_finishes_returns_its_result():
    """The ordinary path has to keep working, deadline or not."""
    completed = run(
        ["true"], what="a command that exits cleanly", timeout=LOCAL, check=True
    )
    assert completed.returncode == 0


def test_output_is_captured_when_asked_for():
    """Several callers read stdout; the helper must not swallow it."""
    completed = run(
        ["echo", "hello"],
        what="printing",
        timeout=LOCAL,
        capture_output=True,
        text=True,
    )
    assert completed.stdout.strip() == "hello"


def test_a_non_zero_status_raises_when_checked():
    """`check=True` is the default, and several callers rely on it."""
    with pytest.raises(subprocess.CalledProcessError):
        run(["false"], what="a command that fails", timeout=LOCAL)


def test_a_non_zero_status_is_returned_when_not_checked():
    """The container probe reads the status rather than catching an exception."""
    completed = run(["false"], what="a command that fails", timeout=LOCAL, check=False)
    assert completed.returncode != 0


def test_a_command_past_its_deadline_is_killed():
    """The whole point: an hour of nothing becomes a second of something."""
    with pytest.raises(DeadlineError):
        run(["sleep", "5"], what="sleeping", timeout=0.3)


def test_the_message_names_what_it_was_doing_and_the_command(capsys):
    """A bare "cancelled" told nobody anything. This has to say more."""
    with pytest.raises(DeadlineError):
        run(["sleep", "5"], what="pulling the fixture's images", timeout=0.3)
    reported = capsys.readouterr().err
    assert "pulling the fixture's images" in reported
    assert "sleep 5" in reported
    assert "stalled image pull" in reported


def test_the_deadline_is_in_the_exception_too():
    """A caller that catches it should not have to scrape stderr."""
    with pytest.raises(DeadlineError, match="did not finish in 1s"):
        run(["sleep", "5"], what="sleeping", timeout=1)


def test_a_pull_is_allowed_longer_than_a_local_call():
    """A cold runner pulling seven images is slow and legitimate."""
    assert PULL > LOCAL


def test_a_command_deadline_is_under_pytests_own_ceiling():
    """pytest-timeout stops the test at 900s.

    When both would fire, the failure that names the command is the useful one,
    so this has to arrive first.
    """
    assert COMMAND < 900
