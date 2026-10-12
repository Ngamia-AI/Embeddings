package python

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/camel-creatives/embeddings-api/internal/domain"
)

func TestEmbedSendsVideoDataURI(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_, _ = w.Write([]byte(`{
			"object":"list","model":"embeddinggemma-2",
			"data":[{"object":"embedding","index":0,"embedding":[1,2,3]}],
			"usage":{"prompt_tokens":140,"total_tokens":140,"video_seconds":1.0}
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL, time.Second, time.Second)
	res, err := c.Embed(context.Background(), []domain.Input{
		{Parts: []domain.Part{{Kind: domain.PartVideo, Data: []byte("\x00\x00\x00\x18ftypmp42"), Mime: "video/mp4"}}},
	})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(res.Vectors) != 1 || len(res.Vectors[0]) != 3 {
		t.Fatalf("unexpected vectors: %v", res.Vectors)
	}
	if res.Usage.VideoSeconds != 1.0 {
		t.Fatalf("video seconds = %v", res.Usage.VideoSeconds)
	}

	items, ok := captured["input"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("input not an item array: %v", captured["input"])
	}
	item, _ := items[0].(map[string]any)
	parts, _ := item["content"].([]any)
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %v", parts)
	}
	part, _ := parts[0].(map[string]any)
	if part["type"] != "video_url" {
		t.Fatalf("part type = %v", part["type"])
	}
	video, _ := part["video_url"].(map[string]any)
	url, _ := video["url"].(string)
	if !strings.HasPrefix(url, "data:video/mp4;base64,") {
		t.Fatalf("video url = %q", url)
	}
}

func TestEmbedMapsOversize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	}))
	defer srv.Close()

	c := New(srv.URL, time.Second, time.Second)
	_, err := c.Embed(context.Background(), []domain.Input{
		{Parts: []domain.Part{{Kind: domain.PartVideo, Data: []byte("x"), Mime: "video/mp4"}}},
	})
	de, ok := domain.AsError(err)
	if !ok || de.Code != "payload_too_large" {
		t.Fatalf("error = %v", err)
	}
}

func TestSupportsCoversVideoNotAudio(t *testing.T) {
	c := New("http://example", time.Second, time.Second)
	if !c.Supports(domain.PartVideo) || !c.Supports(domain.PartText) || !c.Supports(domain.PartImage) {
		t.Fatal("expected text, image and video support")
	}
	if c.Supports(domain.PartAudio) {
		t.Fatal("audio must stay on the llama.cpp path")
	}
}

func TestReady(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, time.Second, time.Second)
	if err := c.Ready(context.Background()); err != nil {
		t.Fatalf("ready: %v", err)
	}
}
