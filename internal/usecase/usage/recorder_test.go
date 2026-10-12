package usage

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/camel-creatives/embeddings-api/internal/domain"
)

type fakeUsageRepo struct {
	mu     sync.Mutex
	events []domain.UsageEvent
	daily  []domain.UsageEvent
}

func (f *fakeUsageRepo) InsertEvents(_ context.Context, events []domain.UsageEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, events...)
	return nil
}

func (f *fakeUsageRepo) UpsertDaily(_ context.Context, events []domain.UsageEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.daily = append(f.daily, events...)
	return nil
}

func (f *fakeUsageRepo) Rollups(context.Context, uuid.UUID, time.Time, time.Time) ([]domain.UsageDaily, error) {
	return nil, nil
}

func (f *fakeUsageRepo) PurgeEventsBefore(context.Context, time.Time) (int64, error) { return 0, nil }

func TestRecorderFlushOnClose(t *testing.T) {
	repo := &fakeUsageRepo{}
	log := slog.New(slog.NewTextHandler(discard{}, nil))
	r := NewRecorder(repo, 16, 100, time.Hour, log)

	appID := uuid.New()
	keyID := uuid.New()
	for i := 0; i < 3; i++ {
		r.Record(domain.UsageEvent{AppID: appID, KeyID: keyID, Inputs: 1, Tokens: 10, Status: 200, CreatedAt: time.Now()})
	}
	if err := r.Close(context.Background()); err != nil {
		t.Fatal(err)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.events) != 3 {
		t.Fatalf("events = %d, want 3", len(repo.events))
	}
	if len(repo.daily) != 3 {
		t.Fatalf("daily = %d, want 3", len(repo.daily))
	}
}

func TestRecorderFlush(t *testing.T) {
	repo := &fakeUsageRepo{}
	log := slog.New(slog.NewTextHandler(discard{}, nil))
	r := NewRecorder(repo, 16, 100, time.Hour, log)
	r.Record(domain.UsageEvent{AppID: uuid.New(), KeyID: uuid.New(), Tokens: 1, Status: 200, CreatedAt: time.Now()})
	if err := r.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	repo.mu.Lock()
	n := len(repo.events)
	repo.mu.Unlock()
	if n != 1 {
		t.Fatalf("events = %d, want 1", n)
	}
	_ = r.Close(context.Background())
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
