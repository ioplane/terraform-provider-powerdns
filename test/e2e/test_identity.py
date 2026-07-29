"""Adopting a record by its identity rather than by a parsed string.

The provider declares an identity schema for nine resources, and the contract
table has been checked against those declarations in both directions. Neither of
those facts is a consumer using one. `terraform import <address> <id>` had a
scenario; the `import { identity = … }` block — the form a consumer commits to a
repository rather than types once — had none, so the attribute names in those
schemas had never been read by anything but a test comparing them to a table.

The zone is created and removed here, not by the module. A record adopted from a
zone Terraform also created would be a record that existed because Terraform
made it, which is the opposite of the case being tested.
"""

from __future__ import annotations

import contextlib
import json
import urllib.request

import pytest

ZONE = "adopted.e2e.example."
RECORD = f"adopted.{ZONE}"
API = "http://127.0.0.1:18081/api/v1/servers/localhost/zones"
ADDRESSES = ["198.51.100.30", "198.51.100.31"]
TTL = 1800


def api_call(method: str, url: str, body: bytes | None = None) -> None:
    """Reach the API directly, and fail if it refuses.

    Unlike the equivalent in test_lifecycle.py this does not suppress errors.
    There, the calls are teardown around an assertion; here they *are* the
    premise — a record that was never created is a record the import block
    cannot adopt, and swallowing the failure would turn that into a confusing
    plan diff several assertions later.
    """
    request = urllib.request.Request(  # noqa: S310
        url,
        data=body,
        method=method,
        headers={"X-API-Key": "labapikey", "Content-Type": "application/json"},
    )
    urllib.request.urlopen(request, timeout=10).close()  # noqa: S310  # nosemgrep


@pytest.fixture(scope="module")
def out_of_band():
    """A zone and a record the provider has never heard of.

    Module-scoped: every scenario in this file reads the same adopted record,
    and creating it per test would mean adopting it four times.
    """
    # A leftover from an interrupted run would make the POST a 409.
    with contextlib.suppress(OSError):
        urllib.request.urlopen(  # noqa: S310  # nosemgrep
            urllib.request.Request(  # noqa: S310
                f"{API}/{ZONE}",
                method="DELETE",
                headers={"X-API-Key": "labapikey"},
            ),
            timeout=10,
        ).close()

    api_call("POST", API, json.dumps({"name": ZONE, "kind": "Native"}).encode())
    api_call(
        "PATCH",
        f"{API}/{ZONE}",
        json.dumps(
            {
                "rrsets": [
                    {
                        "name": RECORD,
                        "type": "A",
                        "ttl": TTL,
                        "changetype": "REPLACE",
                        "records": [
                            {"content": address, "disabled": False}
                            for address in ADDRESSES
                        ],
                    }
                ]
            }
        ).encode(),
    )
    yield
    with contextlib.suppress(OSError):
        api_call("DELETE", f"{API}/{ZONE}")


class TestAdoptionByIdentity:
    """The import block, planned and applied."""

    def test_the_plan_imports_rather_than_creates(
        self, terragrunt_identity, out_of_band
    ):
        """A create here would mean the identity found nothing and Terraform gave up.

        Which is the failure worth naming: the record already exists, so a plan
        that creates it would collide with the server on apply, and the
        diagnostic would be about a conflict rather than about the identity.
        """
        terragrunt_identity.run("init")
        result = terragrunt_identity.run("plan")
        combined = result.stdout + result.stderr
        assert "will be imported" in combined, combined[-3000:]
        assert "will be created" not in combined, combined[-3000:]

    def test_it_applies_and_the_record_is_managed(
        self, terragrunt_identity, out_of_band
    ):
        """The state now describes a record nobody created through Terraform."""
        terragrunt_identity.run("apply", "-auto-approve")
        pulled = json.loads(terragrunt_identity.run("state", "pull").stdout)
        records = [
            resource
            for resource in pulled.get("resources", [])
            if resource.get("type") == "powerdns_record"
        ]
        assert len(records) == 1
        attributes = records[0]["instances"][0]["attributes"]
        assert attributes["name"] == RECORD
        assert sorted(attributes["values"]) == sorted(ADDRESSES)
        assert attributes["ttl"] == TTL

    def test_the_next_plan_is_empty(self, terragrunt_identity, out_of_band):
        """Adoption that leaves a diff behind has adopted the wrong thing.

        This is where a mismatch between what the identity resolves to and what
        the resource then reads would surface — as a plan proposing to change
        the record it just adopted.
        """
        result = terragrunt_identity.run(
            "plan", "-detailed-exitcode", expect_success=False
        )
        assert result.returncode == 0, (result.stdout + result.stderr)[-3000:]

    def test_the_record_still_answers(
        self, terragrunt_identity, out_of_band, dns_query
    ):
        """Adoption is a state operation; the server should be untouched by it."""
        rcode, values = dns_query(RECORD, "A")
        assert rcode == "NOERROR"
        assert sorted(values) == sorted(ADDRESSES)

    def test_the_identity_is_readable_from_state(
        self, terragrunt_identity, out_of_band
    ):
        """The three attributes the import block named, as the engine stored them.

        Asserting the identity rather than only the id: the id is a delimited
        string a consumer no longer has to construct, and the identity is what
        replaced it.
        """
        pulled = json.loads(terragrunt_identity.run("state", "pull").stdout)
        instance = next(
            resource["instances"][0]
            for resource in pulled["resources"]
            if resource.get("type") == "powerdns_record"
        )
        identity = instance.get("identity_schema_version"), instance.get("identity")
        assert identity[1] is not None, (
            "the state records no identity for an imported-by-identity resource"
        )
        assert identity[1] == {
            "zone_name": ZONE,
            "record_name": RECORD,
            "record_type": "A",
        }

    def test_removing_it_from_state_leaves_the_record_alone(
        self, terragrunt_identity, out_of_band, dns_query
    ):
        """Cleanup, asserted rather than assumed.

        `state rm` must not delete anything on the server — the next scenario in
        the suite, and the fixture's own teardown, both depend on that being
        true rather than on it being likely.
        """
        terragrunt_identity.run("state", "rm", "powerdns_record.adopted")
        rcode, values = dns_query(RECORD, "A")
        assert rcode == "NOERROR"
        assert sorted(values) == sorted(ADDRESSES)
