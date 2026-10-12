package middleware

import (
	"sync"

	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/camel-creatives/embeddings-api/internal/domain"
)

type appLimiters struct {
	general  *rate.Limiter
	media    *rate.Limiter
	rps      int
	burst    int
	mediaRPS int
}

// Registry keeps per-app rate limiters, rebuilt when limits change.
type Registry struct {
	mu sync.Mutex
	m  map[uuid.UUID]*appLimiters
}

// NewRegistry builds an empty registry.
func NewRegistry() *Registry {
	return &Registry{m: make(map[uuid.UUID]*appLimiters)}
}

func (r *Registry) get(app *domain.App) *appLimiters {
	r.mu.Lock()
	defer r.mu.Unlock()

	l := r.m[app.ID]
	if l == nil || l.rps != app.RPS || l.burst != app.Burst || l.mediaRPS != app.MediaRPS {
		burst := app.Burst
		if burst < 1 {
			burst = 1
		}
		mediaRPS := app.MediaRPS
		if mediaRPS < 1 {
			mediaRPS = 1
		}
		l = &appLimiters{
			general:  rate.NewLimiter(rate.Limit(app.RPS), burst),
			media:    rate.NewLimiter(rate.Limit(mediaRPS), mediaRPS),
			rps:      app.RPS,
			burst:    app.Burst,
			mediaRPS: app.MediaRPS,
		}
		r.m[app.ID] = l
	}
	return l
}

// Allow reports whether the app may proceed, applying the media limiter too
// when the request carries media.
func (r *Registry) Allow(app *domain.App, hasMedia bool) bool {
	l := r.get(app)
	if !l.general.Allow() {
		return false
	}
	if hasMedia && !l.media.Allow() {
		return false
	}
	return true
}
