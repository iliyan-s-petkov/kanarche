package pdf

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

const goodPath = "/in/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.pdf"

// fakeRunner answers by tool name and records every argv.
type fakeRunner struct {
	out   map[string][]byte
	err   map[string]error
	calls [][]string
}

func (f *fakeRunner) Run(_ context.Context, argv []string, _ int) ([]byte, error) {
	f.calls = append(f.calls, argv)
	return f.out[argv[0]], f.err[argv[0]]
}

func info(enc, js, pages string) []byte {
	return []byte("Title:          x\nEncrypted:      " + enc + "\nJavaScript:     " + js + "\nPages:          " + pages + "\n")
}

func tools(f *fakeRunner) Tools { return Tools{Runner: f, Timeout: time.Second} }

func TestTextRunsFixedArgv(t *testing.T) {
	f := &fakeRunner{out: map[string][]byte{"pdfinfo": info("no", "no", "12"), "pdftotext": []byte("hello")}}
	got, err := tools(f).textAt(context.Background(), goodPath)
	if err != nil || got != "hello" {
		t.Fatalf("got %q, %v", got, err)
	}
	want := [][]string{
		{"pdfinfo", goodPath},
		{"pdftotext", "-layout", "-enc", "UTF-8", "-l", "40", goodPath, "-"},
	}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("argv = %v", f.calls)
	}
}

func TestPages40Allowed(t *testing.T) {
	f := &fakeRunner{out: map[string][]byte{"pdfinfo": info("no", "no", "40"), "pdftotext": []byte("x")}}
	if _, err := tools(f).textAt(context.Background(), goodPath); err != nil {
		t.Fatal(err)
	}
}

// I9: each gate refuses and pdftotext never runs.
func TestGateRefuses(t *testing.T) {
	cases := []struct {
		name string
		out  []byte
		want error
	}{
		{"encrypted", info("yes", "no", "3"), ErrEncrypted},
		{"javascript", info("no", "yes", "3"), ErrJavaScript},
		{"pages41", info("no", "no", "41"), ErrPages},
		{"pages0", info("no", "no", "0"), ErrInfo},
		{"pagesJunk", info("no", "no", "many"), ErrInfo},
		{"missingFields", []byte("Title: x\n"), ErrInfo},
		{"encryptedJunk", info("maybe", "no", "3"), ErrInfo},
		{"empty", nil, ErrInfo},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeRunner{out: map[string][]byte{"pdfinfo": c.out, "pdftotext": []byte("secret")}}
			_, err := tools(f).textAt(context.Background(), goodPath)
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			for _, call := range f.calls {
				if call[0] == "pdftotext" {
					t.Fatal("pdftotext ran after a refused gate")
				}
			}
		})
	}
}

func TestToolFailureIsFixedError(t *testing.T) {
	f := &fakeRunner{out: map[string][]byte{"pdfinfo": info("no", "no", "1")}, err: map[string]error{"pdftotext": errors.New("boom /secret/path")}}
	_, err := tools(f).textAt(context.Background(), goodPath)
	if !errors.Is(err, ErrTool) || strings.Contains(err.Error(), "secret") {
		t.Fatalf("err = %v", err)
	}
	f = &fakeRunner{err: map[string]error{"pdfinfo": errors.New("boom")}}
	if _, err = tools(f).textAt(context.Background(), goodPath); !errors.Is(err, ErrTool) {
		t.Fatalf("err = %v", err)
	}
}

func TestOutputCapMapsToErrTextCap(t *testing.T) {
	f := &fakeRunner{out: map[string][]byte{"pdfinfo": info("no", "no", "1")}, err: map[string]error{"pdftotext": ErrOutputCap}}
	if _, err := tools(f).textAt(context.Background(), goodPath); !errors.Is(err, ErrTextCap) {
		t.Fatalf("err = %v", err)
	}
}

func TestPathMustBeValidatedName(t *testing.T) {
	for _, p := range []string{
		"/in/../etc/passwd",
		"/in/-rf.pdf",
		"/in/ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef0123456789.pdf",
		"/in/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.xls",
		"/in/short.pdf",
		"",
	} {
		f := &fakeRunner{}
		if _, err := tools(f).textAt(context.Background(), p); !errors.Is(err, ErrPath) {
			t.Errorf("%q: err = %v", p, err)
		}
		if len(f.calls) != 0 {
			t.Errorf("%q: tool ran", p)
		}
	}
}

func TestExecRunnerCapsOutput(t *testing.T) {
	_, err := ExecRunner{}.Run(context.Background(), []string{"head", "-c", "100000", "/dev/zero"}, 1000)
	if !errors.Is(err, ErrOutputCap) {
		t.Fatalf("err = %v", err)
	}
	out, err := ExecRunner{}.Run(context.Background(), []string{"head", "-c", "10", "/dev/zero"}, 1000)
	if err != nil || len(out) != 10 {
		t.Fatalf("out %d, err %v", len(out), err)
	}
}

func TestExecRunnerTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := (ExecRunner{}).Run(ctx, []string{"sleep", "10"}, 1000); err == nil {
		t.Fatal("want error")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("timeout not enforced")
	}
}

func TestExecRunnerMissingBinary(t *testing.T) {
	if _, err := (ExecRunner{}).Run(context.Background(), []string{"no-such-tool-xyz"}, 10); err == nil {
		t.Fatal("want error")
	}
}
