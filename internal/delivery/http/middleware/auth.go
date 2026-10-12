package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/camel-creatives/embeddings-api/internal/delivery/http/httpx"
	"github.com/camel-creatives/embeddings-api/internal/domain"
	"github.com/google/uuid"
)

// Authenticator validates bearer keys against a key repository.
type Authenticator struct {
	keys domain.KeyRepository
}

// NewAuthenticator builds the auth middleware.
func NewAuthenticator(keys domain.KeyRepository) *Authenticator {
	return &Authenticator{keys: keys}
}

// Middleware rejects requests without a valid, active key and injects the app.
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if token == "" {
			httpx.WriteOpenAIError(w, domain.ErrInvalidAPIKey)
			return
		}
		sum := sha256.Sum256([]byte(token))
		key, app, err := a.keys.FindByHash(r.Context(), sum[:])
		if err != nil || key == nil || app == nil || key.Status != "active" || app.Status != "active" {
			httpx.WriteOpenAIError(w, domain.ErrInvalidAPIKey)
			return
		}
		go a.touch(key.ID)
		next.ServeHTTP(w, r.WithContext(httpx.WithAppKey(r.Context(), app, key)))
	})
}

func (a *Authenticator) touch(id uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = a.keys.TouchLastUsed(ctx, id)
}

// AdminAuth guards the control plane with a shared admin token.
type AdminAuth struct {
	token string
}

// NewAdminAuth builds the admin auth middleware.
func NewAdminAuth(token string) *AdminAuth { return &AdminAuth{token: token} }

// Middleware rejects requests that do not present the admin token.
func (m *AdminAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := bearer(r)
		if got == "" {
			got = r.Header.Get("X-Admin-Token")
		}
		if m.token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(m.token)) != 1 {
			httpx.WriteError(w, http.StatusUnauthorized, "Unauthorized", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const p = "Bearer "
	if len(h) > len(p) && strings.EqualFold(h[:len(p)], p) {
		return strings.TrimSpace(h[len(p):])
	}
	return ""
}
