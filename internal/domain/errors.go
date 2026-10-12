package domain

import (
	"errors"
	"fmt"
	"net/http"
)

// Error is a domain error carrying enough information to render an
// OpenAI-shaped data-plane error. Control-plane handlers only use Code.
type Error struct {
	Status     int
	Type       string
	Code       string
	Message    string
	RetryAfter int // seconds, only meaningful for 429
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// WithMessage returns a copy of the error with a different message.
func (e *Error) WithMessage(msg string) *Error {
	c := *e
	c.Message = msg
	return &c
}

// HTTPStatus lets the error writer treat *Error as an HTTPStatus carrier.
func (e *Error) HTTPStatus() int { return e.Status }

func newErr(status int, typ, code, msg string) *Error {
	return &Error{Status: status, Type: typ, Code: code, Message: msg}
}

// Data-plane error constructors. Kept here so every layer speaks one vocabulary.
var (
	ErrInvalidJSON     = newErr(http.StatusBadRequest, "invalid_request_error", "invalid_json", "Request body is not valid JSON")
	ErrEmptyInput      = newErr(http.StatusBadRequest, "invalid_request_error", "empty_input", "input must contain at least one item")
	ErrBatchTooLarge   = newErr(http.StatusBadRequest, "invalid_request_error", "batch_too_large", "Batch exceeds the maximum number of items")
	ErrTooManyMedia    = newErr(http.StatusBadRequest, "invalid_request_error", "too_many_media_items", "Request contains too many media items")
	ErrInvalidDims     = newErr(http.StatusBadRequest, "invalid_request_error", "invalid_dimensions", "dimensions must be one of 768, 512, 256 or 128")
	ErrContextTooLong  = newErr(http.StatusBadRequest, "invalid_request_error", "context_length_exceeded", "Item exceeds the token budget")
	ErrUnsupportedIn   = newErr(http.StatusBadRequest, "invalid_request_error", "unsupported_input", "Input is not supported by this service")
	ErrUnsupportedMime = newErr(http.StatusBadRequest, "invalid_request_error", "unsupported_media_type", "Declared media type does not match the content")
	ErrInvalidMedia    = newErr(http.StatusBadRequest, "invalid_request_error", "invalid_media", "Media could not be decoded")
	ErrImageTooLarge   = newErr(http.StatusBadRequest, "invalid_request_error", "image_too_large", "Image exceeds the size or pixel limit")
	ErrAudioTooLong    = newErr(http.StatusBadRequest, "invalid_request_error", "audio_too_long", "Audio is longer than 120 seconds")
	ErrPayloadTooLarge = newErr(http.StatusRequestEntityTooLarge, "invalid_request_error", "payload_too_large", "Request body exceeds the size limit")

	ErrInvalidAPIKey  = newErr(http.StatusUnauthorized, "authentication_error", "invalid_api_key", "Invalid or revoked API key")
	ErrModalityDenied = newErr(http.StatusForbidden, "permission_error", "modality_not_allowed", "This key is not allowed to use this modality")
	ErrRateLimited    = newErr(http.StatusTooManyRequests, "rate_limit_error", "rate_limit_exceeded", "Rate limit exceeded")
	ErrServerBusy     = newErr(http.StatusTooManyRequests, "rate_limit_error", "server_busy", "No engine slot became available in time")
	ErrEngineUnavail  = newErr(http.StatusServiceUnavailable, "server_error", "engine_unavailable", "Embedding engine is unavailable")
	ErrEngineTimeout  = newErr(http.StatusGatewayTimeout, "server_error", "engine_timeout", "Embedding engine timed out")
	ErrModelNotFound  = newErr(http.StatusBadRequest, "invalid_request_error", "model_not_found", "Unknown model")
	ErrInternal       = newErr(http.StatusInternalServerError, "server_error", "internal_error", "Internal server error")
)

// ErrRetryAfter builds a 429 carrying a Retry-After hint.
func ErrRetryAfter(base *Error, seconds int) *Error {
	c := *base
	c.RetryAfter = seconds
	return &c
}

// ErrNotFound is the storage sentinel for a missing row.
var ErrNotFound = errors.New("not found")

// AsError extracts a *Error if err is or wraps one.
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// Validation error for the control plane (Camel envelope).
type ValidationError struct {
	Field   string
	Code    string
	Message string
}

func (v ValidationError) Error() string { return v.Message }

// MultiValidation aggregates control-plane field errors.
type MultiValidation []ValidationError

func (m MultiValidation) Error() string { return "Validation failed" }
