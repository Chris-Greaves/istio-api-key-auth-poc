package main

import (
	"context"
	"testing"
	"time"
)

func TestNewTrafficWindowContext_AnchorsDeadlineAtCallTimeNotCtxCreation(t *testing.T) {
	ctx := context.Background()

	// Simulate time elapsed during bootstrap/baseline-scrape setup, which
	// happens before newTrafficWindowContext is called in main(). If the
	// deadline were mistakenly anchored earlier (e.g. at ctx's creation, as
	// --duration used to be applied), this sleep would already have
	// consumed most or all of the traffic window before it even starts.
	time.Sleep(50 * time.Millisecond)

	trafficCtx, cancel := newTrafficWindowContext(ctx, 30*time.Millisecond)
	defer cancel()

	if err := trafficCtx.Err(); err != nil {
		t.Fatalf("expected the traffic-window context to still be valid immediately after creation, got %v", err)
	}

	deadline, ok := trafficCtx.Deadline()
	if !ok {
		t.Fatal("expected a deadline to be set")
	}
	if remaining := time.Until(deadline); remaining <= 0 {
		t.Fatalf("expected time remaining before the deadline, got %v", remaining)
	}
}

func TestNewTrafficWindowContext_ZeroDurationIsUnbounded(t *testing.T) {
	ctx := context.Background()

	trafficCtx, cancel := newTrafficWindowContext(ctx, 0)
	defer cancel()

	if trafficCtx != ctx {
		t.Fatal("expected the original context to be returned unchanged for a zero duration")
	}
	if _, ok := trafficCtx.Deadline(); ok {
		t.Fatal("expected no deadline when duration is zero")
	}
}

func TestNewTrafficWindowContext_ExpiresAfterItsOwnDuration(t *testing.T) {
	ctx := context.Background()

	trafficCtx, cancel := newTrafficWindowContext(ctx, 10*time.Millisecond)
	defer cancel()

	select {
	case <-trafficCtx.Done():
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected the traffic-window context to expire after its duration")
	}
}
