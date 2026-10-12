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

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = domain.ErrNotFound

// AppRepo stores apps.
type AppRepo struct{ pool *pgxpool.Pool }

// NewAppRepo builds an AppRepo.
func NewAppRepo(pool *pgxpool.Pool) *AppRepo { return &AppRepo{pool: pool} }

const appCols = `id, name, status, modalities, rps, burst, media_rps, max_batch, max_media, created_at`

// Create inserts an app.
func (r *AppRepo) Create(ctx context.Context, a *domain.App) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.Modalities == nil {
		a.Modalities = []string{"text"}
	}
	err := r.pool.QueryRow(ctx,
		`insert into apps (id, name, status, modalities, rps, burst, media_rps, max_batch, max_media)
		 values ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 returning created_at`,
		a.ID, a.Name, a.Status, a.Modalities, a.RPS, a.Burst, a.MediaRPS, a.MaxBatch, a.MaxMedia,
	).Scan(&a.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert app: %w", err)
	}
	return nil
}

// Get reads one app.
func (r *AppRepo) Get(ctx context.Context, id uuid.UUID) (*domain.App, error) {
	row := r.pool.QueryRow(ctx, `select `+appCols+` from apps where id = $1`, id)
	a, err := scanApp(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

// List returns a page of apps and the total count.
func (r *AppRepo) List(ctx context.Context, limit, offset int) ([]domain.App, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `select count(*) from apps`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count apps: %w", err)
	}
	rows, err := r.pool.Query(ctx,
		`select `+appCols+` from apps order by created_at desc limit $1 offset $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list apps: %w", err)
	}
	defer rows.Close()

	apps := make([]domain.App, 0, limit)
	for rows.Next() {
		a, err := scanApp(rows)
		if err != nil {
			return nil, 0, err
		}
		apps = append(apps, *a)
	}
	return apps, total, rows.Err()
}

// Update writes the mutable fields of an app.
func (r *AppRepo) Update(ctx context.Context, a *domain.App) error {
	tag, err := r.pool.Exec(ctx,
		`update apps set name=$2, status=$3, modalities=$4, rps=$5, burst=$6, media_rps=$7, max_batch=$8, max_media=$9
		 where id=$1`,
		a.ID, a.Name, a.Status, a.Modalities, a.RPS, a.Burst, a.MediaRPS, a.MaxBatch, a.MaxMedia)
	if err != nil {
		return fmt.Errorf("update app: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanApp(row pgx.Row) (*domain.App, error) {
	var a domain.App
	err := row.Scan(&a.ID, &a.Name, &a.Status, &a.Modalities, &a.RPS, &a.Burst, &a.MediaRPS, &a.MaxBatch, &a.MaxMedia, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}
