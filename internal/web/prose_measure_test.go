package web

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The prose line length is one token, defined in both theme.css copies, and
// app.css caps prose with it instead of a hard-coded ch value.
func TestProseMeasureTokenInBothThemes(t *testing.T) {
	for _, p := range []string{"static/theme.css", "../../web/src/styles/theme.css"} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if !strings.Contains(string(b), "--prose-measure: clamp(65ch, 55vw, 90ch);") {
			t.Errorf("%s does not define --prose-measure", p)
		}
	}
}

var hardProseCap = regexp.MustCompile(`max-(?:inline-size|width):\s*\d+ch`)

func TestAppCSSHasNoHardCodedProseCap(t *testing.T) {
	b, err := os.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(b), "\n") {
		if hardProseCap.MatchString(line) {
			t.Errorf("app.css:%d hard-coded ch cap, use var(--prose-measure): %s", i+1, strings.TrimSpace(line))
		}
	}
}
