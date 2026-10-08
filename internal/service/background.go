package service

import (
	"context"
	"sync"
)

// taskGroup tracks background work so shutdown can wait for it before the
// database is closed. Once closed it refuses new work.
type taskGroup struct {
	mu     sync.Mutex
	closed bool
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

func newTaskGroup() *taskGroup {
	ctx, cancel := context.WithCancel(context.Background())
	return &taskGroup{ctx: ctx, cancel: cancel}
}

// track registers one unit of work. It returns false once shutdown started.
func (g *taskGroup) track() (done func(), ok bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return nil, false
	}
	g.wg.Add(1)
	return g.wg.Done, true
}

// goTask runs fn in a tracked goroutine. fn's context is cancelled when
// shutdown starts, so long jobs can stop early and record their outcome.
func (g *taskGroup) goTask(fn func(ctx context.Context)) bool {
	done, ok := g.track()
	if !ok {
		return false
	}
	go func() {
		defer done()
		fn(g.ctx)
	}()
	return true
}

// shutdown refuses new work, cancels the work context and waits for running
// tasks until ctx ends.
func (g *taskGroup) shutdown(ctx context.Context) error {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
	g.cancel()
	finished := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(finished)
	}()
	select {
	case <-finished:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

var background = newTaskGroup()

// Go runs fn as tracked background work. It returns false during shutdown.
func Go(fn func(ctx context.Context)) bool {
	return background.goTask(fn)
}

// Track registers work that runs on an existing goroutine. Call done when it
// finishes. It returns false during shutdown.
func Track() (done func(), ok bool) {
	return background.track()
}

// WaitBackground stops accepting background work and waits for running work
// until ctx ends. Call it after the HTTP server stopped and before closing the
// database.
func WaitBackground(ctx context.Context) error {
	return background.shutdown(ctx)
}
