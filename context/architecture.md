# Architecture

## Layering (clean architecture)

```
delivery/http  ->  usecase  ->  domain  <-  repository / engine / media
                  platform (config, logger, db, metrics, slots)
```

Engines: `engine/llamacpp` (text, image, audio) and `engine/python` (text,
image, video — a CPU-only `sentence-transformers` service in `engine-py/`).

- `internal/domain` holds the types, the sentinel errors and the **ports**
  (`Embedder`, `KeyRepository`, `AppRepository`, `KeyAdminRepository`,
  `UsageRecorder`, `UsageRepository`, `MediaNormalizer`). No I/O, no imports
  from other internal packages.
- `internal/usecase` contains business rules: `embed` (validate, prefix,
  budget, normalize routing, truncate, meter), `apps` (create app, issue/revoke
  key), `usage` (recorder, rollups). It depends only on `domain`, `media`,
  `slots` and `metrics` — never on HTTP or SQL.
- `internal/repository`, `internal/engine`, `internal/media` implement ports.
- `internal/delivery/http` translates HTTP <-> use cases. It owns the two
  response formats: OpenAI shape for `/v1/*`, Camel envelope for `/admin/v1/*`.
- `internal/platform` is infrastructure with no domain dependency.

## Data flow (data plane)

```
Nginx (TLS, 25m body cap, admin allow-list)
  -> middleware: request id, recover, logging
  -> /v1 group: auth middleware (bearer -> sha256 -> key cache -> Postgres)
  -> openai handler: decode JSON, parse polymorphic input
  -> rate limiter: general + media token buckets (per app)
  -> usecase/embed:
       validate dimensions / batch / modality permissions
       apply text prefixes (text only)
       sniff declared vs magic bytes, decode base64, charge media byte budget
       normalize images (Go, JPEG q95, <=2048px) and audio (ffmpeg -> mono 16k WAV)
       estimate tokens + 280/image + 25/audio-sec, enforce item budget
       acquire slots (global, bulk, media) with wait budgets
       call engine.Router -> llamacpp.Embed
       truncate to `dimensions` and L2-renormalize
  -> openai handler: shape response, record usage event, metrics
  -> usage recorder: buffered -> InsertEvents + UpsertDaily
```

## Concurrency model

Three scenario permits and one byte budget, all context-aware
(`platform/slots`):

- `global` = engine `-np` (default 2)
- `bulk` = 1, so bulk keys never hold more than one engine slot
- `media` = `MEDIA_CONCURRENCY` (default 1), so media never starves text
- media byte budget = `MEDIA_BYTES_BUDGET` (default 128 MB)

A bulk media request takes media, then bulk, then global; released in reverse.
Wait budgets: live text 2s, live media 10s, bulk 30s; on timeout the gateway
returns 429 `server_busy` with `Retry-After`.

## Extensibility

`engine.Router` implements `domain.Embedder` and dispatches each input to the
first engine whose `Supports` covers all of its parts, then reassembles vectors
in input order. Adding a GPU box or the Phase 8 video engine is a matter of
registering another `domain.Embedder`.

When `VIDEO_ENABLED=true` the `engine/python` adapter is registered after
`engine/llamacpp`, so text/image/audio keep the fast path and only items that
contain a `video_url` part fall through to the Python engine. Audio is
deliberately unsupported there.
