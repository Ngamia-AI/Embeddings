package cache

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/camel-creatives/embeddings-api/internal/domain"
)

type countingRepo struct{ calls int }

func (c *countingRepo) FindByHash(context.Context, []byte) (*domain.APIKey, *domain.App, error) {
	c.calls++
	k := &domain.APIKey{ID: uuid.New(), Status: "active"}
	a := &domain.App{ID: uuid.New(), Name: "app", Status: "active"}
	return k, a, nil
}

func (c *countingRepo) TouchLastUsed(context.Context, uuid.UUID) error { return nil }

func TestKeyCacheCaches(t *testing.T) {
	repo := &countingRepo{}
	c := NewKeyCache(repo, time.Minute)
	ctx := context.Background()

	if _, _, err := c.FindByHash(ctx, []byte("hash")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.FindByHash(ctx, []byte("hash")); err != nil {
		t.Fatal(err)
	}
	if repo.calls != 1 {
		t.Fatalf("repo calls = %d, want 1 (second served from cache)", repo.calls)
	}
}

func TestKeyCacheInvalidateByID(t *testing.T) {
	repo := &countingRepo{}
	c := NewKeyCache(repo, time.Minute)
	ctx := context.Background()

	key, _, _ := c.FindByHash(ctx, []byte("hash"))
	c.InvalidateByID(key.ID)
	if _, _, err := c.FindByHash(ctx, []byte("hash")); err != nil {
		t.Fatal(err)
	}
	if repo.calls != 2 {
		t.Fatalf("repo calls = %d, want 2 after invalidation", repo.calls)
	}
}
