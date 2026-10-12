# Conventions

- **No comments unless they carry non-obvious intent.** Prefer clear names.
- **Errors**: data-plane failures are `*domain.Error` with `Status`, `Type`,
  `Code`, `Message` and optionally `RetryAfter`. Build them with the sentinels
  in `internal/domain/errors.go`; never construct ad-hoc HTTP errors in
  handlers. Storage "missing row" is `domain.ErrNotFound`. Control-plane
  validation returns `domain.MultiValidation`.
- **Two response shapes, never mixed**: OpenAI shape under `/v1/*`, Camel
  envelope under `/admin/v1/*`. Use `httpx.WriteOpenAIError` and
  `httpx.WriteJSON`/`WriteSuccess`/`WriteValidation`.
- **Request-scoped values** (request id, app, key) travel in the context via
  `httpx.WithRequestID` / `httpx.WithAppKey` and are read with the matching
  accessors. Do not pass them positionally through layers.
- **Config** comes from the environment via `platform/config`; add a field and
  an `env*` helper, never read `os.Getenv` outside that package.
- **Logging** is `log/slog` JSON. Log identifiers and counts, never content.
- **Tests** live next to the code (`*_test.go`), use fakes for ports and
  `httptest` for boundaries. Keep `go vet` and `gofmt` clean.
- **Money/privacy rule**: input text and media must never be persisted or
  logged. Media lives only in memory or a size-capped tmpfs for the request
  duration.
- **Imports**: stdlib, then third-party, then `github.com/camel-creatives/embeddings-api/...`,
  each group separated by a blank line.
