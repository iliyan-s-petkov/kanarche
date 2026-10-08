//go:build darwin || linux

package e2e

import (
	"bufio"
	"context"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// startGroup builds a command in its own process group. Cancelling ctx sends
// SIGTERM to the whole group, then SIGKILL after grace. Without the group,
// only the direct child (npx) dies and the Playwright runner and its Chrome
// workers keep running.
func startGroup(ctx context.Context, grace time.Duration, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		pgid := cmd.Process.Pid
		time.AfterFunc(grace, func() { _ = syscall.Kill(-pgid, syscall.SIGKILL) })
		return syscall.Kill(-pgid, syscall.SIGTERM)
	}
	cmd.WaitDelay = grace + 2*time.Second
	return cmd
}

// killGroup SIGKILLs the group of a started command; safe if already gone.
func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// waitGone polls until every pid is gone. It checks the pids themselves, not
// -pgid, so the test cannot pass when no group was ever created.
func waitGone(pids []int, within time.Duration) bool {
	end := time.Now().Add(within)
	for time.Now().Before(end) {
		all := true
		for _, p := range pids {
			all = all && syscall.Kill(p, 0) == syscall.ESRCH
		}
		if all {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// spawnTree runs a shell script that prints the pids of n background
// children, one per line, then blocks. It mimics npx -> playwright -> chrome.
func spawnTree(t *testing.T, ctx context.Context, grace time.Duration, script string, n int) (*exec.Cmd, []int) {
	t.Helper()
	cmd := startGroup(ctx, grace, "sh", "-c", script)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { killGroup(cmd) })
	sc := bufio.NewScanner(out)
	var pids []int
	for len(pids) < n && sc.Scan() {
		pid, err := strconv.Atoi(sc.Text())
		if err != nil {
			t.Fatal(err)
		}
		pids = append(pids, pid)
	}
	if len(pids) != n {
		t.Fatalf("got %d child pids, want %d", len(pids), n)
	}
	// Last resort so a failing run does not leave sleeps behind.
	t.Cleanup(func() {
		if !t.Failed() {
			return // pids may be recycled by now
		}
		for _, p := range pids {
			_ = syscall.Kill(p, syscall.SIGKILL)
		}
	})
	return cmd, pids
}

func TestStartGroupCancelKillsGrandchildren(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd, pids := spawnTree(t, ctx, 2*time.Second,
		"sleep 300 & echo $!; sleep 300 & echo $!; wait", 2)

	cancel()
	_ = cmd.Wait()
	if !waitGone(pids, 5*time.Second) {
		t.Fatalf("grandchildren %v survived cancel", pids)
	}
}

// A child that ignores SIGTERM must still die, via the SIGKILL after grace.
func TestStartGroupEscalatesToKill(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd, pids := spawnTree(t, ctx, time.Second,
		"trap '' TERM; (trap '' TERM; exec sleep 300) & echo $!; wait", 1)

	cancel()
	_ = cmd.Wait()
	if !waitGone(pids, 5*time.Second) {
		t.Fatalf("grandchildren %v survived SIGKILL escalation", pids)
	}
}
