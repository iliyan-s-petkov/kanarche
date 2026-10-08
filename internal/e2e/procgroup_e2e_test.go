//go:build e2e && (darwin || linux)

package e2e

import (
	"context"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

const (
	// groupGrace is how long a group gets after SIGTERM before SIGKILL.
	groupGrace = 10 * time.Second
	// deadlineMargin leaves time to tear the group down before go test's own
	// timeout panics and abandons the children.
	deadlineMargin = 30 * time.Second
)

// e2eContext is cancelled on SIGINT/SIGTERM and shortly before the test
// deadline.
func e2eContext(t *testing.T) context.Context {
	t.Helper()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	t.Cleanup(stop)
	if d, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, d.Add(-deadlineMargin))
		t.Cleanup(cancel)
	}
	return ctx
}
