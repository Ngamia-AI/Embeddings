import pytest

pytest.importorskip("fastapi")
pytest.importorskip("numpy")

from fastapi import HTTPException  # noqa: E402

from app.main import _normalize_items  # noqa: E402


def test_string_input():
    assert _normalize_items("hello") == [[{"type": "text", "text": "hello"}]]


def test_list_of_strings():
    assert _normalize_items(["a", "b"]) == [
        [{"type": "text", "text": "a"}],
        [{"type": "text", "text": "b"}],
    ]


def test_content_parts_preserved():
    items = _normalize_items(
        [{"content": [{"type": "text", "text": "x"}, {"type": "video_url", "video_url": {"url": "u"}}]}]
    )
    assert len(items) == 1
    assert items[0][1]["type"] == "video_url"


def test_string_content():
    assert _normalize_items([{"content": "hi"}]) == [[{"type": "text", "text": "hi"}]]


def test_invalid_input_rejected():
    with pytest.raises(HTTPException):
        _normalize_items(123)
