"""Actions, ephemeral resources and an autoprimary.

The three parts of the surface that exist because a resource could not express
them: an operation with no object to own, a value that must not be stored, and
a server-side list nothing else manages.

This unit runs on Terraform rather than OpenTofu. Actions are a Terraform 1.14
feature and the other engine does not have them, so the difference is in the
fixture rather than hidden by it.
"""

from __future__ import annotations

import contextlib
import json
import urllib.request

import pytest

AUTH_API = "http://127.0.0.1:18081/api/v1/servers/localhost"
API_KEY = "labapikey"
ZONE = "imperative.e2e.example."
HOST = "host.imperative.e2e.example."
TSIG_NAME = "ephemeral-key"
STATE_KEY = "dns-imperative/terraform.tfstate"
BUCKET = "e2e-state"

pytestmark = pytest.mark.timeout(900)


def api(method: str, path: str, body: bytes | None = None) -> object:
    """Reach the API directly, for setup and for reading back."""
    request = urllib.request.Request(  # noqa: S310
        f"{AUTH_API}{path}",
        data=body,
        method=method,
        headers={"X-API-Key": API_KEY, "Content-Type": "application/json"},
    )
    with urllib.request.urlopen(request, timeout=10) as response:  # noqa: S310  # nosemgrep
        raw = response.read()
        return json.loads(raw) if raw else {}


@pytest.fixture(scope="module", autouse=True)
def applied(terragrunt_imperative):
    """The unit, with the TSIG key it reads created outside it.

    Outside on purpose. An ephemeral resource is opened while the graph is
    walked, before the resources in the same apply exist, so it can only read
    something already there — `depends_on` does not defer it, and a module
    that tried met a 404.
    """
    with contextlib.suppress(OSError):
        api("DELETE", f"/zones/{ZONE}")
    with contextlib.suppress(OSError):
        api("DELETE", f"/tsigkeys/{TSIG_NAME}.")
    with contextlib.suppress(OSError):
        api(
            "POST",
            "/tsigkeys",
            json.dumps({"name": TSIG_NAME, "algorithm": "hmac-sha256"}).encode(),
        )

    terragrunt_imperative.run("apply", "-auto-approve")
    yield
    terragrunt_imperative.run("destroy", "-auto-approve", expect_success=False)
    with contextlib.suppress(OSError):
        api("DELETE", f"/tsigkeys/{TSIG_NAME}.")


class TestActions:
    """Operations with no object to own, attached to a resource's lifecycle."""

    def test_both_actions_were_invoked(self, terragrunt_imperative):
        """Terraform reports what it ran, and it ran two.

        An action that silently does nothing is indistinguishable from one
        that worked, so the count is asserted rather than the absence of an
        error.
        """
        result = terragrunt_imperative.run("apply", "-auto-approve")
        combined = result.stdout + result.stderr
        # A second apply changes nothing, so no action fires. The first apply
        # is in the fixture; this asserts the steady state does not re-trigger.
        assert "Apply complete" in combined
        assert "0 added, 0 changed, 0 destroyed" in combined, combined[-1500:]

    def test_actions_only_exist_on_terraform(self, terragrunt_imperative):
        """OpenTofu refuses the configuration outright.

        Documented upstream, and worth pinning: if a future OpenTofu gains
        actions this fails, and the fixture's engine pin can be reconsidered
        rather than silently kept.
        """
        result = terragrunt_imperative.run(
            "plan",
            expect_success=False,
            env_overrides={"TG_TF_PATH": "/usr/local/bin/tofu"},
        )
        combined = (result.stdout + result.stderr).lower()
        assert result.returncode != 0
        assert "action" in combined, combined[-1200:]


class TestEphemeral:
    """A value that is read and must not be kept."""

    def test_the_secret_is_not_in_state(self, s3):
        """Nothing from the ephemeral read reaches the state file."""
        raw = s3.get_object(Bucket=BUCKET, Key=STATE_KEY)["Body"].read().decode()
        state = json.loads(raw)
        types = {r["type"] for r in state["resources"]}
        assert "powerdns_tsigkey_secret" not in types, types
        # And the key's own name is the only trace of it anywhere.
        assert "hmac-sha256" not in raw or "powerdns_tsigkey" not in types

    def test_the_check_block_confirmed_a_real_secret(self, terragrunt_imperative):
        """The module's check block asserts the secret was non-empty.

        A check block runs during apply and cannot persist what it read, which
        is the only way to establish the read returned something without
        storing it.
        """
        result = terragrunt_imperative.run("apply", "-auto-approve")
        combined = result.stdout + result.stderr
        assert "the ephemeral read returned an empty secret" not in combined


class TestAutoprimary:
    """A server-side list with no other manager."""

    def test_the_autoprimary_is_on_the_server(self):
        """Read from PowerDNS, not from state."""
        entries = api("GET", "/autoprimaries")
        addresses = {e["ip"] for e in entries}
        assert "192.0.2.200" in addresses, addresses


class TestActionsReachedTheServer:
    """What the actions did, seen from the zone itself."""

    def test_the_zone_is_a_master(self):
        """`notify` and `rectify` only mean anything for a zone like this."""
        zone = api("GET", f"/zones/{ZONE}")
        assert zone["kind"] == "Master", zone["kind"]

    def test_the_record_the_actions_fired_for_exists(self, dns_query):
        """And the record whose creation triggered notify resolves."""
        rcode, values = dns_query(HOST, "A")
        assert rcode == "NOERROR"
        assert values == {"192.0.2.50"}, values
