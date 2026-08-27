"""Bounded subprocess execution for the fixture drivers.

Nothing here blocks forever. A stalled image pull consumed an entire
sixty-minute end-to-end job on 2026-07-29 and produced no diagnostic at all:
the step was cancelled at the job ceiling, GitHub reported "cancelled" rather
than a failure, and the log for the step was never written. The re-run took two
minutes, which is how the hang was identified as a stalled pull rather than a
defect — by inference, because nothing had recorded what it was waiting on.

So every call that can block on the network carries a deadline and says what it
was doing when the deadline passed. The numbers are generous: they are there to
turn an hour of nothing into a minute of something, not to police normal
variation.
"""

from __future__ import annotations

import os
import signal
import subprocess
import sys
import threading
import time
from pathlib import Path
from typing import TYPE_CHECKING, NoReturn

if TYPE_CHECKING:
    from types import FrameType

# A pull of the fixture's images on a cold runner is the slowest legitimate
# case. Ten minutes is several times what it takes and a sixth of the job.
PULL = 600.0
# Anything that only talks to the local daemon.
LOCAL = 60.0
# A terragrunt or terraform command against the fixture. Under pytest's own
# 900-second per-test ceiling on purpose: when both would fire, the one that
# names the command is the more useful failure.
COMMAND = 600.0
# A cooperative command receives this long to stop before the process group is
# force-killed. The second bound is the maximum wait for the killed leader to
# be reaped; neither cleanup phase can silently consume its caller's job.
TERMINATE_GRACE = 1.0
KILL_GRACE = 1.0
CLEANUP_GRACE = TERMINATE_GRACE + KILL_GRACE
GROUP_EXIT_POLL = 0.01
INTERRUPTION_SIGNALS = (signal.SIGINT, signal.SIGTERM)
PROC_ROOT = Path("/proc")
PROC_STAT_MIN_FIELDS = 3
TERMINAL_PROCESS_STATES = frozenset({"X", "Z"})


class DeadlineError(RuntimeError):
    """A command passed its deadline."""


def run(  # noqa: PLR0913 - each one is a subprocess option a caller needs
    argv: list[str],
    *,
    what: str,
    timeout: float = LOCAL,
    cwd: Path | None = None,
    check: bool = True,
    capture_output: bool = False,
    text: bool = False,
    env: dict[str, str] | None = None,
    deadline: float | None = None,
) -> subprocess.CompletedProcess:
    """Run `argv` with a deadline, naming `what` it was doing if it expires.

    The keyword arguments are enumerated rather than forwarded as `**kwargs`:
    a passthrough cannot be type-checked, and the set a fixture driver needs is
    small enough to write down.

    Args:
        argv: The command, as a list. No shell.
        what: What this call is doing, in words, for the failure message.
        timeout: Maximum seconds of child work before cleanup starts.
        cwd: Working directory.
        check: Raise on a non-zero exit status.
        capture_output: Collect stdout and stderr instead of inheriting them.
        text: Decode the captured output.
        env: Replace the environment rather than inheriting it.
        deadline: Absolute monotonic outer deadline including cleanup. When
            absent, derive one from the child timeout and the fixed cleanup
            grace periods.

    Returns:
        The completed process.

    Raises:
        DeadlineError: when the command did not finish in `timeout` seconds.
    """
    # Popen with its own session, not subprocess.run. `run`'s timeout kills the
    # direct child only: every command here is `bash -c "a && b"` or a podman
    # client, so the thing actually doing the work — go build, terraform, the
    # process behind `podman exec` — is a grandchild and survives. It then keeps
    # writing to the mirror, the remote state or the lab while cleanup or a
    # retry starts, which is worse than no deadline at all.
    started = time.monotonic()
    if deadline is None:
        outer_deadline = started + timeout + CLEANUP_GRACE
        child_timeout = timeout
    else:
        outer_deadline = deadline
        remaining_outer = _remaining_time(outer_deadline, phase="child")
        child_timeout = min(timeout, remaining_outer - CLEANUP_GRACE)
        if child_timeout <= 0:
            msg = "process outer deadline leaves no time before cleanup reserve"
            raise RuntimeError(msg)
    previous_handlers = (
        [
            (interruption_signal, signal.getsignal(interruption_signal))
            for interruption_signal in INTERRUPTION_SIGNALS
        ]
        if threading.current_thread() is threading.main_thread()
        else []
    )

    try:
        for interruption_signal, _previous_handler in previous_handlers:
            signal.signal(interruption_signal, _raise_interruption)
        process = subprocess.Popen(
            argv,
            cwd=cwd,
            env=env,
            stdout=subprocess.PIPE if capture_output else None,
            stderr=subprocess.PIPE if capture_output else None,
            text=text,
            start_new_session=True,
        )
        try:
            stdout, stderr = process.communicate(timeout=child_timeout)
        except subprocess.TimeoutExpired as failure:
            _ignore_further_interruptions(previous_handlers)
            try:
                stdout, stderr = _stop_session(process, deadline=outer_deadline)
            except Exception as cleanup_failure:
                raise cleanup_failure from failure
            message = (
                f"{what} did not finish in {timeout:.0f}s; "
                "it and everything it started were killed.\n"
                f"  command: {' '.join(argv)}\n"
                "  This is usually a stalled image pull. `podman images` and "
                "`podman ps -a` show how far it got."
            )
            print(message, file=sys.stderr)
            raise DeadlineError(message) from None
        except BaseException as failure:
            _ignore_further_interruptions(previous_handlers)
            try:
                _stop_session(process, deadline=outer_deadline)
            except Exception as cleanup_failure:
                raise cleanup_failure from failure
            raise
    finally:
        for interruption_signal, previous_handler in previous_handlers:
            signal.signal(interruption_signal, previous_handler)

    completed = subprocess.CompletedProcess(argv, process.returncode, stdout, stderr)
    if check:
        completed.check_returncode()
    return completed


def _raise_interruption(signum: int, _frame: FrameType | None) -> NoReturn:
    """Turn terminal signals into exceptions so `run` can clean its child."""
    if signum == signal.SIGINT:
        raise KeyboardInterrupt
    raise SystemExit(128 + signum)


def _ignore_further_interruptions(previous_handlers: list[tuple]) -> None:
    """Keep repeated terminal signals from escaping the bounded cleanup path."""
    if not previous_handlers:
        return
    for interruption_signal in INTERRUPTION_SIGNALS:
        signal.signal(interruption_signal, signal.SIG_IGN)


def _signal_session(
    process: subprocess.Popen, process_group: int, sent: signal.Signals
) -> None:
    """Signal the complete child session, with a direct-child last resort."""
    try:
        os.killpg(process_group, sent)
    except ProcessLookupError:
        return
    except PermissionError:
        if sent == signal.SIGTERM:
            process.terminate()
        else:
            process.kill()


def session_has_live_members(process_group: int) -> bool:
    """Return whether the child process group has an executable member."""
    try:
        os.killpg(process_group, 0)
    except ProcessLookupError:
        return False

    try:
        processes = PROC_ROOT.iterdir()
    except OSError:
        return True
    live_member = False
    for process in processes:
        if not process.name.isdecimal():
            continue
        try:
            raw_stat = (process / "stat").read_text(encoding="utf-8")
        except FileNotFoundError:
            continue
        except OSError:
            live_member = True
            break
        closing_parenthesis = raw_stat.rfind(")")
        fields = raw_stat[closing_parenthesis + 1 :].split()
        if closing_parenthesis < 0 or len(fields) < PROC_STAT_MIN_FIELDS:
            live_member = True
            break
        try:
            member_group = int(fields[2])
        except ValueError:
            live_member = True
            break
        if member_group == process_group and fields[0] not in TERMINAL_PROCESS_STATES:
            live_member = True
            break
    return live_member


def _remaining_time(deadline: float, *, phase: str) -> float:
    """Return positive time under one absolute deadline or fail closed."""
    remaining = deadline - time.monotonic()
    if remaining <= 0:
        msg = f"process cleanup deadline expired before {phase}"
        raise RuntimeError(msg)
    return remaining


def _wait_for_session_exit(process_group: int, *, deadline: float) -> None:
    """Boundedly prove a force-killed process group has become quiescent."""
    while session_has_live_members(process_group):
        remaining = _remaining_time(deadline, phase="process-group quiescence")
        time.sleep(min(GROUP_EXIT_POLL, remaining))


def _stop_session(
    process: subprocess.Popen,
    *,
    deadline: float,
) -> tuple[str | bytes | None, str | bytes | None]:
    """Terminate, force-kill and reap a child session within fixed bounds.

    `start_new_session=True` made it a group leader, so one signal reaches
    everything it spawned. A leader may exit while one of its descendants
    ignores SIGTERM, so even a successfully reaped leader is followed by a
    live-member check before cleanup returns.
    """
    process_group = process.pid
    try:
        terminate_timeout = min(
            TERMINATE_GRACE,
            _remaining_time(deadline, phase="SIGTERM"),
        )
    except RuntimeError:
        _signal_session(process, process_group, signal.SIGKILL)
        raise
    _signal_session(process, process_group, signal.SIGTERM)
    try:
        stdout, stderr = process.communicate(timeout=terminate_timeout)
    except subprocess.TimeoutExpired:
        try:
            kill_timeout = min(
                KILL_GRACE,
                _remaining_time(deadline, phase="SIGKILL reap"),
            )
        except RuntimeError:
            _signal_session(process, process_group, signal.SIGKILL)
            raise
        _signal_session(process, process_group, signal.SIGKILL)
        output = process.communicate(timeout=kill_timeout)
        _wait_for_session_exit(process_group, deadline=deadline)
        return output

    if session_has_live_members(process_group):
        try:
            _remaining_time(deadline, phase="surviving process-group SIGKILL")
        except RuntimeError:
            _signal_session(process, process_group, signal.SIGKILL)
            raise
        _signal_session(process, process_group, signal.SIGKILL)
        _wait_for_session_exit(process_group, deadline=deadline)
    return stdout, stderr
