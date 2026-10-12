package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/camel-creatives/embeddings-api/internal/delivery/http/httpx"
	"github.com/camel-creatives/embeddings-api/internal/domain"
	"github.com/camel-creatives/embeddings-api/internal/usecase/apps"
	"github.com/camel-creatives/embeddings-api/internal/usecase/usage"
)

// Handler serves the control plane.
type Handler struct {
	apps     *apps.Service
	usageSvc *usage.Service
}

// NewHandler builds the admin handler.
func NewHandler(appsSvc *apps.Service, usageSvc *usage.Service) *Handler {
	return &Handler{apps: appsSvc, usageSvc: usageSvc}
}

// Register mounts the control-plane routes. The caller wraps this group with
// the admin auth middleware.
func (h *Handler) Register(r chi.Router) {
	r.Post("/admin/v1/apps", h.createApp)
	r.Get("/admin/v1/apps", h.listApps)
	r.Get("/admin/v1/apps/{id}", h.getApp)
	r.Patch("/admin/v1/apps/{id}", h.patchApp)
	r.Post("/admin/v1/apps/{id}/keys", h.issueKey)
	r.Delete("/admin/v1/keys/{id}", h.revokeKey)
	r.Get("/admin/v1/apps/{id}/usage", h.usage)
}

type appRequest struct {
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Modalities []string `json:"modalities"`
	RPS        int      `json:"rps"`
	Burst      int      `json:"burst"`
	MediaRPS   int      `json:"media_rps"`
	MaxBatch   int      `json:"max_batch"`
	MaxMedia   int      `json:"max_media"`
}

func (h *Handler) createApp(w http.ResponseWriter, r *http.Request) {
	var req appRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Invalid JSON", nil)
		return
	}
	a := &domain.App{
		ID:         uuid.New(),
		Name:       req.Name,
		Status:     orDefault(req.Status, "active"),
		Modalities: orStrings(req.Modalities, []string{"text"}),
		RPS:        orInt(req.RPS, 10),
		Burst:      orInt(req.Burst, 20),
		MediaRPS:   orInt(req.MediaRPS, 2),
		MaxBatch:   orInt(req.MaxBatch, 32),
		MaxMedia:   orInt(req.MaxMedia, 8),
	}
	if err := h.apps.CreateApp(r.Context(), a); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteSuccess(w, http.StatusCreated, toResponse(a), "App created", nil)
}

func (h *Handler) listApps(w http.ResponseWriter, r *http.Request) {
	page := atoiDefault(r.URL.Query().Get("page"), 1)
	perPage := atoiDefault(r.URL.Query().Get("per_page"), 20)
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	list, total, err := h.apps.ListApps(r.Context(), perPage, (page-1)*perPage)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "Failed to list apps", nil)
		return
	}
	resp := make([]map[string]any, len(list))
	for i := range list {
		resp[i] = toResponse(&list[i])
	}
	httpx.WriteSuccess(w, http.StatusOK, resp, "OK", map[string]any{
		"page":     page,
		"per_page": perPage,
		"total":    total,
	})
}

func (h *Handler) getApp(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	a, err := h.apps.GetApp(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteSuccess(w, http.StatusOK, toResponse(a), "OK", nil)
}

func (h *Handler) patchApp(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	a, err := h.apps.GetApp(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	var req appRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Invalid JSON", nil)
		return
	}
	if req.Name != "" {
		a.Name = req.Name
	}
	if req.Status != "" {
		a.Status = req.Status
	}
	if req.Modalities != nil {
		a.Modalities = req.Modalities
	}
	if req.RPS != 0 {
		a.RPS = req.RPS
	}
	if req.Burst != 0 {
		a.Burst = req.Burst
	}
	if req.MediaRPS != 0 {
		a.MediaRPS = req.MediaRPS
	}
	if req.MaxBatch != 0 {
		a.MaxBatch = req.MaxBatch
	}
	if req.MaxMedia != 0 {
		a.MaxMedia = req.MaxMedia
	}
	if err := h.apps.UpdateApp(r.Context(), a); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteSuccess(w, http.StatusOK, toResponse(a), "App updated", nil)
}

func (h *Handler) issueKey(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct {
		Class string `json:"class"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	plaintext, key, err := h.apps.IssueKey(r.Context(), id, req.Class)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteSuccess(w, http.StatusCreated, map[string]any{
		"id":     key.ID,
		"app_id": key.AppID,
		"prefix": key.Prefix,
		"class":  key.Class,
		"key":    plaintext,
	}, "Key created. Store it now; it cannot be shown again.", nil)
}

func (h *Handler) revokeKey(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Invalid id", nil)
		return
	}
	if err := h.apps.RevokeKey(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteSuccess(w, http.StatusOK, nil, "Key revoked", nil)
}

func (h *Handler) usage(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	from, to, err := parseRange(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	rollups, err := h.usageSvc.Rollups(r.Context(), id, from, to)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "Failed to read usage", nil)
		return
	}
	httpx.WriteSuccess(w, http.StatusOK, rollups, "OK", map[string]any{
		"from": from.Format("2006-01-02"),
		"to":   to.Format("2006-01-02"),
	})
}

// toResponse shapes an app for the control plane.
func toResponse(a *domain.App) map[string]any {
	return map[string]any{
		"id":         a.ID,
		"name":       a.Name,
		"status":     a.Status,
		"modalities": a.Modalities,
		"rps":        a.RPS,
		"burst":      a.Burst,
		"media_rps":  a.MediaRPS,
		"max_batch":  a.MaxBatch,
		"max_media":  a.MaxMedia,
		"created_at": a.CreatedAt,
	}
}

func writeServiceError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	var mv domain.MultiValidation
	if errors.As(err, &mv) {
		httpx.WriteValidation(w, mv)
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "Not found", nil)
		return
	}
	if httpErr, ok := domain.AsError(err); ok {
		httpx.WriteError(w, httpErr.Status, httpErr.Message, nil)
		return
	}
	httpx.WriteError(w, http.StatusInternalServerError, "Internal error", nil)
}

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Invalid id", nil)
		return uuid.Nil, false
	}
	return id, true
}

func parseRange(from, to string) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	if to == "" {
		to = now.Format("2006-01-02")
	}
	if from == "" {
		from = now.AddDate(0, 0, -30).Format("2006-01-02")
	}
	f, err := time.Parse("2006-01-02", from)
	if err != nil {
		return time.Time{}, time.Time{}, errInvalidRange
	}
	t, err := time.Parse("2006-01-02", to)
	if err != nil {
		return time.Time{}, time.Time{}, errInvalidRange
	}
	return f, t, nil
}

var errInvalidRange = &rangeError{}

type rangeError struct{}

func (*rangeError) Error() string { return "from and to must be YYYY-MM-DD" }

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func orStrings(v, def []string) []string {
	if len(v) == 0 {
		return def
	}
	return v
}

func orInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
