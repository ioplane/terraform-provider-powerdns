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
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from pathlib import Path

# A pull of the fixture's images on a cold runner is the slowest legitimate
# case. Ten minutes is several times what it takes and a sixth of the job.
PULL = 600.0
# Anything that only talks to the local daemon.
LOCAL = 60.0
# A terragrunt or terraform command against the fixture. Under pytest's own
# 900-second per-test ceiling on purpose: when both would fire, the one that
# names the command is the more useful failure.
COMMAND = 600.0


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
) -> subprocess.CompletedProcess:
    """Run `argv` with a deadline, naming `what` it was doing if it expires.

    The keyword arguments are enumerated rather than forwarded as `**kwargs`:
    a passthrough cannot be type-checked, and the set a fixture driver needs is
    small enough to write down.

    Args:
        argv: The command, as a list. No shell.
        what: What this call is doing, in words, for the failure message.
        timeout: Seconds before the command is killed.
        cwd: Working directory.
        check: Raise on a non-zero exit status.
        capture_output: Collect stdout and stderr instead of inheriting them.
        text: Decode the captured output.
        env: Replace the environment rather than inheriting it.

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
    with subprocess.Popen(
        argv,
        cwd=cwd,
        env=env,
        stdout=subprocess.PIPE if capture_output else None,
        stderr=subprocess.PIPE if capture_output else None,
        text=text,
        start_new_session=True,
    ) as process:
        try:
            stdout, stderr = process.communicate(timeout=timeout)
        except subprocess.TimeoutExpired:
            _kill_session(process)
            stdout, stderr = process.communicate()
            message = (
                f"{what} did not finish in {timeout:.0f}s; "
                "it and everything it started were killed.\n"
                f"  command: {' '.join(argv)}\n"
                "  This is usually a stalled image pull. `podman images` and "
                "`podman ps -a` show how far it got."
            )
            print(message, file=sys.stderr)
            raise DeadlineError(message) from None

    completed = subprocess.CompletedProcess(argv, process.returncode, stdout, stderr)
    if check:
        completed.check_returncode()
    return completed


def _kill_session(process: subprocess.Popen) -> None:
    """Kill the process group `process` leads, then the process itself.

    `start_new_session=True` made it a group leader, so one signal reaches
    everything it spawned. The group may already be gone — the child can exit
    between the timeout and this call — which is not an error.
    """
    try:
        os.killpg(os.getpgid(process.pid), signal.SIGKILL)
    except (ProcessLookupError, PermissionError):
        process.kill()
