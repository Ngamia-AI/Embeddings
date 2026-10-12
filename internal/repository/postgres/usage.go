package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/camel-creatives/embeddings-api/internal/domain"
)

// UsageRepo persists usage events and reads rollups.
type UsageRepo struct{ pool *pgxpool.Pool }

// NewUsageRepo builds a UsageRepo.
func NewUsageRepo(pool *pgxpool.Pool) *UsageRepo { return &UsageRepo{pool: pool} }

// InsertEvents bulk-inserts raw events.
func (r *UsageRepo) InsertEvents(ctx context.Context, events []domain.UsageEvent) error {
	if len(events) == 0 {
		return nil
	}
	rows := make([][]any, len(events))
	for i, e := range events {
		rows[i] = []any{
			e.AppID, e.KeyID, e.RequestID, e.Inputs, e.Tokens,
			e.Images, e.AudioSeconds, e.VideoSeconds, e.LatencyMS, statusOrDefault(e.Status), e.CreatedAt,
		}
	}
	_, err := r.pool.CopyFrom(ctx,
		pgx.Identifier{"usage_events"},
		[]string{"app_id", "key_id", "request_id", "inputs", "tokens", "images", "audio_seconds", "video_seconds", "latency_ms", "status", "created_at"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("copy usage events: %w", err)
	}
	return nil
}

// UpsertDaily aggregates events by (app, day) and adds them to usage_daily.
func (r *UsageRepo) UpsertDaily(ctx context.Context, events []domain.UsageEvent) error {
	if len(events) == 0 {
		return nil
	}
	type key struct {
		app uuid.UUID
		day string
	}
	agg := map[key]*domain.UsageDaily{}
	for _, e := range events {
		k := key{app: e.AppID, day: e.CreatedAt.UTC().Format("2006-01-02")}
		d := agg[k]
		if d == nil {
			d = &domain.UsageDaily{AppID: e.AppID, Day: k.day}
			agg[k] = d
		}
		d.Requests++
		d.Inputs += int64(e.Inputs)
		d.Tokens += int64(e.Tokens)
		d.Images += int64(e.Images)
		d.AudioSeconds += e.AudioSeconds
		d.VideoSeconds += e.VideoSeconds
		if e.Status >= 400 {
			d.Errors++
		}
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, d := range agg {
		_, err := tx.Exec(ctx,
			`insert into usage_daily (app_id, day, requests, inputs, tokens, images, audio_seconds, video_seconds, errors)
			 values ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			 on conflict (app_id, day) do update set
			   requests = usage_daily.requests + excluded.requests,
			   inputs = usage_daily.inputs + excluded.inputs,
			   tokens = usage_daily.tokens + excluded.tokens,
			   images = usage_daily.images + excluded.images,
			   audio_seconds = usage_daily.audio_seconds + excluded.audio_seconds,
			   video_seconds = usage_daily.video_seconds + excluded.video_seconds,
			   errors = usage_daily.errors + excluded.errors`,
			d.AppID, d.Day, d.Requests, d.Inputs, d.Tokens, d.Images, d.AudioSeconds, d.VideoSeconds, d.Errors)
		if err != nil {
			return fmt.Errorf("upsert daily: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// Rollups returns daily rows for an app over [from, to] inclusive.
func (r *UsageRepo) Rollups(ctx context.Context, appID uuid.UUID, from, to time.Time) ([]domain.UsageDaily, error) {
	rows, err := r.pool.Query(ctx,
		`select app_id, to_char(day,'YYYY-MM-DD'), requests, inputs, tokens, images, audio_seconds, video_seconds, errors
		 from usage_daily where app_id=$1 and day between $2 and $3 order by day`,
		appID, from, to)
	if err != nil {
		return nil, fmt.Errorf("query rollups: %w", err)
	}
	defer rows.Close()

	out := []domain.UsageDaily{}
	for rows.Next() {
		var d domain.UsageDaily
		if err := rows.Scan(&d.AppID, &d.Day, &d.Requests, &d.Inputs, &d.Tokens, &d.Images, &d.AudioSeconds, &d.VideoSeconds, &d.Errors); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// PurgeEventsBefore deletes raw events older than the cutoff.
func (r *UsageRepo) PurgeEventsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, `delete from usage_events where created_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge events: %w", err)
	}
	return tag.RowsAffected(), nil
}

func statusOrDefault(s int) int {
	if s == 0 {
		return 200
	}
	return s
}
