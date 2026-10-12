package cache

import (
	"context"
	"encoding/hex"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/camel-creatives/embeddings-api/internal/domain"
)

type entry struct {
	key *domain.APIKey
	app *domain.App
	exp time.Time
}

// KeyCache is a TTL cache in front of a domain.KeyRepository.
type KeyCache struct {
	repo domain.KeyRepository
	ttl  time.Duration

	mu     sync.RWMutex
	byHash map[string]entry
	byID   map[uuid.UUID]string
}

// NewKeyCache wraps repo with a TTL cache.
func NewKeyCache(repo domain.KeyRepository, ttl time.Duration) *KeyCache {
	return &KeyCache{
		repo:   repo,
		ttl:    ttl,
		byHash: make(map[string]entry),
		byID:   make(map[uuid.UUID]string),
	}
}

// FindByHash returns the cached key/app, falling back to the repository.
func (c *KeyCache) FindByHash(ctx context.Context, hash []byte) (*domain.APIKey, *domain.App, error) {
	h := hex.EncodeToString(hash)

	c.mu.RLock()
	e, ok := c.byHash[h]
	c.mu.RUnlock()
	if ok && time.Now().Before(e.exp) {
		return e.key, e.app, nil
	}

	key, app, err := c.repo.FindByHash(ctx, hash)
	if err != nil {
		return nil, nil, err
	}
	c.mu.Lock()
	c.byHash[h] = entry{key: key, app: app, exp: time.Now().Add(c.ttl)}
	c.byID[key.ID] = h
	c.mu.Unlock()
	return key, app, nil
}

// TouchLastUsed forwards to the repository.
func (c *KeyCache) TouchLastUsed(ctx context.Context, keyID uuid.UUID) error {
	return c.repo.TouchLastUsed(ctx, keyID)
}

// InvalidateByID drops a cached key by its id (used on revoke).
func (c *KeyCache) InvalidateByID(id uuid.UUID) {
	c.mu.Lock()
	if h, ok := c.byID[id]; ok {
		delete(c.byHash, h)
		delete(c.byID, id)
	}
	c.mu.Unlock()
}
