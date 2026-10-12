package usage

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/camel-creatives/embeddings-api/internal/domain"
)

// Recorder buffers usage events and flushes them in batches.
type Recorder struct {
	repo     domain.UsageRepository
	log      *slog.Logger
	ch       chan domain.UsageEvent
	maxBatch int
	interval time.Duration

	mu    sync.Mutex
	batch []domain.UsageEvent

	done      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
	dropped   atomic.Int64
}

// NewRecorder starts a buffered recorder.
func NewRecorder(repo domain.UsageRepository, bufferSize, maxBatch int, interval time.Duration, log *slog.Logger) *Recorder {
	if bufferSize <= 0 {
		bufferSize = 1024
	}
	if maxBatch <= 0 {
		maxBatch = 500
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	r := &Recorder{
		repo:     repo,
		log:      log,
		ch:       make(chan domain.UsageEvent, bufferSize),
		maxBatch: maxBatch,
		interval: interval,
		done:     make(chan struct{}),
	}
	r.wg.Add(1)
	go r.run()
	return r
}

// Record enqueues an event without blocking; events are dropped only if the
// buffer is full or the recorder is shutting down.
func (r *Recorder) Record(e domain.UsageEvent) {
	select {
	case <-r.done:
		r.dropped.Add(1)
		return
	default:
	}
	select {
	case r.ch <- e:
	default:
		r.dropped.Add(1)
	}
}

func (r *Recorder) run() {
	defer r.wg.Done()
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case e := <-r.ch:
			r.appendEvent(e)
			if r.batchLen() >= r.maxBatch {
				r.persist()
			}
		case <-ticker.C:
			r.persist()
		case <-r.done:
			drain(r.ch, r.appendEvent)
			r.persist()
			return
		}
	}
}

func (r *Recorder) appendEvent(e domain.UsageEvent) {
	r.mu.Lock()
	r.batch = append(r.batch, e)
	r.mu.Unlock()
}

func (r *Recorder) batchLen() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.batch)
}

// persist writes the current batch, detaching it under lock.
func (r *Recorder) persist() {
	r.mu.Lock()
	if len(r.batch) == 0 {
		r.mu.Unlock()
		return
	}
	events := r.batch
	r.batch = nil
	r.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := r.repo.InsertEvents(ctx, events); err != nil {
		r.log.Error("usage: insert events", "count", len(events), "err", err)
	}
	if err := r.repo.UpsertDaily(ctx, events); err != nil {
		r.log.Error("usage: upsert daily", "count", len(events), "err", err)
	}
}

// Flush persists everything buffered right now.
func (r *Recorder) Flush(context.Context) error {
	drain(r.ch, r.appendEvent)
	r.persist()
	return nil
}

// Close stops the recorder, draining and persisting remaining events.
func (r *Recorder) Close(ctx context.Context) error {
	r.closeOnce.Do(func() { close(r.done) })
	wait := make(chan struct{})
	go func() { r.wg.Wait(); close(wait) }()
	select {
	case <-wait:
	case <-ctx.Done():
	}
	if n := r.dropped.Load(); n > 0 {
		r.log.Warn("usage: dropped events", "count", n)
	}
	return nil
}

func drain(ch chan domain.UsageEvent, fn func(domain.UsageEvent)) {
	for {
		select {
		case e := <-ch:
			fn(e)
		default:
			return
		}
	}
}
