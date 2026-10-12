package apps

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/camel-creatives/embeddings-api/internal/domain"
)

// Invalidator drops a cached key when it is revoked.
type Invalidator interface {
	InvalidateByID(id uuid.UUID)
}

// Service implements app and key administration.
type Service struct {
	apps domain.AppRepository
	keys domain.KeyAdminRepository
	inv  Invalidator
}

// New builds an apps Service.
func New(apps domain.AppRepository, keys domain.KeyAdminRepository, inv Invalidator) *Service {
	return &Service{apps: apps, keys: keys, inv: inv}
}

var validModalities = map[string]bool{"text": true, "image": true, "audio": true, "video": true}

// CreateApp validates and stores a new app.
func (s *Service) CreateApp(ctx context.Context, a *domain.App) error {
	if errs := Validate(a, true); errs != nil {
		return errs
	}
	if a.Status == "" {
		a.Status = "active"
	}
	return s.apps.Create(ctx, a)
}

// GetApp reads one app.
func (s *Service) GetApp(ctx context.Context, id uuid.UUID) (*domain.App, error) {
	return s.apps.Get(ctx, id)
}

// ListApps returns a page of apps.
func (s *Service) ListApps(ctx context.Context, limit, offset int) ([]domain.App, int, error) {
	return s.apps.List(ctx, limit, offset)
}

// UpdateApp validates and writes app changes.
func (s *Service) UpdateApp(ctx context.Context, a *domain.App) error {
	if errs := Validate(a, false); errs != nil {
		return errs
	}
	return s.apps.Update(ctx, a)
}

// IssueKey creates a new plaintext key for an app and persists its hash.
func (s *Service) IssueKey(ctx context.Context, appID uuid.UUID, class string) (string, *domain.APIKey, error) {
	if _, err := s.apps.Get(ctx, appID); err != nil {
		return "", nil, err
	}
	if class == "" {
		class = "live"
	}
	if class != "live" && class != "bulk" {
		return "", nil, domain.MultiValidation{{Field: "class", Code: "oneof", Message: "class must be live or bulk"}}
	}
	plaintext, hash, prefix, err := newKey(class)
	if err != nil {
		return "", nil, fmt.Errorf("generate key: %w", err)
	}
	k := &domain.APIKey{
		ID:      uuid.New(),
		AppID:   appID,
		Prefix:  prefix,
		KeyHash: hash,
		Class:   class,
		Status:  "active",
	}
	if err := s.keys.Issue(ctx, k); err != nil {
		return "", nil, err
	}
	return plaintext, k, nil
}

// RevokeKey revokes a key and drops it from the cache.
func (s *Service) RevokeKey(ctx context.Context, id uuid.UUID) error {
	if err := s.keys.Revoke(ctx, id); err != nil {
		return err
	}
	if s.inv != nil {
		s.inv.InvalidateByID(id)
	}
	return nil
}

// Validate checks app fields. create requires a name.
func Validate(a *domain.App, create bool) domain.MultiValidation {
	var errs domain.MultiValidation
	if create && a.Name == "" {
		errs = append(errs, domain.ValidationError{Field: "name", Code: "required", Message: "name is required"})
	}
	if a.Status != "" && a.Status != "active" && a.Status != "disabled" {
		errs = append(errs, domain.ValidationError{Field: "status", Code: "oneof", Message: "status must be active or disabled"})
	}
	for _, m := range a.Modalities {
		if !validModalities[m] {
			errs = append(errs, domain.ValidationError{Field: "modalities", Code: "oneof", Message: "modality must be text, image, audio or video"})
			break
		}
	}
	minPositive := func(field string, v int) {
		if v < 1 {
			errs = append(errs, domain.ValidationError{Field: field, Code: "min", Message: field + " must be >= 1"})
		}
	}
	minPositive("rps", a.RPS)
	minPositive("burst", a.Burst)
	minPositive("media_rps", a.MediaRPS)
	minPositive("max_batch", a.MaxBatch)
	minPositive("max_media", a.MaxMedia)
	if len(errs) > 0 {
		return errs
	}
	return nil
}
