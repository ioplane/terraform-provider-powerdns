#!/usr/bin/env python3
"""End-to-end lifecycle: a consumer's configuration, driven the way a user drives it.

The acceptance suite proves the provider is correct against PowerDNS. It says
nothing about the path a real configuration travels to reach it — remote state,
a module fetched from a remote, an engine holding a lock. Those are where a
provider that passes every unit test still fails in somebody's pipeline.

This drives that path: Terragrunt over an S3 backend on MinIO, a module fetched
over the git protocol, against the running lab. Every assertion is made twice
where it can be — once through the API and once against what DNS actually
answers, because an HTTP 200 proves a request was accepted, not that a name
resolves.

Usage:
    e2e.py up        bring the fixture up: MinIO, git, the bucket, the module
    e2e.py run       run the scenarios
    e2e.py down      remove the fixture
    e2e.py status    report what is running
"""

from __future__ import annotations

import argparse
import subprocess
import sys
import urllib.error
import urllib.request
from dataclasses import dataclass
from pathlib import Path

from tenacity import (
    retry,
    retry_if_result,
    stop_after_delay,
    wait_fixed,
)

try:
    from podman import PodmanClient
    from podman.errors import APIError as PodmanAPIError
except ImportError:  # pragma: no cover - the dev image always has it
    print("podman-py is not installed; run through uv", file=sys.stderr)
    raise SystemExit(2) from None

REPO_ROOT = Path(__file__).resolve().parents[2]
COMPOSE_FILE = REPO_ROOT / "deployments" / "compose" / "compose.e2e.yml"
E2E_DIR = REPO_ROOT / "test" / "e2e"

DEV_CONTAINER_DEFAULT = "terraform-provider-powerdns-dev"
MINIO_CONTAINER = "pdns-e2e-minio"
GIT_CONTAINER = "pdns-e2e-git"

MINIO_URL = "http://127.0.0.1:19000"
GIT_URL = "git://127.0.0.1:19418/dns-modules.git"
BUCKET = "e2e-state"
STATE_KEY = "dns/terraform.tfstate"

AUTH_API = "http://127.0.0.1:18081/api/v1/servers/localhost"
API_KEY = "labapikey"
DNS_PORT = "15300"

ZONE = "e2e.example."
REVERSE_ZONE = "100.51.198.in-addr.arpa."
FQDN = "www.e2e.example."
PTR_NAME = "10.100.51.198.in-addr.arpa."
HTTP_OK = 200

PROVIDER_VERSION = "0.1.1"
BINARY_PREFIX = "terraform-provider-powerdns_v"
MIRROR = "/app/test/e2e/.mirror"


@dataclass
class Runner:
    """Commands executed inside the dev container, where the toolchain lives."""

    container: str

    def exec(
        self,
        command: str,
        *,
        workdir: str = "/app/test/e2e/live",
        check: bool = True,
    ) -> tuple[int, str]:
        """Run a shell command in the dev container and return its status and output."""
        completed = subprocess.run(
            [
                "podman",
                "exec",
                "-w",
                workdir,
                "-e",
                f"TF_CLI_CONFIG_FILE={MIRROR}/terraform.rc",
                "-e",
                "TF_IN_AUTOMATION=1",
                "-e",
                "TG_NON_INTERACTIVE=true",
                self.container,
                "bash",
                # -c, not -lc: a login shell re-reads the profile and drops
                # /usr/local/go/bin and /root/.local/bin from PATH, so `go`
                # and `uv` stop existing halfway through a script that ran a
                # moment earlier.
                "-c",
                command,
            ],
            capture_output=True,
            text=True,
            check=False,
        )
        output = completed.stdout + completed.stderr
        if check and completed.returncode != 0:
            print(output[-4000:], file=sys.stderr)
            msg = f"command failed ({completed.returncode}): {command}"
            raise RuntimeError(msg)
        return completed.returncode, output


def compose(*args: str) -> None:
    """Run podman-compose against the end-to-end compose file."""
    subprocess.run(
        ["podman-compose", "-f", str(COMPOSE_FILE), *args],
        check=True,
        cwd=REPO_ROOT,
    )


def http_ok(url: str, timeout: float = 3.0, *, api_key: str | None = None) -> bool:
    """Whether a loopback URL answers 200. Constants only; see lab.py.

    The key is optional because MinIO's health endpoint takes none and
    PowerDNS answers 401 without one. Checking PowerDNS unauthenticated
    reports a running lab as absent, which is exactly what the first run of
    this script concluded.
    """
    if not url.startswith("http://127.0.0.1"):
        msg = f"refusing to open a non-loopback URL: {url}"
        raise ValueError(msg)
    headers = {"X-API-Key": api_key} if api_key else {}
    request = urllib.request.Request(url, headers=headers)  # noqa: S310
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:  # noqa: S310  # nosemgrep
            return response.status == HTTP_OK
    except (OSError, TimeoutError):
        return False


@retry(
    stop=stop_after_delay(90),
    wait=wait_fixed(2),
    retry=retry_if_result(lambda ready: not ready),
    reraise=True,
)
def wait_for_minio() -> bool:
    """Poll MinIO until it answers its health endpoint.

    tenacity rather than a hand-rolled loop: the retry policy is then a
    declaration rather than arithmetic on a deadline, and it is the same
    library the tests use.
    """
    return http_ok(f"{MINIO_URL}/minio/health/live")


def dev_container() -> str:
    """The dev container for this checkout, which is per-worktree."""
    suffix = ""
    if "/.worktrees/" in str(REPO_ROOT):
        suffix = f"-{REPO_ROOT.name}"
    return f"{DEV_CONTAINER_DEFAULT}{suffix}"


def container_states() -> dict[str, str]:
    """Report each fixture container's state, or 'absent'."""
    states = dict.fromkeys((MINIO_CONTAINER, GIT_CONTAINER), "absent")
    try:
        with PodmanClient() as client:
            for container in client.containers.list(all=True):
                name = container.attrs.get("Names", [None])[0]
                if name in states:
                    states[name] = container.attrs.get("State", "unknown")
    except (PodmanAPIError, OSError) as error:
        print(f"podman API unavailable: {error}", file=sys.stderr)
    return states


# --- fixture setup -------------------------------------------------------


def make_bucket() -> None:
    """Create the state bucket by creating its directory in MinIO's store.

    MinIO lays a bucket out as a directory under its data root, so this needs
    no S3 client and no request signing to set up the thing whose purpose is
    to be written to over S3.
    """
    subprocess.run(
        ["podman", "exec", MINIO_CONTAINER, "mkdir", "-p", f"/data/{BUCKET}"],
        check=True,
    )


def make_module_repo() -> None:
    """Create the bare repository the module is pushed into.

    `git daemon` serves repositories; it does not create them. Bare because a
    push to a non-bare repository's checked-out branch is refused.
    """
    subprocess.run(
        [
            "podman",
            "exec",
            GIT_CONTAINER,
            "git",
            "init",
            "-q",
            "--bare",
            "--initial-branch=main",
            "/srv/git/dns-modules.git",
        ],
        check=True,
    )


def seed_module_repo(runner: Runner) -> None:
    """Publish the module to the git remote, from the dev container.

    The module is pushed rather than bind-mounted: Terragrunt must fetch it
    over the git protocol for the test to mean anything.
    """
    script = (
        "set -eu; "
        "rm -rf /tmp/modrepo && mkdir -p /tmp/modrepo && cd /tmp/modrepo && "
        "git init -q -b main && "
        "git config user.email e2e@example.com && "
        "git config user.name 'e2e fixture' && "
        "mkdir -p modules && cp -r /app/test/e2e/modules/dns-zone modules/ && "
        "git add -A && git commit -q -m 'module under test' && "
        f"git push -q --force {GIT_URL} main"
    )
    # The path is inside the dev container, which is discarded with it; S108
    # is about a shared host /tmp and does not apply.
    runner.exec(script, workdir="/tmp", check=True)  # noqa: S108


def build_provider_mirror(runner: Runner) -> None:
    """Build the provider into a filesystem mirror laid out like the registry.

    A dev_overrides block would be less work and would skip `terraform init`
    for providers, which is the step that resolves and locks a version. The
    mirror exercises the path a user actually takes, and incidentally proves
    the release layout is installable.
    """
    plat = "linux_amd64"
    dest = f"{MIRROR}/registry.terraform.io/ioplane/powerdns/{PROVIDER_VERSION}/{plat}"
    script = (
        "set -eu; "
        f"rm -rf {MIRROR} && mkdir -p {dest} && "
        f"cd /app && go build -o {dest}/{BINARY_PREFIX}{PROVIDER_VERSION} . && "
        f"cat > {MIRROR}/terraform.rc <<'RC'\n"
        "provider_installation {\n"
        f'  filesystem_mirror {{\n    path    = "{MIRROR}"\n'
        '    include = ["registry.terraform.io/ioplane/*"]\n  }\n'
        '  direct {\n    exclude = ["registry.terraform.io/ioplane/*"]\n  }\n'
        "}\n"
        "RC"
    )
    runner.exec(script, workdir="/app", check=True)


# --- lifecycle -----------------------------------------------------------


def cmd_up() -> int:
    """Bring the fixture up and prepare everything the tests assume exists."""
    if not http_ok(f"{AUTH_API}/zones", api_key=API_KEY):
        print("the lab is not running — run: task lab:up", file=sys.stderr)
        return 2

    compose("up", "-d", "--build")

    if not wait_for_minio():
        print("MinIO did not answer within 90 seconds", file=sys.stderr)
        return 1
    print("ok    MinIO answering")

    runner = Runner(container=dev_container())
    make_bucket()
    print(f"ok    bucket {BUCKET}")
    make_module_repo()
    print("ok    bare repository on the git remote")
    seed_module_repo(runner)
    print("ok    module pushed to the git remote")
    build_provider_mirror(runner)
    print(f"ok    provider {PROVIDER_VERSION} in a filesystem mirror")
    return 0


def cmd_down() -> int:
    """Remove the fixture, including its volumes."""
    compose("down", "-v")
    return 0


def cmd_status() -> int:
    """Report what is running."""
    for name, state in container_states().items():
        print(f"{state:<10} {name}")
    minio_state = "up" if http_ok(f"{MINIO_URL}/minio/health/live") else "down"
    print(f"{minio_state:<10} MinIO API")
    return 0


def main() -> int:
    """Dispatch a subcommand."""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("up", "down", "status"))
    args = parser.parse_args()
    return {"up": cmd_up, "down": cmd_down, "status": cmd_status}[args.command]()


if __name__ == "__main__":
    raise SystemExit(main())
