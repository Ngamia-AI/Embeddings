# Decisions

Lightweight ADR log. Newest at the top. Each entry: decision, context,
consequences. Phase 0 numbers (RAM, latency, precision) are recorded here once
measured on the real VPS.

## D-006 Fully containerized, Caddy terminates TLS
Everything ships in one compose file: `postgres`, `embed` (llama.cpp),
`gateway`, `caddy`, plus the optional `embed-py` profile. Caddy publishes only
`80/443`, obtains/renews the Let's Encrypt cert for `$DOMAIN` automatically from
`deployments/Caddyfile`, caps the body, returns 404 for `/metrics`, and gates
`/admin/*` by `ADMIN_ALLOW_CIDRS`. The gateway is loopback-only for smoke tests.
One-command deploy: `scripts/deploy.sh`. The host nginx sample is kept as an
alternative but Caddy is the supported default.

## D-005 Video path is a Python engine behind a flag
Video ships last and stays off until the SRS Phase 8 gate passes (parity, a 58 s
clip, RAM budget). We implement Path B: a CPU-only `sentence-transformers`
engine in `engine-py/`, reached through `internal/engine/python` behind
`domain.Embedder`. The router sends any item containing a `video_url` part to it;
the gateway keeps prefixes, truncation and re-normalization. Never fp16 — fp32
(default) or bfloat16 only. The engine samples frames at 1 fps, caps at 58 s,
mean-pools frame vectors and L2-normalizes. Set `VIDEO_ENABLED=true` and start
the `video` compose profile to enable.

## D-004 Media never starves text
`MEDIA_CONCURRENCY` (default 1) plus a dedicated `media_rps` bucket and a media
byte budget (128 MB) keep image/audio work from exhausting RAM or the engine.
Text and media share the global slot pool sized to engine `-np`.

## D-003 Truncation then re-normalization
`dimensions` slices the 768-d vector to 512/256/128 and divides by the L2 norm;
unit tests assert the norm is within 1e-6 of 1. Consumers must use the same
dimension for queries and documents.

## D-002 One API key per app, hashed at rest
Keys are `emb_<class>_` + 32 random bytes (base62), stored as SHA-256. High
entropy makes SHA-256 sufficient (no KDF needed). A 60s in-memory cache fronts
the lookup; revocation invalidates the cache entry immediately on the same
instance and within the TTL across instances.

## D-001 Clean architecture, Go does no inference
The gateway owns auth, limits, media validation/normalization, truncation and
metering. Inference is behind `domain.Embedder`; `engine.Router` dispatches per
input. This keeps the engine replaceable (Q8_0, a second slot pool, a GPU box)
without touching use cases.

## D-000 Deferred to Phase 0 (measure on the VPS)
- Precision: BF16 vs Q8_0 (default BF16 unless it breaks the 3 GB cap or is
  >2x slower at p95).
- Slot count (`-np`) and item token budget.
- `MEDIA_CONCURRENCY` and per-app default limits.
- Mixed text+media item support (FR-5) and the video spike.
Record the measured numbers and the resulting choice here and in
`docs/benchmark.md`.
