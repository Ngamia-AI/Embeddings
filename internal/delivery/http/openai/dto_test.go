package openai

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/camel-creatives/embeddings-api/internal/domain"
)

func TestParseInputString(t *testing.T) {
	items, err := parseInput(json.RawMessage(`"hello world"`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Parts[0].Type != "text" || items[0].Parts[0].Text != "hello world" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestParseInputArrayMixed(t *testing.T) {
	img := base64.StdEncoding.EncodeToString([]byte{0xFF, 0xD8, 0xFF})
	raw := `[
		"bei ya mchele Kariakoo",
		{"content":[{"type":"image_url","image_url":{"url":"data:image/jpeg;base64,` + img + `"}}]},
		{"content":[{"type":"input_audio","input_audio":{"data":"T2dnUwAC","format":"ogg"}}]}
	]`
	items, err := parseInput(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3", len(items))
	}
	if items[1].Parts[0].Mime != "image/jpeg" || items[1].Parts[0].DataB64 != img {
		t.Fatalf("image part wrong: %+v", items[1].Parts[0])
	}
	if items[2].Parts[0].Mime != "ogg" || items[2].Parts[0].DataB64 != "T2dnUwAC" {
		t.Fatalf("audio part wrong: %+v", items[2].Parts[0])
	}
}

func TestParseInputRejectsRemoteURL(t *testing.T) {
	raw := `[{"content":[{"type":"image_url","image_url":{"url":"https://example.com/a.jpg"}}]}]`
	_, err := parseInput(json.RawMessage(raw))
	if de, ok := domain.AsError(err); !ok || de.Code != "unsupported_input" {
		t.Fatalf("err = %v, want unsupported_input", err)
	}
}

func TestParseInputEmpty(t *testing.T) {
	if _, err := parseInput(json.RawMessage(``)); err == nil {
		t.Fatal("want error for empty input")
	}
	if _, err := parseInput(json.RawMessage(`[]`)); err == nil {
		t.Fatal("want error for empty array")
	}
}
