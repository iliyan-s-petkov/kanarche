package api

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const flightTimeout = 5 * time.Second

// waitJoined polls until n callers have attached, or fails after a deadline.
func waitJoined[T any](t *testing.T, f *flight[T], n int32) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for f.joined.Load() < n {
		if time.Now().After(deadline) {
			t.Fatalf("only %d callers joined, want %d", f.joined.Load(), n)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestFlightSharesOneBuild(t *testing.T) {
	var f flight[int]
	var builds atomic.Int32
	release := make(chan struct{})
	build := func(context.Context) (int, error) {
		builds.Add(1)
		<-release
		return 42, nil
	}
	const n = 20
	var wg sync.WaitGroup
	got := make([]int, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := f.do(context.Background(), flightTimeout, build)
			if err != nil {
				t.Errorf("caller %d: %v", i, err)
			}
			got[i] = v
		}()
	}
	waitJoined(t, &f, n)
	close(release)
	wg.Wait()
	if b := builds.Load(); b != 1 {
		t.Errorf("%d builds, want 1", b)
	}
	for i, v := range got {
		if v != 42 {
			t.Errorf("caller %d got %d, want 42", i, v)
		}
	}
}

func TestFlightErrorIsSharedNotKept(t *testing.T) {
	var f flight[int]
	var builds atomic.Int32
	release := make(chan struct{})
	boom := errors.New("boom")
	failing := func(context.Context) (int, error) {
		builds.Add(1)
		<-release
		return 0, boom
	}
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.do(context.Background(), flightTimeout, failing); !errors.Is(err, boom) {
				t.Errorf("err = %v, want boom", err)
			}
		}()
	}
	waitJoined(t, &f, 5)
	close(release)
	wg.Wait()
	if b := builds.Load(); b != 1 {
		t.Errorf("%d builds for the failing burst, want 1", b)
	}
	v, err := f.do(context.Background(), flightTimeout, func(context.Context) (int, error) { return 7, nil })
	if err != nil || v != 7 {
		t.Errorf("retry = %d, %v; want 7, nil", v, err)
	}
}

func TestFlightCancelledWaiterDoesNotCancelBuild(t *testing.T) {
	var f flight[int]
	release := make(chan struct{})
	build := func(ctx context.Context) (int, error) {
		<-release
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		return 9, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() {
		_, err := f.do(ctx, flightTimeout, build)
		first <- err
	}()
	waitJoined(t, &f, 1)
	second := make(chan int, 1)
	go func() {
		v, _ := f.do(context.Background(), flightTimeout, build)
		second <- v
	}()
	waitJoined(t, &f, 2)
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled caller err = %v, want Canceled", err)
	}
	close(release)
	if v := <-second; v != 9 {
		t.Errorf("surviving caller got %d, want 9", v)
	}
}

func TestFlightAllWaitersCancelBuildEndsAtTimeout(t *testing.T) {
	var f flight[int]
	ended := make(chan error, 1)
	build := func(ctx context.Context) (int, error) {
		<-ctx.Done()
		ended <- ctx.Err()
		return 0, ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	errs := make(chan error, 3)
	for range 3 {
		go func() {
			_, err := f.do(ctx, 50*time.Millisecond, build)
			errs <- err
		}()
	}
	waitJoined(t, &f, 3)
	cancel()
	for range 3 {
		if err := <-errs; err == nil {
			t.Error("a cancelled caller got no error")
		}
	}
	select {
	case err := <-ended:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("build ended with %v, want DeadlineExceeded", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("build goroutine never finished")
	}
}

func TestFlightCallAfterCompletionBuildsAgain(t *testing.T) {
	var f flight[int]
	var builds atomic.Int32
	build := func(context.Context) (int, error) { return int(builds.Add(1)), nil }
	a, _ := f.do(context.Background(), flightTimeout, build)
	b, _ := f.do(context.Background(), flightTimeout, build)
	if a != 1 || b != 2 {
		t.Errorf("got %d then %d, want 1 then 2", a, b)
	}
}

func TestFlightAlreadyCancelledContextDoesNotBuild(t *testing.T) {
	var f flight[int]
	var builds atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.do(ctx, flightTimeout, func(context.Context) (int, error) { builds.Add(1); return 1, nil })
	if !errors.Is(err, context.Canceled) || builds.Load() != 0 {
		t.Errorf("err = %v, builds = %d; want Canceled, 0", err, builds.Load())
	}
}

func TestFlightGoexitDoesNotWedge(t *testing.T) {
	var f flight[int]
	_, err := f.do(context.Background(), flightTimeout, func(context.Context) (int, error) {
		runtime.Goexit()
		return 0, nil
	})
	if !errors.Is(err, errFlightAborted) {
		t.Errorf("err = %v, want errFlightAborted", err)
	}
	v, err := f.do(context.Background(), flightTimeout, func(context.Context) (int, error) { return 3, nil })
	if err != nil || v != 3 {
		t.Errorf("after Goexit = %d, %v; want 3, nil", v, err)
	}
}

func TestFlightPanicBecomesAnError(t *testing.T) {
	var f flight[int]
	_, err := f.do(context.Background(), flightTimeout, func(context.Context) (int, error) { panic("bad") })
	if err == nil {
		t.Fatal("panic not reported as an error")
	}
	if v, err := f.do(context.Background(), flightTimeout, func(context.Context) (int, error) { return 1, nil }); err != nil || v != 1 {
		t.Errorf("after panic = %d, %v; want 1, nil", v, err)
	}
}

func TestFlightBuildHasItsOwnTimeout(t *testing.T) {
	var f flight[int]
	_, err := f.do(context.Background(), 20*time.Millisecond, func(ctx context.Context) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want DeadlineExceeded", err)
	}
}
