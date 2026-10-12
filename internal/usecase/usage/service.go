package usage

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/camel-creatives/embeddings-api/internal/domain"
)

// Service answers control-plane usage queries.
type Service struct {
	repo domain.UsageRepository
}

// NewService builds a usage Service.
func NewService(repo domain.UsageRepository) *Service { return &Service{repo: repo} }

// Rollups returns daily usage for an app over [from, to].
func (s *Service) Rollups(ctx context.Context, appID uuid.UUID, from, to time.Time) ([]domain.UsageDaily, error) {
	return s.repo.Rollups(ctx, appID, from, to)
}

// PurgeOldEvents deletes raw events older than the retention window.
func (s *Service) PurgeOldEvents(ctx context.Context, retention time.Duration) (int64, error) {
	return s.repo.PurgeEventsBefore(ctx, time.Now().Add(-retention))
}
