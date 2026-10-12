package llamacpp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/camel-creatives/embeddings-api/internal/domain"
)

// Client is an HTTP adapter for a llama.cpp llama-server running with --mmproj.
type Client struct {
	baseURL      string
	http         *http.Client
	textTimeout  time.Duration
	mediaTimeout time.Duration
}

// New builds a client. baseURL is the engine root, e.g. http://embed:8080.
func New(baseURL string, textTimeout, mediaTimeout time.Duration) *Client {
	transport := &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 32,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		http:         &http.Client{Transport: transport},
		textTimeout:  textTimeout,
		mediaTimeout: mediaTimeout,
	}
}

// Supports reports that the llama.cpp adapter handles text, images and audio.
func (c *Client) Supports(k domain.PartKind) bool {
	switch k {
	case domain.PartText, domain.PartImage, domain.PartAudio:
		return true
	default:
		return false
	}
}

// Ready checks the engine /health endpoint.
func (c *Client) Ready(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrEngineUnavail, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return domain.ErrEngineUnavail
	}
	return nil
}

type embedRequest struct {
	Input          []any  `json:"input"`
	Model          string `json:"model"`
	EncodingFormat string `json:"encoding_format"`
}

type contentPart struct {
	Type  string      `json:"type"`
	Text  string      `json:"text,omitempty"`
	Image *imageURL   `json:"image_url,omitempty"`
	Audio *inputAudio `json:"input_audio,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type inputAudio struct {
	Data   string `json:"data"`
	Format string `json:"format"`
}

type embedResponse struct {
	Object string `json:"object"`
	Model  string `json:"model"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// Embed sends the batch to llama.cpp and returns vectors in input order.
func (c *Client) Embed(ctx context.Context, in []domain.Input) (domain.EmbedResult, error) {
	if len(in) == 0 {
		return domain.EmbedResult{}, nil
	}

	items, hasMedia := buildItems(in)
	payload, err := json.Marshal(embedRequest{
		Input:          items,
		Model:          "embeddinggemma-2",
		EncodingFormat: "float",
	})
	if err != nil {
		return domain.EmbedResult{}, fmt.Errorf("marshal request: %w", err)
	}

	timeout := c.textTimeout
	if hasMedia {
		timeout = c.mediaTimeout
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, status, err := c.do(reqCtx, payload, hasMedia)
	if err != nil {
		return domain.EmbedResult{}, err
	}
	if status != http.StatusOK {
		return domain.EmbedResult{}, mapEngineError(status, body)
	}

	var parsed embedResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return domain.EmbedResult{}, fmt.Errorf("decode engine response: %w", err)
	}
	if len(parsed.Data) != len(in) {
		return domain.EmbedResult{}, fmt.Errorf("engine returned %d vectors for %d inputs", len(parsed.Data), len(in))
	}

	vectors := make([][]float32, len(in))
	for _, d := range parsed.Data {
		if d.Index < 0 || d.Index >= len(vectors) {
			return domain.EmbedResult{}, fmt.Errorf("engine returned out-of-range index %d", d.Index)
		}
		vectors[d.Index] = d.Embedding
	}
	return domain.EmbedResult{
		Vectors: vectors,
		Usage: domain.Usage{
			PromptTokens: parsed.Usage.PromptTokens,
			TotalTokens:  parsed.Usage.TotalTokens,
		},
	}, nil
}

// do performs the request and retries once on a connection reset.
func (c *Client) do(ctx context.Context, payload []byte, hasMedia bool) ([]byte, int, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/embeddings", bytes.NewReader(payload))
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, 0, domain.ErrEngineTimeout
			}
			lastErr = err
			if attempt == 0 && isConnReset(err) {
				continue
			}
			return nil, 0, fmt.Errorf("%w: %v", domain.ErrEngineUnavail, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt == 0 && isConnReset(readErr) {
				continue
			}
			return nil, 0, fmt.Errorf("%w: %v", domain.ErrEngineUnavail, readErr)
		}
		return body, resp.StatusCode, nil
	}
	return nil, 0, fmt.Errorf("%w: %v", domain.ErrEngineUnavail, lastErr)
}

// buildItems converts normalized inputs into llama.cpp input items.
func buildItems(in []domain.Input) ([]any, bool) {
	items := make([]any, 0, len(in))
	hasMedia := false
	for _, item := range in {
		if len(item.Parts) == 1 && item.Parts[0].Kind == domain.PartText {
			items = append(items, item.Parts[0].Text)
			continue
		}
		parts := make([]contentPart, 0, len(item.Parts))
		for _, p := range item.Parts {
			switch p.Kind {
			case domain.PartText:
				parts = append(parts, contentPart{Type: "text", Text: p.Text})
			case domain.PartImage:
				hasMedia = true
				parts = append(parts, contentPart{
					Type:  "image_url",
					Image: &imageURL{URL: dataURI(p.Mime, p.Data)},
				})
			case domain.PartAudio:
				hasMedia = true
				parts = append(parts, contentPart{
					Type: "input_audio",
					Audio: &inputAudio{
						Data:   base64.StdEncoding.EncodeToString(p.Data),
						Format: "wav",
					},
				})
			}
		}
		items = append(items, map[string]any{"content": parts})
	}
	return items, hasMedia
}

func dataURI(mime string, data []byte) string {
	if mime == "" {
		mime = "application/octet-stream"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// mapEngineError maps engine responses onto domain errors.
func mapEngineError(status int, body []byte) error {
	msg := strings.ToLower(string(body))
	switch {
	case status == http.StatusBadRequest && strings.Contains(msg, "context"):
		return domain.ErrContextTooLong
	case status == http.StatusBadRequest:
		return domain.ErrInvalidMedia
	case status == http.StatusRequestEntityTooLarge:
		return domain.ErrContextTooLong
	case status >= 500:
		return domain.ErrEngineUnavail
	default:
		return domain.ErrEngineUnavail
	}
}

func isConnReset(err error) bool {
	return errors.Is(err, io.ErrUnexpectedEOF) || strings.Contains(strings.ToLower(err.Error()), "connection reset")
}
