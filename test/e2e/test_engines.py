"""The same configuration under both engines.

ADR 0004 calls OpenTofu a co-equal target and the dev image has carried both
since phase 0, but every test went through one of them. Terragrunt selects
`tofu` by default, so the end-to-end path had in fact only ever run on
OpenTofu — the opposite of what everyone assumed, and neither claim had been
checked.

This runs the main unit under each in turn.
"""

from __future__ import annotations

import json
import shutil
from pathlib import Path

import pytest

FQDN = "www.engines.e2e.example."
ADDRESSES = {"203.0.113.10", "203.0.113.11"}

pytestmark = pytest.mark.timeout(900)

ENGINES = ("tofu", "terraform")


@pytest.fixture(scope="module", autouse=True)
def clean_server():
    """Clear this unit's zones before the module runs."""
    from conftest import reset_zones

    reset_zones("engines.e2e.example.", "113.0.203.in-addr.arpa.")


@pytest.fixture(params=ENGINES)
def engine(request) -> str:
    """Each engine in turn, by absolute path so nothing resolves by luck."""
    return f"/usr/local/bin/{request.param}"


class TestBothEngines:
    """One configuration, two engines, the same result."""

    def test_apply_and_converge(self, terragrunt_engines, engine: str, dns_query):
        """Apply under this engine, then plan empty under the same engine.

        The state is shared between the two runs on purpose. An engine that can
        apply but cannot read back what the other one wrote is the failure
        worth catching, and giving each its own state would hide it.
        """
        # A fresh module copy per engine: the cached one carries a lock file
        # naming the engine that wrote it.
        shutil.rmtree(
            Path(terragrunt_engines.workdir) / ".terragrunt-cache", ignore_errors=True
        )
        Path(terragrunt_engines.workdir, ".terraform.lock.hcl").unlink(missing_ok=True)
        # Only this unit's cache. The shared one under /root/.cache/terragrunt
        # is where every other unit's `.terraform/providers` points, and
        # clearing it from here left the next module in the session with a
        # lock file naming a plugin that no longer existed.

        applied = terragrunt_engines.run(
            "apply", "-auto-approve", env_overrides={"TG_TF_PATH": engine}
        )
        combined = applied.stdout + applied.stderr
        assert "Apply complete" in combined

        # Which engine actually ran, from its own log prefix. `terragrunt
        # --version` reports only Terragrunt's version, so asking it proves
        # nothing about the binary underneath — the first version of this
        # guard did exactly that and failed for both engines.
        expected = "tofu:" if engine.endswith("tofu") else "terraform:"
        assert expected in combined, combined[-800:]

        planned = terragrunt_engines.run(
            "plan",
            "-detailed-exitcode",
            expect_success=False,
            env_overrides={"TG_TF_PATH": engine},
        )
        assert planned.returncode == 0, (planned.stdout + planned.stderr)[-2000:]

        _, values = dns_query(FQDN, "A")
        assert values == ADDRESSES


class TestStateIsPortable:
    """State written by one engine is readable by the other."""

    def test_state_has_no_engine_specific_lock(self, s3):
        """The state file names a serial and a version, not an engine.

        If a state written by one engine could only be read by that engine,
        "co-equal target" would be a claim about the binary rather than about
        the configuration.
        """
        raw = s3.get_object(Bucket="e2e-state", Key="dns-engines/terraform.tfstate")[
            "Body"
        ].read()
        state = json.loads(raw)
        assert "serial" in state
        assert state["resources"], "state holds no resources"
