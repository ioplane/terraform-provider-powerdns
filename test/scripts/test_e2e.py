"""Fail-closed teardown contracts for the end-to-end fixture.

The runtime objects must not disappear until every managed PowerDNS name has
been observed absent.  These tests deliberately describe that ordering before
the driver implements it: a transport failure or an unexpected HTTP response
must preserve the Compose fixture so recovery evidence is not destroyed.
"""

from __future__ import annotations

import io
import json
import urllib.error
import urllib.parse
import urllib.request
from collections import defaultdict
from dataclasses import dataclass
from email.message import Message
from typing import NoReturn, Self

import pytest
from scripts.automation import e2e

REC_ABSENCE = {"error": "Could not find domain 'internal.e2e.example'"}


@dataclass
class _Response:
    status: int
    body: bytes = b""

    def read(self) -> bytes:
        return self.body

    def close(self) -> None:
        return None

    def __enter__(self) -> Self:
        return self

    def __exit__(self, *_exc_info: object) -> bool:
        return False


class _PowerDNS:
    """A deterministic three-product HTTP API used only by teardown tests."""

    def __init__(self) -> None:
        self.present = {zone: True for _api, zone in e2e.MANAGED_ZONES}
        self.calls: list[tuple[str, str, str]] = []
        self.compose_calls: list[tuple[tuple[str, ...], int]] = []
        self.reads: defaultdict[tuple[str, str], int] = defaultdict(int)
        self.read_results: dict[tuple[str, str, int], _Response | BaseException] = {}
        self.delete_results: dict[tuple[str, str], _Response | BaseException] = {}

    @staticmethod
    def _request_parts(
        request: urllib.request.Request | str,
    ) -> tuple[str, str, str]:
        if isinstance(request, urllib.request.Request):
            method = request.get_method()
            url = request.full_url
            headers = {key.lower(): value for key, value in request.header_items()}
            assert headers.get("x-api-key") == e2e.API_KEY
        else:
            method = "GET"
            url = request
        api = next(name for name, base in e2e.ZONE_APIS.items() if url.startswith(base))
        zone = urllib.parse.unquote(url.split("/zones/", 1)[1])
        return method, api, zone

    @staticmethod
    def _raise_http(url: str, status: int, body: bytes) -> NoReturn:
        raise urllib.error.HTTPError(
            url, status, "planned response", Message(), io.BytesIO(body)
        )

    def urlopen(
        self,
        request: urllib.request.Request | str,
        **_kwargs: object,
    ) -> _Response:
        method, api, zone = self._request_parts(request)
        self.calls.append((method, api, zone))
        url = (
            request.full_url if isinstance(request, urllib.request.Request) else request
        )

        if method == "DELETE":
            result = self.delete_results.get((api, zone), _Response(204))
            if isinstance(result, BaseException):
                raise result
            if result.status == 204 and result.body == b"":
                self.present[zone] = False
            return result

        read_index = self.reads[(api, zone)]
        self.reads[(api, zone)] += 1
        result = self.read_results.get((api, zone, read_index))
        if isinstance(result, BaseException):
            raise result
        if result is not None:
            if result.status >= 400:
                self._raise_http(url, result.status, result.body)
            return result
        if self.present[zone]:
            return _Response(200, b"{}")
        if api == "recursor":
            return self._raise_http(url, 422, json.dumps(REC_ABSENCE).encode())
        return self._raise_http(url, 404, b'{"error":"Not Found"}')


@pytest.fixture
def pdns(
    monkeypatch: pytest.MonkeyPatch,
) -> tuple[_PowerDNS, list[tuple[tuple[str, ...], int]]]:
    """Replace all PowerDNS and Compose effects with deterministic fakes."""
    server = _PowerDNS()
    monkeypatch.setattr(e2e.urllib.request, "urlopen", server.urlopen)
    monkeypatch.setattr(
        e2e,
        "compose",
        lambda *args: server.compose_calls.append((args, len(server.calls))),
    )
    return server, server.compose_calls


def test_down_reads_deletes_and_proves_absence_before_compose(pdns):
    """Every zone is read first and re-read absent before broad teardown."""
    server, compose_calls = pdns

    assert e2e.cmd_down() == 0

    expected_initial = [
        operation
        for api, zone in e2e.MANAGED_ZONES
        for operation in (("GET", api, zone), ("DELETE", api, zone))
    ]
    expected_post = [("GET", api, zone) for api, zone in e2e.MANAGED_ZONES]
    assert sum(api != "recursor" for api, _zone in e2e.MANAGED_ZONES) == 11
    assert [(api, zone) for api, zone in e2e.MANAGED_ZONES if api == "recursor"] == [
        ("recursor", "internal.e2e.example.")
    ]
    assert server.calls == [*expected_initial, *expected_post]
    assert compose_calls == [(("down", "-v"), len(server.calls))]


@pytest.mark.parametrize("api", ["gpgsql", "lmdb", "recursor"])
def test_an_already_absent_zone_uses_its_product_read_response(pdns, api):
    """An absent name is accepted from GET and is never blindly deleted."""
    server, _compose_calls = pdns
    zone = next(zone for product, zone in e2e.MANAGED_ZONES if product == api)
    server.present[zone] = False

    assert e2e.cmd_down() == 0

    assert ("DELETE", api, zone) not in server.calls
    assert server.calls.count(("GET", api, zone)) == 2


def test_initial_read_transport_error_propagates_without_compose(pdns):
    """A failed initial GET is observable and preserves the fixture."""
    server, compose_calls = pdns
    api, zone = e2e.MANAGED_ZONES[0]
    server.read_results[(api, zone, 0)] = ConnectionResetError("planned reset")

    with pytest.raises(ConnectionResetError, match="planned reset"):
        e2e.cmd_down()

    assert compose_calls == []


@pytest.mark.parametrize("status", [200, 202, 404])
def test_delete_requires_exact_http_204(pdns, status):
    """No other successful or error DELETE status is accepted."""
    server, compose_calls = pdns
    api, zone = e2e.MANAGED_ZONES[0]
    server.delete_results[(api, zone)] = _Response(status)

    with pytest.raises(RuntimeError, match=r"DELETE.*204"):
        e2e.cmd_down()

    assert compose_calls == []


@pytest.mark.parametrize("body", [b"\n", b"{}", b"null"])
def test_delete_204_requires_an_empty_body(pdns, body):
    """HTTP 204 with any response bytes is not the exact success contract."""
    server, compose_calls = pdns
    api, zone = e2e.MANAGED_ZONES[0]
    server.delete_results[(api, zone)] = _Response(204, body)

    with pytest.raises(RuntimeError, match="empty body"):
        e2e.cmd_down()

    assert compose_calls == []


def test_delete_transport_error_propagates_without_compose(pdns):
    """DELETE transport errors are never suppressed before broad teardown."""
    server, compose_calls = pdns
    api, zone = e2e.MANAGED_ZONES[0]
    server.delete_results[(api, zone)] = TimeoutError("planned delete timeout")

    with pytest.raises(TimeoutError, match="planned delete timeout"):
        e2e.cmd_down()

    assert compose_calls == []


def test_post_delete_read_transport_error_blocks_compose(pdns):
    """Failure to prove final absence preserves all Compose state."""
    server, compose_calls = pdns
    api, zone = e2e.MANAGED_ZONES[0]
    server.read_results[(api, zone, 1)] = OSError("planned post-delete failure")

    with pytest.raises(OSError, match="planned post-delete failure"):
        e2e.cmd_down()

    assert compose_calls == []


def test_authoritative_residue_blocks_compose(pdns):
    """A remaining authoritative zone prevents fixture deletion."""
    server, compose_calls = pdns
    api, zone = e2e.MANAGED_ZONES[0]
    server.read_results[(api, zone, 1)] = _Response(200, b"{}")

    with pytest.raises(RuntimeError, match="still exists"):
        e2e.cmd_down()

    assert compose_calls == []


@pytest.mark.parametrize("status", [400, 410, 422])
def test_authoritative_absence_requires_exact_http_404(pdns, status):
    """Authoritative and LMDB absence cannot borrow Recursor semantics."""
    server, compose_calls = pdns
    api, zone = e2e.MANAGED_ZONES[0]
    server.read_results[(api, zone, 1)] = _Response(status, b'{"error":"gone"}')

    with pytest.raises(RuntimeError, match=r"Authoritative.*404"):
        e2e.cmd_down()

    assert compose_calls == []


@pytest.mark.parametrize(
    ("status", "body"),
    [
        (404, b'{"error":"Not Found"}'),
        (422, b"not-json"),
        (422, b'{"error":"wrong"}'),
        (
            422,
            b'{"error":"Could not find domain \'internal.e2e.example\'","extra":true}',
        ),
    ],
    ids=("wrong-status", "malformed-json", "wrong-message", "extra-field"),
)
def test_recursor_absence_requires_exact_422_json(pdns, status, body):
    """Recursor absence is exact in status and parsed JSON object shape."""
    server, compose_calls = pdns
    server.read_results[("recursor", "internal.e2e.example.", 1)] = _Response(
        status, body
    )

    with pytest.raises(RuntimeError, match=r"Recursor.*422"):
        e2e.cmd_down()

    assert compose_calls == []


def test_no_success_message_precedes_the_complete_absence_oracle(pdns, capsys):
    """Operators never receive a success line before the last absence GET."""
    server, compose_calls = pdns
    api, zone = e2e.MANAGED_ZONES[-2]
    server.read_results[(api, zone, 1)] = OSError("last oracle failed")

    with pytest.raises(OSError, match="last oracle failed"):
        e2e.cmd_down()

    assert "zones created by the units removed" not in capsys.readouterr().out
    assert compose_calls == []
