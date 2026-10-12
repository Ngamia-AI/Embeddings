# API contracts

Base URL: `https://embeddings.ngamia.cc`. Model id: `embeddinggemma-2`.
Every response carries `X-Request-ID`.

## Data plane (`/v1/*`) — OpenAI-compatible, no envelope

### `POST /v1/embeddings`

Auth: `Authorization: Bearer emb_<class>_<token>`.

Request fields:

| field | notes |
| --- | --- |
| `model` | must be `embeddinggemma-2` |
| `input` | string, `[]string`, or `[]item`; an item is a string or `{content: [...]}` |
| `task` | `search_query`, `question_answering`, `fact_checking`, `code_retrieval`, `classification`, `clustering`, `similarity` |
| `input_type` | `query` or `document` (alternative to `task`) |
| `title` | document title, used with `input_type: document` |
| `dimensions` | `768` (default), `512`, `256`, `128` — truncated and re-normalized |
| `encoding_format` | `float` (default) or `base64` (little-endian float32) |

Content parts: `text`, `image_url` (`{url: "data:image/jpeg;base64,..."}`),
`input_audio` (`{data: "<base64>", format: "wav|mp3|ogg|m4a|webm"}`),
`video_url` (feature-flagged). Remote URLs are rejected. Text prefixes apply to
text only, never to media.

Response:

```json
{
  "object": "list",
  "model": "embeddinggemma-2",
  "data": [{ "object": "embedding", "index": 0, "embedding": [0.01, -0.02] }],
  "usage": { "prompt_tokens": 12, "total_tokens": 12, "images": 1, "audio_seconds": 4.2 }
}
```

### `GET /v1/models`

Lists `embeddinggemma-2` with its modalities and supported dimensions.

### Error shape

```json
{ "error": { "message": "...", "type": "...", "code": "..." } }
```

| status | codes |
| --- | --- |
| 400 | `invalid_json`, `empty_input`, `batch_too_large`, `too_many_media_items`, `invalid_dimensions`, `context_length_exceeded`, `unsupported_input`, `unsupported_media_type`, `invalid_media`, `image_too_large`, `audio_too_long`, `model_not_found` |
| 401 | `invalid_api_key` |
| 403 | `modality_not_allowed` |
| 413 | `payload_too_large` |
| 429 | `rate_limit_exceeded`, `server_busy` (with `Retry-After`) |
| 503 | `engine_unavailable` |
| 504 | `engine_timeout` |

## Control plane (`/admin/v1/*`) — Camel envelope

Auth: `Authorization: Bearer <ADMIN_TOKEN>` or `X-Admin-Token`. Allow-listed at
Nginx. Envelope: `{success, data, message, errors, meta}`.

| method | path | purpose |
| --- | --- | --- |
| POST | `/admin/v1/apps` | create app (limits + modalities) |
| GET | `/admin/v1/apps` | list apps (page info in `meta`) |
| GET | `/admin/v1/apps/{id}` | read one |
| PATCH | `/admin/v1/apps/{id}` | update limits / modalities / status |
| POST | `/admin/v1/apps/{id}/keys` | issue key (plaintext shown once) |
| DELETE | `/admin/v1/keys/{id}` | revoke key |
| GET | `/admin/v1/apps/{id}/usage?from=&to=` | daily rollups per app, split by modality |

Keys are `emb_<class>_` + 32 random bytes (base62), SHA-256 hashed at rest.
Revocation applies within the 60s cache TTL, or immediately on the same instance.

## Ops

| method | path | notes |
| --- | --- | --- |
| GET | `/healthz` | process up |
| GET | `/readyz` | engine reachable + test text/image embed |
| GET | `/metrics` | Prometheus, localhost only (Nginx denies) |
