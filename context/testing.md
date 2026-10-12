# Testing

## Commands

```bash
make test            # go test ./...
make vet             # go vet ./...
make lint            # golangci-lint run ./... (if installed)
```

## Layers of tests

- **Unit** — use cases against fakes for `Embedder` / repositories /
  `MediaNormalizer`. Covers prefixing, truncation/normalization, budget and
  modality errors, key generation, the slot manager and the usage recorder.
  See `internal/usecase/embed/service_test.go`, `internal/platform/slots/slots_test.go`.
- **Boundary** — the llama.cpp client against `httptest` servers (vector order,
  data-URI building, error mapping, `/health`). See
  `internal/engine/llamacpp/client_test.go`.
- **Parser** — the polymorphic `input` decoder and media sniffing. See
  `internal/delivery/http/openai/dto_test.go`, `internal/media/media_test.go`.
- **Integration** (later) — Postgres via testcontainers; a stub engine HTTP
  server for deterministic vectors.
- **Contract** (later) — the official OpenAI Python/Go SDKs against a running
  instance, plus the documented multimodal examples.
- **Parity** (Phase 0) — the reference comparison in `docs/benchmark.md`,
  re-run whenever the model file, projector, llama.cpp build or the image
  pipeline changes.
- **Fuzz** — `go test -fuzz` on the request and media parsers.
- **Race/load** — `go test -race`, then k6/vegeta with mixed text+media traffic.

## Gate expectations from the SRS

- Phase 1: `/readyz` returns 503 with the engine down, 200 when up.
- Phase 3: wrong MIME -> 400; 50 MP image -> 400; truncated WAV -> 400; 130s
  clip -> `audio_too_long`; OGG/Opus embeds; 10 concurrent 20 MB uploads keep
  the gateway under 512 MB RSS.
- Phase 4: 401/403/429/400 paths and a `-race` test proving media concurrency
  does not block text and bulk never holds more than one slot.
- Phase 5: no events lost on graceful shutdown; rollups equal the event sums.
