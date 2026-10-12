package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/camel-creatives/embeddings-api/internal/delivery/http/httpx"
)

// RequestID assigns an X-Request-ID to every request.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(httpx.WithRequestID(r.Context(), id)))
	})
}

// Inflight tracks the in-flight gauge.
func Inflight(gauge interface {
	Inc()
	Dec()
}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gauge.Inc()
			defer gauge.Dec()
			next.ServeHTTP(w, r)
		})
	}
}

// statusRecorder captures the response status.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// Logging logs one structured line per request, never logging content.
func Logging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			app := httpx.App(r.Context())
			attrs := []any{
				"request_id", httpx.RequestID(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"bytes", rec.bytes,
				"ms", time.Since(start).Milliseconds(),
			}
			if app != nil {
				attrs = append(attrs, "app", app.Name)
			}
			if key := httpx.Key(r.Context()); key != nil {
				attrs = append(attrs, "key_prefix", key.Prefix)
			}
			log.Info("request", attrs...)
		})
	}
}

// Recover turns panics into 500s.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic", "request_id", httpx.RequestID(r.Context()), "panic", rec)
					httpx.WriteOpenAIError(w, nil)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
