package main

import (
	"os/exec"
	"strings"
	"testing"
)

// The test fixture builder must not link into the production binary.
func TestBinaryHasNoTestFixtureDeps(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range strings.Fields(string(out)) {
		if strings.HasSuffix(pkg, "/xlsxtest") {
			t.Errorf("cmd/airbg links %s", pkg)
		}
	}
}
