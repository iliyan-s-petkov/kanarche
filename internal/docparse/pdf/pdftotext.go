// Package pdf runs poppler's pdfinfo and pdftotext on one untrusted PDF.
// Argv is fixed, no shell is involved, every run has a timeout and bounded
// output, and errors are fixed texts that never carry document content.
package pdf

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Caps for one document.
const (
	MaxPages     = 40
	MaxTextBytes = 2 << 20
	maxInfoBytes = 64 << 10
	maxErrBytes  = 64 << 10
	// DefaultTimeout is per tool run.
	DefaultTimeout = 20 * time.Second
)

// Fixed refusal texts. Callers may put Error() into a record.
var (
	ErrPath       = errors.New("path is not a validated input name")
	ErrTool       = errors.New("pdf tool failed")
	ErrInfo       = errors.New("pdfinfo output not usable")
	ErrEncrypted  = errors.New("pdf is encrypted")
	ErrJavaScript = errors.New("pdf contains javascript")
	ErrPages      = errors.New("pdf has too many pages")
	ErrTextCap    = errors.New("pdf text over size cap")
	ErrOutputCap  = errors.New("tool output over cap")
)

var reName = regexp.MustCompile(`^[0-9a-f]{64}\.pdf$`)

// Runner runs one command by argv (argv[0] is looked up on PATH) and
// returns stdout. It must return ErrOutputCap when stdout exceeds maxOut.
type Runner interface {
	Run(ctx context.Context, argv []string, maxOut int) ([]byte, error)
}

// Tools wires a Runner to the two poppler commands.
type Tools struct {
	Runner  Runner
	Timeout time.Duration
}

// Default returns Tools backed by real processes.
func Default() Tools { return Tools{Runner: ExecRunner{}, Timeout: DefaultTimeout} }

// Text gates the PDF with pdfinfo, then returns its pdftotext -layout text.
// path must be a validated /<dir>/<sha256>.pdf name.
func (t Tools) Text(ctx context.Context, path string) (string, error) {
	if !validPath(path) {
		return "", ErrPath
	}
	if err := t.gate(ctx, path); err != nil {
		return "", err
	}
	out, err := t.run(ctx, []string{"pdftotext", "-layout", "-enc", "UTF-8", "-l", strconv.Itoa(MaxPages), path, "-"}, MaxTextBytes)
	if errors.Is(err, ErrOutputCap) {
		return "", ErrTextCap
	}
	if err != nil {
		return "", ErrTool
	}
	return string(out), nil
}

// validPath checks the last path element only; the directory is the
// caller's fixed input directory.
func validPath(p string) bool {
	i := strings.LastIndexByte(p, '/')
	return reName.MatchString(p[i+1:])
}

func (t Tools) run(ctx context.Context, argv []string, maxOut int) ([]byte, error) {
	if t.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.Timeout)
		defer cancel()
	}
	return t.Runner.Run(ctx, argv, maxOut)
}

// gate is the pre-check: unreadable, encrypted, scripted or long PDFs are
// refused before pdftotext sees them. Missing fields fail closed.
func (t Tools) gate(ctx context.Context, path string) error {
	out, err := t.run(ctx, []string{"pdfinfo", path}, maxInfoBytes)
	if err != nil {
		return ErrTool
	}
	fields := map[string]string{}
	for _, l := range strings.Split(string(out), "\n") {
		k, v, ok := strings.Cut(l, ":")
		if ok {
			fields[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	enc, err := yesNo(fields["Encrypted"])
	if err != nil {
		return err
	}
	js, err := yesNo(fields["JavaScript"])
	if err != nil {
		return err
	}
	pages, perr := strconv.Atoi(fields["Pages"])
	switch {
	case enc:
		return ErrEncrypted
	case js:
		return ErrJavaScript
	case perr != nil || pages < 1:
		return ErrInfo
	case pages > MaxPages:
		return ErrPages
	}
	return nil
}

func yesNo(s string) (bool, error) {
	switch s {
	case "yes":
		return true, nil
	case "no":
		return false, nil
	}
	return false, ErrInfo
}

// ExecRunner runs real processes. Stdout is read up to maxOut, stderr is
// kept to a small bound and dropped.
type ExecRunner struct{}

// Run implements Runner.
func (ExecRunner) Run(ctx context.Context, argv []string, maxOut int) ([]byte, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	out := &capWriter{max: maxOut}
	cmd.Stdout = out
	cmd.Stderr = &capWriter{max: maxErrBytes, drop: true}
	cmd.Stdin = nil
	// Without WaitDelay a stuck child holding the pipes could block Wait.
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if out.over {
		return nil, ErrOutputCap
	}
	if err != nil {
		return nil, ErrTool
	}
	return out.buf.Bytes(), nil
}

// capWriter stops accepting data past max. With drop set it discards the
// excess silently instead of failing the copy.
type capWriter struct {
	buf  bytes.Buffer
	max  int
	drop bool
	over bool
}

func (w *capWriter) Write(p []byte) (int, error) {
	room := w.max - w.buf.Len()
	if len(p) <= room {
		w.buf.Write(p)
		return len(p), nil
	}
	if room > 0 {
		w.buf.Write(p[:room])
	}
	if w.drop {
		return len(p), nil
	}
	w.over = true
	return room, ErrOutputCap
}
