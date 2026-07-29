"""Fixtures for the end-to-end suite.

Everything here answers a question through the interface a real consumer uses:
S3 through boto3 rather than by listing MinIO's data directory, DNS through
dnspython rather than by parsing `dig`, the database through psycopg rather
than through `psql -tAc`. Shelling out and scraping text is how a test comes to
assert the shape of an error message instead of the fact underneath it.

The one exception is Terragrunt itself, which has no library interface and is
driven as a process — the same way its users drive it.
"""

from __future__ import annotations

import os
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import TYPE_CHECKING

import boto3
import psycopg
import pytest
from botocore.client import Config
from tenacity import retry, retry_if_exception_type, stop_after_delay, wait_fixed

if TYPE_CHECKING:
    from collections.abc import Iterator

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "scripts" / "automation"))

import e2e as fixture

LIVE_DIR = "/app/test/e2e/live"
DNS_SERVER = "127.0.0.1"
DNS_PORT = 15300
PG_DSN = "host=127.0.0.1 port=15432 user=pdns password=pdns dbname=pdns"


AUTH_APIS = {
    "gpgsql": "http://127.0.0.1:18081/api/v1/servers/localhost",
    "lmdb": "http://127.0.0.1:18091/api/v1/servers/localhost",
    "recursor": "http://127.0.0.1:18082/api/v1/servers/localhost",
}


def reset_zones(*names: str, api: str = "gpgsql") -> None:
    """Remove zones left by an interrupted run.

    Every module fixture calls this before applying. A run whose teardown did
    not finish leaves the object on the server and nothing in state, and the
    next apply fails with "already exists" — a fixture problem wearing the
    costume of a provider defect, and the one that cost the most time here.
    """
    import contextlib
    import urllib.request

    for name in names:
        request = urllib.request.Request(  # noqa: S310
            f"{AUTH_APIS[api]}/zones/{name}",
            method="DELETE",
            headers={"X-API-Key": "labapikey"},
        )
        with contextlib.suppress(OSError):
            urllib.request.urlopen(request, timeout=10).close()  # noqa: S310  # nosemgrep


def _token() -> str:
    """The Forgejo token the fixture issued."""
    path = Path(fixture.E2E_DIR) / ".token"
    if not path.is_file():
        pytest.exit("no Forgejo token; run: task e2e:up", returncode=2)
    return path.read_text().strip()


@dataclass(frozen=True)
class Terragrunt:
    """Terragrunt, driven as a process because that is its only interface.

    The suite runs inside the dev container, where the toolchain already is,
    so this is a local subprocess rather than a `podman exec`. Driving podman
    from inside a container to reach the container it is already in was the
    first shape of this and it did not survive contact with `podman` not being
    installed there.
    """

    workdir: str = LIVE_DIR

    def run(
        self,
        *args: str,
        expect_success: bool = True,
        env_overrides: dict[str, str] | None = None,
    ) -> subprocess.CompletedProcess[str]:
        """Run terragrunt in the live directory and return the completed process."""
        completed = subprocess.run(
            ["terragrunt", *args, "-no-color"],
            cwd=self.workdir,
            capture_output=True,
            text=True,
            check=False,
            env={
                **os.environ,
                "TF_CLI_CONFIG_FILE": f"{fixture.MIRROR}/terraform.rc",
                "TF_IN_AUTOMATION": "1",
                "TG_NON_INTERACTIVE": "true",
                # The module source authenticates. The token comes from the
                # file the fixture wrote, so it is never in the configuration
                # and never in a shell history.
                "E2E_TOKEN": _token(),
                **(env_overrides or {}),
            },
        )
        if expect_success and completed.returncode != 0:
            pytest.fail(
                f"terragrunt {' '.join(args)} exited {completed.returncode}\n"
                f"{(completed.stdout + completed.stderr)[-3000:]}"
            )
        return completed


@pytest.fixture(scope="session")
def s3():
    """An S3 client against MinIO, configured the way the backend is.

    Path-style addressing and a fixed region, because MinIO is not AWS and
    there is no bucket-per-hostname or STS behind it.
    """
    return boto3.client(
        "s3",
        endpoint_url=fixture.MINIO_URL,
        aws_access_key_id="e2eaccesskey",
        aws_secret_access_key="e2esecretkey",  # noqa: S106
        region_name="us-east-1",
        config=Config(s3={"addressing_style": "path"}, signature_version="s3v4"),
    )


@pytest.fixture(scope="session")
def dns_query():
    """Ask the authoritative server directly and return the rcode with the answer.

    A resolver object hides the rcode: with the zone deleted the server is no
    longer authoritative and answers REFUSED, which dnspython raises as
    "all nameservers failed" — indistinguishable from a server that is down,
    and a destroy test that cannot tell those apart passes when nothing is
    listening.

    Queries go straight to the authoritative server; a recursor in the path
    would answer from cache and keep a failed apply looking healthy for as
    long as the TTL lasts.
    """
    import dns.message
    import dns.query
    import dns.rcode

    def ask(name: str, rrtype: str) -> tuple[str, set[str]]:
        response = dns.query.udp(
            dns.message.make_query(name, rrtype),
            DNS_SERVER,
            port=DNS_PORT,
            timeout=5,
        )
        values = {
            rdata.to_text().strip('"') for rrset in response.answer for rdata in rrset
        }
        return dns.rcode.to_text(response.rcode()), values

    return ask


@pytest.fixture(scope="session")
def db() -> Iterator[psycopg.Connection]:
    """A connection to the gpgsql backend.

    The API and DNS can both be satisfied by something other than storage.
    This is the only assertion that reaches what was actually persisted.
    """
    with psycopg.connect(PG_DSN) as connection:
        yield connection


@pytest.fixture(scope="session")
def terragrunt() -> Terragrunt:
    """The engine under test, pointed at the live configuration."""
    return Terragrunt()


@pytest.fixture(scope="session")
def terragrunt_import() -> Terragrunt:
    """A second unit with its own state, for the adoption scenario."""
    return Terragrunt(workdir=f"{LIVE_DIR}-import")


@pytest.fixture(scope="session")
def terragrunt_secrets() -> Terragrunt:
    """The unit holding a DNSSEC key and a TSIG secret."""
    return Terragrunt(workdir=f"{LIVE_DIR}-secrets")


@pytest.fixture(scope="session")
def terragrunt_family() -> Terragrunt:
    """The unit covering the Recursor and dnsdist."""
    return Terragrunt(workdir=f"{LIVE_DIR}-family")


@pytest.fixture(scope="session")
def terragrunt_views() -> Terragrunt:
    """The unit covering views and networks, which exist only on LMDB."""
    return Terragrunt(workdir=f"{LIVE_DIR}-views")


@pytest.fixture(scope="session")
def terragrunt_imperative() -> Terragrunt:
    """Actions, ephemeral and autoprimary — the unit pinned to Terraform."""
    return Terragrunt(workdir=f"{LIVE_DIR}-imperative")


@pytest.fixture(scope="session")
def terragrunt_engines() -> Terragrunt:
    """The unit used to compare engines, with state nothing else touches."""
    return Terragrunt(workdir=f"{LIVE_DIR}-engines")


@pytest.fixture(scope="session")
def terragrunt_views_gpgsql() -> Terragrunt:
    """The views unit pointed at gpgsql, to see what the provider says."""
    return Terragrunt(workdir=f"{LIVE_DIR}-views-gpgsql")


@pytest.fixture(scope="session")
def dns_query_dnssec():
    """Ask with DO set and return the signatures in the answer.

    Separate from `dns_query` because asking for signatures changes the
    question: a resolver that does not set DO gets an unsigned-looking answer
    from a signed zone, and a test using it would report a signing failure
    that is really a query failure.
    """
    import dns.flags
    import dns.message
    import dns.query
    import dns.rdatatype

    def ask(name: str, rrtype: str) -> list[str]:
        request = dns.message.make_query(name, rrtype, want_dnssec=True)
        response = dns.query.udp(request, DNS_SERVER, port=DNS_PORT, timeout=5)
        return [
            rdata.to_text()
            for rrset in response.answer
            if rrset.rdtype == dns.rdatatype.RRSIG
            for rdata in rrset
        ]

    return ask


@retry(
    stop=stop_after_delay(60),
    wait=wait_fixed(2),
    retry=retry_if_exception_type(Exception),
    reraise=True,
)
def _bucket_ready(client, bucket: str) -> None:
    client.head_bucket(Bucket=bucket)


@pytest.fixture(scope="session", autouse=True)
def fixture_is_up(s3) -> None:
    """Refuse to run against a fixture that is not there.

    A suite that quietly starts what it is meant to be testing hides the case
    where the thing never comes up at all.
    """
    try:
        _bucket_ready(s3, fixture.BUCKET)
    except Exception as error:  # noqa: BLE001
        pytest.exit(
            f"the end-to-end fixture is not ready ({error}). Run: task e2e:up",
            returncode=2,
        )

    if not Path(f"{fixture.MIRROR}/terraform.rc").is_file():
        pytest.exit("the provider mirror is missing. Run: task e2e:up", returncode=2)
    _token()
