package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/camel-creatives/embeddings-api/internal/domain"
)

type ctxKey int

const (
	ctxRequestID ctxKey = iota
	ctxApp
	ctxKeyObj
)

// WithRequestID stores a request id in the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxRequestID, id)
}

// RequestID returns the request id, or "" if absent.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxRequestID).(string); ok {
		return v
	}
	return ""
}

// WithAppKey stores the authenticated app and key in the context.
func WithAppKey(ctx context.Context, app *domain.App, key *domain.APIKey) context.Context {
	ctx = context.WithValue(ctx, ctxApp, app)
	return context.WithValue(ctx, ctxKeyObj, key)
}

// App returns the authenticated app, or nil.
func App(ctx context.Context) *domain.App {
	if v, ok := ctx.Value(ctxApp).(*domain.App); ok {
		return v
	}
	return nil
}

// Key returns the authenticated key, or nil.
func Key(ctx context.Context) *domain.APIKey {
	if v, ok := ctx.Value(ctxKeyObj).(*domain.APIKey); ok {
		return v
	}
	return nil
}

// WriteOpenAIError renders an OpenAI-shaped error body.
func WriteOpenAIError(w http.ResponseWriter, err error) {
	de, ok := domain.AsError(err)
	if !ok {
		de = domain.ErrInternal
	}
	if de.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(de.RetryAfter))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(de.Status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": de.Message,
			"type":    de.Type,
			"code":    de.Code,
		},
	})
}

// Envelope is the standard Camel control-plane response shape.
type Envelope struct {
	Success bool           `json:"success"`
	Data    any            `json:"data"`
	Message string         `json:"message"`
	Errors  any            `json:"errors"`
	Meta    map[string]any `json:"meta"`
}

// FieldError is one validation error in the envelope.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteJSON writes an envelope with the given status.
func WriteJSON(w http.ResponseWriter, status int, env Envelope) {
	if env.Meta == nil {
		env.Meta = map[string]any{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(env)
}

// WriteSuccess writes a success envelope.
func WriteSuccess(w http.ResponseWriter, status int, data any, message string, meta map[string]any) {
	WriteJSON(w, status, Envelope{Success: true, Data: data, Message: message, Errors: nil, Meta: meta})
}

// WriteError writes a failure envelope, mapping domain errors to codes.
func WriteError(w http.ResponseWriter, status int, message string, errs any) {
	WriteJSON(w, status, Envelope{Success: false, Data: nil, Message: message, Errors: errs})
}

// WriteValidation renders a domain.MultiValidation as an envelope.
func WriteValidation(w http.ResponseWriter, mv domain.MultiValidation) {
	errs := make([]FieldError, 0, len(mv))
	for _, e := range mv {
		errs = append(errs, FieldError{Field: e.Field, Code: e.Code, Message: e.Message})
	}
	WriteError(w, http.StatusBadRequest, "Validation failed", errs)
}
