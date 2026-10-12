package llamacpp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/camel-creatives/embeddings-api/internal/domain"
)

func TestEmbedParsesVectorsInOrder(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_, _ = w.Write([]byte(`{
			"object":"list","model":"embeddinggemma-2",
			"data":[
				{"object":"embedding","index":1,"embedding":[3,4]},
				{"object":"embedding","index":0,"embedding":[1,2]}
			],
			"usage":{"prompt_tokens":42,"total_tokens":42}
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL, time.Second, time.Second)
	res, err := c.Embed(context.Background(), []domain.Input{
		{Parts: []domain.Part{{Kind: domain.PartText, Text: "first"}}},
		{Parts: []domain.Part{{Kind: domain.PartImage, Data: []byte{0xFF, 0xD8, 0xFF}, Mime: "image/jpeg"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Vectors) != 2 {
		t.Fatalf("vectors = %d", len(res.Vectors))
	}
	if res.Vectors[0][0] != 1 || res.Vectors[1][0] != 3 {
		t.Fatalf("order not preserved: %v", res.Vectors)
	}
	if res.Usage.PromptTokens != 42 {
		t.Fatalf("tokens = %d", res.Usage.PromptTokens)
	}
	if captured["input"] == nil {
		t.Fatal("input not captured")
	}
}

func TestEmbedBuildsDataURI(t *testing.T) {
	var raw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		raw = string(b)
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1]}],"usage":{"prompt_tokens":1}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, time.Second, time.Second)
	_, err := c.Embed(context.Background(), []domain.Input{
		{Parts: []domain.Part{{Kind: domain.PartImage, Data: []byte{0xFF, 0xD8, 0xFF}, Mime: "image/jpeg"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "data:image/jpeg;base64,") {
		t.Fatalf("no data URI in request: %s", raw)
	}
}

func TestEmbedErrorMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"context length exceeded"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, time.Second, time.Second)
	_, err := c.Embed(context.Background(), []domain.Input{
		{Parts: []domain.Part{{Kind: domain.PartText, Text: "x"}}},
	})
	if de, ok := domain.AsError(err); !ok || de.Code != "context_length_exceeded" {
		t.Fatalf("err = %v, want context_length_exceeded", err)
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
