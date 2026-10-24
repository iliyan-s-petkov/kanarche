package web_test

import (
	"net/http"
	"strings"
	"testing"
)

// TestOpenGraphTagsOnIndexablePages checks every required og:*/twitter:*
// tag, that URLs are absolute and built from the configured BaseURL (not a
// hard-coded host), and that og:url equals the page's own canonical link.
func TestOpenGraphTagsOnIndexablePages(t *testing.T) {
	rr := renderer(t, seoFixture(t))

	for _, path := range []string{"/", "/en/", "/areas", "/about-the-data", "/area/plovdiv", "/en/area/plovdiv"} {
		body := fetch(t, rr, path).Body.String()

		canonical := tagAttr(t, body, `<link rel="canonical" href="`)
		if !strings.HasPrefix(canonical, "https://airbg.org") {
			t.Errorf("%s: canonical %q is not built from the configured BaseURL", path, canonical)
		}

		ogURL := tagAttr(t, body, `<meta property="og:url" content="`)
		if ogURL != canonical {
			t.Errorf("%s: og:url = %q, want it to equal the canonical %q", path, ogURL, canonical)
		}

		if !hasMarker(body, `<meta property="og:type" content="website">`) {
			t.Errorf("%s: missing og:type", path)
		}
		siteName := tagAttr(t, body, `<meta property="og:site_name" content="`)
		if siteName == "" {
			t.Errorf("%s: missing og:site_name", path)
		}
		ogTitle := tagAttr(t, body, `<meta property="og:title" content="`)
		if ogTitle == "" || strings.Contains(ogTitle, siteName) {
			t.Errorf("%s: og:title = %q, want the page title core without the brand suffix", path, ogTitle)
		}
		title := pageTitle(t, body)
		if !strings.HasPrefix(title, ogTitle) {
			t.Errorf("%s: og:title %q is not a prefix of the rendered <title> %q", path, ogTitle, title)
		}
		ogDesc := tagAttr(t, body, `<meta property="og:description" content="`)
		if ogDesc != pageDescription(t, body) {
			t.Errorf("%s: og:description = %q, want it to equal the static meta description %q", path, ogDesc, pageDescription(t, body))
		}

		ogImage := tagAttr(t, body, `<meta property="og:image" content="`)
		// Area pages carry their own 1200x630 card, every other page the shared preview.
		wantImage, wantW, wantH := "https://airbg.org/static/social-preview.png", "1280", "640"
		if strings.Contains(path, "/area/") {
			wantImage, wantW, wantH = "https://airbg.org/static/og/", "1200", "630"
		}
		if !strings.HasPrefix(ogImage, wantImage) {
			t.Errorf("%s: og:image = %q, want an absolute %s URL", path, ogImage, wantImage)
		}
		if !hasMarker(body, `<meta property="og:image:width" content="`+wantW+`">`) {
			t.Errorf("%s: og:image:width != %s", path, wantW)
		}
		if !hasMarker(body, `<meta property="og:image:height" content="`+wantH+`">`) {
			t.Errorf("%s: og:image:height != %s", path, wantH)
		}
		if tagAttr(t, body, `<meta property="og:image:alt" content="`) == "" {
			t.Errorf("%s: og:image:alt is empty", path)
		}
		locale := tagAttr(t, body, `<meta property="og:locale" content="`)
		if locale != "bg_BG" && locale != "en_US" {
			t.Errorf("%s: og:locale = %q, want bg_BG or en_US", path, locale)
		}
		if !hasMarker(body, `<meta property="og:locale:alternate" content="`) {
			t.Errorf("%s: missing og:locale:alternate", path)
		}
		if !hasMarker(body, `<meta name="twitter:card" content="summary_large_image">`) {
			t.Errorf("%s: missing twitter:card", path)
		}

		// The image the tag points at must actually exist and be a PNG.
		imgPath := strings.TrimPrefix(ogImage, "https://airbg.org")
		img := fetch(t, rr, imgPath)
		if img.Code != http.StatusOK {
			t.Errorf("%s: og:image %s served status %d, want 200", path, imgPath, img.Code)
		}
		if ct := img.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
			t.Errorf("%s: og:image %s Content-Type = %q, want image/png", path, imgPath, ct)
		}
	}
}

// TestOpenGraphTagsAbsentOnErrorAndEmbedPages: the 404 skips canonical
// entirely (see TestNotFoundPageHasNoIndexAndNoCanonical), and /embed
// redefines its own head with no OG block — og:url would otherwise have
// nothing correct to equal.
func TestOpenGraphTagsAbsentOnErrorAndEmbedPages(t *testing.T) {
	rr := renderer(t, seoFixture(t))

	notFound := fetch(t, rr, "/area/does-not-exist").Body.String()
	if hasMarker(notFound, `property="og:`) {
		t.Error("404 page carries Open Graph tags")
	}

	embed := framed(t, rr, "/embed").Body.String()
	if hasMarker(embed, `property="og:`) {
		t.Error("/embed carries Open Graph tags")
	}
}

// TestOGTitleNeverCarriesTheBrandSuffix. og:site_name already states the
// brand; og:title repeating it would be redundant on every share card
// (seo-copy.md §5), independent of whether the visible <title> itself carries
// the suffix.
func TestOGTitleNeverCarriesTheBrandSuffix(t *testing.T) {
	rr := renderer(t, seoFixture(t))
	for _, path := range []string{"/", "/en/", "/area/plovdiv", "/area/veliko-tarnovo-oblast"} {
		body := fetch(t, rr, path).Body.String()
		ogTitle := tagAttr(t, body, `<meta property="og:title" content="`)
		if strings.Contains(ogTitle, "airbg.org") || strings.HasSuffix(ogTitle, " | Kanarche") || strings.HasSuffix(ogTitle, " | Канарче") {
			t.Errorf("%s: og:title %q carries the brand suffix", path, ogTitle)
		}
	}
}
