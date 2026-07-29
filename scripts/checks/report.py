"""The output convention every check in this package shares.

One line per assertion, `ok` for a pass and `FAIL` for a failure, and a summary
naming how many things were looked at. The shape is not decoration: a check
that prints only on failure cannot be distinguished from a check that did
nothing, and several of these run in CI where nobody watches them succeed.

Failures accumulate rather than exiting at the first one. A run that reports
every problem is one round of fixing; a run that stops at the first is as many
rounds as there are problems.
"""

from __future__ import annotations

import sys


class Report:
    """Collects the outcome of one check and decides its exit status."""

    def __init__(self, name: str) -> None:
        """Start a report for the check called `name`."""
        self.name = name
        self.failures: list[str] = []
        self.warnings: list[str] = []

    def ok(self, message: str) -> None:
        """Record an assertion that held."""
        print(f"ok    {message}")

    def _to_stderr(self, line: str) -> None:
        """Write to stderr with stdout flushed first.

        stdout is block-buffered when redirected and stderr is not, so without
        this the `ok` lines arrive after the failures they precede and a CI log
        reads in the wrong order.
        """
        sys.stdout.flush()
        print(line, file=sys.stderr, flush=True)

    def warn(self, message: str) -> None:
        """Record something worth saying that is not a failure.

        Used where the check could not reach an answer — an unreachable host,
        a tool that is not installed — and treating that as a failure would
        turn an outage somewhere else into a red build here.
        """
        self.warnings.append(message)
        self._to_stderr(f"WARN  {message}")

    def fail(self, message: str) -> None:
        """Record an assertion that did not hold."""
        self.failures.append(message)
        self._to_stderr(f"FAIL  {message}")

    def summary(self, message: str) -> int:
        """Print the closing line and return the process exit status."""
        if self.failures:
            self._to_stderr(f"\n{self.name}: {len(self.failures)} failed")
            return 1
        print(f"\n{self.name}: {message}")
        return 0
