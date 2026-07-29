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
import base64
import contextlib
import json
import os
import ssl
import subprocess
import sys
import urllib.error
import urllib.request
from dataclasses import dataclass
from pathlib import Path

import boto3
from botocore.client import Config
from botocore.exceptions import ClientError
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
S3_CONTAINER = "pdns-e2e-s3"
FORGEJO_CONTAINER = "pdns-e2e-forgejo"

S3_URL = "http://127.0.0.1:19000"
FORGEJO_URL = "https://127.0.0.1:19300"
FORGEJO_USER = "e2e"
FORGEJO_PASSWORD = "e2e-fixture-password"  # noqa: S105
FORGEJO_REPO = "dns-modules"
BUCKET = "e2e-state"
S3_ACCESS_KEY = "e2eaccesskey"
S3_SECRET_KEY = "e2esecretkey"  # noqa: S105
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
# Derived from the checkout rather than written as a container path. Inside
# the dev container the repository is /app, so this is the same string it
# always was; on a runner it is wherever the checkout landed. One definition,
# correct in both places.
MIRROR = str(E2E_DIR / ".mirror")

# A home of its own for local execution.
#
# The fixture configures `credential.helper store` and writes
# `~/.git-credentials`. Run against a developer's real home that is not a test
# fixture, it is a change to their machine: the helper then captures every
# credential git handles and writes it in plaintext, and the file write
# truncates whatever was there. Both happened here before this existed.
#
# HOME is redirected for anything the driver runs locally, so `--global` means
# this directory and nothing else.
FIXTURE_HOME = E2E_DIR / ".home"
TLS_DIR_NAME = ".tls"
TLS_CERT_NAME = "cert.pem"
TLS_KEY_NAME = "key.pem"


@dataclass
class Runner:
    """Commands run where the toolchain is.

    On a developer's machine that is the dev container, and the paths the
    fixture uses are the container's. On a hosted runner there is no dev
    container — CI installs the toolchain onto the runner itself — so the same
    commands run locally against the checkout.

    One driver either way. The alternative was building the dev image in CI to
    have somewhere to exec into, which costs more per run than the run.
    """

    container: str

    @property
    def in_container(self) -> bool:
        """Whether a dev container is available to execute in."""
        if not self.container:
            return False
        # Running, not merely existing. `podman container exists` is true for a
        # stopped container, and the driver then tried to exec into one and got
        # 255 with the command echoed back at it.
        probe = subprocess.run(
            [
                "podman",
                "container",
                "inspect",
                "--format",
                "{{.State.Running}}",
                self.container,
            ],
            check=False,
            capture_output=True,
            text=True,
        )
        return probe.returncode == 0 and probe.stdout.strip() == "true"

    def exec(
        self,
        command: str,
        *,
        workdir: str = "",
        check: bool = True,
    ) -> tuple[int, str]:
        """Run a shell command where the toolchain is, and return status and output."""
        workdir = workdir or str(REPO_ROOT)
        environment = {
            "TF_CLI_CONFIG_FILE": f"{MIRROR}/terraform.rc",
            "TF_IN_AUTOMATION": "1",
            "TG_NON_INTERACTIVE": "true",
        }

        if self.in_container:
            # Commands are written in the checkout's own paths. Inside the
            # container the checkout is bind-mounted at /app, so the one
            # translation happens here rather than in every caller.
            command = command.replace(str(REPO_ROOT), "/app")
            workdir = workdir.replace(str(REPO_ROOT), "/app")
            environment = {
                k: v.replace(str(REPO_ROOT), "/app") for k, v in environment.items()
            }
            argv = ["podman", "exec", "-w", workdir]
            for key, value in environment.items():
                argv += ["-e", f"{key}={value}"]
            argv += [
                self.container,
                "bash",
                # -c, not -lc: a login shell re-reads the profile and drops
                # /usr/local/go/bin and /root/.local/bin from PATH, so `go`
                # and `uv` stop existing halfway through a script that ran a
                # moment earlier.
                "-c",
                command,
            ]
            completed = subprocess.run(
                argv, capture_output=True, text=True, check=False
            )
        else:
            FIXTURE_HOME.mkdir(parents=True, exist_ok=True)
            completed = subprocess.run(
                ["bash", "-c", command],
                cwd=workdir if Path(workdir).is_dir() else str(REPO_ROOT),
                env={
                    **os.environ,
                    **environment,
                    # `--global` and `~` resolve here, not in the home of
                    # whoever ran this.
                    "HOME": str(FIXTURE_HOME),
                    "XDG_CACHE_HOME": str(FIXTURE_HOME / ".cache"),
                },
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


def s3_answering() -> bool:
    """Whether the S3 gateway is serving.

    Any HTTP answer means it is. SeaweedFS replies 403 to an unsigned list,
    which is a served request and not an outage — treating only 200 as ready
    waits for something that never happens.
    """
    try:
        with urllib.request.urlopen(S3_URL, timeout=3):  # nosemgrep
            return True
    except urllib.error.HTTPError:
        return True
    except OSError:
        return False


def _tls_context() -> ssl.SSLContext | None:
    """Trust the fixture's own certificate, and only that one.

    `verify_mode = CERT_NONE` would be one line shorter and would make every
    later reader wonder whether the suite verifies anything.
    """
    cert = E2E_DIR / TLS_DIR_NAME / TLS_CERT_NAME
    if not cert.is_file():
        return None
    context = ssl.create_default_context(cafile=str(cert))
    # Stated rather than inherited. The default floor moves with the Python
    # version, and a fixture that quietly negotiates down on an older
    # interpreter is a fixture that stops testing what it says it tests.
    context.minimum_version = ssl.TLSVersion.TLSv1_2
    return context


def http_ok(url: str, timeout: float = 3.0, *, api_key: str | None = None) -> bool:
    """Whether a loopback URL answers 200. Constants only; see lab.py.

    The key is optional because MinIO's health endpoint takes none and
    PowerDNS answers 401 without one. Checking PowerDNS unauthenticated
    reports a running lab as absent, which is exactly what the first run of
    this script concluded.
    """
    if not url.startswith(("http://127.0.0.1", "https://127.0.0.1")):
        msg = f"refusing to open a non-loopback URL: {url}"
        raise ValueError(msg)
    headers = {"X-API-Key": api_key} if api_key else {}
    request = urllib.request.Request(url, headers=headers)  # noqa: S310
    try:
        with urllib.request.urlopen(  # noqa: S310  # nosemgrep
            request, timeout=timeout, context=_tls_context()
        ) as response:
            return response.status == HTTP_OK
    except OSError:
        return False


@retry(
    stop=stop_after_delay(90),
    wait=wait_fixed(2),
    retry=retry_if_result(lambda ready: not ready),
    reraise=True,
)
def wait_for_s3() -> bool:
    """Poll the S3 gateway until it answers.

    SeaweedFS has no health endpoint on the gateway; a 403 from an
    unauthenticated list is a served request, which is what is being waited
    for. Waiting for a 200 would wait forever.

    tenacity rather than a hand-rolled loop: the retry policy is then a
    declaration rather than arithmetic on a deadline, and it is the same
    library the tests use.
    """
    return s3_answering()


@retry(
    stop=stop_after_delay(180),
    wait=wait_fixed(3),
    retry=retry_if_result(lambda ready: not ready),
    reraise=True,
)
def wait_for_forgejo() -> bool:
    """Poll Forgejo until it is serving.

    Longer than MinIO's budget: it migrates its database on first start, and
    that is slower than answering a health check on an object store.
    """
    return http_ok(f"{FORGEJO_URL}/api/healthz")


def dev_container() -> str:
    """The dev container for this checkout, which is per-worktree."""
    suffix = ""
    if "/.worktrees/" in str(REPO_ROOT):
        suffix = f"-{REPO_ROOT.name}"
    return f"{DEV_CONTAINER_DEFAULT}{suffix}"


def container_states() -> dict[str, str]:
    """Report each fixture container's state, or 'absent'."""
    states = dict.fromkeys((S3_CONTAINER, FORGEJO_CONTAINER), "absent")
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


def make_tls_certificate() -> None:
    """Generate the self-signed certificate Forgejo serves and git trusts.

    Before the fixture starts, because Forgejo reads the files at boot. One
    certificate, one host, regenerated whenever the fixture is rebuilt — there
    is nothing here worth reusing between runs.
    """
    tls = E2E_DIR / TLS_DIR_NAME
    tls.mkdir(parents=True, exist_ok=True)
    if (tls / TLS_CERT_NAME).is_file() and (tls / TLS_KEY_NAME).is_file():
        return

    subprocess.run(
        [
            "openssl",
            "req",
            "-x509",
            "-newkey",
            "rsa:2048",
            "-nodes",
            "-days",
            "365",
            "-subj",
            "/CN=127.0.0.1",
            "-addext",
            "subjectAltName=IP:127.0.0.1",
            "-keyout",
            str(tls / TLS_KEY_NAME),
            "-out",
            str(tls / TLS_CERT_NAME),
        ],
        check=True,
        capture_output=True,
    )
    (tls / TLS_KEY_NAME).chmod(0o644)


def make_bucket() -> None:
    """Create the state bucket through the S3 API.

    Through the API, because that is the interface. The first version made a
    directory in the server's data root, which worked against MinIO because
    MinIO lays a bucket out that way — and stopped meaning anything the moment
    the server changed, since SeaweedFS keeps buckets in its filer. A fixture
    that reaches behind an interface is a fixture that passes for the wrong
    reason.
    """
    client = boto3.client(
        "s3",
        endpoint_url=S3_URL,
        aws_access_key_id=S3_ACCESS_KEY,
        aws_secret_access_key=S3_SECRET_KEY,
        region_name="us-east-1",
        config=Config(s3={"addressing_style": "path"}, signature_version="s3v4"),
    )
    try:
        client.create_bucket(Bucket=BUCKET)
    except ClientError as error:
        # Already there is the desired end state, not a failure.
        code = error.response.get("Error", {}).get("Code", "")
        if code not in ("BucketAlreadyOwnedByYou", "BucketAlreadyExists"):
            raise
    client.head_bucket(Bucket=BUCKET)


def forgejo_admin() -> None:
    """Create the administrator, if the fixture does not already have one."""
    subprocess.run(
        [
            "podman",
            "exec",
            FORGEJO_CONTAINER,
            "forgejo",
            "admin",
            "user",
            "create",
            "--admin",
            "--username",
            FORGEJO_USER,
            "--password",
            FORGEJO_PASSWORD,
            "--email",
            "e2e@example.com",
            "--must-change-password=false",
        ],
        check=False,
        capture_output=True,
    )


def forgejo_api(
    method: str, path: str, payload: dict | None = None, *, token: str | None = None
) -> dict:
    """Call the Forgejo API with either basic auth or a token."""
    body = json.dumps(payload).encode() if payload is not None else None
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"token {token}"
    else:
        basic = base64.b64encode(f"{FORGEJO_USER}:{FORGEJO_PASSWORD}".encode()).decode()
        headers["Authorization"] = f"Basic {basic}"

    request = urllib.request.Request(  # noqa: S310
        f"{FORGEJO_URL}/api/v1{path}", data=body, method=method, headers=headers
    )
    try:
        with urllib.request.urlopen(  # noqa: S310  # nosemgrep
            request, timeout=20, context=_tls_context()
        ) as response:
            raw = response.read()
            return json.loads(raw) if raw else {}
    except urllib.error.HTTPError as error:
        # 409 and 422 mean "already there", which is the desired end state.
        if error.code in (409, 422):
            return {}
        raise


def forgejo_token() -> str:
    """Issue an access token, replacing any left by an earlier run.

    A token is shown once. Reusing the fixture without recreating it would
    leave the driver holding a name it cannot exchange for a secret.
    """
    name = "e2e-fixture"
    for existing in forgejo_api("GET", f"/users/{FORGEJO_USER}/tokens") or []:
        if existing.get("name") == name:
            forgejo_api("DELETE", f"/users/{FORGEJO_USER}/tokens/{existing['id']}")
    created = forgejo_api(
        "POST",
        f"/users/{FORGEJO_USER}/tokens",
        {"name": name, "scopes": ["write:repository", "write:user"]},
    )
    return created["sha1"]


def make_module_repo(token: str) -> None:
    """Create the repository the module is pushed into."""
    forgejo_api(
        "POST",
        "/user/repos",
        # Private, and that is the whole point. A public repository is fetched
        # without a credential, so the token in the source URL would be
        # decoration and the "wrong token" scenario would pass while proving
        # nothing. This was public first, and the scenario caught it.
        {"name": FORGEJO_REPO, "private": True, "auto_init": False},
        token=token,
    )


def seed_module_repo(runner: Runner, token: str) -> None:
    """Publish the module to the Forgejo repository, from the dev container.

    Pushed to a remote that asks who is calling, rather than bind-mounted:
    that is the shape every real module source has, and the one an anonymous
    daemon could not test.

    The credential goes into git's credential store rather than into the URL.
    Terragrunt logs a module source verbatim, so a token embedded in the URL
    is a token in every log line that mentions the module — and in the process
    list of every git it spawns.
    """
    credentials = f"https://{FORGEJO_USER}:{token}@127.0.0.1:19300"
    remote = f"https://127.0.0.1:19300/{FORGEJO_USER}/{FORGEJO_REPO}.git"

    script = (
        "set -eu; "
        # A generated directory, not a fixed name. A predictable path in a
        # shared temporary directory is a name something else can claim first.
        'work="$(mktemp -d)"; trap \'rm -rf "$work"\' EXIT; cd "$work"; '
        "git config --global credential.helper store; "
        # Trust this certificate for this host, and nothing else. `sslVerify
        # false` would also work and would teach the reader the wrong lesson.
        'git config --global http."https://127.0.0.1:19300/".sslCAInfo '
        f"{MIRROR.rsplit('/', 1)[0]}/{TLS_DIR_NAME}/{TLS_CERT_NAME}; "
        f"umask 077; printf '%s\\n' '{credentials}' > ~/.git-credentials; "
        "git init -q -b main; "
        "git config user.email e2e@example.com; "
        "git config user.name 'e2e fixture'; "
        f"cp -r {E2E_DIR}/modules .; "
        "git add -A; "
        "git commit -q -m 'module under test'; "
        f"git push -q --force {remote} main"
    )
    runner.exec(script, check=True)


def build_provider_mirror(runner: Runner) -> None:
    """Build the provider into a filesystem mirror laid out like the registry.

    A dev_overrides block would be less work and would skip `terraform init`
    for providers, which is the step that resolves and locks a version. The
    mirror exercises the path a user actually takes, and incidentally proves
    the release layout is installable.
    """
    # Rebuilding changes the binary's checksum, and a lock file recorded from
    # the previous build then refuses the new one — "doesn't match any of the
    # checksums previously recorded". Whatever this step invalidates, this step
    # removes: every unit, not the two that existed when this was written.
    runner.exec(
        "set -eu; "
        f"rm -rf {E2E_DIR}/live*/.terraform.lock.hcl "
        f"{E2E_DIR}/live*/.terragrunt-cache "
        "/root/.cache/terragrunt "
        "/root/.terraform.d/plugin-cache/registry.terraform.io/ioplane",
        check=False,
    )

    # The platform the container actually is. Hard-coding linux_amd64 puts the
    # binary in a directory the engine will not look in on an arm64 host, and
    # the failure reads as "provider unavailable" rather than as a wrong path.
    # Containerfile.dev supports both architectures on purpose.
    _, platform = runner.exec(
        'printf "%s_%s" "$(go env GOOS)" "$(go env GOARCH)"',
        check=True,
    )
    plat = platform.strip().splitlines()[-1].strip()

    dest = f"{MIRROR}/registry.terraform.io/ioplane/powerdns/{PROVIDER_VERSION}/{plat}"
    script = (
        "set -eu; "
        f"rm -rf {MIRROR} && mkdir -p {dest} && "
        f"cd {REPO_ROOT} && go build -o {dest}/{BINARY_PREFIX}{PROVIDER_VERSION} . && "
        f"cat > {MIRROR}/terraform.rc <<'RC'\n"
        "provider_installation {\n"
        f'  filesystem_mirror {{\n    path    = "{MIRROR}"\n'
        '    include = ["registry.terraform.io/ioplane/*"]\n  }\n'
        '  direct {\n    exclude = ["registry.terraform.io/ioplane/*"]\n  }\n'
        "}\n"
        "RC"
    )
    runner.exec(script, check=True)


# --- lifecycle -----------------------------------------------------------


def cmd_up() -> int:
    """Bring the fixture up and prepare everything the tests assume exists."""
    if not http_ok(f"{AUTH_API}/zones", api_key=API_KEY):
        print("the lab is not running — run: task lab:up", file=sys.stderr)
        return 2

    make_tls_certificate()
    print("ok    TLS certificate for the module remote")

    compose("up", "-d", "--build")

    if not wait_for_s3():
        print("the S3 gateway did not answer within 90 seconds", file=sys.stderr)
        return 1
    print("ok    S3 gateway answering")

    if not wait_for_forgejo():
        print("Forgejo did not answer within 180 seconds", file=sys.stderr)
        return 1
    print("ok    Forgejo answering")

    runner = Runner(container=dev_container())
    make_bucket()
    print(f"ok    bucket {BUCKET}")

    forgejo_admin()
    token = forgejo_token()
    print(f"ok    Forgejo user {FORGEJO_USER} with an access token")

    make_module_repo(token)
    print(f"ok    repository {FORGEJO_USER}/{FORGEJO_REPO}")

    seed_module_repo(runner, token)
    print("ok    module pushed over HTTPS")
    print("ok    credential store configured for the module remote")

    # The token is still written out, but for the tests rather than for the
    # configuration: one scenario deliberately fetches with a wrong one, and
    # needs a right one to restore afterwards.
    token_file = E2E_DIR / ".token"
    token_file.write_text(token)
    token_file.chmod(0o600)
    print(f"ok    token written to {token_file.name}")

    build_provider_mirror(runner)
    print(f"ok    provider {PROVIDER_VERSION} in a filesystem mirror")
    return 0


MANAGED_ZONES = (
    ("gpgsql", "e2e.example."),
    ("gpgsql", "100.51.198.in-addr.arpa."),
    ("gpgsql", "engines.e2e.example."),
    ("gpgsql", "113.0.203.in-addr.arpa."),
    ("gpgsql", "signed.e2e.example."),
    ("gpgsql", "imperative.e2e.example."),
    ("gpgsql", "viewed-gpgsql.e2e.example."),
    ("gpgsql", "imported.e2e.example."),
    ("lmdb", "viewed.e2e.example."),
    ("recursor", "internal.e2e.example."),
)

ZONE_APIS = {
    "gpgsql": AUTH_API,
    "lmdb": "http://127.0.0.1:18091/api/v1/servers/localhost",
    "recursor": "http://127.0.0.1:18082/api/v1/servers/localhost",
}


def drop_managed_zones() -> None:
    """Remove what the units created, before the state describing it is gone.

    The lab outlives this fixture. Deleting MinIO takes the state with it and
    leaves the zones behind, so the next `up` starts with empty state, applies,
    and meets a 409 on a zone it believes it is creating — a failure three
    commands away from its cause.

    Deleting by name rather than running `terragrunt destroy`: destroy needs
    the module, the mirror and a reachable remote, and `down` has to work when
    the reason for running it is that one of those is broken.
    """
    for api, zone in MANAGED_ZONES:
        request = urllib.request.Request(  # noqa: S310
            f"{ZONE_APIS[api]}/zones/{zone}",
            method="DELETE",
            headers={"X-API-Key": API_KEY},
        )
        with contextlib.suppress(OSError):
            urllib.request.urlopen(request, timeout=10).close()  # noqa: S310  # nosemgrep


def cmd_down() -> int:
    """Remove the fixture, including its volumes and what the units created."""
    drop_managed_zones()
    print("ok    zones created by the units removed from the lab")
    compose("down", "-v")
    return 0


def cmd_status() -> int:
    """Report what is running."""
    for name, state in container_states().items():
        print(f"{state:<10} {name}")
    print(f"{'up' if s3_answering() else 'down':<10} S3 gateway")
    return 0


def main() -> int:
    """Dispatch a subcommand."""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("up", "down", "status"))
    args = parser.parse_args()
    return {"up": cmd_up, "down": cmd_down, "status": cmd_status}[args.command]()


if __name__ == "__main__":
    raise SystemExit(main())
