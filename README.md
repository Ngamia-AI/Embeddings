# Embeddings Service (embeddings.ngamia.cc)

Self-hosted multimodal embeddings gateway for the Camel Creatives ecosystem. It
runs the full **EmbeddingGemma 2** model (text, image, audio; video behind a
flag) on llama.cpp and exposes an **OpenAI-compatible data plane** plus a Camel
control plane.

- Data plane: `POST /v1/embeddings`, `GET /v1/models` — OpenAI JSON, no envelope.
- Control plane: `/admin/v1/*` — Camel envelope `{success, data, message, errors, meta}`.
- The Go gateway owns auth, limits, media validation/normalization, truncation,
  metering and response shaping. **Go does no inference.**
- Stateless: input text and media are never stored or logged.

See `docs/deployment.md` for the VPS runbook (including the model download) and
the SRS at the repository root for the full specification.

## Layout

```
cmd/api/                 wiring + graceful shutdown
internal/domain/         types, errors, ports (interfaces)
internal/usecase/        embed, apps, usage
internal/media/          sniff + image/audio normalization (ffmpeg)
internal/media/          sniff + image/audio normalization (ffmpeg)
internal/engine/         llamacpp + python adapters, router
internal/repository/     postgres + in-memory key cache
internal/delivery/http/  openai, admin, middleware, ops
internal/platform/       config, logger, db, metrics, slots
migrations/              golang-migrate SQL
deployments/             docker-compose, Caddyfile, engine entrypoint, nginx (alt)
engine-py/               Python video engine (Phase 8, Path B)
models/                  MANIFEST.sha256 (weights downloaded on the VPS)
scripts/                 model download / verify / manifest
context/ docs/           project docs
```

## Quickstart (VPS / production)

Everything runs in Docker, including Caddy for automatic TLS:

```bash
cd deployments && cp .env.example .env && $EDITOR .env   # DOMAIN, ACME_EMAIL, ADMIN_ALLOW_CIDRS, secrets
cd .. && ./scripts/deploy.sh                             # models + build + up + wait
```

That publishes only `80/443`; `https://<DOMAIN>` is live once Caddy issues the
certificate. Full details in `docs/deployment.md`.

## Quickstart (development)

Requires Go 1.26+ and a Postgres. The engine is only needed for real embeddings;
unit tests use fakes and stubs.

```bash
make test          # unit tests
make vet
make build
DATABASE_URL=postgres://embeddings:embeddings@localhost:5432/embeddings?sslmode=disable \
ENGINE_URL=http://localhost:8080 ADMIN_TOKEN=dev go run ./cmd/api
```

## Data plane example

```bash
curl -s https://embeddings.ngamia.cc/v1/embeddings \
  -H "Authorization: Bearer emb_live_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "embeddinggemma-2",
    "task": "search_query",
    "dimensions": 512,
    "input": ["bei ya mchele Kariakoo"]
  }' | jq .
```

Multimodal items use the documented extension: an item may be a string or an
object with a `content` array of `text`, `image_url` (base64 data URI),
`input_audio` (base64 `data` + `format`) or `video_url` (base64 data URI, only
when `VIDEO_ENABLED=true`). URLs are rejected; media is never fetched.

## Video (Phase 8, optional)

Video is off by default and served by a separate CPU-only Python engine
(`engine-py/`) when enabled. It samples frames at 1 fps, caps the clip at 58 s
and mean-pools the frame vectors into one embedding. See
`engine-py/README.md` and `docs/deployment.md` §12. Never enable it before the
SRS Phase 8 gate (parity, 58 s clip, RAM budget) passes.

## Conversational/embedded Go

Nothing is exported as a library; the gateway is a service. The engine is behind
`domain.Embedder`, so a GPU box, a second engine, or a Python video path can
replace it without touching use cases.
