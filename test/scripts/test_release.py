"""The release gate's reasoning, on documents built for the purpose.

Every assertion here corresponds to something that went wrong once. The section
parser and the added-lines arithmetic are the two that a reviewer caught rather
than a check, which is the reason they exist at all.
"""

from __future__ import annotations

import pytest
from scripts.checks.release import (
    SEMVER,
    added_since,
    changelog_section,
    declared_protocols,
    served_protocol,
)
from scripts.checks.release_artifacts import parse_sums

CHANGELOG = """\
# Changelog

## [Unreleased]

### Added

- something new

## [0.1.1] - 2026-07-29

### Fixed

- the registry refused v0.1.0

## [0.1.0] - 2026-07-28

### Added

- first release

[Unreleased]: https://github.com/o/r/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/o/r/releases/tag/v0.1.1
"""


def test_a_section_stops_at_the_next_heading():
    """Otherwise every released section would appear to contain all the later ones."""
    section = [line for line in changelog_section(CHANGELOG, "0.1.1") if line.strip()]
    assert section == ["### Fixed", "- the registry refused v0.1.0"]


def test_the_last_section_stops_at_the_link_definitions():
    """The link block belongs to the document, not to the oldest release."""
    section = [line for line in changelog_section(CHANGELOG, "0.1.0") if line.strip()]
    assert section == ["### Added", "- first release"]


def test_an_absent_section_is_empty_rather_than_the_whole_file():
    """A missing section must not read as a full one and pass the emptiness check."""
    assert changelog_section(CHANGELOG, "9.9.9") == []


def test_a_line_added_to_a_released_section_is_counted():
    """Twelve entries had accumulated in [0.1.1] before a reviewer noticed."""
    after = CHANGELOG.replace(
        "- the registry refused v0.1.0",
        "- the registry refused v0.1.0\n- and something written afterwards",
    )
    assert added_since(CHANGELOG, after, "0.1.1") == 1


def test_removing_a_line_from_a_released_section_is_not_an_addition():
    """Removal is how a wrong entry is corrected; the check must allow the fix."""
    after = CHANGELOG.replace("- the registry refused v0.1.0\n", "")
    assert added_since(CHANGELOG, after, "0.1.1") == 0


def test_an_unchanged_section_has_gained_nothing():
    """The ordinary case, and the one this repository is in."""
    assert added_since(CHANGELOG, CHANGELOG, "0.1.1") == 0


def test_a_change_in_another_section_does_not_implicate_this_one():
    """Editing [Unreleased] is the normal activity between releases."""
    after = CHANGELOG.replace("- something new", "- something new\n- and more")
    assert added_since(CHANGELOG, after, "0.1.1") == 0


@pytest.mark.parametrize(
    ("version", "valid"),
    [
        ("0.1.1", True),
        ("1.0.0-rc.1", True),
        ("0.1", False),
        ("v0.1.1", False),
        ("0.1.1.1", False),
    ],
)
def test_the_version_must_be_semantic(version, valid):
    """The Registry orders versions by this, and cannot be corrected afterwards."""
    assert bool(SEMVER.match(version)) is valid


def test_the_framework_serves_six_unless_asked_for_five():
    """A manifest declaring the wrong one breaks every consumer's init."""
    assert served_protocol("providerserver.Serve(ctx, New, opts)") == "6.0"
    assert served_protocol("opts := providerserver.ServeOpts{ProtocolVersion: 5}") == (
        "5.0"
    )


def test_the_manifest_protocols_are_read_as_declared():
    """The comparison is textual, so the shape of the file matters."""
    manifest = '{"version": 1, "metadata": {"protocol_versions": ["6.0"]}}'
    assert declared_protocols(manifest) == "6.0"


def test_a_checksum_line_is_a_digest_and_a_name():
    """The plain form is what goreleaser writes; the binary marker is tolerated."""
    text = (
        "abc123  terraform-provider-powerdns_0.1.1_linux_amd64.zip\n"
        "def456 *terraform-provider-powerdns_0.1.1_manifest.json\n"
        "\n"
    )
    assert parse_sums(text) == [
        ("abc123", "terraform-provider-powerdns_0.1.1_linux_amd64.zip"),
        ("def456", "terraform-provider-powerdns_0.1.1_manifest.json"),
    ]
