package slots

import (
	"context"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"
)

// Slots coordinates engine concurrency and the in-flight media byte budget.
//
// Acquisition order matches the spec: a bulk media request takes the media
// and bulk permits first, then the global engine slot.
type Slots struct {
	global *semaphore.Weighted
	bulk   *semaphore.Weighted
	media  *semaphore.Weighted
	bytes  *semaphore.Weighted

	mediaBytes int64 // atomic, for the gauge
}

// New builds a slot manager. globalSlots is the engine -np count.
func New(globalSlots, mediaConcurrency int, mediaBytesBudget int64) *Slots {
	if globalSlots < 1 {
		globalSlots = 1
	}
	if mediaConcurrency < 1 {
		mediaConcurrency = 1
	}
	if mediaBytesBudget < 1 {
		mediaBytesBudget = 1
	}
	return &Slots{
		global: semaphore.NewWeighted(int64(globalSlots)),
		bulk:   semaphore.NewWeighted(1),
		media:  semaphore.NewWeighted(int64(mediaConcurrency)),
		bytes:  semaphore.NewWeighted(mediaBytesBudget),
	}
}

// Acquire waits up to wait for the required permits and returns a release
// function. A nil error means all permits were taken.
func (s *Slots) Acquire(ctx context.Context, bulk, hasMedia bool, wait time.Duration) (func(), error) {
	wctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()

	acquired := make([]*semaphore.Weighted, 0, 3)
	release := func() {
		for i := len(acquired) - 1; i >= 0; i-- {
			acquired[i].Release(1)
		}
	}

	if hasMedia {
		if err := s.media.Acquire(wctx, 1); err != nil {
			return nil, err
		}
		acquired = append(acquired, s.media)
	}
	if bulk {
		if err := s.bulk.Acquire(wctx, 1); err != nil {
			release()
			return nil, err
		}
		acquired = append(acquired, s.bulk)
	}
	if err := s.global.Acquire(wctx, 1); err != nil {
		release()
		return nil, err
	}
	acquired = append(acquired, s.global)
	return release, nil
}

// ChargeMedia reserves n bytes of the in-flight media budget.
func (s *Slots) ChargeMedia(ctx context.Context, n int64, wait time.Duration) error {
	if n <= 0 {
		return nil
	}
	wctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	if err := s.bytes.Acquire(wctx, n); err != nil {
		return err
	}
	atomic.AddInt64(&s.mediaBytes, n)
	return nil
}

// ReleaseMedia returns n bytes to the budget.
func (s *Slots) ReleaseMedia(n int64) {
	if n <= 0 {
		return
	}
	s.bytes.Release(n)
	atomic.AddInt64(&s.mediaBytes, -n)
}

// MediaBytes returns the currently charged media bytes.
func (s *Slots) MediaBytes() int64 { return atomic.LoadInt64(&s.mediaBytes) }
