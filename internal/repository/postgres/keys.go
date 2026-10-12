package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/camel-creatives/embeddings-api/internal/domain"
)

// KeyRepo stores API keys and resolves them for the data plane.
type KeyRepo struct{ pool *pgxpool.Pool }

// NewKeyRepo builds a KeyRepo.
func NewKeyRepo(pool *pgxpool.Pool) *KeyRepo { return &KeyRepo{pool: pool} }

// FindByHash resolves a key hash to its key and owning app. The key is returned
// regardless of status so callers can distinguish revoked from unknown.
func (r *KeyRepo) FindByHash(ctx context.Context, hash []byte) (*domain.APIKey, *domain.App, error) {
	row := r.pool.QueryRow(ctx,
		`select k.id, k.app_id, k.prefix, k.key_hash, k.class, k.status, k.last_used_at, k.created_at, k.revoked_at,
		        a.id, a.name, a.status, a.modalities, a.rps, a.burst, a.media_rps, a.max_batch, a.max_media, a.created_at
		 from api_keys k join apps a on a.id = k.app_id
		 where k.key_hash = $1`, hash)

	var k domain.APIKey
	var a domain.App
	err := row.Scan(
		&k.ID, &k.AppID, &k.Prefix, &k.KeyHash, &k.Class, &k.Status, &k.LastUsedAt, &k.CreatedAt, &k.RevokedAt,
		&a.ID, &a.Name, &a.Status, &a.Modalities, &a.RPS, &a.Burst, &a.MediaRPS, &a.MaxBatch, &a.MaxMedia, &a.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("find key: %w", err)
	}
	return &k, &a, nil
}

// TouchLastUsed updates last_used_at.
func (r *KeyRepo) TouchLastUsed(ctx context.Context, keyID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `update api_keys set last_used_at = now() where id = $1`, keyID)
	return err
}

// Issue inserts a new key.
func (r *KeyRepo) Issue(ctx context.Context, k *domain.APIKey) error {
	if k.ID == uuid.Nil {
		k.ID = uuid.New()
	}
	err := r.pool.QueryRow(ctx,
		`insert into api_keys (id, app_id, prefix, key_hash, class, status)
		 values ($1,$2,$3,$4,$5,$6) returning created_at`,
		k.ID, k.AppID, k.Prefix, k.KeyHash, k.Class, k.Status,
	).Scan(&k.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert key: %w", err)
	}
	return nil
}

// Revoke marks a key revoked.
func (r *KeyRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`update api_keys set status='revoked', revoked_at=now() where id=$1 and status='active'`, id)
	if err != nil {
		return fmt.Errorf("revoke key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
