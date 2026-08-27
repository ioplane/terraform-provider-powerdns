"""Security-policy regression tests."""

import tomllib
from pathlib import Path

OSV_CONFIG = Path("osv-scanner.toml")
OPENPGP_REASON = (
    "The provider and its tests do not import golang.org/x/crypto/openpgp; "
    "govulncheck and go list -deps -test ./... confirm the affected package "
    "is unreachable."
)


def test_osv_exception_is_narrow_and_explained():
    """A module-level alert may be ignored only with exact reachability evidence."""
    config = tomllib.loads(OSV_CONFIG.read_text(encoding="utf-8"))

    assert config == {
        "IgnoredVulns": [
            {
                "id": "GO-2026-5932",
                "reason": OPENPGP_REASON,
            }
        ]
    }
