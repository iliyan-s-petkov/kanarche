package web_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// hostingSection returns the #hosting block of the About page.
func hostingSection(t *testing.T, path string) (body, section string) {
	t.Helper()
	body = fetch(t, renderer(t, fixture(t)), path).Body.String()
	start := strings.Index(body, `id="hosting"`)
	if start < 0 {
		t.Fatalf("%s has no #hosting section", path)
	}
	start = strings.LastIndex(body[:start], "<section")
	end := strings.Index(body[start:], "</section>")
	if start < 0 || end < 0 {
		t.Fatalf("%s: #hosting is not a closed section", path)
	}
	return body, body[start : start+end]
}

// The credit sits after the "more" links and before the back link.
func TestAboutHostingSitsBetweenMoreAndBack(t *testing.T) {
	for _, p := range aboutPaths() {
		body, _ := hostingSection(t, p)
		more := strings.Index(body, `id="more"`)
		host := strings.Index(body, `id="hosting"`)
		back := strings.Index(body, `class="about-back"`)
		if !(more >= 0 && more < host && host < back) {
			t.Errorf("%s: order more=%d hosting=%d back=%d", p, more, host, back)
		}
	}
}

func TestAboutHostingHasCaptionLinkAndLogo(t *testing.T) {
	captions := map[string]string{
		"/about":    "Хостингът на Канарче е дарен от",
		"/en/about": "Kanarche's hosting is donated by",
	}
	for _, p := range aboutPaths() {
		_, sec := hostingSection(t, p)
		if !strings.Contains(sec, html2(captions[p])) {
			t.Errorf("%s: caption %q missing:\n%s", p, captions[p], sec)
		}
		if n := strings.Count(sec, `href="https://hostellation.com/"`); n != 1 {
			t.Errorf("%s: %d hostellation links, want 1", p, n)
		}
		if n := strings.Count(sec, "<a "); n != 1 {
			t.Errorf("%s: %d anchors, want 1", p, n)
		}
		tag := sec[strings.Index(sec, "<a "):]
		tag = tag[:strings.Index(tag, ">")+1]
		rel := map[string]bool{}
		if m := regexp.MustCompile(`rel="([^"]*)"`).FindStringSubmatch(tag); m != nil {
			for _, tok := range strings.Fields(m[1]) {
				rel[tok] = true
			}
		}
		if !rel["sponsored"] || !rel["noopener"] {
			t.Errorf("%s: rel lacks sponsored/noopener: %s", p, tag)
		}
		if !strings.Contains(sec, `alt="Hostellation"`) || !strings.Contains(sec, "<img ") {
			t.Errorf("%s: logo img with alt Hostellation missing", p)
		}
		if !strings.Contains(sec, "host-hostellation-light") || !strings.Contains(sec, "host-hostellation-dark") {
			t.Errorf("%s: both theme logos must be referenced", p)
		}
	}
}

func TestAboutHostingLogoFilesExist(t *testing.T) {
	for _, f := range []string{"host-hostellation-light.webp", "host-hostellation-dark.webp"} {
		if _, err := os.Stat(filepath.Join("static", "about", f)); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

// html2 escapes the apostrophe the way html/template does.
func html2(s string) string { return strings.ReplaceAll(s, "'", "&#39;") }
