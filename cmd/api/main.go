package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/camel-creatives/embeddings-api/internal/delivery/http/admin"
	"github.com/camel-creatives/embeddings-api/internal/delivery/http/middleware"
	"github.com/camel-creatives/embeddings-api/internal/delivery/http/openai"
	"github.com/camel-creatives/embeddings-api/internal/delivery/http/ops"
	"github.com/camel-creatives/embeddings-api/internal/engine"
	"github.com/camel-creatives/embeddings-api/internal/engine/llamacpp"
	"github.com/camel-creatives/embeddings-api/internal/engine/python"
	"github.com/camel-creatives/embeddings-api/internal/media"
	"github.com/camel-creatives/embeddings-api/internal/platform/config"
	"github.com/camel-creatives/embeddings-api/internal/platform/db"
	"github.com/camel-creatives/embeddings-api/internal/platform/logger"
	"github.com/camel-creatives/embeddings-api/internal/platform/metrics"
	"github.com/camel-creatives/embeddings-api/internal/platform/slots"
	"github.com/camel-creatives/embeddings-api/internal/repository/cache"
	"github.com/camel-creatives/embeddings-api/internal/repository/postgres"
	"github.com/camel-creatives/embeddings-api/internal/usecase/apps"
	"github.com/camel-creatives/embeddings-api/internal/usecase/embed"
	usageuc "github.com/camel-creatives/embeddings-api/internal/usecase/usage"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	log := logger.New(logger.LevelFromEnv())
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.Migrate(cfg.DatabaseURL, envOr("MIGRATIONS_DIR", "migrations")); err != nil {
		return err
	}

	appRepo := postgres.NewAppRepo(pool)
	keyRepo := postgres.NewKeyRepo(pool)
	usageRepo := postgres.NewUsageRepo(pool)
	keyCache := cache.NewKeyCache(keyRepo, cfg.KeyCacheTTL)

	llama := llamacpp.New(cfg.EngineURL, cfg.TextTimeout, cfg.MediaTimeout)
	routerEngine := buildRouter(cfg, llama, log)
	normalizer := media.New(media.Options{
		FFmpegPath:      cfg.FFmpegPath,
		MaxPixels:       cfg.MaxImagePixels,
		MaxSide:         cfg.MaxImageSide,
		Downscale:       cfg.ImageDownscale,
		JPEGQuality:     cfg.ImageJPEGQual,
		MaxAudioSeconds: cfg.MaxAudioSeconds,
		FFmpegTimeout:   cfg.FFmpegTimeout,
	})

	slotMgr := slots.New(cfg.EngineSlots, cfg.MediaConcurrency, cfg.MediaBytesBudget)
	m := metrics.New()

	embedSvc := embed.New(embed.Deps{
		Embedder:       routerEngine,
		Media:          normalizer,
		Slots:          slotMgr,
		Metrics:        m,
		MaxItemTokens:  cfg.MaxItemTokens,
		MaxBatch:       cfg.MaxBatch,
		MaxMediaItems:  cfg.MaxMediaItems,
		MediaBytesWait: cfg.WaitLiveMedia,
		WaitLiveText:   cfg.WaitLiveText,
		WaitLiveMedia:  cfg.WaitLiveMedia,
		WaitBulk:       cfg.WaitBulk,
		VideoEnabled:   cfg.VideoEnabled,
		VideoMaxBytes:  cfg.VideoMaxBytes,
	})

	recorder := usageuc.NewRecorder(usageRepo, 1024, 500, 5*time.Second, log)

	appsSvc := apps.New(appRepo, keyRepo, keyCache)
	usageSvc := usageuc.NewService(usageRepo)

	rateReg := middleware.NewRegistry()
	auth := middleware.NewAuthenticator(keyCache)
	adminAuth := middleware.NewAdminAuth(cfg.AdminToken)

	openaiHandler := openai.NewHandler(embedSvc, rateReg, recorder, log, cfg.MaxBodyBytes, m)
	adminHandler := admin.NewHandler(appsSvc, usageSvc)
	opsHandler := ops.New(routerEngine)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recover(log))
	r.Use(middleware.Logging(log))

	r.Get("/healthz", opsHandler.Healthz)
	r.Get("/readyz", opsHandler.Readyz)
	r.Handle("/metrics", opsHandler.Metrics())

	r.Group(func(gr chi.Router) {
		gr.Use(auth.Middleware)
		openaiHandler.Register(gr)
	})

	r.Group(func(gr chi.Router) {
		gr.Use(adminAuth.Middleware)
		adminHandler.Register(gr)
	})

	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       150 * time.Second,
		WriteTimeout:      180 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go purgeLoop(ctx, usageSvc, cfg.RawEventRetention, log)

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", srv.Addr, "engine", cfg.EngineURL, "video", cfg.VideoEnabled)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http shutdown", "err", err)
	}
	if err := recorder.Close(shutdownCtx); err != nil {
		log.Error("recorder close", "err", err)
	}
	log.Info("stopped")
	return nil
}

// buildRouter wires the concrete engines into a domain.Embedder router.
func buildRouter(cfg config.Config, llama *llamacpp.Client, log *slog.Logger) *engine.Router {
	if cfg.VideoEnabled {
		log.Info("video enabled; routing video inputs to the Python engine", "url", cfg.VideoEngineURL)
		return engine.New(llama, python.New(cfg.VideoEngineURL, cfg.TextTimeout, cfg.VideoTimeout))
	}
	return engine.New(llama)
}

func purgeLoop(ctx context.Context, svc *usageuc.Service, retention time.Duration, log *slog.Logger) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := svc.PurgeOldEvents(ctx, retention); err != nil {
				log.Error("usage purge", "err", err)
			} else if n > 0 {
				log.Info("usage purged", "events", n)
			}
		}
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
