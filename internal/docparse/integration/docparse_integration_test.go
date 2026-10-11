//go:build docparse

// Package integration runs the docparse image against the hostile corpus
// under the sandbox flags. See internal/docparse/README.md.
package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kanarche.eu/internal/upstream/rzi"
	"kanarche.eu/internal/upstream/rzi/burgas"
	"kanarche.eu/internal/upstream/rzi/varna"
)

const (
	image      = "kanarche-docparse:it"
	probeImage = "kanarche-docparse:it-probe"
	// wallClock is the per-file limit; a run past it is killed.
	wallClock = 20 * time.Second
	repoRoot  = "../../.."
)

// sandboxFlags are the sandbox row of the RZI safety design; the host unit
// (Task 11) uses the same set.
var sandboxFlags = []string{
	"--network", "none",
	"--read-only",
	"--tmpfs", "/tmp:size=16m,noexec",
	"--cap-drop", "ALL",
	"--security-opt", "no-new-privileges",
	"--user", "65534:65534",
	"--memory", "256m", "--memory-swap", "256m",
	"--cpus", "0.5",
	"--pids-limit", "64",
}

func TestMain(m *testing.M) {
	if err := buildImages(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	docker(context.Background(), "rmi", "-f", image, probeImage)
	os.Exit(code)
}

func buildImages() error {
	for _, b := range [][]string{
		{"build", "-q", "-f", "Dockerfile.docparse", "-t", image, "."},
		{"build", "-q", "-f", "Dockerfile.docparse", "--target", "probe", "-t", probeImage, "."},
	} {
		cmd := exec.Command("docker", b...)
		cmd.Dir = repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("docker %s: %v\n%s", strings.Join(b, " "), err, out)
		}
	}
	return nil
}

func docker(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

var seq atomic.Int64

// result is one finished or killed container run.
type result struct {
	exit    int
	killed  bool // by the wall clock
	oom     bool
	elapsed time.Duration
	logs    string
}

// runContainer creates, starts and waits for one container, killing it at
// limit. The container is removed when the test ends.
func runContainer(t *testing.T, flags []string, img string, args []string, limit time.Duration) result {
	t.Helper()
	ctx := context.Background()
	name := fmt.Sprintf("rzi-t8-%d-%d", os.Getpid(), seq.Add(1))
	create := append([]string{"create", "--name", name}, flags...)
	create = append(append(create, img), args...)
	if out, err := docker(ctx, create...); err != nil {
		t.Fatalf("docker create: %v\n%s", err, out)
	}
	t.Cleanup(func() { docker(context.Background(), "rm", "-f", name) })

	start := time.Now()
	if out, err := docker(ctx, "start", name); err != nil {
		t.Fatalf("docker start: %v\n%s", err, out)
	}
	wctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	_, werr := docker(wctx, "wait", name)
	r := result{elapsed: time.Since(start)}
	if errors.Is(wctx.Err(), context.DeadlineExceeded) {
		r.killed = true
		docker(ctx, "kill", name)
		docker(ctx, "wait", name)
	} else if werr != nil {
		t.Fatalf("docker wait: %v", werr)
	}
	st, err := docker(ctx, "inspect", "-f", "{{.State.ExitCode}} {{.State.OOMKilled}}", name)
	if err != nil {
		t.Fatalf("docker inspect: %v\n%s", err, st)
	}
	f := strings.Fields(st)
	r.exit, _ = strconv.Atoi(f[0])
	r.oom = f[1] == "true"
	r.logs, _ = docker(ctx, "logs", name)
	return r
}

// dirFor makes a host directory the container user can use.
func dirFor(t *testing.T, mode os.FileMode) string {
	t.Helper()
	d := t.TempDir()
	if err := os.Chmod(d, mode); err != nil {
		t.Fatal(err)
	}
	return d
}

// runOne runs the image on one input under the sandbox flags and returns
// the run, the input hash and the record read back.
func runOne(t *testing.T, ext string, data []byte) (result, string, rzi.File, error) {
	t.Helper()
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	in, out := dirFor(t, 0o755), dirFor(t, 0o777)
	if err := os.WriteFile(filepath.Join(in, hash+"."+ext), data, 0o644); err != nil {
		t.Fatal(err)
	}
	mounts := append([]string{}, sandboxFlags...)
	mounts = append(mounts, "-v", in+":/in:ro", "-v", out+":/out")
	r := runContainer(t, mounts, image, nil, wallClock)
	rec, err := readRecord(out, hash)
	return r, hash, rec, err
}

// TestPositiveControls proves the image can produce an ok record under the
// same flags: a broken poppler, uid or mount would refuse these too. The
// image's blocks must equal what the extractor gives in process.
func TestPositiveControls(t *testing.T) {
	pdfData, text := varnaControl(t)
	xls, err := os.ReadFile(burgasControl)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, ext string
		data      []byte
		want      []rzi.Block
	}{
		{"varna.pdf", "pdf", pdfData, varna.Parse(text).Blocks},
		{"burgas.xls", "xls", xls, burgas.Extract(xls).Blocks},
	} {
		t.Run(c.name, func(t *testing.T) {
			if len(c.want) == 0 {
				t.Fatal("in-process extraction gives no blocks; the control is inert")
			}
			r, hash, rec, err := runOne(t, c.ext, c.data)
			if err != nil {
				t.Fatalf("no valid record (exit %d killed %v oom %v): %v\n%s", r.exit, r.killed, r.oom, err, r.logs)
			}
			t.Logf("OUTCOME %s ok=%v %s blocks=%d error=%q", c.name, rec.Error == "", r.elapsed.Round(time.Millisecond), len(rec.Blocks), rec.Error)
			if rec.Error != "" || len(rec.Blocks) == 0 {
				t.Fatalf("record error %q, %d blocks; want ok", rec.Error, len(rec.Blocks))
			}
			if rec.SHA256 != hash || r.killed || r.exit != 0 {
				t.Fatalf("sha %s exit %d killed %v", rec.SHA256, r.exit, r.killed)
			}
			if !reflect.DeepEqual(rec.Blocks, c.want) {
				t.Fatalf("image blocks differ from in-process extraction:\n got %+v\nwant %+v", rec.Blocks, c.want)
			}
		})
	}
}

func TestHostileCorpus(t *testing.T) {
	for _, f := range append(hostileCorpus(), controlPDF()) {
		t.Run(f.name, func(t *testing.T) {
			r, hash, rec, recErr := runOne(t, f.ext, f.data)
			// oom alone is not a kill of the run: the kernel may kill only a
			// poppler child and rzi-extract still writes its refusal.
			if errors.Is(recErr, fs.ErrNotExist) && (r.killed || r.oom) {
				t.Logf("OUTCOME %s killed %s oom=%v exit=%d", f.name, r.elapsed.Round(time.Millisecond), r.oom, r.exit)
				if f.want != "" {
					t.Fatalf("killed, want refusal %q", f.want)
				}
				return
			}
			if recErr != nil {
				t.Fatalf("no valid record (exit %d): %v\n%s", r.exit, recErr, r.logs)
			}
			t.Logf("OUTCOME %s rejected %s %q oom=%v", f.name, r.elapsed.Round(time.Millisecond), rec.Error, r.oom)
			if rec.Error == "" || len(rec.Blocks) != 0 {
				t.Fatalf("record is ok, want rejected: %+v", rec)
			}
			if f.want != "" && rec.Error != f.want {
				t.Fatalf("refusal %q, want %q", rec.Error, f.want)
			}
			if rec.SHA256 != hash {
				t.Fatalf("record sha %s", rec.SHA256)
			}
			if !r.killed && r.exit != 0 {
				t.Fatalf("exit %d with a record\n%s", r.exit, r.logs)
			}
			if r.elapsed > wallClock+5*time.Second {
				t.Fatalf("run took %s", r.elapsed)
			}
		})
	}
}

func readRecord(dir, hash string) (rzi.File, error) {
	fh, err := os.Open(filepath.Join(dir, hash+".json"))
	if err != nil {
		return rzi.File{}, err
	}
	defer fh.Close()
	return rzi.Decode(fh)
}

// TestNetworkNone is invariant I1 at runtime. The listener is a sibling
// container the test owns on the default bridge; no public address is used.
// The control run proves the probe works when a network exists.
func TestNetworkNone(t *testing.T) {
	ctx := context.Background()
	name := fmt.Sprintf("rzi-t8-listener-%d", os.Getpid())
	// Registered first: a failed run -d can still leave a created container.
	t.Cleanup(func() { docker(context.Background(), "rm", "-f", name) })
	if out, err := docker(ctx, "run", "-d", "--name", name, "--read-only", "--cap-drop", "ALL",
		"--user", "65534:65534", probeImage, "-listen", ":8080"); err != nil {
		t.Fatalf("listener: %v\n%s", err, out)
	}
	ip, err := docker(ctx, "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", name)
	if err != nil || ip == "" {
		t.Fatalf("listener ip %q: %v", ip, err)
	}
	addr := ip + ":8080"

	var withNet []string
	for i := 0; i < len(sandboxFlags); i++ {
		if sandboxFlags[i] == "--network" {
			i++
			continue
		}
		withNet = append(withNet, sandboxFlags[i])
	}
	reached := false
	for range 10 {
		if r := runContainer(t, withNet, probeImage, []string{"-dial", addr}, wallClock); r.exit == 0 {
			reached = true
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if !reached {
		t.Fatal("control: probe with a network did not reach the listener; the I1 check would be inert")
	}

	r := runContainer(t, sandboxFlags, probeImage, []string{"-dial", addr}, wallClock)
	if r.killed || r.exit == 0 {
		t.Fatalf("sandboxed probe: exit %d killed %v logs %q; want a fast failure", r.exit, r.killed, r.logs)
	}
}

// TestImageShape checks the published image: non-root, rzi-extract as the
// entrypoint, no probe binary, and the pinned poppler.
func TestImageShape(t *testing.T) {
	ctx := context.Background()
	cfg, err := docker(ctx, "image", "inspect", "-f", "{{.Config.User}}|{{json .Config.Entrypoint}}", image)
	if err != nil {
		t.Fatal(err, cfg)
	}
	if cfg != `65534:65534|["/usr/local/bin/rzi-extract"]` {
		t.Fatalf("image config %s", cfg)
	}
	// The listing must succeed and hold only rzi-extract; a daemon or image
	// error fails here instead of reading as "netprobe absent".
	bins, err := docker(ctx, "run", "--rm", "--network", "none", "--entrypoint", "ls", image, "-1", "/usr/local/bin")
	if err != nil || bins != "rzi-extract" {
		t.Fatalf("/usr/local/bin in the published image: %q (%v); want only rzi-extract", bins, err)
	}

	v, err := docker(ctx, "run", "--rm", "--network", "none", "--entrypoint", "pdftotext", image, "-v")
	if err != nil {
		t.Fatalf("pdftotext -v: %v\n%s", err, v)
	}
	// Debian security revisions move freely; the poppler series must not.
	m := regexp.MustCompile(`pdftotext version 25\.(\d+)\.\d+`).FindStringSubmatch(v)
	if m == nil {
		t.Fatalf("image poppler %q, want 25.x", v)
	}
	if minor, _ := strconv.Atoi(m[1]); minor < 3 {
		t.Fatalf("image poppler %q, want 25.03 or later", v)
	}
	readme, err := os.ReadFile(filepath.Join(repoRoot, "internal/docparse/README.md"))
	if err != nil || !bytes.Contains(readme, []byte("poppler-utils 25.03")) {
		t.Fatal("internal/docparse/README.md does not record the poppler series")
	}
}
