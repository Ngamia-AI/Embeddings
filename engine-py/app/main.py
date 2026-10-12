"""FastAPI application for the Phase 8 Python video engine.

Serves an OpenAI-compatible /v1/embeddings endpoint using sentence-transformers
with the vision encoder for images and video frames. Text and image parts are
also supported so a mixed item that contains video routes here as a whole.
"""

from __future__ import annotations

import logging
from typing import Any

from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import JSONResponse

from . import config
from .embedding import Embedder, EmbeddingError
from .media import MediaError

log = logging.getLogger("engine")

app = FastAPI(title="embeddings-python-engine")
embedder = Embedder()
_state = {"ready": False}


@app.on_event("startup")
def _startup() -> None:
    try:
        embedder.load()
        _state["ready"] = True
    except Exception:  # noqa: BLE001
        log.exception("model load failed; engine will return 503 until fixed")


def _embedder() -> Embedder:
    if not _state["ready"]:
        raise HTTPException(status_code=503, detail="model not loaded")
    return embedder


@app.get("/health")
def health() -> dict[str, Any]:
    return {
        "status": "ok" if _state["ready"] else "loading",
        "model": config.MODEL_ID,
        "device": config.DEVICE,
        "dtype": config.DTYPE,
    }


@app.get("/v1/models")
def models() -> dict[str, Any]:
    return {
        "object": "list",
        "data": [
            {"id": "embeddinggemma-2", "object": "model", "owned_by": "camel-creatives"}
        ],
    }


@app.post("/v1/embeddings")
async def embeddings(request: Request) -> JSONResponse:
    try:
        body = await request.json()
    except Exception as exc:  # noqa: BLE001
        raise HTTPException(status_code=400, detail="invalid JSON body") from exc

    try:
        items = _normalize_items(body.get("input"))
    except HTTPException:
        raise
    if not items:
        raise HTTPException(status_code=400, detail="input is required")

    try:
        result = _embedder().embed_items(items)
    except (MediaError, EmbeddingError) as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except HTTPException:
        raise
    except Exception as exc:  # noqa: BLE001
        log.exception("embedding failed")
        raise HTTPException(status_code=500, detail="embedding failed") from exc

    return JSONResponse(_response(result))


def _normalize_items(inp: Any) -> list[list[dict]]:
    if inp is None:
        return []
    if isinstance(inp, str):
        return [[{"type": "text", "text": inp}]]
    if isinstance(inp, dict):
        inp = [inp]
    if not isinstance(inp, list):
        raise HTTPException(status_code=400, detail="input must be a string or array")

    items: list[list[dict]] = []
    for entry in inp:
        if isinstance(entry, str):
            items.append([{"type": "text", "text": entry}])
        elif isinstance(entry, dict):
            content = entry.get("content")
            if content is None:
                if "text" in entry:
                    items.append([{"type": "text", "text": entry["text"]}])
                    continue
                raise HTTPException(status_code=400, detail="item has no content")
            if isinstance(content, str):
                items.append([{"type": "text", "text": content}])
            elif isinstance(content, list):
                items.append([p for p in content if isinstance(p, dict)])
            else:
                raise HTTPException(status_code=400, detail="invalid content")
        else:
            raise HTTPException(status_code=400, detail="invalid input item")
    return items


def _response(result) -> dict[str, Any]:
    data = [
        {"object": "embedding", "index": i, "embedding": vec}
        for i, vec in enumerate(result.vectors)
    ]
    usage = {
        "prompt_tokens": result.usage.prompt_tokens,
        "total_tokens": result.usage.total_tokens,
        "images": result.usage.images,
        "video_seconds": round(result.usage.video_seconds, 3),
    }
    return {
        "object": "list",
        "data": data,
        "model": "embeddinggemma-2",
        "usage": usage,
    }
