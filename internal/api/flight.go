package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

// flight shares one build among concurrent callers. The build runs on a context
// detached from every caller and bounded by its own timeout, so one caller
// giving up neither cancels the others nor wastes the build. Errors reach every
// waiter of that build and are not kept; the next call builds again.
type flight[T any] struct {
	mu   sync.Mutex
	call *flightCall[T]
	// joined counts callers that attached to a build. Tests poll it.
	joined atomic.Int32
}

type flightCall[T any] struct {
	done chan struct{}
	val  T
	err  error
}

// errFlightAborted is what waiters get when the build goroutine exits without returning.
var errFlightAborted = errors.New("flight build aborted")

// safeBuild turns a panic into an error. The build runs off the request
// goroutine, where the server's recovery cannot see it.
func safeBuild[T any](ctx context.Context, build func(context.Context) (T, error)) (v T, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("flight build panicked", "panic", r, "stack", string(debug.Stack()))
			var zero T
			v, err = zero, fmt.Errorf("flight build panicked: %v", r)
		}
	}()
	return build(ctx)
}

func (f *flight[T]) do(ctx context.Context, timeout time.Duration, build func(context.Context) (T, error)) (T, error) {
	if err := ctx.Err(); err != nil {
		var zero T
		return zero, err
	}
	f.mu.Lock()
	c := f.call
	if c == nil {
		c = &flightCall[T]{done: make(chan struct{})}
		f.call = c
		bctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
		go func() {
			defer cancel()
			// Runs on panic and Goexit too. The call is cleared before done
			// closes so a woken caller that retries starts a new build.
			defer func() {
				f.mu.Lock()
				f.call = nil
				f.mu.Unlock()
				close(c.done)
			}()
			c.err = errFlightAborted
			c.val, c.err = safeBuild(bctx, build)
		}()
	}
	f.joined.Add(1)
	f.mu.Unlock()
	select {
	case <-c.done:
		return c.val, c.err
	case <-ctx.Done():
		// Prefer a result that is already in.
		select {
		case <-c.done:
			return c.val, c.err
		default:
		}
		var zero T
		return zero, ctx.Err()
	}
}
