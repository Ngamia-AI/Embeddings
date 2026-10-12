package embed

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"github.com/camel-creatives/embeddings-api/internal/domain"
	"github.com/camel-creatives/embeddings-api/internal/media"
	"github.com/camel-creatives/embeddings-api/internal/platform/metrics"
	"github.com/camel-creatives/embeddings-api/internal/platform/slots"
)

// RawPart is a part as parsed from the request, before normalization.
type RawPart struct {
	Type    string // text | image_url | input_audio | video_url
	Text    string
	DataB64 string // base64 payload for media
	Mime    string // declared MIME (image) or format (audio)
}

// Item is a single input item with one or more parts.
type Item struct {
	Parts []RawPart
}

// Request is a validated data-plane embed request.
type Request struct {
	Items      []Item
	Task       string
	InputType  string
	Title      string
	Dimensions int
}

// Limits carries the per-app constraints relevant to one request.
type Limits struct {
	MaxBatch int
	MaxMedia int
	Allowed  map[domain.PartKind]bool
	Bulk     bool
}

// Result is the outcome of an embed request.
type Result struct {
	Vectors [][]float32
	Usage   domain.Usage
}

// Deps are the service dependencies.
type Deps struct {
	Embedder       domain.Embedder
	Media          domain.MediaNormalizer
	Slots          *slots.Slots
	Metrics        *metrics.Metrics
	MaxItemTokens  int
	MaxBatch       int
	MaxMediaItems  int
	MediaBytesWait time.Duration
	WaitLiveText   time.Duration
	WaitLiveMedia  time.Duration
	WaitBulk       time.Duration
	VideoEnabled   bool
	VideoMaxBytes  int64
}

// Service implements the embedding use case.
type Service struct{ d Deps }

// New builds an embed Service.
func New(d Deps) *Service { return &Service{d: d} }

// Embed validates, normalizes, meters and routes an embed request.
func (s *Service) Embed(ctx context.Context, req Request, lim Limits) (Result, error) {
	if !validDimension(req.Dimensions) {
		return Result{}, domain.ErrInvalidDims
	}
	if len(req.Items) == 0 {
		return Result{}, domain.ErrEmptyInput
	}
	if mb := minPositive(s.d.MaxBatch, lim.MaxBatch); mb > 0 && len(req.Items) > mb {
		return Result{}, domain.ErrBatchTooLarge
	}

	prefix := textPrefix(req.Task, req.InputType, req.Title)

	inputs := make([]domain.Input, 0, len(req.Items))
	itemTokens := make([]int, 0, len(req.Items))
	var usage domain.Usage
	mediaItems := 0
	var charged int64
	defer func() {
		if s.d.Slots != nil {
			s.d.Slots.ReleaseMedia(charged)
		}
	}()

	for _, item := range req.Items {
		parts := make([]domain.Part, 0, len(item.Parts))
		tokens := 0
		hasMedia := false

		for _, rp := range item.Parts {
			switch rp.Type {
			case "text":
				tokens += estimateTextTokens(rp.Text)
				parts = append(parts, domain.Part{Kind: domain.PartText, Text: prefix + rp.Text})

			case "image_url":
				if !allowed(lim, domain.PartImage) {
					return Result{}, domain.ErrModalityDenied
				}
				data, err := decodeB64(rp.DataB64)
				if err != nil {
					return Result{}, domain.ErrInvalidMedia.WithMessage("Image is not valid base64")
				}
				detected := media.DetectMime(data)
				if !media.IsImage(detected) {
					return Result{}, domain.ErrUnsupportedMime.WithMessage("Image must be JPEG, PNG or WebP")
				}
				declared := media.NormalizeDeclared(rp.Mime)
				if declared != "" && declared != detected {
					return Result{}, domain.ErrUnsupportedMime.WithMessage("Declared image type does not match content")
				}
				if err := s.charge(ctx, int64(len(data))); err != nil {
					return Result{}, err
				}
				charged += int64(len(data))
				norm, mime, err := s.d.Media.NormalizeImage(data, rp.Mime)
				if err != nil {
					if media.PixelError(err) {
						return Result{}, domain.ErrImageTooLarge
					}
					return Result{}, domain.ErrInvalidMedia
				}
				parts = append(parts, domain.Part{Kind: domain.PartImage, Data: norm, Mime: mime})
				usage.Images++
				tokens += TokensPerImage
				hasMedia = true

			case "input_audio":
				if !allowed(lim, domain.PartAudio) {
					return Result{}, domain.ErrModalityDenied
				}
				data, err := decodeB64(rp.DataB64)
				if err != nil {
					return Result{}, domain.ErrInvalidMedia.WithMessage("Audio is not valid base64")
				}
				detected := media.DetectMime(data)
				if !media.IsAudio(detected) {
					return Result{}, domain.ErrUnsupportedMime.WithMessage("Audio must be WAV, MP3, OGG/Opus, M4A or WebM")
				}
				declared := media.NormalizeDeclared(rp.Mime)
				if declared != "" && declared != detected {
					return Result{}, domain.ErrUnsupportedMime.WithMessage("Declared audio type does not match content")
				}
				if err := s.charge(ctx, int64(len(data))); err != nil {
					return Result{}, err
				}
				charged += int64(len(data))
				norm, seconds, err := s.d.Media.NormalizeAudio(ctx, data, rp.Mime)
				if err != nil {
					if media.AudioTooLong(err) {
						return Result{}, domain.ErrAudioTooLong
					}
					return Result{}, domain.ErrInvalidMedia.WithMessage("Audio could not be decoded")
				}
				parts = append(parts, domain.Part{Kind: domain.PartAudio, Data: norm, Mime: media.MIMEWAV})
				usage.AudioSeconds += seconds
				tokens += int(seconds * TokensPerAudioSec)
				hasMedia = true

			case "video_url":
				if !s.d.VideoEnabled {
					return Result{}, domain.ErrUnsupportedIn.WithMessage("Video is not enabled")
				}
				if !allowed(lim, domain.PartVideo) {
					return Result{}, domain.ErrModalityDenied
				}
				data, err := decodeB64(rp.DataB64)
				if err != nil {
					return Result{}, domain.ErrInvalidMedia.WithMessage("Video is not valid base64")
				}
				if s.d.VideoMaxBytes > 0 && int64(len(data)) > s.d.VideoMaxBytes {
					return Result{}, domain.ErrPayloadTooLarge.WithMessage("Video exceeds the size limit")
				}
				detected := media.DetectVideo(data)
				if !media.IsVideo(detected) {
					return Result{}, domain.ErrUnsupportedMime.WithMessage("Video must be MP4 or WebM")
				}
				if err := s.charge(ctx, int64(len(data))); err != nil {
					return Result{}, err
				}
				charged += int64(len(data))
				// The Python engine extracts frames at a fixed rate and caps the
				// duration itself, so no frame-accurate token estimate is possible
				// here; the byte cap and engine guard bound the work.
				parts = append(parts, domain.Part{Kind: domain.PartVideo, Data: data, Mime: detected})
				hasMedia = true

			default:
				return Result{}, domain.ErrUnsupportedIn.WithMessage("Unknown part type: " + rp.Type)
			}
		}

		if hasMedia {
			mediaItems++
		}
		if s.d.MaxItemTokens > 0 && tokens > s.d.MaxItemTokens {
			return Result{}, domain.ErrContextTooLong
		}
		itemTokens = append(itemTokens, tokens)
		inputs = append(inputs, domain.Input{Parts: parts})
	}

	if mm := minPositive(s.d.MaxMediaItems, lim.MaxMedia); mm > 0 && mediaItems > mm {
		return Result{}, domain.ErrTooManyMedia
	}

	hasMedia := mediaItems > 0
	wait := s.d.WaitLiveText
	if lim.Bulk {
		wait = s.d.WaitBulk
	} else if hasMedia {
		wait = s.d.WaitLiveMedia
	}

	awaitStart := time.Now()
	release, err := s.d.Slots.Acquire(ctx, lim.Bulk, hasMedia, wait)
	if err != nil {
		return Result{}, domain.ErrRetryAfter(domain.ErrServerBusy, 1)
	}
	defer release()
	queueWait := time.Since(awaitStart)

	engineStart := time.Now()
	res, err := s.d.Embedder.Embed(ctx, inputs)
	engineDur := time.Since(engineStart)

	modality := modalityOf(usage)
	if s.d.Metrics != nil {
		class := "live"
		if lim.Bulk {
			class = "bulk"
		}
		s.d.Metrics.QueueWait.WithLabelValues(class).Observe(queueWait.Seconds())
		s.d.Metrics.EngineDuration.WithLabelValues(modality).Observe(engineDur.Seconds())
	}
	if err != nil {
		if de, ok := domain.AsError(err); ok {
			return Result{}, de
		}
		return Result{}, err
	}

	dim := req.Dimensions
	if dim == 0 {
		dim = FullDimensions
	}
	vectors := make([][]float32, len(res.Vectors))
	for i, v := range res.Vectors {
		vectors[i] = truncateNormalize(v, dim)
	}

	tokens := res.Usage.PromptTokens
	if tokens == 0 {
		for _, t := range itemTokens {
			tokens += t
		}
	}
	usage.PromptTokens = tokens
	usage.TotalTokens = tokens
	if res.Usage.TotalTokens > 0 {
		usage.TotalTokens = res.Usage.TotalTokens
	}
	if res.Usage.VideoSeconds > 0 {
		usage.VideoSeconds = res.Usage.VideoSeconds
	}

	return Result{Vectors: vectors, Usage: usage}, nil
}

func (s *Service) charge(ctx context.Context, n int64) error {
	if s.d.Slots == nil {
		return nil
	}
	wait := s.d.MediaBytesWait
	if wait <= 0 {
		wait = s.d.WaitLiveMedia
	}
	if err := s.d.Slots.ChargeMedia(ctx, n, wait); err != nil {
		return domain.ErrRetryAfter(domain.ErrServerBusy, 1)
	}
	return nil
}

func decodeB64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(strings.TrimSpace(s))
}

func allowed(lim Limits, k domain.PartKind) bool {
	if lim.Allowed == nil {
		return true
	}
	return lim.Allowed[k]
}

func minPositive(a, b int) int {
	if a <= 0 {
		return b
	}
	if b <= 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}
