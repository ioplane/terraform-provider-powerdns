"""The path a configuration travels, from Terragrunt to what DNS answers.

The acceptance suite establishes that the provider is correct against PowerDNS.
This establishes that a consumer's configuration reaches it — through remote
state, through a module fetched from a remote, through a lock — and that what
comes out the far end is a name that resolves and a row that exists.

The tests run in file order and share state deliberately: applying, re-planning
and destroying the same configuration is the sequence under test, not three
independent facts.
"""

from __future__ import annotations

import contextlib
import shutil
import subprocess
from pathlib import Path

import pytest
from conftest import GIT_CREDENTIALS, TERRAGRUNT_CACHE

ZONE = "e2e.example."
FQDN = "www.e2e.example."
PTR_NAME = "10.100.51.198.in-addr.arpa."
ADDRESSES = {"198.51.100.10", "198.51.100.11"}
STATE_KEY = "dns/terraform.tfstate"
BUCKET = "e2e-state"

pytestmark = pytest.mark.timeout(600)


def clear_module_cache(terragrunt) -> None:
    """Remove every copy of the fetched module, local and shared."""
    shutil.rmtree(Path(terragrunt.workdir) / ".terragrunt-cache", ignore_errors=True)
    shutil.rmtree(TERRAGRUNT_CACHE, ignore_errors=True)


def pdns_call(method: str, url: str, body: bytes | None = None) -> None:
    """Reach the API directly, to set up or clean up outside Terraform."""
    import urllib.request

    request = urllib.request.Request(  # noqa: S310
        url,
        data=body,
        method=method,
        headers={"X-API-Key": "labapikey", "Content-Type": "application/json"},
    )
    # Suppressed: this is setup and teardown around the assertion, not the
    # assertion. A zone that is already absent on DELETE, or already present
    # on POST, is a fine state for the test that follows to start from.
    with contextlib.suppress(OSError):
        urllib.request.urlopen(request, timeout=10).close()  # noqa: S310  # nosemgrep


def row_count(db, name: str) -> int:
    """How many rows the gpgsql backend holds for a name."""
    with db.cursor() as cursor:
        cursor.execute(
            "SELECT count(*) FROM records WHERE name = %s", (name.rstrip("."),)
        )
        return cursor.fetchone()[0]


class TestApply:
    """Bringing the configuration up."""

    def test_apply_fetches_the_module_over_an_authenticated_remote(self, terragrunt):
        """The module comes from a remote that asks who is calling.

        The cache is cleared first so the fetch happens in this run. Asserting
        on output without doing that passes or fails on whether an earlier run
        left the module behind, which is a fact about the working directory
        rather than about the configuration.
        """
        clear_module_cache(terragrunt)

        result = terragrunt.run("apply", "-auto-approve")
        combined = result.stdout + result.stderr
        assert "Apply complete" in combined
        assert "git::https://" in combined, combined[-2000:]
        assert "127.0.0.1:19300" in combined, combined[-2000:]

        # And the module is on disk where a fetch would have put it.
        cache = Path(terragrunt.workdir) / ".terragrunt-cache"
        assert list(cache.glob("*/*/modules/dns-zone/main.tf")), "module not cached"

    def test_the_second_plan_is_empty(self, terragrunt):
        """Idempotence. This is the property the normalisation layer exists for.

        `-detailed-exitcode`: 0 means no changes, 2 means changes pending.
        """
        result = terragrunt.run("plan", "-detailed-exitcode", expect_success=False)
        assert result.returncode == 0, (result.stdout + result.stderr)[-2000:]


class TestModuleSource:
    """Authenticating to the module source — the reason the remote is a forge.

    An anonymous `git://` daemon cannot fail this way, which is why it was
    replaced: the error people actually meet when wiring up a private module
    is a credential one, and nothing here could produce it.
    """

    def test_a_bad_credential_fails_and_says_so(self, terragrunt):
        """A wrong credential stops the run, and the message names the remote.

        The credential lives in git's store, so this corrupts the store rather
        than an environment variable — which is also why it restores it in a
        `finally`: every later test fetches the same module.

        Not merely "it fails": a module source that fails opaquely sends
        somebody looking at their provider configuration for an hour.
        """
        store = GIT_CREDENTIALS
        good = store.read_text()
        try:
            store.write_text(
                "https://e2e:0000000000000000000000000000000000000000@127.0.0.1:19300\n"
            )
            clear_module_cache(terragrunt)

            result = terragrunt.run("init", expect_success=False)
            combined = result.stdout + result.stderr
            assert result.returncode != 0, combined[-1500:]

            # The remote is named, which is the part a reader needs. The
            # wording around it is Terragrunt's and moves: a rejected
            # credential surfaced as "Authentication failed" when the fetch
            # went straight to git and as "could not read Username" once it
            # went through the central git store, for the same cause. Asserting
            # the phrase pinned the test to a sentence rather than to the fact.
            assert "127.0.0.1:19300" in combined, combined[-1500:]
            assert any(
                word in combined
                for word in ("uthenticat", "redential", "Username", "denied")
            ), combined[-1500:]
        finally:
            store.write_text(good)
            clear_module_cache(terragrunt)

        # And the right credential still works, so the failure was the
        # credential and not the fixture falling over.
        terragrunt.run("init")

    def test_the_certificate_is_actually_verified(self):
        """Trust is configured, not disabled.

        `sslVerify = false` would make the fixture work and would teach the
        wrong lesson. This overrides the URL-specific `sslCAInfo` — the
        generic `http.sslCAInfo` is less specific and git ignores it, which
        made the first version of this check pass against a broken premise.
        """
        result = subprocess.run(
            [
                "git",
                "-c",
                "http.https://127.0.0.1:19300/.sslCAInfo=/dev/null",
                "ls-remote",
                "https://127.0.0.1:19300/e2e/dns-modules.git",
            ],
            capture_output=True,
            text=True,
            check=False,
            cwd="/tmp",  # noqa: S108
        )
        combined = result.stdout + result.stderr
        assert result.returncode != 0, combined[-500:]

        # The refusal is about the certificate; the sentence describing it is
        # the TLS backend's. This host says "certificate signer not trusted"
        # and a hosted runner says "server certificate verification failed" —
        # same git, same command, different build. Matching the first phrase
        # passed here and failed in CI on the first run.
        assert "certificate" in combined.lower(), combined[-500:]

    def test_the_token_is_not_in_the_module_source(self, terragrunt):
        """The source URL is HTTPS and carries no credential.

        Terragrunt prints it verbatim, so a token in the URL is a token in
        every log line naming the module — and in the process list of every
        git it spawns. It was written that way first.
        """
        source = Path(terragrunt.workdir, "terragrunt.hcl").read_text()
        assert "@127.0.0.1:19300" not in source, source
        assert "git::https://" in source, source


class TestServerState:
    """What actually reached PowerDNS, asked three different ways."""

    def test_dns_answers_the_forward_records(self, dns_query):
        """DNS answers, which an HTTP 200 did not establish."""
        rcode, values = dns_query(FQDN, "A")
        assert rcode == "NOERROR"
        assert values == ADDRESSES

    def test_dns_answers_the_reverse_record(self, dns_query):
        """The PTR sits at the name the provider function computed."""
        rcode, values = dns_query(PTR_NAME, "PTR")
        assert rcode == "NOERROR"
        assert values == {FQDN}

    def test_the_rrset_holds_both_values(self, dns_query):
        """One RRSet, two values — not two resources fighting over one name."""
        _, values = dns_query(FQDN, "A")
        assert len(values) == 2

    def test_the_database_holds_the_rows(self, db):
        """The storage engine, past both the API and any cache in front of it."""
        assert row_count(db, FQDN) >= 2


class TestRemoteState:
    """Where the state went, and what happens when it is held."""

    def test_state_is_in_the_bucket(self, s3):
        """State is in the object store, not on disk beside the configuration."""
        head = s3.head_object(Bucket=BUCKET, Key=STATE_KEY)
        assert head["ContentLength"] > 0

    def test_no_lock_is_left_behind(self, s3):
        """A finished run releases its lock.

        A stale lock object is indistinguishable from a run in progress, and
        the next person to touch this state waits for a process that ended.
        """
        with pytest.raises(s3.exceptions.ClientError):
            s3.head_object(Bucket=BUCKET, Key=f"{STATE_KEY}.tflock")

    def test_a_held_lock_blocks_a_second_run(self, s3, terragrunt):
        """The lock is planted rather than raced.

        Racing two applies tests the same mechanism and fails intermittently
        for reasons that have nothing to do with the provider.
        """
        s3.put_object(Bucket=BUCKET, Key=f"{STATE_KEY}.tflock", Body=b"{}")
        try:
            result = terragrunt.run("plan", "-lock-timeout=0s", expect_success=False)
            combined = (result.stdout + result.stderr).lower()
            assert result.returncode != 0
            assert "lock" in combined
        finally:
            s3.delete_object(Bucket=BUCKET, Key=f"{STATE_KEY}.tflock")


class TestChange:
    """Changing something, and what that costs."""

    def test_changing_a_ttl_does_not_replace_the_record(self, terragrunt):
        """A replacement means the name stops resolving for the width of an apply.

        PowerDNS patches an RRSet in place, so a TTL change must plan as an
        update. Anything else is the provider forcing an outage nobody asked
        for.
        """
        result = terragrunt.run(
            "plan",
            "-var",
            "ttl=1800",
            expect_success=False,
        )
        combined = result.stdout + result.stderr
        assert "must be replaced" not in combined, combined[-2000:]
        assert "forces replacement" not in combined, combined[-2000:]
        assert "update in-place" in combined, combined[-2000:]


class TestAdoption:
    """Taking over something that already exists.

    In its own unit with its own state: importing into an address that the
    live unit manages orphaned the real zone when this cleaned up, and the
    next apply met a 409 whose cause was three tests away.
    """

    def test_an_out_of_band_zone_imports(self, terragrunt_import):
        """A zone created outside Terraform can be adopted rather than recreated."""
        api = "http://127.0.0.1:18081/api/v1/servers/localhost/zones"
        name = "imported.e2e.example."

        pdns_call("POST", api, f'{{"name":"{name}","kind":"Native"}}'.encode())
        try:
            terragrunt_import.run("init", expect_success=False)
            result = terragrunt_import.run(
                "import", "powerdns_zone.this", name, expect_success=False
            )
            combined = result.stdout + result.stderr
            assert "Import successful" in combined or "Imported" in combined, combined[
                -2000:
            ]
        finally:
            terragrunt_import.run(
                "state", "rm", "powerdns_zone.this", expect_success=False
            )
            pdns_call("DELETE", f"{api}/{name}")


class TestDestroy:
    """Taking it down, and proving it is gone from where it was."""

    def test_destroy_removes_it_everywhere(self, terragrunt, dns_query, db):
        """Gone from DNS and from storage, not merely gone from state.

        REFUSED, not NXDOMAIN: with the zone deleted the server is no longer
        authoritative for the name, so it declines to answer for it at all.
        Asserting the rcode rather than an empty answer set also means a dead
        server fails this test instead of passing it.
        """
        terragrunt.run("apply", "-auto-approve")
        result = terragrunt.run("destroy", "-auto-approve")
        assert "Destroy complete" in (result.stdout + result.stderr)

        rcode, values = dns_query(FQDN, "A")
        assert rcode == "REFUSED", f"expected the zone to be gone, got {rcode}"
        assert values == set()
        assert row_count(db, FQDN) == 0
