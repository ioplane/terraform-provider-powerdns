"""What must never be in the state file, checked against the state file.

This is the provider's central claim: a DNSSEC private key and a TSIG secret do
not reach Terraform state. Until now it was checked against a local state file
in an acceptance test — which is the right check in the wrong place, because
the risk the claim exists to answer is a state file in a bucket a team shares.

So this reads the object out of S3 and looks.
"""

from __future__ import annotations

import json
import re

import pytest

ZONE = "signed.e2e.example."
PROBE = "probe.signed.e2e.example."
STATE_KEY = "dns-secrets/terraform.tfstate"
BUCKET = "e2e-state"

# The literal from live-secrets/terragrunt.hcl. If this ever appears in state,
# the write-only attribute is not write-only.
CONFIGURED_TSIG_SECRET = "aGVsbG8tdGhpcy1pcy1hLXRlc3Qtc2VjcmV0LXZhbHVl"  # noqa: S105

pytestmark = pytest.mark.timeout(600)


@pytest.fixture(scope="module", autouse=True)
def applied(terragrunt_secrets):
    """The unit is applied once for the module, and torn down after.

    Leftovers are cleared first: an interrupted teardown leaves the zone on
    the server with nothing in state, and the next apply fails with "already
    exists" — a fixture problem that reads as a provider one.
    """
    import contextlib
    import urllib.request

    for url in (
        "http://127.0.0.1:18081/api/v1/servers/localhost/zones/signed.e2e.example.",
        "http://127.0.0.1:18081/api/v1/servers/localhost/tsigkeys/transfer-key.",
    ):
        request = urllib.request.Request(  # noqa: S310
            url, method="DELETE", headers={"X-API-Key": "labapikey"}
        )
        with contextlib.suppress(OSError):
            urllib.request.urlopen(request, timeout=10).close()  # noqa: S310  # nosemgrep

    terragrunt_secrets.run("apply", "-auto-approve")
    yield
    terragrunt_secrets.run("destroy", "-auto-approve", expect_success=False)


@pytest.fixture(scope="module")
def state(s3) -> str:
    """The state file, as it sits in the bucket."""
    return s3.get_object(Bucket=BUCKET, Key=STATE_KEY)["Body"].read().decode()


class TestNothingSecretInState:
    """Four ways of asking the same question, because one of them might miss."""

    def test_no_private_key_material(self, state: str):
        """No PEM block, by any of the headers a private key is written under."""
        assert not re.search(r"BEGIN [A-Z ]*PRIVATE KEY", state)

    def test_the_cryptokey_has_no_private_attribute(self, state: str):
        """The resource keeps the public halves and not the private one.

        `dnskey` and `ds` are public by definition — they are served in DNS.
        `private_key` is the one that would matter, and the resource reconciles
        against the collection endpoint, which does not return it.
        """
        resources = {r["type"]: r for r in json.loads(state)["resources"]}
        attributes = resources["powerdns_zone_cryptokey"]["instances"][0]["attributes"]
        assert "dnskey" in attributes
        assert "private_key" not in attributes

    def test_the_tsig_secret_is_absent(self, state: str):
        """The value handed to the provider is not in the file it wrote."""
        assert CONFIGURED_TSIG_SECRET not in state

    def test_the_write_only_attribute_holds_nothing(self, state: str):
        """`secret_wo` is present as a name and null as a value.

        Present because the schema declares it; null because Terraform sends a
        write-only attribute to the provider and stores it nowhere. A schema
        that dropped the attribute entirely would also pass the test above,
        and would be a different provider.
        """
        resources = {r["type"]: r for r in json.loads(state)["resources"]}
        attributes = resources["powerdns_tsigkey"]["instances"][0]["attributes"]
        assert "secret_wo" in attributes
        assert not attributes["secret_wo"]


class TestTheZoneIsActuallySigned:
    """DNSSEC, established from DNS rather than from the API's account of it."""

    def test_the_zone_serves_a_dnskey(self, dns_query):
        """A key exists in the zone, not merely in a response body."""
        rcode, values = dns_query(ZONE, "DNSKEY")
        assert rcode == "NOERROR"
        assert values, "the zone serves no DNSKEY"

    def test_records_carry_a_signature(self, dns_query_dnssec):
        """An RRSIG accompanies the answer.

        This is the assertion that distinguishes "the provider created a key"
        from "the zone is signed". The first is an API call; the second is
        what a resolver validates against.
        """
        rrsigs = dns_query_dnssec(PROBE, "TXT")
        assert rrsigs, "no RRSIG in the answer section"

    def test_the_soa_serial_function_produced_the_record(self, dns_query):
        """`soa_serial` built the value, in the YYYYMMDDnn form it promises."""
        _, values = dns_query(PROBE, "TXT")
        assert any(re.fullmatch(r"\d{8}\d{2}", v) for v in values), values
