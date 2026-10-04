package web_test

import (
	"regexp"
	"strings"
	"testing"
)

var aboutLinkCardRe = regexp.MustCompile(`(?s)<a [^>]*>.*?</a>`)

// moreCards returns the three anchors inside the .about-links block.
func moreCards(t *testing.T, body string) []string {
	t.Helper()
	i := strings.Index(body, `class="about-links"`)
	if i < 0 {
		t.Fatal("no .about-links block")
	}
	rest := body[i:]
	rest = rest[:strings.Index(rest, "</section>")]
	cards := aboutLinkCardRe.FindAllString(rest, -1)
	if len(cards) != 3 {
		t.Fatalf("want 3 link cards, got %d", len(cards))
	}
	return cards
}

// Every card carries a decorative icon; external cards carry the footer's arrow, the internal one a different one.
func TestAboutLinkCardsHaveIconsAndAffordances(t *testing.T) {
	for _, p := range aboutPaths() {
		cards := moreCards(t, fetch(t, renderer(t, nil), p).Body.String())
		for i, c := range cards {
			svg := regexp.MustCompile(`<svg class="about-links__icon"[^>]*>`).FindString(c)
			if svg == "" || !strings.Contains(svg, `aria-hidden="true"`) {
				t.Errorf("%s card %d: missing aria-hidden icon", p, i)
			}
			if !strings.Contains(c, `stroke="currentColor"`) && !strings.Contains(c, `fill="currentColor"`) {
				t.Errorf("%s card %d: icon does not use currentColor", p, i)
			}
		}
		if !strings.Contains(cards[0], `<span class="about-links__go" aria-hidden="true">→</span>`) {
			t.Errorf("%s: internal card lacks the → affordance", p)
		}
		for i := 1; i <= 2; i++ {
			if !strings.Contains(cards[i], `<span class="about-links__go" aria-hidden="true">↗</span>`) {
				t.Errorf("%s: external card %d lacks the ↗ affordance", p, i)
			}
			if !strings.Contains(cards[i], `rel="noopener`) {
				t.Errorf("%s: external card %d lacks rel=noopener", p, i)
			}
		}
		if strings.Contains(cards[0], "↗") {
			t.Errorf("%s: internal card is marked external", p)
		}
	}
}

// The embed block is hooked by the copycode island; labels come from the catalogue, in both languages.
func TestAboutEmbedCopyButtonMarkup(t *testing.T) {
	want := map[string][3]string{
		"/en/about": {"Copy", "Copied", "Could not copy"},
		"/about":    {"Копирай", "Копирано", "Копирането не успя"},
	}
	for p, l := range want {
		body := fetch(t, renderer(t, nil), p).Body.String()
		for _, frag := range []string{
			`data-island="copycode"`,
			`data-t-copy="` + l[0] + `"`,
			`data-t-copied="` + l[1] + `"`,
			`data-t-failed="` + l[2] + `"`,
		} {
			if !strings.Contains(body, frag) {
				t.Errorf("%s: missing %s", p, frag)
			}
		}
		// The button itself is added by JS: without it there is no dead control.
		if strings.Contains(body, "about-code__copy") {
			t.Errorf("%s: server renders the copy button; it must be JS-added", p)
		}
	}
}
