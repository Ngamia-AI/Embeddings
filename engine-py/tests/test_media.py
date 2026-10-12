import base64

import pytest

pytest.importorskip("PIL")

from app.media import MediaError, parse_data_uri  # noqa: E402


def test_parse_bare_base64():
    payload = base64.b64encode(b"hello").decode()
    out = parse_data_uri(payload)
    assert out.data == b"hello"
    assert out.mime == ""


def test_parse_data_uri():
    payload = base64.b64encode(b"\x00\x01\x02").decode()
    out = parse_data_uri(f"data:video/mp4;base64,{payload}")
    assert out.data == b"\x00\x01\x02"
    assert out.mime == "video/mp4"


def test_parse_rejects_non_base64():
    with pytest.raises(MediaError):
        parse_data_uri("data:video/mp4,notbase64")


def test_parse_rejects_empty():
    with pytest.raises(MediaError):
        parse_data_uri("")
