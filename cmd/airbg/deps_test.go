package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The test fixture builder must not link into the production binary.
func TestBinaryHasNoTestFixtureDeps(t *testing.T) {
	gobin := filepath.Join(runtime.GOROOT(), "bin", "go")
	out, err := exec.Command(gobin, "list", "-deps", ".").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range strings.Fields(string(out)) {
		if strings.HasSuffix(pkg, "/xlsxtest") {
			t.Errorf("cmd/airbg links %s", pkg)
		}
	}
}
