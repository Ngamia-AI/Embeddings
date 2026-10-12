package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Embedder is the port the engine adapters implement.
type Embedder interface {
	Embed(ctx context.Context, in []Input) (EmbedResult, error)
	Supports(k PartKind) bool
	Ready(ctx context.Context) error
}

// MediaNormalizer normalizes decoded media before it reaches the engine.
type MediaNormalizer interface {
	NormalizeImage(data []byte, declaredMime string) (norm []byte, mime string, err error)
	NormalizeAudio(ctx context.Context, data []byte, format string) (norm []byte, seconds float64, err error)
}

// KeyRepository resolves keys to apps for the data plane.
type KeyRepository interface {
	FindByHash(ctx context.Context, hash []byte) (*APIKey, *App, error)
	TouchLastUsed(ctx context.Context, keyID uuid.UUID) error
}

// AppRepository is the control-plane store for apps.
type AppRepository interface {
	Create(ctx context.Context, a *App) error
	Get(ctx context.Context, id uuid.UUID) (*App, error)
	List(ctx context.Context, limit, offset int) ([]App, int, error)
	Update(ctx context.Context, a *App) error
}

// KeyAdminRepository issues and revokes keys.
type KeyAdminRepository interface {
	Issue(ctx context.Context, k *APIKey) error
	Revoke(ctx context.Context, id uuid.UUID) error
}

// UsageRecorder buffers usage events for asynchronous persistence.
type UsageRecorder interface {
	Record(e UsageEvent) // non-blocking
	Flush(ctx context.Context) error
}

// UsageRepository persists events and reads rollups.
type UsageRepository interface {
	InsertEvents(ctx context.Context, events []UsageEvent) error
	UpsertDaily(ctx context.Context, events []UsageEvent) error
	Rollups(ctx context.Context, appID uuid.UUID, from, to time.Time) ([]UsageDaily, error)
	PurgeEventsBefore(ctx context.Context, cutoff time.Time) (int64, error)
}
