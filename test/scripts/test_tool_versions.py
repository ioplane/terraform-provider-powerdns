"""The anti-drift check, on the cases that decide whether it works.

The shell version could only be tested by editing the repository's real
Containerfile and watching what happened, so in practice it was tested once,
by hand, when it was written.
"""

from __future__ import annotations

from pathlib import Path

import pytest
from scripts.checks.tool_versions import (
    REQUIRED,
    declared_versions,
    marked_pin,
    pinned_lines,
    satisfies,
)

DIGEST = "a" * 64
OTHER_DIGEST = "b" * 64

CONTAINERFILE = f"""\
FROM docker.io/library/golang:1.27-trixie@sha256:{DIGEST} AS base
ARG GO_IMAGE=docker.io/library/golang:1.27-trixie@sha256:{DIGEST}
ARG RUFF_VERSION=0.16.0
ARG NODE_MAJOR=22
# ARG COMMENTED_OUT=1.0.0
RUN echo not an arg
"""


def test_the_full_go_image_reference_is_compared():
    """Both the readable release channel and immutable digest are contractual."""
    assert declared_versions(CONTAINERFILE)["GO_IMAGE"] == (
        f"docker.io/library/golang:1.27-trixie@sha256:{DIGEST}"
    )


def test_a_wrong_go_image_tag_with_the_same_digest_is_rejected():
    """A valid digest cannot hide a workflow left on an older Go channel."""
    expected = declared_versions(CONTAINERFILE)["GO_IMAGE"]
    line = (
        f"image: docker.io/library/golang:1.26-trixie@sha256:{DIGEST} # pin: GO_IMAGE"
    )
    assert not satisfies(line, expected)


def test_a_wrong_go_image_digest_with_the_same_tag_is_rejected():
    """A matching tag cannot hide a workflow using another manifest."""
    expected = declared_versions(CONTAINERFILE)["GO_IMAGE"]
    line = (
        "image: docker.io/library/golang:1.27-trixie@sha256:"
        f"{OTHER_DIGEST} # pin: GO_IMAGE"
    )
    assert not satisfies(line, expected)


def test_a_short_docker_hub_go_image_is_rejected():
    """Every executable image identifier must name its registry explicitly."""
    expected = f"docker.io/library/golang:1.27-trixie@sha256:{DIGEST}"
    assert not satisfies(f"image: golang:1.27-trixie@sha256:{DIGEST}", expected)
    with pytest.raises(ValueError, match="fully qualified"):
        declared_versions(f"ARG GO_IMAGE=golang:1.27-trixie@sha256:{DIGEST}\n")


@pytest.mark.parametrize(
    "reference",
    [
        "docker.io/library/golang:1.27-trixie",
        "docker.io/library/golang:1.27-trixie@sha256:abc123",
        f"docker.io/library/golang:1.27-trixie@sha256:{'g' * 64}",
        f"docker.io/library/golang:1.27-trixie@sha256:{DIGEST}a",
        f"docker.io/library/golang:1.27-trixie@sha256:{DIGEST}evil",
        f"docker.io/library/golang:1.27-trixie@sha256:{DIGEST}zsuffix",
        f"prefix docker.io/library/golang:1.27-trixie@sha256:{DIGEST}",
    ],
)
def test_an_invalid_go_image_reference_fails_closed(reference):
    """Malformed, floating, and non-hex image identifiers are never accepted."""
    with pytest.raises(ValueError, match="tag and 64-hex sha256 digest"):
        declared_versions(f"ARG GO_IMAGE={reference}\n")


def test_a_digest_with_trailing_garbage_cannot_satisfy_a_workflow_pin():
    """The parser must not truncate a malformed digest to its first 64 hex digits."""
    expected = f"docker.io/library/golang:1.27-trixie@sha256:{DIGEST}"
    malformed = (
        f"image: docker.io/library/golang:1.27-trixie@sha256:{DIGEST}evil "
        "# pin: GO_IMAGE"
    )
    assert not satisfies(
        malformed,
        expected,
    )


def test_yaml_quotes_and_a_comment_are_valid_reference_delimiters():
    """Workflow syntax around a complete image reference is not part of the pin."""
    expected = f"docker.io/library/golang:1.27-trixie@sha256:{DIGEST}"
    assert satisfies(
        "image: 'docker.io/library/"
        f"golang:1.27-trixie@sha256:{DIGEST}' # pin: GO_IMAGE",
        expected,
    )


def test_plain_versions_are_read_whole():
    """Everything without an `@` is the value itself."""
    versions = declared_versions(CONTAINERFILE)
    assert versions["RUFF_VERSION"] == "0.16.0"
    assert versions["NODE_MAJOR"] == "22"


def test_a_commented_arg_is_not_a_declaration():
    """Otherwise a commented-out ARG would satisfy a pin that names it."""
    assert "COMMENTED_OUT" not in declared_versions(CONTAINERFILE)


@pytest.mark.parametrize(
    ("line", "expected"),
    [
        ('version: "0.16.0" # pin: RUFF_VERSION', "RUFF_VERSION"),
        ("    uses: x@y  #pin:GO_IMAGE", "GO_IMAGE"),
        ('    "ruff==0.16.0", # pin: RUFF_VERSION', "RUFF_VERSION"),
        ("no marker here", None),
        ("# pin: lowercase_is_not_an_arg", None),
    ],
)
def test_the_marker_names_the_arg(line, expected):
    """The marker is the whole coupling between a workflow line and the image."""
    assert marked_pin(line) == expected


def test_a_comment_alone_cannot_satisfy_the_check():
    """The failure this prevents: bump the version, leave the old one in the comment."""
    assert not satisfies("version: 0.15.0 # pin: RUFF_VERSION uses 0.16.0", "0.16.0")
    assert satisfies("version: 0.16.0 # pin: RUFF_VERSION", "0.16.0")


def test_markers_are_found_in_a_directory_and_in_a_single_file(tmp_path):
    """pyproject.toml is a file and .github/workflows is a tree; both carry pins."""
    workflows = tmp_path / "workflows"
    workflows.mkdir()
    (workflows / "ci.yml").write_text('a: "1.0" # pin: A_VERSION\nb: plain\n')
    (workflows / "nested").mkdir()
    (workflows / "nested" / "deep.yml").write_text('c: "2.0" # pin: C_VERSION\n')
    single = tmp_path / "pyproject.toml"
    single.write_text('"x==3.0", # pin: X_VERSION\n')

    hits = pinned_lines((workflows, single))
    assert {marked_pin(line) for _, _, line in hits} == {
        "A_VERSION",
        "C_VERSION",
        "X_VERSION",
    }


def test_a_binary_file_in_the_tree_is_skipped(tmp_path):
    """Rglob walks whatever is there; a stray archive must not abort the check."""
    root = tmp_path / "workflows"
    root.mkdir()
    (root / "ci.yml").write_text('a: "1.0" # pin: A_VERSION\n')
    (root / "blob.bin").write_bytes(b"\xff\xfe\x00binary")
    assert len(pinned_lines((root,))) == 1


def test_the_repositorys_own_containerfile_declares_every_required_arg():
    """The list and the file are two hands that have to agree.

    If they drift, the check reports a problem that is really a stale list.
    """
    declared = declared_versions(
        Path("deployments/containers/Containerfile.dev").read_text(encoding="utf-8")
    )
    assert [name for name in REQUIRED if name not in declared] == []
