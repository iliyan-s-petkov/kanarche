package api

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// flight shares one build among concurrent callers. The build runs on a context
// detached from every caller and bounded by its own timeout, so one caller
// giving up neither cancels the others nor wastes the build. Errors reach every
// waiter of that build and are not kept; the next call builds again.
type flight[T any] struct {
	mu   sync.Mutex
	call *flightCall[T]
}

type flightCall[T any] struct {
	done chan struct{}
	val  T
	err  error
}

// safeBuild turns a panic into an error. The build runs off the request
// goroutine, where the server's recovery cannot see it.
func safeBuild[T any](ctx context.Context, build func(context.Context) (T, error)) (v T, err error) {
	defer func() {
		if r := recover(); r != nil {
			var zero T
			v, err = zero, fmt.Errorf("flight build panicked: %v", r)
		}
	}()
	return build(ctx)
}

func (f *flight[T]) do(ctx context.Context, timeout time.Duration, build func(context.Context) (T, error)) (T, error) {
	f.mu.Lock()
	c := f.call
	if c == nil {
		c = &flightCall[T]{done: make(chan struct{})}
		f.call = c
		bctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
		go func() {
			defer cancel()
			c.val, c.err = safeBuild(bctx, build)
			f.mu.Lock()
			f.call = nil
			f.mu.Unlock()
			close(c.done)
		}()
	}
	f.mu.Unlock()
	select {
	case <-c.done:
		return c.val, c.err
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}
