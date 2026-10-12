package media

import (
	"time"
)

// Options configures a Normalizer.
type Options struct {
	FFmpegPath      string
	MaxPixels       int
	MaxSide         int
	Downscale       int
	JPEGQuality     int
	MaxAudioSeconds float64
	FFmpegTimeout   time.Duration
}

// Normalizer implements domain.MediaNormalizer for images and audio.
type Normalizer struct {
	ffmpegPath      string
	maxPixels       int
	maxSide         int
	downscale       int
	jpegQuality     int
	maxAudioSeconds float64
	ffmpegTimeout   time.Duration
}

// New builds a Normalizer, applying defaults for unset options.
func New(opts Options) *Normalizer {
	if opts.FFmpegPath == "" {
		opts.FFmpegPath = "ffmpeg"
	}
	if opts.Downscale <= 0 {
		opts.Downscale = 2048
	}
	if opts.JPEGQuality <= 0 {
		opts.JPEGQuality = 95
	}
	if opts.MaxAudioSeconds <= 0 {
		opts.MaxAudioSeconds = 120
	}
	if opts.FFmpegTimeout <= 0 {
		opts.FFmpegTimeout = 20 * time.Second
	}
	return &Normalizer{
		ffmpegPath:      opts.FFmpegPath,
		maxPixels:       opts.MaxPixels,
		maxSide:         opts.MaxSide,
		downscale:       opts.Downscale,
		jpegQuality:     opts.JPEGQuality,
		maxAudioSeconds: opts.MaxAudioSeconds,
		ffmpegTimeout:   opts.FFmpegTimeout,
	}
}
