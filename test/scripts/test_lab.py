"""Contracts for the disposable PowerDNS lab topology."""

import re
from pathlib import Path

import pytest

COMPOSE = Path("deployments/compose/compose.lab.yml").read_text(encoding="utf-8")
E2E_COMPOSE = Path("deployments/compose/compose.e2e.yml").read_text(encoding="utf-8")
POSTGRES_17 = (
    "docker.io/library/postgres:17@sha256:"
    "a426e44bac0b759c95894d68e1a0ac03ecc20b619f498a91aae373bf06d8508d"
)
POSTGRES_IMAGE = (
    "docker.io/library/postgres:18.6-alpine3.24@sha256:"
    "d3e1620b530c944afa6e887d22eb899824da68e19c52024bf98f5220c88a65b2"
)
POSTGRES_TMPFS = "/var/lib/postgresql:rw,nosuid,nodev,noexec,size=512m"


def service_block(compose: str, service: str) -> str:
    """Return one service mapping from the repository's Compose grammar."""
    match = re.search(
        rf"(?ms)^  {re.escape(service)}:\n(?P<body>.*?)(?=^  [\w-]+:\n|^volumes:\n|\Z)",
        compose,
    )
    assert match is not None
    return match.group("body")


def sequence(block: str, key: str) -> list[str]:
    """Return scalar sequence entries for one service key."""
    match = re.search(
        rf"(?m)^    {re.escape(key)}:\n(?P<body>(?:      .*\n)+)",
        block,
    )
    if match is None:
        return []
    return [
        line.strip().removeprefix("- ").strip("\"'")
        for line in match.group("body").splitlines()
        if line.strip().startswith("- ")
    ]


def assert_postgres_image(compose: str) -> None:
    """Require the exact official disposable Alpine image."""
    block = service_block(compose, "postgres")
    match = re.search(r"(?m)^    image: (?P<image>.+)$", block)
    assert match is not None
    assert match.group("image") == POSTGRES_IMAGE


def assert_postgres_storage(compose: str) -> None:
    """Require bounded ephemeral storage over the image-declared volume."""
    block = service_block(compose, "postgres")
    assert sequence(block, "tmpfs") == [POSTGRES_TMPFS]

    mounts = sequence(block, "volumes")
    assert not any("/var/lib/postgresql" in mount for mount in mounts)
    assert "type: volume" not in block
    assert "target: /var/lib/postgresql" not in block


def target_compose() -> str:
    """Return the intended topology for bounded mutation tests."""
    compose = COMPOSE.replace(POSTGRES_17, POSTGRES_IMAGE, 1)
    if sequence(service_block(compose, "postgres"), "tmpfs") == []:
        compose = compose.replace(
            "    healthcheck:\n",
            f"    tmpfs:\n      - {POSTGRES_TMPFS}\n    healthcheck:\n",
            1,
        )
    return compose


def test_postgres_image_is_the_exact_18_6_oci_index():
    """Compose starts directly from the pinned official Alpine image."""
    assert_postgres_image(COMPOSE)


def test_postgres_storage_is_bounded_tmpfs_without_a_data_volume():
    """Every lab run starts from a new in-memory PostgreSQL 18 cluster."""
    assert_postgres_storage(COMPOSE)


def test_postgres_preserves_the_existing_lab_topology():
    """The database bump must not quietly change credentials or ordering."""
    postgres = service_block(COMPOSE, "postgres")
    auth_pg = service_block(COMPOSE, "auth-pg")

    for setting in (
        "POSTGRES_USER: pdns",
        "POSTGRES_PASSWORD: pdns",
        "POSTGRES_DB: pdns",
    ):
        assert setting in postgres
    assert '"127.0.0.1:15432:5432"' in postgres
    assert (
        "../../test/lab/schema.pgsql.sql:/docker-entrypoint-initdb.d/10-schema.sql:ro,z"
    ) in postgres
    assert 'test: ["CMD-SHELL", "pg_isready -U pdns -d pdns"]' in postgres
    assert re.search(
        r"(?ms)^    depends_on:\n      postgres:\n        condition: service_healthy$",
        auth_pg,
    )


def test_disposable_fixture_listeners_are_loopback_only():
    """Public test credentials and writable APIs must never bind all interfaces."""
    expected_lab_ports = {
        "127.0.0.1:15432:5432",
        "127.0.0.1:15300:5300/udp",
        "127.0.0.1:15300:5300/tcp",
        "127.0.0.1:18081:8081/tcp",
        "127.0.0.1:18091:8081/tcp",
        "127.0.0.1:15301:5301/udp",
        "127.0.0.1:15301:5301/tcp",
        "127.0.0.1:18082:8082/tcp",
        "127.0.0.1:15302:5302/udp",
        "127.0.0.1:15302:5302/tcp",
        "127.0.0.1:18083:8083/tcp",
    }
    actual_lab_ports = {
        port
        for service in ("postgres", "auth-pg", "auth-lmdb", "recursor", "dnsdist")
        for port in sequence(service_block(COMPOSE, service), "ports")
    }
    assert actual_lab_ports == expected_lab_ports

    assert "- -ip=127.0.0.1" in E2E_COMPOSE
    assert "FORGEJO__server__HTTP_ADDR: 127.0.0.1" in E2E_COMPOSE


def test_compose_does_not_claim_an_unimplemented_transaction_protocol():
    """Compose must not emit empty ownership labels no controller consumes."""
    assert "PDNS_LAB_TRANSACTION_ID" not in COMPOSE
    assert "PDNS_E2E_TRANSACTION_ID" not in E2E_COMPOSE


@pytest.mark.parametrize(
    "invalid",
    [
        POSTGRES_17,
        (
            "docker.io/library/postgres:18.6@sha256:"
            "06cad38a5d9f5d24b4d83d86def30795d5e4b757fedbf5281172b576dedcd941"
        ),
        "docker.io/library/postgres:18.6",
        (
            "docker.io/library/postgres:18.5@sha256:"
            "06cad38a5d9f5d24b4d83d86def30795d5e4b757fedbf5281172b576dedcd941"
        ),
        (
            "postgres:18.6@sha256:"
            "06cad38a5d9f5d24b4d83d86def30795d5e4b757fedbf5281172b576dedcd941"
        ),
    ],
    ids=("old-tag", "external-image", "floating", "wrong-tag", "unqualified"),
)
def test_image_contract_rejects_invalid_references(invalid):
    """Every bounded image-identity mutation must be rejected."""
    with pytest.raises(AssertionError):
        assert_postgres_image(target_compose().replace(POSTGRES_IMAGE, invalid, 1))


@pytest.mark.parametrize(
    "mutation",
    [
        lambda text: text.replace(f"    tmpfs:\n      - {POSTGRES_TMPFS}\n", "", 1),
        lambda text: text.replace(
            "/var/lib/postgresql:rw", "/var/lib/postgresql/data:rw", 1
        ),
        lambda text: text.replace(
            "    volumes:\n",
            "    volumes:\n      - pg-data:/var/lib/postgresql\n",
            1,
        ),
        lambda text: text.replace(
            "    volumes:\n",
            "    volumes:\n"
            "      - type: volume\n"
            "        source: pg-data\n"
            "        target: /var/lib/postgresql\n",
            1,
        ),
    ],
    ids=("missing-tmpfs", "wrong-destination", "short-volume", "long-volume"),
)
def test_storage_contract_rejects_persistent_or_wrong_mounts(mutation):
    """No persistent or misplaced mount can satisfy the storage contract."""
    with pytest.raises(AssertionError):
        assert_postgres_storage(mutation(target_compose()))
