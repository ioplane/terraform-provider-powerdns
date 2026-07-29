#!/usr/bin/env python3
"""Lab lifecycle automation for the acceptance-test fixture.

Drives the Podman REST API through podman-py (https://github.com/containers/podman-py)
rather than shelling out to the CLI, so failures surface as exceptions with a
status code instead of a parsed string.

The compose file declares the fixture; this script answers the questions
compose cannot: is every service actually serving its API, and is each one the
version the tests were written against.

Usage:
    lab.py up        bring the fixture up and wait until all APIs answer
    lab.py down      remove the fixture, including its volumes
    lab.py status    report container state and reachable API versions
    lab.py verify    assert the fixture matches the pinned reference points

Every subcommand takes --auth to choose the authoritative branch, because the
provider claims a range and one version cannot demonstrate a range:

    lab.py up --auth 5.0
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
import time
import urllib.error
import urllib.request
from dataclasses import dataclass
from pathlib import Path

try:
    from podman import PodmanClient
    from podman.errors import APIError as PodmanAPIError
except ImportError:  # pragma: no cover - the dev image always has it
    print("podman-py is not installed; run inside the dev container", file=sys.stderr)
    raise SystemExit(2) from None

REPO_ROOT = Path(__file__).resolve().parents[2]
COMPOSE_DIR = REPO_ROOT / "deployments" / "compose"
COMPOSE_FILE = COMPOSE_DIR / "compose.lab.yml"
API_KEY = "labapikey"
HTTP_OK = 200

# Pinned reference points. When one of these moves, the claims that depend on
# it are re-verified rather than assumed still correct — see
# docs/standards/powerdns-api-discipline.md §5.
EXPECTED_REC_VERSION = "5.4.4"
EXPECTED_DNSDIST_VERSION = "2.1.0"

# The authoritative branches the provider is tested against. The version here
# and the digest in the corresponding compose file are two halves of one pin:
# the compose file decides what runs, this decides what the fixture is allowed
# to be, and `verify` fails when they disagree.
AUTH_BRANCHES: dict[str, tuple[str, Path | None]] = {
    "5.1": ("5.1.3", None),
    "5.0": ("5.0.6", COMPOSE_DIR / "compose.lab-auth-50.yml"),
}
DEFAULT_AUTH_BRANCH = "5.1"


@dataclass(frozen=True)
class Service:
    """One lab service and how to prove it is serving."""

    container: str
    url: str
    daemon_type: str
    version: str
    role: str


def services(auth_version: str) -> tuple[Service, ...]:
    """The fixture's services, with the authoritative pair on `auth_version`."""
    return (
        Service(
            container="pdns-lab-auth-pg",
            url="http://127.0.0.1:18081/api/v1/servers",
            daemon_type="authoritative",
            version=auth_version,
            role="authoritative on PostgreSQL — the common deployment",
        ),
        Service(
            container="pdns-lab-auth-lmdb",
            url="http://127.0.0.1:18091/api/v1/servers",
            daemon_type="authoritative",
            version=auth_version,
            role="authoritative on LMDB — the only backend implementing views",
        ),
        Service(
            container="pdns-lab-rec",
            url="http://127.0.0.1:18082/api/v1/servers",
            daemon_type="recursor",
            version=EXPECTED_REC_VERSION,
            role="recursor with api_dir set — without it every write is 422",
        ),
        Service(
            container="pdns-lab-dnsdist",
            url="http://127.0.0.1:18083/api/v1/servers/localhost",
            daemon_type="",
            version=EXPECTED_DNSDIST_VERSION,
            role="dnsdist — the only place its two write operations exist",
        ),
    )


def compose(overlay: Path | None, *args: str) -> None:
    """Run podman-compose against the lab compose file and any overlay."""
    files = ["-f", str(COMPOSE_FILE)]
    if overlay is not None:
        files += ["-f", str(overlay)]
    subprocess.run(
        ["podman-compose", *files, *args],
        check=True,
        cwd=REPO_ROOT,
    )


def api_get(url: str, timeout: float = 3.0) -> list | dict | None:
    """GET a PowerDNS API endpoint, returning None when it is not answering."""
    # urllib honours file:// and other schemes, so anything reaching urlopen is
    # checked first. Every caller here passes a URL built by services(), but the
    # check costs nothing and keeps that true if a caller ever stops being
    # constant.
    if not url.startswith(("http://", "https://")):
        msg = f"refusing to open a non-HTTP URL: {url}"
        raise ValueError(msg)

    request = urllib.request.Request(url, headers={"X-API-Key": API_KEY})  # noqa: S310

    # The urlopen below is suppressed for semgrep's dynamic-urllib rule. The URL
    # is not attacker-controlled — every caller passes a URL built by services(),
    # and the scheme is checked above. Adopting requests purely to satisfy the
    # pattern would add a dependency to a loopback health check, which is the
    # worse trade.
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:  # noqa: S310  # nosemgrep
            if response.status != HTTP_OK:
                return None
            return json.loads(response.read())
    # OSError covers URLError and the bare ConnectionResetError a server raises
    # when it accepts the connection during startup and closes it before
    # replying — the normal case while the fixture is still coming up.
    # TimeoutError is a subclass of OSError; naming both says the
    # author believed otherwise.
    except (OSError, json.JSONDecodeError):
        return None


def wait_for_apis(fixture: tuple[Service, ...], deadline_seconds: int = 120) -> bool:
    """Block until every service answers its API, or the deadline passes."""
    deadline = time.monotonic() + deadline_seconds
    pending = {service.container: service for service in fixture}

    while pending and time.monotonic() < deadline:
        for name, service in list(pending.items()):
            if api_get(service.url) is not None:
                print(f"  ready: {name}")
                del pending[name]
        if pending:
            time.sleep(2)

    for name in pending:
        print(f"  TIMEOUT: {name}", file=sys.stderr)
    return not pending


def container_states() -> dict[str, str]:
    """Map lab container names to their Podman state.

    Prefers the Podman REST API through podman-py, which reports failures as
    typed exceptions rather than text to be parsed. Falls back to the CLI when
    the API socket is not running, because requiring `podman system service`
    would make a diagnostic command fail exactly when diagnosis is wanted.
    """
    try:
        with PodmanClient() as client:
            return {
                container.name: container.status
                for container in client.containers.list(all=True)
                if container.name and container.name.startswith("pdns-lab-")
            }
    except PodmanAPIError:
        print(
            "podman API socket unavailable, falling back to the CLI; "
            "enable it with: systemctl --user enable --now podman.socket",
            file=sys.stderr,
        )

    result = subprocess.run(
        [
            "podman",
            "ps",
            "-a",
            "--filter",
            "name=pdns-lab-",
            "--format",
            "{{.Names}}\t{{.State}}",
        ],
        capture_output=True,
        text=True,
        check=True,
    )
    states: dict[str, str] = {}
    for line in result.stdout.splitlines():
        if not line.strip():
            continue
        name, _, state = line.partition("\t")
        states[name] = state
    return states


def cmd_up(branch: str) -> int:
    """Start the fixture and wait until every API answers."""
    auth_version, overlay = AUTH_BRANCHES[branch]
    compose(overlay, "up", "-d")
    print(f"waiting for APIs (authoritative {auth_version})")
    if not wait_for_apis(services(auth_version)):
        return 1
    print()
    print("Lab endpoints:")
    print(f"  auth gpgsql  http://127.0.0.1:18081/api/v1   X-API-Key: {API_KEY}")
    print(f"  auth lmdb    http://127.0.0.1:18091/api/v1   X-API-Key: {API_KEY}")
    print(f"  recursor     http://127.0.0.1:18082/api/v1   X-API-Key: {API_KEY}")
    print(f"  dnsdist      http://127.0.0.1:18083/api/v1   X-API-Key: {API_KEY}")
    print("  postgres     127.0.0.1:15432  pdns/pdns/pdns")
    return 0


def cmd_down(branch: str) -> int:
    """Remove the fixture, including its volumes."""
    _, overlay = AUTH_BRANCHES[branch]
    compose(overlay, "down", "-v")
    return 0


def cmd_status(branch: str) -> int:
    """Report container state and the version each API reports."""
    states = container_states()
    if not states:
        print("lab is not running")
        return 1
    for service in services(AUTH_BRANCHES[branch][0]):
        state = states.get(service.container, "absent")
        payload = api_get(service.url)
        if payload:
            server = payload[0] if isinstance(payload, list) else payload
            version = server["version"]
        else:
            version = "unreachable"
        print(f"{service.container:24s} {state:10s} version={version}")
    return 0


def cmd_verify(branch: str) -> int:
    """Assert the fixture is the one the tests were written against."""
    fixture = services(AUTH_BRANCHES[branch][0])
    failures: list[str] = []
    for service in fixture:
        payload = api_get(service.url)
        if payload is None:
            failures.append(f"{service.container}: API not answering")
            continue
        server = payload[0] if isinstance(payload, list) else payload
        if server["version"] != service.version:
            failures.append(
                f"{service.container}: version {server['version']}, "
                f"expected {service.version}"
            )
        if service.daemon_type and server.get("daemon_type") != service.daemon_type:
            failures.append(
                f"{service.container}: daemon_type {server['daemon_type']}, "
                f"expected {service.daemon_type}"
            )

    # The two authoritative instances must genuinely differ in backend,
    # otherwise the matrix is a duplicate and proves nothing (ADR 0005).
    lmdb_views = api_get("http://127.0.0.1:18091/api/v1/servers/localhost/views")
    if lmdb_views is None:
        failures.append("auth-lmdb: /views not reachable")

    for failure in failures:
        print(f"FAIL {failure}", file=sys.stderr)
    if failures:
        return 1

    for service in fixture:
        print(f"OK   {service.container:24s} {service.role}")
    return 0


def main() -> int:
    """Dispatch the requested subcommand."""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("up", "down", "status", "verify"))
    parser.add_argument(
        "--auth",
        choices=tuple(AUTH_BRANCHES),
        default=DEFAULT_AUTH_BRANCH,
        help="authoritative branch to run (default: %(default)s)",
    )
    args = parser.parse_args()

    return {
        "up": cmd_up,
        "down": cmd_down,
        "status": cmd_status,
        "verify": cmd_verify,
    }[args.command](args.auth)


if __name__ == "__main__":
    raise SystemExit(main())
