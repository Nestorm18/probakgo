package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestTaskGroupShutdownWaitsForRunningWorkAndRefusesNewWork(t *testing.T) {
	g := newTaskGroup()
	release := make(chan struct{})
	var finished atomic.Bool
	if !g.goTask(func(context.Context) {
		<-release
		finished.Store(true)
	}) {
		t.Fatal("task refused before shutdown")
	}
	done, ok := g.track()
	if !ok {
		t.Fatal("tracked work refused before shutdown")
	}

	result := make(chan error, 1)
	go func() { result <- g.shutdown(context.Background()) }()

	select {
	case err := <-result:
		t.Fatalf("shutdown returned while work was running: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if g.goTask(func(context.Context) {}) {
		t.Fatal("task accepted after shutdown started")
	}
	if _, ok := g.track(); ok {
		t.Fatal("tracked work accepted after shutdown started")
	}

	close(release)
	done()
	if err := <-result; err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if !finished.Load() {
		t.Fatal("shutdown returned before the task finished")
	}
}

func TestTaskGroupShutdownCancelsTaskContextAndHonoursDeadline(t *testing.T) {
	g := newTaskGroup()
	cancelled := make(chan struct{})
	g.goTask(func(ctx context.Context) {
		<-ctx.Done()
		close(cancelled)
	})
	block := make(chan struct{})
	defer close(block)
	g.goTask(func(context.Context) { <-block })

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := g.shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown with stuck work: got %v, want deadline exceeded", err)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel the task context")
	}
}
