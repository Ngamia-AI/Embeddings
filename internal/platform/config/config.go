package config

import (
	"os"
	"strconv"
	"time"
)

// Config is the process configuration, sourced from the environment.
type Config struct {
	Port int

	// Engine
	EngineURL        string
	EngineSlots      int
	MediaConcurrency int
	VideoEnabled     bool

	// Python video engine (Phase 8, Path B)
	VideoEngineURL       string
	VideoTimeout         time.Duration
	VideoMaxBytes        int64
	VideoFramesPerSecond int
	VideoMaxSeconds      float64

	// Budgets and limits (gateway defaults; apps may narrow them)
	MaxItemTokens    int
	MaxBodyBytes     int64
	MediaBytesBudget int64
	MaxMediaItems    int
	MaxBatch         int

	// Media pipeline
	FFmpegPath      string
	MaxImagePixels  int
	MaxImageSide    int
	ImageDownscale  int
	ImageJPEGQual   int
	MaxAudioSeconds float64

	// Timeouts and wait budgets
	TextTimeout   time.Duration
	MediaTimeout  time.Duration
	FFmpegTimeout time.Duration
	WaitLiveText  time.Duration
	WaitLiveMedia time.Duration
	WaitBulk      time.Duration

	// Storage and auth
	DatabaseURL string
	AdminToken  string
	KeyCacheTTL time.Duration

	// Retention
	RawEventRetention time.Duration
}

// Load reads configuration from the environment, applying spec defaults.
func Load() Config {
	return Config{
		Port: envInt("PORT", 8090),

		EngineURL:        env("ENGINE_URL", "http://embed:8080"),
		EngineSlots:      envInt("ENGINE_SLOTS", 2),
		MediaConcurrency: envInt("MEDIA_CONCURRENCY", 1),
		VideoEnabled:     envBool("VIDEO_ENABLED", false),

		VideoEngineURL:       env("VIDEO_ENGINE_URL", "http://embed-py:8080"),
		VideoTimeout:         envDuration("VIDEO_TIMEOUT", 300*time.Second),
		VideoMaxBytes:        int64(envInt("VIDEO_MAX_BYTES", 64*1024*1024)),
		VideoFramesPerSecond: envInt("VIDEO_FRAMES_PER_SECOND", 1),
		VideoMaxSeconds:      envFloat("VIDEO_MAX_SECONDS", 58),

		MaxItemTokens:    envInt("MAX_ITEM_TOKENS", 4096),
		MaxBodyBytes:     int64(envInt("MAX_BODY_BYTES", 25*1024*1024)),
		MediaBytesBudget: int64(envInt("MEDIA_BYTES_BUDGET", 128*1024*1024)),
		MaxMediaItems:    envInt("MAX_MEDIA_ITEMS", 8),
		MaxBatch:         envInt("MAX_BATCH", 32),

		FFmpegPath:      env("FFMPEG_PATH", "ffmpeg"),
		MaxImagePixels:  envInt("MAX_IMAGE_PIXELS", 40_000_000),
		MaxImageSide:    envInt("MAX_IMAGE_SIDE", 16384),
		ImageDownscale:  envInt("IMAGE_DOWNSCALE", 2048),
		ImageJPEGQual:   envInt("IMAGE_JPEG_QUALITY", 95),
		MaxAudioSeconds: envFloat("MAX_AUDIO_SECONDS", 120),

		TextTimeout:   envDuration("TEXT_TIMEOUT", 30*time.Second),
		MediaTimeout:  envDuration("MEDIA_TIMEOUT", 120*time.Second),
		FFmpegTimeout: envDuration("FFMPEG_TIMEOUT", 20*time.Second),
		WaitLiveText:  envDuration("WAIT_LIVE_TEXT", 2*time.Second),
		WaitLiveMedia: envDuration("WAIT_LIVE_MEDIA", 10*time.Second),
		WaitBulk:      envDuration("WAIT_BULK", 30*time.Second),

		DatabaseURL: env("DATABASE_URL", "postgres://embeddings:embeddings@localhost:5432/embeddings?sslmode=disable"),
		AdminToken:  env("ADMIN_TOKEN", ""),
		KeyCacheTTL: envDuration("KEY_CACHE_TTL", 60*time.Second),

		RawEventRetention: envDuration("RAW_EVENT_RETENTION", 30*24*time.Hour),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
