package openai

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/camel-creatives/embeddings-api/internal/delivery/http/httpx"
	"github.com/camel-creatives/embeddings-api/internal/delivery/http/middleware"
	"github.com/camel-creatives/embeddings-api/internal/domain"
	"github.com/camel-creatives/embeddings-api/internal/platform/metrics"
	"github.com/camel-creatives/embeddings-api/internal/usecase/embed"
)

// Handler serves the OpenAI-compatible data plane.
type Handler struct {
	svc     *embed.Service
	reg     *middleware.Registry
	rec     domain.UsageRecorder
	log     *slog.Logger
	maxBody int64
	metrics *metrics.Metrics
}

// NewHandler builds the data-plane handler.
func NewHandler(svc *embed.Service, reg *middleware.Registry, rec domain.UsageRecorder, log *slog.Logger, maxBody int64, m *metrics.Metrics) *Handler {
	return &Handler{svc: svc, reg: reg, rec: rec, log: log, maxBody: maxBody, metrics: m}
}

// Register mounts the data-plane routes.
func (h *Handler) Register(r chi.Router) {
	r.Post("/v1/embeddings", h.Embed)
	r.Get("/v1/models", h.Models)
}

// Embed handles POST /v1/embeddings.
func (h *Handler) Embed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	app := httpx.App(ctx)
	key := httpx.Key(ctx)
	reqID := httpx.RequestID(ctx)

	body := http.MaxBytesReader(w, r.Body, h.maxBody)
	var req embedRequest
	dec := json.NewDecoder(body)
	if err := dec.Decode(&req); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			h.fail(w, app, key, reqID, 0, domain.Usage{}, http.StatusRequestEntityTooLarge, start, domain.ErrPayloadTooLarge)
			return
		}
		h.fail(w, app, key, reqID, 0, domain.Usage{}, http.StatusBadRequest, start, domain.ErrInvalidJSON)
		return
	}

	if req.Model != "" && req.Model != embed.ModelID {
		h.fail(w, app, key, reqID, 0, domain.Usage{}, http.StatusBadRequest, start, domain.ErrModelNotFound)
		return
	}

	items, err := parseInput(req.Input)
	if err != nil {
		de, _ := domain.AsError(err)
		h.fail(w, app, key, reqID, 0, domain.Usage{}, statusOf(de), start, err)
		return
	}

	hasMedia := itemsHaveMedia(items)
	if !h.reg.Allow(app, hasMedia) {
		h.fail(w, app, key, reqID, len(items), domain.Usage{}, http.StatusTooManyRequests, start, domain.ErrRetryAfter(domain.ErrRateLimited, 1))
		return
	}

	lim := embed.Limits{
		MaxBatch: app.MaxBatch,
		MaxMedia: app.MaxMedia,
		Allowed:  allowedMap(app),
		Bulk:     key.Class == "bulk",
	}
	preq := embed.Request{
		Items:      items,
		Task:       req.Task,
		InputType:  req.InputType,
		Title:      req.Title,
		Dimensions: req.Dimensions,
	}

	res, err := h.svc.Embed(ctx, preq, lim)
	if err != nil {
		de, _ := domain.AsError(err)
		h.fail(w, app, key, reqID, len(items), domain.Usage{}, statusOf(de), start, err)
		return
	}

	out := buildResponse(res, req.EncodingFormat)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(out)

	h.record(app, key, reqID, len(items), res.Usage, http.StatusOK, start)
}

// Models handles GET /v1/models.
func (h *Handler) Models(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"object": "list",
		"data": []map[string]any{{
			"id":       embed.ModelID,
			"object":   "model",
			"owned_by": "camel-creatives",
			"modalities": []string{
				"text", "image", "audio",
			},
			"dimensions": embed.SupportedDimensions,
		}},
	})
}

func (h *Handler) fail(w http.ResponseWriter, app *domain.App, key *domain.APIKey, reqID string, inputs int, u domain.Usage, status int, start time.Time, err error) {
	httpx.WriteOpenAIError(w, err)
	h.record(app, key, reqID, inputs, u, status, start)
}

func (h *Handler) record(app *domain.App, key *domain.APIKey, reqID string, inputs int, u domain.Usage, status int, start time.Time) {
	if app == nil {
		return
	}
	latency := int(time.Since(start).Milliseconds())
	modality := modalityLabel(u)

	if h.metrics != nil {
		h.metrics.RequestsTotal.WithLabelValues(app.Name, modality, strconv.Itoa(status)).Inc()
		h.metrics.RequestDuration.WithLabelValues(modality).Observe(time.Since(start).Seconds())
		if u.PromptTokens > 0 {
			h.metrics.TokensTotal.WithLabelValues(app.Name).Add(float64(u.PromptTokens))
		}
	}
	if h.rec != nil && key != nil {
		h.rec.Record(domain.UsageEvent{
			AppID:        app.ID,
			KeyID:        key.ID,
			RequestID:    reqID,
			Inputs:       inputs,
			Tokens:       u.PromptTokens,
			Images:       u.Images,
			AudioSeconds: u.AudioSeconds,
			VideoSeconds: u.VideoSeconds,
			LatencyMS:    latency,
			Status:       status,
			CreatedAt:    time.Now().UTC(),
		})
	}
}

// buildResponse shapes the OpenAI embeddings response.
func buildResponse(res embed.Result, format string) map[string]any {
	data := make([]map[string]any, len(res.Vectors))
	for i, v := range res.Vectors {
		var emb any = v
		if format == "base64" {
			emb = encodeBase64(v)
		}
		data[i] = map[string]any{
			"object":    "embedding",
			"index":     i,
			"embedding": emb,
		}
	}
	usage := map[string]any{
		"prompt_tokens": res.Usage.PromptTokens,
		"total_tokens":  res.Usage.TotalTokens,
	}
	if res.Usage.Images > 0 {
		usage["images"] = res.Usage.Images
	}
	if res.Usage.AudioSeconds > 0 {
		usage["audio_seconds"] = res.Usage.AudioSeconds
	}
	if res.Usage.VideoSeconds > 0 {
		usage["video_seconds"] = res.Usage.VideoSeconds
	}
	return map[string]any{
		"object": "list",
		"model":  embed.ModelID,
		"data":   data,
		"usage":  usage,
	}
}

// encodeBase64 encodes a float32 vector as little-endian base64.
func encodeBase64(v []float32) string {
	buf := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func itemsHaveMedia(items []embed.Item) bool {
	for _, it := range items {
		for _, p := range it.Parts {
			if p.Type != "text" {
				return true
			}
		}
	}
	return false
}

func allowedMap(app *domain.App) map[domain.PartKind]bool {
	m := make(map[domain.PartKind]bool, 4)
	for _, mod := range app.Modalities {
		switch mod {
		case "text":
			m[domain.PartText] = true
		case "image":
			m[domain.PartImage] = true
		case "audio":
			m[domain.PartAudio] = true
		case "video":
			m[domain.PartVideo] = true
		}
	}
	return m
}

func modalityLabel(u domain.Usage) string {
	switch {
	case u.VideoSeconds > 0:
		return "video"
	case u.AudioSeconds > 0:
		return "audio"
	case u.Images > 0:
		return "image"
	default:
		return "text"
	}
}

func statusOf(e *domain.Error) int {
	if e == nil {
		return http.StatusInternalServerError
	}
	return e.Status
}
