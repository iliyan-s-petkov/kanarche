package pdf

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGateRefusesDuplicateKeys(t *testing.T) {
	cases := map[string]string{
		"jsAfterReal":     "Title: x\nJavaScript:     yes\nEncrypted: no\nPages: 3\nJavaScript: no\n",
		"jsBeforeReal":    "JavaScript: no\nEncrypted:      no\nJavaScript:     yes\nPages:          3\n",
		"encBefore":       "Encrypted: no\nEncrypted:      yes\nJavaScript:     no\nPages:          3\n",
		"pagesAfter":      "Encrypted:      no\nJavaScript:     no\nPages:          99\nPages: 3\n",
		"injectedInTitle": "Title:          a\nJavaScript: no\nEncrypted:      no\nJavaScript:     yes\nPages:          3\n",
	}
	for name, out := range cases {
		t.Run(name, func(t *testing.T) {
			f := &fakeRunner{out: map[string][]byte{"pdfinfo": []byte(out), "pdftotext": []byte("x")}}
			if _, err := tools(f).textAt(context.Background(), goodPath); !errors.Is(err, ErrInfo) {
				t.Fatalf("err = %v", err)
			}
			if len(f.calls) != 1 {
				t.Fatal("pdftotext ran")
			}
		})
	}
}

func TestEncryptedWithPermissionsIsErrEncrypted(t *testing.T) {
	f := &fakeRunner{out: map[string][]byte{"pdfinfo": []byte("Encrypted:      yes (print:yes copy:no change:no addNotes:no algorithm:RC4)\nJavaScript:     no\nPages:          3\n")}}
	if _, err := tools(f).textAt(context.Background(), goodPath); !errors.Is(err, ErrEncrypted) {
		t.Fatalf("err = %v", err)
	}
}

// pathRunner inspects the file the tools are given while they run.
type pathRunner struct {
	paths []string
	data  [][]byte
	mode  os.FileMode
	dir   os.FileMode
}

func (p *pathRunner) Run(_ context.Context, argv []string, _ int) ([]byte, error) {
	path := argv[len(argv)-1]
	if argv[0] == "pdftotext" {
		path = argv[len(argv)-2]
	}
	p.paths = append(p.paths, path)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	p.data = append(p.data, b)
	if st, err := os.Stat(path); err == nil {
		p.mode = st.Mode().Perm()
	}
	if st, err := os.Stat(filepath.Dir(path)); err == nil {
		p.dir = st.Mode().Perm()
	}
	if argv[0] == "pdfinfo" {
		return info("no", "no", "1"), nil
	}
	return []byte("t"), nil
}

func TestTextRunsToolsOnPrivateCopy(t *testing.T) {
	data := []byte("%PDF-1.7 verified bytes")
	r := &pathRunner{}
	if _, err := (Tools{Runner: r}).Text(context.Background(), data); err != nil {
		t.Fatal(err)
	}
	if len(r.paths) != 2 {
		t.Fatalf("paths %v", r.paths)
	}
	for i, p := range r.paths {
		if !bytes.Equal(r.data[i], data) {
			t.Fatal("tool saw other bytes")
		}
		if !strings.HasPrefix(p, os.TempDir()) {
			t.Fatalf("path %s not under temp dir", p)
		}
	}
	if r.mode != 0o600 || r.dir != 0o700 {
		t.Fatalf("modes file %o dir %o", r.mode, r.dir)
	}
	if _, err := os.Stat(filepath.Dir(r.paths[0])); !os.IsNotExist(err) {
		t.Fatal("private copy not removed")
	}
}

func TestExecRunnerKillsChildOnOverCap(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	start := time.Now()
	_, err := ExecRunner{}.Run(ctx, []string{"sh", "-c", "head -c 100000 /dev/zero; sleep 10"}, 1000)
	if !errors.Is(err, ErrOutputCap) {
		t.Fatalf("err = %v", err)
	}
	if d := time.Since(start); d > 4*time.Second {
		t.Fatalf("took %v", d)
	}
}
