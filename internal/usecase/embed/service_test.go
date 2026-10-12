package embed

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/camel-creatives/embeddings-api/internal/domain"
	"github.com/camel-creatives/embeddings-api/internal/platform/slots"
)

type fakeEmbedder struct {
	supports map[domain.PartKind]bool
	last     []domain.Input
	usage    domain.Usage
	err      error
}

func (f *fakeEmbedder) Embed(_ context.Context, in []domain.Input) (domain.EmbedResult, error) {
	f.last = in
	if f.err != nil {
		return domain.EmbedResult{}, f.err
	}
	vecs := make([][]float32, len(in))
	for i := range vecs {
		v := make([]float32, FullDimensions)
		for j := range v {
			v[j] = float32(i+1) * 0.01
		}
		vecs[i] = v
	}
	u := f.usage
	if u.PromptTokens == 0 {
		u.PromptTokens = len(in) * 5
		u.TotalTokens = u.PromptTokens
	}
	return domain.EmbedResult{Vectors: vecs, Usage: u}, nil
}

func (f *fakeEmbedder) Supports(k domain.PartKind) bool { return f.supports[k] }
func (f *fakeEmbedder) Ready(context.Context) error     { return nil }

type fakeMedia struct{ imgErr error }

func (f *fakeMedia) NormalizeImage(data []byte, _ string) ([]byte, string, error) {
	if f.imgErr != nil {
		return nil, "", f.imgErr
	}
	return data, "image/jpeg", nil
}

func (f *fakeMedia) NormalizeAudio(_ context.Context, data []byte, _ string) ([]byte, float64, error) {
	return data, 4.0, nil
}

func newTestService(emb domain.Embedder) *Service {
	return New(Deps{
		Embedder:      emb,
		Media:         &fakeMedia{},
		Slots:         slots.New(2, 1, 64<<20),
		MaxItemTokens: 4096,
		MaxBatch:      32,
		MaxMediaItems: 8,
		WaitLiveText:  2_000_000_000,
		WaitLiveMedia: 2_000_000_000,
		WaitBulk:      2_000_000_000,
	})
}

func allKinds() map[domain.PartKind]bool {
	return map[domain.PartKind]bool{
		domain.PartText: true, domain.PartImage: true,
		domain.PartAudio: true, domain.PartVideo: true,
	}
}

var jpegBytes = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
var oggBytes = []byte("OggS\x00\x02\x00\x00\x00\x00\x00\x00")

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func TestEmbedPrefixAndTruncate(t *testing.T) {
	emb := &fakeEmbedder{supports: map[domain.PartKind]bool{domain.PartText: true}}
	svc := newTestService(emb)

	res, err := svc.Embed(context.Background(), Request{
		Items:      []Item{{Parts: []RawPart{{Type: "text", Text: "bei ya mchele"}}}},
		Task:       "search_query",
		Dimensions: 128,
	}, Limits{MaxBatch: 32, Allowed: allKinds()})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if got := emb.last[0].Parts[0].Text; got != " task: search result | query: bei ya mchele" {
		t.Fatalf("prefix not applied: %q", got)
	}
	if len(res.Vectors[0]) != 128 {
		t.Fatalf("vector dim = %d, want 128", len(res.Vectors[0]))
	}
	if res.Usage.PromptTokens != 5 {
		t.Fatalf("tokens = %d, want 5", res.Usage.PromptTokens)
	}
}

func TestEmbedNoPrefixForMedia(t *testing.T) {
	emb := &fakeEmbedder{supports: allKinds()}
	svc := newTestService(emb)

	_, err := svc.Embed(context.Background(), Request{
		Items: []Item{{Parts: []RawPart{{Type: "image_url", DataB64: b64(jpegBytes), Mime: "image/jpeg"}}}},
		Task:  "search_query",
	}, Limits{MaxBatch: 32, Allowed: allKinds()})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if emb.last[0].Parts[0].Kind != domain.PartImage {
		t.Fatalf("expected image part")
	}
}

func TestEmbedModalityDenied(t *testing.T) {
	emb := &fakeEmbedder{supports: allKinds()}
	svc := newTestService(emb)

	_, err := svc.Embed(context.Background(), Request{
		Items: []Item{{Parts: []RawPart{{Type: "image_url", DataB64: b64(jpegBytes), Mime: "image/jpeg"}}}},
		Task:  "search_query",
	}, Limits{MaxBatch: 32, Allowed: map[domain.PartKind]bool{domain.PartText: true}})
	if de, ok := domain.AsError(err); !ok || de.Code != "modality_not_allowed" {
		t.Fatalf("err = %v, want modality_not_allowed", err)
	}
}

func TestEmbedErrors(t *testing.T) {
	emb := &fakeEmbedder{supports: allKinds()}
	svc := newTestService(emb)

	cases := []struct {
		name string
		req  Request
		lim  Limits
		code string
	}{
		{"empty", Request{}, Limits{Allowed: allKinds()}, "empty_input"},
		{"dims", Request{Items: []Item{{Parts: []RawPart{{Type: "text", Text: "x"}}}}, Dimensions: 100}, Limits{Allowed: allKinds()}, "invalid_dimensions"},
		{"batch", Request{Items: make([]Item, 5)}, Limits{MaxBatch: 2, Allowed: allKinds()}, "batch_too_large"},
		{"unknown part", Request{Items: []Item{{Parts: []RawPart{{Type: "weird"}}}}}, Limits{Allowed: allKinds()}, "unsupported_input"},
		{"http url image", Request{Items: []Item{{Parts: []RawPart{{Type: "image_url", DataB64: "!!notb64"}}}}}, Limits{Allowed: allKinds()}, "invalid_media"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.Embed(context.Background(), c.req, c.lim)
			de, ok := domain.AsError(err)
			if !ok {
				t.Fatalf("err = %v, want domain error", err)
			}
			if de.Code != c.code {
				t.Fatalf("code = %s, want %s", de.Code, c.code)
			}
		})
	}
}

func TestEmbedContextLengthExceeded(t *testing.T) {
	emb := &fakeEmbedder{supports: allKinds()}
	svc := New(Deps{
		Embedder:      emb,
		Media:         &fakeMedia{},
		Slots:         slots.New(2, 1, 64<<20),
		MaxItemTokens: 4,
		MaxBatch:      32,
		MaxMediaItems: 8,
	})
	long := ""
	for i := 0; i < 100; i++ {
		long += "token "
	}
	_, err := svc.Embed(context.Background(), Request{
		Items: []Item{{Parts: []RawPart{{Type: "text", Text: long}}}},
	}, Limits{MaxBatch: 32, Allowed: allKinds()})
	if de, ok := domain.AsError(err); !ok || de.Code != "context_length_exceeded" {
		t.Fatalf("err = %v, want context_length_exceeded", err)
	}
}

func videoService(emb domain.Embedder, maxBytes int64) *Service {
	return New(Deps{
		Embedder:      emb,
		Media:         &fakeMedia{},
		Slots:         slots.New(2, 1, 64<<20),
		MaxItemTokens: 4096,
		MaxBatch:      32,
		MaxMediaItems: 8,
		WaitLiveText:  2_000_000_000,
		WaitLiveMedia: 2_000_000_000,
		WaitBulk:      2_000_000_000,
		VideoEnabled:  true,
		VideoMaxBytes: maxBytes,
	})
}

var mp4Bytes = []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom")

func TestEmbedVideoEnabled(t *testing.T) {
	emb := &fakeEmbedder{supports: allKinds(), usage: domain.Usage{VideoSeconds: 3.5}}
	svc := videoService(emb, 0)

	res, err := svc.Embed(context.Background(), Request{
		Items: []Item{{Parts: []RawPart{{Type: "video_url", DataB64: b64(mp4Bytes), Mime: "video/mp4"}}}},
		Task:  "document",
	}, Limits{MaxBatch: 32, Allowed: allKinds()})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if emb.last[0].Parts[0].Kind != domain.PartVideo {
		t.Fatalf("expected video part, got %v", emb.last[0].Parts[0].Kind)
	}
	if res.Usage.VideoSeconds != 3.5 {
		t.Fatalf("video seconds = %v, want 3.5", res.Usage.VideoSeconds)
	}
}

func TestEmbedVideoDisabled(t *testing.T) {
	emb := &fakeEmbedder{supports: allKinds()}
	svc := newTestService(emb)

	_, err := svc.Embed(context.Background(), Request{
		Items: []Item{{Parts: []RawPart{{Type: "video_url", DataB64: b64(mp4Bytes), Mime: "video/mp4"}}}},
	}, Limits{MaxBatch: 32, Allowed: allKinds()})
	if de, ok := domain.AsError(err); !ok || de.Code != "unsupported_input" {
		t.Fatalf("err = %v, want unsupported_input", err)
	}
}

func TestEmbedVideoTooLarge(t *testing.T) {
	emb := &fakeEmbedder{supports: allKinds()}
	svc := videoService(emb, 4)

	_, err := svc.Embed(context.Background(), Request{
		Items: []Item{{Parts: []RawPart{{Type: "video_url", DataB64: b64(mp4Bytes), Mime: "video/mp4"}}}},
	}, Limits{MaxBatch: 32, Allowed: allKinds()})
	if de, ok := domain.AsError(err); !ok || de.Code != "payload_too_large" {
		t.Fatalf("err = %v, want payload_too_large", err)
	}
}

func TestEmbedVideoUnsupportedContainer(t *testing.T) {
	emb := &fakeEmbedder{supports: allKinds()}
	svc := videoService(emb, 0)

	_, err := svc.Embed(context.Background(), Request{
		Items: []Item{{Parts: []RawPart{{Type: "video_url", DataB64: b64([]byte("not a video")), Mime: "video/mp4"}}}},
	}, Limits{MaxBatch: 32, Allowed: allKinds()})
	if de, ok := domain.AsError(err); !ok || de.Code != "unsupported_media_type" {
		t.Fatalf("err = %v, want unsupported_media_type", err)
	}
}
