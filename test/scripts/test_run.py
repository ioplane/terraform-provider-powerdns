"""The deadline on every command the fixture drivers issue.

Written after a stalled image pull consumed an entire sixty-minute end-to-end
job and left nothing behind: the step was killed at the job ceiling, GitHub
reported "cancelled" rather than a failure, and no log for the step was ever
written. The re-run took two minutes. The hang was identified by inference,
which is the part worth preventing.
"""

from __future__ import annotations

import os
import signal
import subprocess
import sys
import time
from pathlib import Path
from typing import Self

import pytest
import scripts.automation.run as run_module
from scripts.automation.run import COMMAND, LOCAL, PULL, DeadlineError, run

REPO_ROOT = Path(__file__).resolve().parents[2]


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


def test_a_grandchild_does_not_survive_the_deadline(tmp_path):
    """The hole the first version left, stated as the thing that goes wrong.

    `subprocess.run`'s timeout kills the direct child only. Every command the
    drivers issue is `bash -c "a && b"` or a podman client, so the process doing
    the work is a grandchild — and it kept running after the deadline, writing to
    the mirror or the lab while cleanup started.

    The grandchild here writes a file a second after its parent is killed. If it
    survived, the file appears.
    """
    marker = tmp_path / "survivor"
    with pytest.raises(DeadlineError):
        run(
            ["bash", "-c", f"(sleep 1; touch {marker}) & sleep 10"],
            what="a command with a background child",
            timeout=0.3,
        )
    time.sleep(2.0)
    assert not marker.exists(), "a grandchild outlived the deadline"


def test_the_parent_is_reaped_rather_than_left_as_a_zombie():
    """A killed command still has to be waited on, or the driver leaks children."""
    with pytest.raises(DeadlineError):
        run(["sleep", "10"], what="sleeping", timeout=0.3)
    # No assertion beyond returning: a missing wait() shows up as a warning from
    # Popen's destructor, which pytest turns into an error under -W error. What
    # this pins is that the failure path completes at all.


class _InterruptingProcess:
    """A process group whose leader and grandchild survive graceful stop."""

    pid = 4242
    returncode = None

    def __init__(self, interruption: BaseException) -> None:
        self.interruption = interruption
        self.communicate_calls = 0
        self.communicate_timeouts: list[float | None] = []
        self.reaped = False
        self.killed = False

    def __enter__(self) -> Self:
        return self

    def __exit__(self, *_exc_info: object) -> bool:
        return False

    def communicate(self, timeout: float | None = None) -> tuple[None, None]:
        self.communicate_calls += 1
        self.communicate_timeouts.append(timeout)
        if self.communicate_calls == 1:
            raise self.interruption
        if not self.killed:
            assert timeout is not None
            raise subprocess.TimeoutExpired(cmd=["fixture-child"], timeout=timeout)
        self.reaped = True
        self.returncode = -signal.SIGKILL
        return None, None

    def kill(self) -> None:
        self.killed = True
        self.returncode = -signal.SIGKILL

    def poll(self) -> int | None:
        return self.returncode


class _LeaderExitsBeforeGrandchild:
    """A group whose leader reaps after TERM while its grandchild ignores it."""

    pid = 4343
    returncode = None

    def __init__(self) -> None:
        self.communicate_calls = 0
        self.reaped = False

    def communicate(self, timeout: float | None = None) -> tuple[None, None]:
        self.communicate_calls += 1
        if self.communicate_calls == 1:
            assert timeout is not None
            raise subprocess.TimeoutExpired(cmd=["fixture-child"], timeout=timeout)
        self.reaped = True
        self.returncode = 0
        return None, None


class _FakeClock:
    """Deterministic monotonic clock advanced only by bounded waits."""

    def __init__(self, now: float) -> None:
        self.now = now

    def monotonic(self) -> float:
        return self.now

    def sleep(self, seconds: float) -> None:
        assert seconds >= 0
        self.now += seconds


class _DeadlineBudgetProcess:
    """A leader consuming every granted wait before becoming reapable."""

    pid = 4545
    returncode = None

    def __init__(self, clock: _FakeClock) -> None:
        self.clock = clock
        self.communicate_timeouts: list[float] = []
        self.first_failure: subprocess.TimeoutExpired | None = None
        self.reaped = False

    def communicate(self, timeout: float | None = None) -> tuple[None, None]:
        assert timeout is not None
        assert timeout > 0
        self.communicate_timeouts.append(timeout)
        self.clock.sleep(timeout)
        if len(self.communicate_timeouts) == 1:
            self.first_failure = subprocess.TimeoutExpired(
                cmd=["fixture-child"], timeout=timeout
            )
            raise self.first_failure
        if len(self.communicate_timeouts) == 2:
            raise subprocess.TimeoutExpired(cmd=["fixture-child"], timeout=timeout)
        self.reaped = True
        self.returncode = -signal.SIGKILL
        return None, None


def _install_deadline_process(monkeypatch, clock, process) -> list[signal.Signals]:
    """Install one deterministic process whose group remains live after reap."""
    signals: list[signal.Signals] = []
    monkeypatch.setattr(subprocess, "Popen", lambda *_args, **_kwargs: process)
    monkeypatch.setattr(run_module.time, "monotonic", clock.monotonic)
    monkeypatch.setattr(run_module.time, "sleep", clock.sleep)
    monkeypatch.setattr(run_module, "session_has_live_members", lambda _pgid: True)

    def record_signal(_pgid: int, sent: int) -> None:
        signals.append(signal.Signals(sent))

    monkeypatch.setattr(run_module.os, "killpg", record_signal)
    return signals


def test_cleanup_waits_share_one_derived_absolute_deadline(monkeypatch):
    """TERM, KILL, reap and group proof cannot each reset a fresh grace window."""
    clock = _FakeClock(100.0)
    process = _DeadlineBudgetProcess(clock)
    signals = _install_deadline_process(monkeypatch, clock, process)
    outer_deadline = (
        clock.now + 8.0 + run_module.TERMINATE_GRACE + run_module.KILL_GRACE
    )

    with pytest.raises(RuntimeError) as caught:
        run(["fixture-child"], what="worst-case child", timeout=8.0)

    assert clock.now <= outer_deadline
    assert process.communicate_timeouts == [8.0, 1.0, 1.0]
    assert signals == [signal.SIGTERM, signal.SIGKILL]
    assert process.reaped
    assert caught.value.__cause__ is process.first_failure


def test_work_cutoff_leaves_outer_reserve_for_cleanup_and_reap(
    monkeypatch,
):
    """Image work may consume its cutoff while cleanup uses the outer reserve."""
    clock = _FakeClock(200.0)
    process = _DeadlineBudgetProcess(clock)
    signals = _install_deadline_process(monkeypatch, clock, process)
    group_states = iter((True, False))
    monkeypatch.setattr(
        run_module,
        "session_has_live_members",
        lambda _pgid: next(group_states),
    )

    with pytest.raises(DeadlineError):
        run(
            ["fixture-child"],
            what="work-cutoff child",
            timeout=5.0,
            deadline=210.0,
        )

    assert clock.now <= 210.0
    assert process.communicate_timeouts == [5.0, 1.0, 1.0]
    assert signals == [signal.SIGTERM, signal.SIGKILL]
    assert process.reaped


def test_owned_outer_deadline_reserves_shutdown_inside_exact_budget(monkeypatch):
    """An owned child's exact timeout includes TERM, KILL and leader reap."""
    clock = _FakeClock(300.0)
    process = _DeadlineBudgetProcess(clock)
    signals = _install_deadline_process(monkeypatch, clock, process)
    monkeypatch.setattr(
        run_module,
        "session_has_live_members",
        lambda _pgid: False,
    )

    with pytest.raises(DeadlineError):
        run(
            ["fixture-child"],
            what="owned exact-budget child",
            timeout=8.0,
            deadline=308.0,
        )

    assert clock.now == 308.0
    assert process.communicate_timeouts == [6.0, 1.0, 1.0]
    assert signals == [signal.SIGTERM, signal.SIGKILL]
    assert process.reaped


def test_leader_exit_waits_for_killed_grandchild_to_become_quiescent(
    monkeypatch, tmp_path
):
    """A zombie grandchild cannot act and need not be reaped by this process."""
    process = _LeaderExitsBeforeGrandchild()
    proc_root = tmp_path / "proc"
    stat_file = proc_root / str(process.pid + 1) / "stat"
    stat_file.parent.mkdir(parents=True)
    stat_file.write_text(
        f"{process.pid + 1} (fixture grandchild) S 1 {process.pid} 0 0\n"
    )
    group_signals: list[signal.Signals] = []
    state_at_owner_finally: list[tuple[bool, bool]] = []

    monkeypatch.setattr(subprocess, "Popen", lambda *_args, **_kwargs: process)
    monkeypatch.setattr(run_module, "PROC_ROOT", proc_root, raising=False)

    def simulate_group(pgid: int, sent: int) -> None:
        assert pgid == process.pid
        if sent == 0:
            return
        delivered = signal.Signals(sent)
        group_signals.append(delivered)
        if delivered == signal.SIGKILL:
            stat_file.write_text(
                f"{process.pid + 1} (fixture grandchild) Z 1 {process.pid} 0 0\n"
            )

    monkeypatch.setattr("scripts.automation.run.os.killpg", simulate_group)

    def invoke_as_owner() -> None:
        try:
            run(["fixture-child"], what="leader-first fixture child", timeout=0.1)
        finally:
            state_at_owner_finally.append(
                (process.reaped, not run_module.session_has_live_members(process.pid))
            )

    with pytest.raises(DeadlineError):
        invoke_as_owner()

    assert group_signals == [signal.SIGTERM, signal.SIGKILL]
    assert state_at_owner_finally == [(True, True)]


@pytest.mark.parametrize(
    ("state", "expected"),
    [("R", True), ("S", True), ("Z", False), ("X", False)],
    ids=("running", "sleeping", "zombie", "dead"),
)
def test_session_activity_distinguishes_live_and_terminal_proc_states(
    monkeypatch, tmp_path, state, expected
):
    """A retained PGID is unsafe only while it has an executable member."""
    process_group = 4444
    proc_root = tmp_path / "proc"
    stat_file = proc_root / "4445" / "stat"
    stat_file.parent.mkdir(parents=True)
    stat_file.write_text(f"4445 (fixture child) {state} 1 {process_group} 0 0\n")
    monkeypatch.setattr(run_module, "PROC_ROOT", proc_root, raising=False)
    monkeypatch.setattr(run_module.os, "killpg", lambda _pgid, _sent: None)

    assert run_module.session_has_live_members(process_group) is expected


@pytest.mark.parametrize(
    "interruption",
    [
        subprocess.TimeoutExpired(cmd=["fixture-child"], timeout=0.1),
        KeyboardInterrupt(),
    ],
    ids=("timeout", "keyboard-interrupt"),
)
def test_interruption_terminates_then_kills_and_reaps_before_returning(
    monkeypatch, interruption
):
    """Cleanup is complete before an owner can enter its finally revalidation."""
    process = _InterruptingProcess(interruption)
    group_signals: list[signal.Signals] = []
    reaped_at_owner_finally: list[bool] = []

    monkeypatch.setattr(subprocess, "Popen", lambda *_args, **_kwargs: process)
    monkeypatch.setattr("scripts.automation.run.os.getpgid", lambda _pid: process.pid)

    def record_group_signal(_pgid: int, sent: int) -> None:
        if sent == 0:
            raise ProcessLookupError
        delivered = signal.Signals(sent)
        group_signals.append(delivered)
        if delivered == signal.SIGKILL:
            process.killed = True
            process.returncode = -signal.SIGKILL

    monkeypatch.setattr("scripts.automation.run.os.killpg", record_group_signal)

    expected = (
        DeadlineError
        if isinstance(interruption, subprocess.TimeoutExpired)
        else type(interruption)
    )

    def invoke_as_owner() -> None:
        try:
            run(["fixture-child"], what="interrupting a fixture child", timeout=0.1)
        finally:
            reaped_at_owner_finally.append(process.reaped)

    with pytest.raises(expected):
        invoke_as_owner()

    assert group_signals == [signal.SIGTERM, signal.SIGKILL]
    assert reaped_at_owner_finally == [True]
    assert process.reaped, "the process leader was not reaped before run() returned"
    assert process.communicate_calls >= 3
    assert all(
        timeout is not None and timeout > 0 for timeout in process.communicate_timeouts
    ), "an interruption cleanup wait escaped the bounded deadline"


@pytest.mark.parametrize("sent", [signal.SIGINT, signal.SIGTERM])
def test_an_os_signal_cannot_leave_a_surviving_grandchild(tmp_path, sent):
    """SIGINT/SIGTERM must clean the child session before caller revalidation."""
    child_pid = tmp_path / "child-pid"
    group_signal_log = tmp_path / "group-signals"
    survivor = tmp_path / "survivor"
    command = (
        f"echo $$ > {child_pid}; "
        "trap '' INT TERM; "
        f"(trap '' INT TERM; sleep 3; touch {survivor}) & sleep 5"
    )
    driver = subprocess.Popen(
        [
            sys.executable,
            "-c",
            (
                "import scripts.automation.run as run_module\n"
                "real_killpg = run_module.os.killpg\n"
                "def record_killpg(pgid, sent):\n"
                "    if sent != 0:\n"
                f"        with open({str(group_signal_log)!r}, 'a') as stream:\n"
                "            stream.write(f'{sent}\\n')\n"
                "    real_killpg(pgid, sent)\n"
                "run_module.os.killpg = record_killpg\n"
                "try:\n"
                "    run_module.run("
                f"['bash', '-c', {command!r}], "
                "what='signal test', timeout=30)\n"
                "except BaseException:\n"
                "    pass\n"
            ),
        ],
        cwd=REPO_ROOT,
    )
    deadline = time.monotonic() + 5
    while not child_pid.exists() and time.monotonic() < deadline:
        time.sleep(0.02)
    assert child_pid.exists(), "the fixture child did not start"

    try:
        driver.send_signal(sent)
        driver.wait(timeout=8)
        time.sleep(3.2)
        assert not survivor.exists(), f"a grandchild survived {sent.name}"
        assert [
            signal.Signals(int(recorded))
            for recorded in group_signal_log.read_text().splitlines()
        ] == [signal.SIGTERM, signal.SIGKILL]
    finally:
        if driver.poll() is None:
            driver.kill()
            driver.wait(timeout=2)
        try:
            os_pid = int(child_pid.read_text().strip())
            os.killpg(os_pid, signal.SIGKILL)
        except (FileNotFoundError, ProcessLookupError, ValueError):
            pass
