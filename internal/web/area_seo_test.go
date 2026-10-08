package web_test

import (
	"encoding/xml"
	"regexp"
	"strings"
	"testing"
)

// crumb is one visible breadcrumb entry as a reader sees it: its text and the
// URL it links to. The current page is text, so its href is the request path.
type crumb struct{ name, href string }

var (
	crumbNavRE  = regexp.MustCompile(`(?s)<nav class="breadcrumb"[^>]*>(.*?)</nav>`)
	crumbItemRE = regexp.MustCompile(`<a class="link" href="([^"]*)">([^<]*)</a>|<span aria-current="page">([^<]*)</span>`)
)

// visibleCrumbs reads the breadcrumb nav out of body, in document order.
func visibleCrumbs(t *testing.T, path, body string) []crumb {
	t.Helper()
	nav := crumbNavRE.FindStringSubmatch(body)
	if nav == nil {
		t.Fatalf("%s: no breadcrumb nav", path)
	}
	var out []crumb
	for _, m := range crumbItemRE.FindAllStringSubmatch(nav[1], -1) {
		if m[3] != "" {
			out = append(out, crumb{name: m[3], href: path})
			continue
		}
		out = append(out, crumb{name: m[2], href: m[1]})
	}
	return out
}

// TestAreaBreadcrumbMatchesJSONLD: the visible trail and the BreadcrumbList
// carry the same names and item URLs in the same order, for a district, a
// city and a province, in both languages.
func TestAreaBreadcrumbMatchesJSONLD(t *testing.T) {
	rr := renderer(t, areaPageFixture(t))
	paths := []string{
		"/area/mladost", "/area/sofiya", "/area/plovdiv-oblast",
		"/en/area/mladost", "/en/area/sofiya", "/en/area/plovdiv-oblast",
	}
	for _, path := range paths {
		body := fetch(t, rr, path).Body.String()
		visible := visibleCrumbs(t, path, body)
		list := nodeOfType(jsonLD(t, path, body), "BreadcrumbList")
		if list == nil {
			t.Fatalf("%s: no BreadcrumbList", path)
		}
		if len(list.ItemListElement) != len(visible) {
			t.Fatalf("%s: JSON-LD has %d crumbs, visible trail has %d: %+v / %+v",
				path, len(list.ItemListElement), len(visible), list.ItemListElement, visible)
		}
		for i, item := range list.ItemListElement {
			if item.Name != visible[i].name {
				t.Errorf("%s: crumb %d name = %q in JSON-LD, %q on the page", path, i, item.Name, visible[i].name)
			}
			if want := "https://airbg.org" + visible[i].href; item.Item != want {
				t.Errorf("%s: crumb %d item = %q in JSON-LD, want %q", path, i, item.Item, want)
			}
		}
	}
}

// TestAreaNoIndexWhenUncovered: an area without recent data is served with
// robots noindex in both languages; a covered area carries no noindex.
func TestAreaNoIndexWhenUncovered(t *testing.T) {
	rr := renderer(t, areaPageFixture(t))
	const noindex = `<meta name="robots" content="noindex">`
	for _, path := range []string{"/area/silistra", "/en/area/silistra", "/area/silistra-oblast", "/en/area/silistra-oblast"} {
		if body := fetch(t, rr, path).Body.String(); !strings.Contains(body, noindex) {
			t.Errorf("%s: uncovered area has no robots noindex meta", path)
		}
	}
	for _, path := range []string{"/area/plovdiv", "/en/area/plovdiv"} {
		if body := fetch(t, rr, path).Body.String(); strings.Contains(body, `content="noindex"`) {
			t.Errorf("%s: covered area is marked noindex", path)
		}
	}
}

// TestSitemapOmitsUncoveredAreas: an uncovered area's BG and EN URLs are left
// out of the sitemap; a covered area's are listed.
func TestSitemapOmitsUncoveredAreas(t *testing.T) {
	rec := fetch(t, renderer(t, areaPageFixture(t)), "/sitemap.xml")
	var doc sitemapXML
	if err := xml.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("sitemap.xml did not parse: %v", err)
	}
	listed := map[string]bool{}
	for _, u := range doc.URLs {
		listed[u.Loc] = true
	}
	for _, loc := range []string{
		"https://airbg.org/area/silistra", "https://airbg.org/en/area/silistra",
		"https://airbg.org/area/silistra-oblast", "https://airbg.org/en/area/silistra-oblast",
	} {
		if listed[loc] {
			t.Errorf("sitemap lists uncovered area %s", loc)
		}
	}
	for _, loc := range []string{"https://airbg.org/area/plovdiv", "https://airbg.org/en/area/plovdiv"} {
		if !listed[loc] {
			t.Errorf("sitemap omits covered area %s", loc)
		}
	}
}
