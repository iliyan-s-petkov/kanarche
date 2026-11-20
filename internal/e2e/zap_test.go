//go:build e2e

package e2e

import (
	"os"
	"testing"
	"time"
)

// TestServeForZAP boots the e2e stack and keeps it up for the ZAP baseline
// scan (.github/workflows/zap.yml). It writes the base URL to the file named
// by KANARCHE_ZAP_URL_FILE and serves until the file named by
// KANARCHE_ZAP_STOP_FILE appears. Unset, it skips, so the e2e job ignores it.
func TestServeForZAP(t *testing.T) {
	urlFile := os.Getenv("KANARCHE_ZAP_URL_FILE")
	stopFile := os.Getenv("KANARCHE_ZAP_STOP_FILE")
	if urlFile == "" || stopFile == "" {
		t.Skip("KANARCHE_ZAP_URL_FILE and KANARCHE_ZAP_STOP_FILE not set")
	}
	baseURL := bootStack(t)
	// Write then rename so the workflow never reads a half-written URL.
	if err := os.WriteFile(urlFile+".tmp", []byte(baseURL), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(urlFile+".tmp", urlFile); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(stopFile); err == nil {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatal("the ZAP scan never signalled stop")
}
