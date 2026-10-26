package async

import (
	"context"
	"testing"
	"time"
)

func TestNew_PanicsOnNonPositiveCapacity(t *testing.T) {
	for _, cap := range []int{0, -1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("New(%d): expected panic", cap)
				}
			}()
			New(cap)
		}()
	}
}

func TestSubmit_RunsToCompletion(t *testing.T) {
	r := New(1)
	done := make(chan struct{})
	if !r.Submit("ok", time.Second, func(ctx context.Context) error {
		close(done)
		return nil
	}) {
		t.Fatal("submit dropped")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("task never ran")
	}
	if err := r.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r.Submit("late", time.Second, func(context.Context) error { return nil }) {
		t.Fatal("submit after shutdown must be rejected")
	}
}

func TestSubmit_DropsWhenSaturated(t *testing.T) {
	r := New(1)
	block := make(chan struct{})
	if !r.Submit("hold", 0, func(context.Context) error {
		<-block
		return nil
	}) {
		t.Fatal("first submit dropped")
	}
	if r.Submit("overflow", 0, func(context.Context) error { return nil }) {
		t.Fatal("saturated submit must drop")
	}
	close(block)
	_ = r.Shutdown(context.Background())
}
