package api

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const flightTimeout = 5 * time.Second

// settle gives every goroutine time to reach do before the build is released.
func settle() { time.Sleep(100 * time.Millisecond) }

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
	settle()
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
	settle()
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
	settle()
	second := make(chan int, 1)
	go func() {
		v, _ := f.do(context.Background(), flightTimeout, build)
		second <- v
	}()
	settle()
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled caller err = %v, want Canceled", err)
	}
	close(release)
	if v := <-second; v != 9 {
		t.Errorf("surviving caller got %d, want 9", v)
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
