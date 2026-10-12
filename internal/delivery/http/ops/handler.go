package ops

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/camel-creatives/embeddings-api/internal/domain"
)

// Handler exposes operational endpoints.
type Handler struct {
	embedder domain.Embedder
	timeout  time.Duration
	testJPEG []byte
}

// New builds an ops Handler.
func New(embedder domain.Embedder) *Handler {
	return &Handler{
		embedder: embedder,
		timeout:  15 * time.Second,
		testJPEG: makeTestJPEG(),
	}
}

// Healthz reports that the process is up.
func (h *Handler) Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// Readyz checks the engine and runs a built-in text and image embed.
func (h *Handler) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	if err := h.embedder.Ready(ctx); err != nil {
		writeDown(w, "engine_unreachable")
		return
	}
	inputs := []domain.Input{
		{Parts: []domain.Part{{Kind: domain.PartText, Text: "readiness probe"}}},
	}
	if h.embedder.Supports(domain.PartImage) {
		inputs = append(inputs, domain.Input{Parts: []domain.Part{{
			Kind: domain.PartImage, Data: h.testJPEG, Mime: "image/jpeg",
		}}})
	}
	res, err := h.embedder.Embed(ctx, inputs)
	if err != nil || len(res.Vectors) != len(inputs) {
		writeDown(w, "engine_embed_failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ready"}`))
}

// Metrics serves the Prometheus scrape endpoint.
func (h *Handler) Metrics() http.Handler { return promhttp.Handler() }

func writeDown(w http.ResponseWriter, reason string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte(`{"status":"not_ready","reason":"` + reason + `"}`))
}

// makeTestJPEG returns a tiny valid JPEG for the readiness probe.
func makeTestJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 30), G: uint8(y * 30), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil
	}
	return buf.Bytes()
}
