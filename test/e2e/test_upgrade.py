"""Upgrading the provider under an existing state file.

The gap this closes was named when the end-to-end suite landed: every scenario
applied one build of the provider to state that same build had written, so
nothing in the project had ever asked whether a new version can read what an
old one left behind. That is the question a consumer asks every time they bump
a constraint, and the answer cannot be inferred from a suite that never changes
version.

The two builds are real. The fixture mirrors 0.1.1 built from the released tag
and 0.1.2 built from the working tree, so the state read in the second phase was
written by different code — not by the same binary under a different number,
which would have exercised the mechanism and proved nothing.
"""

from __future__ import annotations

import json
from pathlib import Path

import pytest
from scripts.automation import e2e as fixture

RELEASED = fixture.PROVIDER_VERSION
CURRENT = fixture.NEXT_VERSION
ZONE = "upgraded.e2e.example."


def lock_version(terragrunt) -> str:
    """Return the provider version recorded in the unit's lock file."""
    # The lock file is the engine's own record of what it resolved, which is a
    # stronger statement than anything the configuration says it wanted.
    for path in Path(terragrunt.workdir).rglob(".terraform.lock.hcl"):
        for line in path.read_text().splitlines():
            stripped = line.strip()
            if stripped.startswith("version") and "=" in stripped:
                return stripped.split("=", 1)[1].strip().strip('"')
    pytest.fail(f"no .terraform.lock.hcl under {terragrunt.workdir}")


def state(terragrunt) -> dict:
    """Return the unit's state as the engine reports it."""
    completed = terragrunt.run("state", "pull")
    return json.loads(completed.stdout)


class TestTheReleasedVersionWritesTheState:
    """Phase one: apply with the version a consumer already has.

    The two classes run in file order, which pytest guarantees, and the second
    depends on the first having written state. Marking the order with a plugin
    would say it out loud; adding a plugin to say what the file already says is
    the worse trade.
    """

    def test_it_applies(self, terragrunt_upgrade):
        """Nothing later means anything if this does not hold.

        The lock file is removed first. A previous run of this file leaves it
        recording 0.1.2, and Terraform will not move a lock backwards without
        being asked — so without this the phase fails on the second run of the
        suite and the failure reads as an upgrade defect. What is being set up
        is a consumer who has the released version and no history, which is a
        consumer with no lock file.
        """
        for lock in Path(terragrunt_upgrade.workdir).rglob(".terraform.lock.hcl"):
            lock.unlink()
        terragrunt_upgrade.run("init", env_overrides={"E2E_PROVIDER_VERSION": RELEASED})
        terragrunt_upgrade.run(
            "apply",
            "-auto-approve",
            env_overrides={"E2E_PROVIDER_VERSION": RELEASED},
        )

    def test_the_engine_resolved_the_released_build(self, terragrunt_upgrade):
        """The lock file has to say 0.1.1, or the premise is wrong.

        This is the assertion that makes the whole file meaningful: if the
        mirror or the generated requirement were wrong, both phases would run
        the same build and every check below would pass for the wrong reason.
        """
        assert lock_version(terragrunt_upgrade) == RELEASED

    def test_the_zone_exists_on_the_server(self, dns_query):
        """Applied, not merely planned."""
        rcode, values = dns_query("www." + ZONE, "A")
        assert rcode == "NOERROR"
        assert sorted(values) == ["198.51.100.20", "198.51.100.21"]


class TestTheCurrentVersionReadsIt:
    """Phase two: the same state, the build under development."""

    def test_init_upgrade_moves_the_lock(self, terragrunt_upgrade):
        """`-upgrade` is the step a consumer runs when they bump a constraint."""
        terragrunt_upgrade.run(
            "init",
            "-upgrade",
            env_overrides={"E2E_PROVIDER_VERSION": CURRENT},
        )
        assert lock_version(terragrunt_upgrade) == CURRENT

    def test_the_plan_is_empty(self, terragrunt_upgrade):
        """The claim being tested, stated as the engine states it.

        `-detailed-exitcode` returns 0 for no changes and 2 for pending ones.
        A new version that reads the old state differently shows up here as a
        diff nobody asked for — the worst kind, because a consumer meets it on
        a routine bump.
        """
        completed = terragrunt_upgrade.run(
            "plan",
            "-detailed-exitcode",
            expect_success=False,
            env_overrides={"E2E_PROVIDER_VERSION": CURRENT},
        )
        assert completed.returncode == 0, (
            "upgrading the provider produced a plan:\n"
            + (completed.stdout + completed.stderr)[-3000:]
        )

    def test_nothing_is_scheduled_for_replacement(self, terragrunt_upgrade):
        """An empty plan already implies this; the wording is what is checked.

        A `-detailed-exitcode` of 0 cannot coexist with a replacement, so this
        asserts the same fact twice on purpose: replacement of a live DNS zone
        is the outcome that would matter most, and a future change to how the
        plan is invoked should not be able to lose it silently.
        """
        completed = terragrunt_upgrade.run(
            "plan", env_overrides={"E2E_PROVIDER_VERSION": CURRENT}
        )
        combined = completed.stdout + completed.stderr
        assert "must be replaced" not in combined
        assert "forces replacement" not in combined

    def test_the_state_still_describes_every_resource(self, terragrunt_upgrade):
        """A readable state is not the same as a complete one."""
        pulled = state(terragrunt_upgrade)
        addresses = {resource.get("type") for resource in pulled.get("resources", [])}
        assert addresses == {
            "powerdns_zone",
            "powerdns_record",
            "powerdns_zone_metadata",
        }

    def test_applying_again_changes_nothing_on_the_server(
        self, terragrunt_upgrade, dns_query
    ):
        """The end of the consumer's bump: apply, and the zone is as it was."""
        terragrunt_upgrade.run(
            "apply",
            "-auto-approve",
            env_overrides={"E2E_PROVIDER_VERSION": CURRENT},
        )
        rcode, values = dns_query("www." + ZONE, "A")
        assert rcode == "NOERROR"
        assert sorted(values) == ["198.51.100.20", "198.51.100.21"]

    def test_the_metadata_survived_the_upgrade(self, terragrunt_upgrade):
        """Metadata is a separate API and a separate resource; it upgrades too."""
        pulled = state(terragrunt_upgrade)
        metadata = [
            resource
            for resource in pulled.get("resources", [])
            if resource.get("type") == "powerdns_zone_metadata"
        ]
        assert metadata, "the metadata resource is gone from state"
        values = metadata[0]["instances"][0]["attributes"]["values"]
        assert values == ["198.51.100.0/24"]
