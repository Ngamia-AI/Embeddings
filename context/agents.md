# Notes for agents working in this repo

Read the SRS at the repository root first
(`Embeddings Service (embeddings.ngamia.cc) — SRS and Go Implementation Plan.md`).
It is the source of truth for behaviour, limits and phases.

## Rules of the road

1. Keep the clean-architecture boundaries: `delivery` -> `usecase` -> `domain`,
   with `repository` / `engine` / `media` implementing `domain` ports. Do not
   reach from use cases into HTTP or SQL.
2. No comments unless they explain non-obvious intent (see `conventions.md`).
3. Every behaviour change needs a test. Run `make test && make vet` before
   considering a task done.
4. Never persist or log input text or media. This is a hard privacy rule.
5. Maintain the two response shapes: OpenAI for `/v1/*`, Camel envelope for
   `/admin/v1/*`.
6. Data-plane errors are `*domain.Error`; add a sentinel rather than inventing
   HTTP errors in handlers.
7. Model weights are not in git. Only `models/MANIFEST.sha256` is tracked;
   weights are downloaded on the VPS with `make models`.

## Build method

Follow the SRS phases (§11). Each phase is gated by tests; commit only when the
gate is green. `context/` is written before code. When you learn something on
the real server, record it in `context/decisions.md` and `docs/benchmark.md`.

## Useful commands

```bash
make test vet lint
make build
make models verify-models
make up logs smoke
```
