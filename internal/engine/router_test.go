package engine

import (
	"context"
	"testing"

	"github.com/camel-creatives/embeddings-api/internal/domain"
)

type stubEngine struct {
	kinds map[domain.PartKind]bool
	seen  []domain.Input
}

func (s *stubEngine) Embed(_ context.Context, in []domain.Input) (domain.EmbedResult, error) {
	s.seen = append(s.seen, in...)
	vecs := make([][]float32, len(in))
	for i := range vecs {
		vecs[i] = []float32{float32(len(s.seen))}
	}
	return domain.EmbedResult{Vectors: vecs, Usage: domain.Usage{PromptTokens: len(in)}}, nil
}

func (s *stubEngine) Supports(k domain.PartKind) bool { return s.kinds[k] }
func (s *stubEngine) Ready(context.Context) error     { return nil }

func TestRouterDispatchesAndReassembles(t *testing.T) {
	text := &stubEngine{kinds: map[domain.PartKind]bool{domain.PartText: true}}
	media := &stubEngine{kinds: map[domain.PartKind]bool{domain.PartImage: true, domain.PartText: true}}
	r := New(text, media)

	res, err := r.Embed(context.Background(), []domain.Input{
		{Parts: []domain.Part{{Kind: domain.PartText, Text: "a"}}},
		{Parts: []domain.Part{{Kind: domain.PartImage}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Vectors) != 2 {
		t.Fatalf("vectors = %d", len(res.Vectors))
	}
	if len(text.seen) != 1 || len(media.seen) != 1 {
		t.Fatalf("dispatch wrong: text=%d media=%d", len(text.seen), len(media.seen))
	}
}

func TestRouterUnsupportedInput(t *testing.T) {
	text := &stubEngine{kinds: map[domain.PartKind]bool{domain.PartText: true}}
	r := New(text)
	_, err := r.Embed(context.Background(), []domain.Input{
		{Parts: []domain.Part{{Kind: domain.PartAudio}}},
	})
	if de, ok := domain.AsError(err); !ok || de.Code != "unsupported_input" {
		t.Fatalf("err = %v, want unsupported_input", err)
	}
}
