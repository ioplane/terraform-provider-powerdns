"""Someone changed it outside Terraform. What happens next.

Drift is the case a provider's Read exists for, and the one that separates a
provider which reconciles from one which only creates. Nothing in the
end-to-end path had ever changed the server behind Terraform's back.
"""

from __future__ import annotations

import json
import urllib.request

import pytest

AUTH_API = "http://127.0.0.1:18081/api/v1/servers/localhost"
API_KEY = "labapikey"
ZONE = "e2e.example."
FQDN = "www.e2e.example."

pytestmark = pytest.mark.timeout(600)


def patch_rrset(name: str, rrtype: str, ttl: int, values: list[str]) -> None:
    """Change a record on the server, with no Terraform involved."""
    body = json.dumps(
        {
            "rrsets": [
                {
                    "name": name,
                    "type": rrtype,
                    "ttl": ttl,
                    "changetype": "REPLACE",
                    "records": [{"content": v, "disabled": False} for v in values],
                }
            ]
        }
    ).encode()
    request = urllib.request.Request(  # noqa: S310
        f"{AUTH_API}/zones/{ZONE}",
        data=body,
        method="PATCH",
        headers={"X-API-Key": API_KEY, "Content-Type": "application/json"},
    )
    urllib.request.urlopen(request, timeout=10).close()  # noqa: S310  # nosemgrep


@pytest.fixture(scope="module", autouse=True)
def applied(terragrunt):
    """The main unit, up for this module and left as it was found."""
    terragrunt.run("apply", "-auto-approve")
    yield
    terragrunt.run("apply", "-auto-approve", expect_success=False)


class TestDrift:
    """A change made behind Terraform's back."""

    def test_a_changed_value_shows_in_the_plan(self, terragrunt):
        """The plan reports it. Anything else and Terraform has stopped reading.

        `-detailed-exitcode` returns 2 when changes are pending, which is a
        clearer assertion than searching output for a phrase that could be
        reworded upstream.
        """
        patch_rrset(FQDN, "A", 3600, ["198.51.100.99"])
        result = terragrunt.run("plan", "-detailed-exitcode", expect_success=False)
        assert result.returncode == 2, (result.stdout + result.stderr)[-2000:]

    def test_apply_puts_it_back(self, terragrunt, dns_query):
        """And the next apply restores what the configuration says.

        Checked in DNS rather than in state: state agreeing with the
        configuration proves only that Terraform wrote to itself.
        """
        terragrunt.run("apply", "-auto-approve")
        _, values = dns_query(FQDN, "A")
        assert values == {"198.51.100.10", "198.51.100.11"}, values

    def test_a_changed_ttl_shows_in_the_plan(self, terragrunt):
        """TTL is part of the RRSet, so a TTL change is drift too."""
        patch_rrset(FQDN, "A", 60, ["198.51.100.10", "198.51.100.11"])
        result = terragrunt.run("plan", "-detailed-exitcode", expect_success=False)
        assert result.returncode == 2, (result.stdout + result.stderr)[-2000:]

    def test_a_deleted_record_is_recreated(self, terragrunt, dns_query):
        """Deletion is drift in the other direction."""
        request = urllib.request.Request(  # noqa: S310
            f"{AUTH_API}/zones/{ZONE}",
            data=json.dumps(
                {"rrsets": [{"name": FQDN, "type": "A", "changetype": "DELETE"}]}
            ).encode(),
            method="PATCH",
            headers={"X-API-Key": API_KEY, "Content-Type": "application/json"},
        )
        urllib.request.urlopen(request, timeout=10).close()  # noqa: S310  # nosemgrep

        rcode, values = dns_query(FQDN, "A")
        assert values == set(), f"the record survived deletion: {rcode} {values}"

        terragrunt.run("apply", "-auto-approve")
        _, restored = dns_query(FQDN, "A")
        assert restored == {"198.51.100.10", "198.51.100.11"}, restored
