# Python video engine

Phase 8, Path B of the SRS: a small CPU-only service that embeds **video**
(and, for mixed items, text and image) with `sentence-transformers`.

The gateway never embeds video itself. When `VIDEO_ENABLED=true` the Go gateway
registers this engine behind `domain.Embedder` and routes any item containing a
`video_url` part here. The public model id stays `embeddinggemma-2`; this engine
returns the full 768-dim vector and the gateway handles prefixing, truncation and
L2 re-normalization.

## Contract

Mirrors the llama.cpp engine so the gateway adapter is interchangeable:

- `GET /health`
- `GET /v1/models`
- `POST /v1/embeddings` — OpenAI-shaped request/response.

Request `input` accepts a string, an array of strings, or an array of
`{"content": [...]}` items whose parts are `text`, `image_url` or `video_url`
(base64 data URIs). Response `usage` adds `images` and `video_seconds`.

## How video is embedded

1. Decode the `video_url` data URI.
2. Extract frames with ffmpeg at `FRAMES_PER_SECOND` (default 1), for at most
   `MAX_VIDEO_SECONDS` (default 58, the SRS context budget: 58 x 140 = 8120).
3. Embed each frame with the vision encoder.
4. Mean-pool the frame vectors and L2-normalize to one vector per item.

For a mixed item, each part's unit vector is mean-pooled and re-normalized.

## Precision and memory

- Only `float32` (default) or `bfloat16` (`BF16=1`) are allowed. **Never fp16.**
- `BATCH_SIZE` bounds frames per forward pass; `TORCH_THREADS` bounds CPU use.
- The compose service caps memory at 3 GB; keep it off the critical path.

## Run locally

```bash
cd engine-py
python -m venv .venv && . .venv/bin/activate
pip install -r requirements.txt
MAX_VIDEO_SECONDS=58 uvicorn app.main:app --port 8080
```

Model weights download to `HF_HOME` (default `/models/hf`) on first start; they
are cached on a volume, not baked into the image. See `docs/deployment.md`.

## Tests

```bash
cd engine-py
pip install pytest
python -m pytest
```

Tests skip automatically when the heavy dependencies are absent.
