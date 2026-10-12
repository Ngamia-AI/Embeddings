package domain

import (
	"time"

	"github.com/google/uuid"
)

// PartKind enumerates the supported input part types.
type PartKind int

const (
	PartText PartKind = iota
	PartImage
	PartAudio
	PartVideo
)

func (k PartKind) String() string {
	switch k {
	case PartText:
		return "text"
	case PartImage:
		return "image"
	case PartAudio:
		return "audio"
	case PartVideo:
		return "video"
	default:
		return "unknown"
	}
}

// Modality returns the permission name used by apps.modalities.
func (k PartKind) Modality() string { return k.String() }

// Part is one normalized piece of an input.
type Part struct {
	Kind PartKind
	Text string // text parts
	Data []byte // normalized media bytes
	Mime string
}

// Input is a single item to embed; it maps to exactly one vector.
type Input struct {
	Parts []Part
}

// HasMedia reports whether any part is non-text.
func (in Input) HasMedia() bool {
	for _, p := range in.Parts {
		if p.Kind != PartText {
			return true
		}
	}
	return false
}

// Modalities returns the set of modalities used by the input.
func (in Input) Modalities() map[PartKind]bool {
	m := make(map[PartKind]bool, len(in.Parts))
	for _, p := range in.Parts {
		m[p.Kind] = true
	}
	return m
}

// Usage mirrors the OpenAI usage object plus the multimodal extension fields.
type Usage struct {
	PromptTokens int
	TotalTokens  int
	Images       int
	AudioSeconds float64
	VideoSeconds float64
}

// EmbedResult is what an Embedder returns for a batch of inputs.
type EmbedResult struct {
	Vectors [][]float32
	Usage   Usage
}

// App is an API consumer with limits and modality permissions.
type App struct {
	ID         uuid.UUID
	Name       string
	Status     string // active | disabled
	Modalities []string
	RPS        int
	Burst      int
	MediaRPS   int
	MaxBatch   int
	MaxMedia   int
	CreatedAt  time.Time
}

// Allowed reports whether the app may use the given modality.
func (a *App) Allowed(k PartKind) bool {
	if len(a.Modalities) == 0 {
		return k == PartText
	}
	for _, m := range a.Modalities {
		if m == k.Modality() {
			return true
		}
	}
	return false
}

// APIKey is a hashed credential belonging to an app.
type APIKey struct {
	ID         uuid.UUID
	AppID      uuid.UUID
	Prefix     string
	KeyHash    []byte
	Class      string // live | bulk
	Status     string // active | revoked
	LastUsedAt *time.Time
	CreatedAt  time.Time
	RevokedAt  *time.Time
}

// UsageEvent is one metered request.
type UsageEvent struct {
	AppID        uuid.UUID
	KeyID        uuid.UUID
	RequestID    string
	Inputs       int
	Tokens       int
	Images       int
	AudioSeconds float64
	VideoSeconds float64
	LatencyMS    int
	Status       int
	CreatedAt    time.Time
}

// UsageDaily is a per-app per-day rollup.
type UsageDaily struct {
	AppID        uuid.UUID `json:"app_id"`
	Day          string    `json:"day"`
	Requests     int64     `json:"requests"`
	Inputs       int64     `json:"inputs"`
	Tokens       int64     `json:"tokens"`
	Images       int64     `json:"images"`
	AudioSeconds float64   `json:"audio_seconds"`
	VideoSeconds float64   `json:"video_seconds"`
	Errors       int64     `json:"errors"`
}
