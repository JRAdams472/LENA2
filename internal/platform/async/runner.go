// Package async provides a bounded, shutdown-aware runner for detached
// background work such as analytics writes and OCR post-processing.
package async

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Runner executes background tasks on a bounded worker semaphore scoped to
// a service-lifetime context. Submit is non-blocking: when all slots are
// taken it reports the task as dropped so callers can surface BUSY instead
// of silently promising work that never runs.
type Runner struct {
	capacity int

	// mu serializes Submit's counter increment against Shutdown so a
	// submission can never Add concurrently with an in-progress Wait.
	mu     sync.Mutex
	closed bool

	ctx    context.Context
	cancel context.CancelFunc
	sem    chan struct{}
	wg     sync.WaitGroup
}

// New returns a Runner with the given maximum in-flight task count.
func New(capacity int) *Runner {
	ctx, cancel := context.WithCancel(context.Background())
	return &Runner{capacity: capacity, ctx: ctx, cancel: cancel, sem: make(chan struct{}, capacity)}
}

// Submit runs fn detached with its own timeout. It reports whether the
// task was accepted; saturated pools and post-Shutdown submissions drop
// the task and log a warning.
func (r *Runner) Submit(name string, timeout time.Duration, fn func(ctx context.Context) error) bool {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		slog.Default().Warn("async worker closed; dropping task", "task", name)
		return false
	}
	r.wg.Add(1)
	r.mu.Unlock()
	select {
	case r.sem <- struct{}{}:
	default:
		r.wg.Done()
		slog.Default().Warn("async worker saturated; dropping task", "task", name)
		return false
	}
	go func() {
		defer r.wg.Done()
		defer func() { <-r.sem }()
		defer func() {
			if rec := recover(); rec != nil {
				slog.Default().Error("async task panic recovered", "task", name, "recover", rec)
			}
		}()
		ctx := r.ctx
		var cancel context.CancelFunc
		if timeout > 0 {
			ctx, cancel = context.WithTimeout(r.ctx, timeout)
			defer cancel()
		}
		if err := fn(ctx); err != nil {
			slog.Default().Error("async task failed", "task", name, "error", err)
		}
	}()
	return true
}

// Shutdown drains in-flight tasks before returning: it waits for workers
// to finish and only cancels the shared context when the caller's deadline
// expires — tasks are never aborted merely because shutdown was requested.
func (r *Runner) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		r.cancel()
		return ctx.Err()
	}
}
