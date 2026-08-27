"""The parsing the pin check does before it asks anything over the network.

Every bug this check has had was in this layer, not in the HTTP: a reference
read as the wrong repository, a tag left on a digest, a media type left out of
the Accept header. The network part is somebody else's registry and is exercised
for real every time the gate runs.
"""

from __future__ import annotations

import re

import pytest
from scripts.checks.pins import (
    ACTION_REF,
    DIGEST,
    IMAGE_REF,
    MANIFEST_ACCEPT,
    action_repo,
    references,
    resolvable,
    unpinned_from,
)


@pytest.mark.parametrize(
    ("action", "repo"),
    [
        ("github/codeql-action/init", "github/codeql-action"),
        ("anchore/sbom-action/download-syft", "anchore/sbom-action"),
        ("actions/checkout", "actions/checkout"),
    ],
)
def test_the_sha_belongs_to_the_repository_not_the_subdirectory(action, repo):
    """Reading `codeql-action/init` as a repository reports a valid pin as missing."""
    assert action_repo(action) == repo


def test_the_tag_is_dropped_before_the_digest_is_asked_about():
    """A reference may carry both for legibility; only the digest pins."""
    digest = "@sha256:" + "a" * 64
    assert resolvable(f"docker.io/library/postgres:17{digest}") == (
        f"docker.io/library/postgres{digest}"
    )


def test_a_reference_with_no_tag_is_left_alone():
    """Substituting blindly would eat the repository's last path segment."""
    digest = "@sha256:" + "b" * 64
    assert resolvable(f"ghcr.io/owner/name{digest}") == f"ghcr.io/owner/name{digest}"


def test_every_manifest_media_type_is_asked_for():
    """A tag can publish an OCI index and a Docker list with different digests.

    Asking for too few is how the same pin read as present on one machine and
    missing on another — which happened, and was first misdiagnosed as the
    digest not existing upstream.
    """
    for media_type in (
        "application/vnd.oci.image.index.v1+json",
        "application/vnd.docker.distribution.manifest.list.v2+json",
        "application/vnd.oci.image.manifest.v1+json",
        "application/vnd.docker.distribution.manifest.v2+json",
    ):
        assert media_type in MANIFEST_ACCEPT


@pytest.mark.parametrize(
    ("text", "expected"),
    [
        (
            "image: docker.io/library/postgres:17@sha256:" + "c" * 64,
            "docker.io/library/postgres:17@sha256:" + "c" * 64,
        ),
        ("image: docker.io/library/postgres:17", "docker.io/library/postgres:17"),
        (
            "FROM codeberg.org/forgejo/forgejo:16.0.1-rootless",
            "codeberg.org/forgejo/forgejo:16.0.1-rootless",
        ),
    ],
)
def test_image_references_are_recognised_with_and_without_a_digest(text, expected):
    """A floating reference has to be found before it can be reported floating."""
    match = IMAGE_REF.search(text)
    assert match is not None
    assert match.group(0) == expected


def test_a_digest_is_only_a_digest_at_the_end_of_the_reference():
    """Anchoring matters: a digest in the middle is a different string entirely."""
    assert DIGEST.search("x@sha256:" + "d" * 64)
    assert not DIGEST.search("x@sha256:" + "d" * 63)
    assert not DIGEST.search("x@sha256:" + "d" * 64 + "trailing")


def test_action_references_are_taken_from_the_uses_line(tmp_path):
    """`uses:` is the only place a pin lives, and the SHA may be quoted or bare."""
    workflow = tmp_path / "ci.yml"
    workflow.write_text(
        "jobs:\n"
        "  a:\n"
        "    steps:\n"
        "      - uses: actions/checkout@" + "e" * 40 + " # v7.0.1\n"
        "      - uses:  github/codeql-action/init@" + "f" * 40 + "\n"
        "      - run: uses is not a step here\n"
    )
    found = references(ACTION_REF, (tmp_path,))
    assert [ref.split("@")[0] for ref in found] == [
        "actions/checkout",
        "github/codeql-action/init",
    ]


def test_a_from_using_a_pinned_arg_is_not_floating(tmp_path):
    """The Containerfile pins once in an ARG and refers to it; that is the point."""
    (tmp_path / "Containerfile.dev").write_text(
        "ARG GO_IMAGE=docker.io/library/golang:1.26@sha256:" + "0" * 64 + "\n"
        "FROM ${GO_IMAGE} AS base\n"
    )
    assert unpinned_from(tmp_path) == []


def test_a_from_with_a_bare_tag_is_floating(tmp_path):
    """This is the mistake the check exists for."""
    (tmp_path / "Containerfile.dev").write_text("FROM docker.io/library/golang:1.26\n")
    problems = unpinned_from(tmp_path)
    assert len(problems) == 1
    assert "golang:1.26" in problems[0]


def test_scratch_is_a_reserved_empty_base_not_a_floating_registry_image(tmp_path):
    """The OCI scratch stage has no registry object and therefore no digest."""
    (tmp_path / "Containerfile.lab").write_text("FROM scratch\n")
    assert unpinned_from(tmp_path) == []


def test_the_reference_patterns_are_anchored_to_known_registries():
    """An arbitrary host would make every colon-bearing string an image."""
    assert re.search(
        r"docker\\.io|ghcr\\.io|quay\\.io|codeberg\\.org", IMAGE_REF.pattern
    )
