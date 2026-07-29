"""The two products and the one backend the main path never reaches.

Three products is why this is one provider rather than three, and until now
every end-to-end path spoke only to the Authoritative server on gpgsql. That
left the Recursor, dnsdist, and — because gpgsql implements neither — views and
networks entirely outside the consumer's path.
"""

from __future__ import annotations

import contextlib
import json
import urllib.error
import urllib.request

import pytest

RECURSOR_API = "http://127.0.0.1:18082/api/v1/servers/localhost"
DNSDIST_API = "http://127.0.0.1:18083/api/v1/servers/localhost"
LMDB_API = "http://127.0.0.1:18091/api/v1/servers/localhost"
GPGSQL_API = "http://127.0.0.1:18081/api/v1/servers/localhost"
API_KEY = "labapikey"

FORWARD_ZONE = "internal.e2e.example."
VIEWED_ZONE = "viewed.e2e.example."
VIEW = "internal"
NETWORK = "10.20.0.0/16"

pytestmark = pytest.mark.timeout(600)


def api(url: str) -> object:
    """Read an endpoint, so an assertion can be made about the server itself."""
    request = urllib.request.Request(url, headers={"X-API-Key": API_KEY})  # noqa: S310
    with urllib.request.urlopen(request, timeout=10) as response:  # noqa: S310  # nosemgrep
        return json.loads(response.read())


def delete(url: str) -> None:
    """Remove an object, tolerating its absence."""
    request = urllib.request.Request(  # noqa: S310
        url, method="DELETE", headers={"X-API-Key": API_KEY}
    )
    with contextlib.suppress(OSError):
        urllib.request.urlopen(request, timeout=10).close()  # noqa: S310  # nosemgrep


@pytest.fixture(scope="module", autouse=True)
def applied(terragrunt_family, terragrunt_views):
    """Both units up for the module, and down after.

    Server-side leftovers are cleared first. A run whose teardown was
    interrupted leaves the object on the server and nothing in state, and the
    next apply then fails with "already exists" — a message about the fixture
    wearing the costume of a provider defect.
    """
    delete(f"{RECURSOR_API}/zones/{FORWARD_ZONE}")
    delete(f"{LMDB_API}/zones/{VIEWED_ZONE}")
    delete(f"{GPGSQL_API}/zones/viewed-gpgsql.e2e.example.")

    terragrunt_family.run("apply", "-auto-approve")
    terragrunt_views.run("apply", "-auto-approve")
    yield
    terragrunt_views.run("destroy", "-auto-approve", expect_success=False)
    terragrunt_family.run("destroy", "-auto-approve", expect_success=False)


class TestRecursor:
    """The Recursor, which writes two settings and its zones and nothing else."""

    def test_the_forward_zone_exists_on_the_server(self):
        """Read from the Recursor, not from Terraform's account of it."""
        zones = api(f"{RECURSOR_API}/zones")
        names = {z["name"] for z in zones}
        assert FORWARD_ZONE in names or FORWARD_ZONE.rstrip(".") in names, names

    def test_the_acl_reached_the_setting(self):
        """allow-from is one of exactly two settings the Recursor will write."""
        setting = api(f"{RECURSOR_API}/config/allow-from")
        value = setting.get("value")
        assert value, setting
        # Netmasks are compared as subnets, so the server may return them
        # normalised. Membership, not equality.
        joined = " ".join(value) if isinstance(value, list) else str(value)
        assert "10.0.0.0/8" in joined, joined


class TestDNSDist:
    """dnsdist, whose API writes exactly two things."""

    def test_the_acl_reached_the_setting(self):
        """The one ACL dnsdist exposes over HTTP.

        Written as `allow-from` and reported back as `acl`, in a
        comma-separated string rather than a list. The provider writes to the
        name dnsdist accepts and reads from the name dnsdist reports, which is
        the sort of asymmetry that is only ever found by asking the server.
        """
        config = api(f"{DNSDIST_API}/config")
        entries = {c["name"]: c["value"] for c in config}
        assert "acl" in entries, sorted(entries)
        assert "192.168.0.0/16" in str(entries["acl"]), entries["acl"]

    def test_the_data_source_read_a_real_server(self, terragrunt_family):
        """The version in the output came from dnsdist, not from a default."""
        result = terragrunt_family.run("output", "-json")
        outputs = json.loads(result.stdout)
        assert outputs["dnsdist_version"]["value"] == "2.1.0"


class TestViewsAndNetworks:
    """Only LMDB implements these, which is why the lab runs two backends."""

    def test_the_view_holds_the_zone(self):
        """The membership reached the LMDB server."""
        views = api(f"{LMDB_API}/views")
        names = views.get("views", views) if isinstance(views, dict) else views
        assert VIEW in names, names

    def test_the_network_maps_to_the_view(self):
        """A client prefix resolves to the view it was mapped to."""
        networks = api(f"{LMDB_API}/networks")
        entries = networks.get("networks", networks)
        mapped = {n["network"]: n.get("view") for n in entries}
        assert mapped.get(NETWORK) == VIEW, mapped

    def test_gpgsql_reads_views_but_cannot_write_them(self):
        """The asymmetry the provider's diagnostic describes.

        Not "unimplemented": the read endpoint exists and answers an empty
        list, and only the write fails. A test asserting a 404 on the read —
        which this one did first — fails against a server that is behaving
        exactly as documented.
        """
        views = api(f"{GPGSQL_API}/views")
        assert views.get("views") == [], views

    def test_writing_a_view_to_gpgsql_names_the_backend(self, terragrunt_views_gpgsql):
        """The capability diagnostic, which is contract §4.6.

        A bare 422 tells a user their configuration is wrong. This has to tell
        them their *backend* is wrong, and name the setting to change — that
        is the whole reason the transport classifies capability rather than
        passing the status through.
        """
        result = terragrunt_views_gpgsql.run(
            "apply", "-auto-approve", expect_success=False
        )
        combined = result.stdout + result.stderr
        assert result.returncode != 0
        assert "LMDB" in combined, combined[-1500:]
        assert "launch=" in combined, combined[-1500:]
