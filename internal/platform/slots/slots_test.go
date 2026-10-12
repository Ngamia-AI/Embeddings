package slots

import (
	"context"
	"testing"
	"time"
)

func TestGlobalSlotExhaustion(t *testing.T) {
	s := New(1, 1, 1<<20)
	release, err := s.Acquire(context.Background(), false, false, time.Second)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if _, err := s.Acquire(context.Background(), false, false, 50*time.Millisecond); err == nil {
		t.Fatal("second acquire should time out while slot busy")
	}
	release()
	release2, err := s.Acquire(context.Background(), false, false, time.Second)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	release2()
}

func TestBulkSlotExhaustion(t *testing.T) {
	s := New(4, 2, 1<<20)
	release, err := s.Acquire(context.Background(), true, false, time.Second)
	if err != nil {
		t.Fatalf("bulk acquire: %v", err)
	}
	if _, err := s.Acquire(context.Background(), true, false, 50*time.Millisecond); err == nil {
		t.Fatal("second bulk acquire should time out")
	}
	// Text request can still take a global slot.
	rel2, err := s.Acquire(context.Background(), false, false, time.Second)
	if err != nil {
		t.Fatalf("text acquire should succeed: %v", err)
	}
	rel2()
	release()
}

func TestMediaByteBudget(t *testing.T) {
	s := New(2, 1, 100)
	if err := s.ChargeMedia(context.Background(), 60, time.Second); err != nil {
		t.Fatalf("charge 60: %v", err)
	}
	if err := s.ChargeMedia(context.Background(), 60, 50*time.Millisecond); err == nil {
		t.Fatal("charge beyond budget should time out")
	}
	if got := s.MediaBytes(); got != 60 {
		t.Fatalf("media bytes = %d, want 60", got)
	}
	s.ReleaseMedia(60)
	if got := s.MediaBytes(); got != 0 {
		t.Fatalf("media bytes = %d, want 0", got)
	}
	if err := s.ChargeMedia(context.Background(), 60, time.Second); err != nil {
		t.Fatalf("charge after release: %v", err)
	}
}
