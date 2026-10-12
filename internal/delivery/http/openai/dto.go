package openai

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/camel-creatives/embeddings-api/internal/domain"
	"github.com/camel-creatives/embeddings-api/internal/usecase/embed"
)

// embedRequest is the data-plane request body.
type embedRequest struct {
	Model          string          `json:"model"`
	Input          json.RawMessage `json:"input"`
	Task           string          `json:"task"`
	InputType      string          `json:"input_type"`
	Title          string          `json:"title"`
	Dimensions     int             `json:"dimensions"`
	EncodingFormat string          `json:"encoding_format"`
}

// parseInput decodes the polymorphic input field into items.
func parseInput(raw json.RawMessage) ([]embed.Item, error) {
	if len(raw) == 0 {
		return nil, domain.ErrEmptyInput
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, domain.ErrInvalidJSON
	}

	switch t := v.(type) {
	case string:
		return []embed.Item{textItem(t)}, nil
	case []any:
		if len(t) == 0 {
			return nil, domain.ErrEmptyInput
		}
		items := make([]embed.Item, 0, len(t))
		for _, el := range t {
			it, err := parseElement(el)
			if err != nil {
				return nil, err
			}
			items = append(items, it)
		}
		return items, nil
	case map[string]any:
		it, err := parseObject(t)
		if err != nil {
			return nil, err
		}
		return []embed.Item{it}, nil
	default:
		return nil, domain.ErrInvalidJSON
	}
}

func parseElement(v any) (embed.Item, error) {
	switch e := v.(type) {
	case string:
		return textItem(e), nil
	case map[string]any:
		return parseObject(e)
	default:
		return embed.Item{}, domain.ErrUnsupportedIn
	}
}

func textItem(s string) embed.Item {
	return embed.Item{Parts: []embed.RawPart{{Type: "text", Text: s}}}
}

func parseObject(o map[string]any) (embed.Item, error) {
	rawContent, ok := o["content"]
	if !ok {
		if txt, ok := o["text"].(string); ok {
			return textItem(txt), nil
		}
		return embed.Item{}, domain.ErrUnsupportedIn
	}
	arr, ok := rawContent.([]any)
	if !ok {
		return embed.Item{}, domain.ErrUnsupportedIn
	}
	parts := make([]embed.RawPart, 0, len(arr))
	for _, p := range arr {
		pm, ok := p.(map[string]any)
		if !ok {
			return embed.Item{}, domain.ErrUnsupportedIn
		}
		typ, _ := pm["type"].(string)
		switch typ {
		case "text":
			txt, _ := pm["text"].(string)
			parts = append(parts, embed.RawPart{Type: "text", Text: txt})
		case "image_url":
			mime, data, err := dataURI(pm["image_url"])
			if err != nil {
				return embed.Item{}, err
			}
			parts = append(parts, embed.RawPart{Type: "image_url", DataB64: data, Mime: mime})
		case "input_audio":
			ia, _ := pm["input_audio"].(map[string]any)
			data, _ := ia["data"].(string)
			format, _ := ia["format"].(string)
			if data == "" {
				return embed.Item{}, domain.ErrInvalidMedia.WithMessage("input_audio.data is required")
			}
			parts = append(parts, embed.RawPart{Type: "input_audio", DataB64: data, Mime: format})
		case "video_url":
			mime, data, err := dataURI(pm["video_url"])
			if err != nil {
				return embed.Item{}, err
			}
			parts = append(parts, embed.RawPart{Type: "video_url", DataB64: data, Mime: mime})
		default:
			return embed.Item{}, domain.ErrUnsupportedIn.WithMessage("Unknown part type: " + typ)
		}
	}
	return embed.Item{Parts: parts}, nil
}

// dataURI parses a {"url":"data:<mime>;base64,<data>"} object. Non-data URLs
// are rejected: this service never fetches remote media.
func dataURI(v any) (mime, data string, err error) {
	obj, ok := v.(map[string]any)
	if !ok {
		return "", "", domain.ErrUnsupportedIn.WithMessage("Expected image_url or video_url object")
	}
	urlStr, _ := obj["url"].(string)
	if urlStr == "" {
		return "", "", domain.ErrUnsupportedIn.WithMessage("Media url is required")
	}
	if !strings.HasPrefix(urlStr, "data:") {
		return "", "", domain.ErrUnsupportedIn.WithMessage("Media must be a base64 data URI")
	}
	rest := urlStr[len("data:"):]
	comma := strings.IndexByte(rest, ',')
	if comma < 0 {
		return "", "", domain.ErrUnsupportedIn.WithMessage("Malformed data URI")
	}
	meta := rest[:comma]
	data = rest[comma+1:]
	if !strings.HasSuffix(meta, ";base64") {
		return "", "", domain.ErrUnsupportedIn.WithMessage("Data URI must be base64-encoded")
	}
	mime = strings.TrimSuffix(meta, ";base64")
	return mime, data, nil
}
