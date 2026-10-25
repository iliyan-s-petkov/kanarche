package web_test

import (
	"html"
	"regexp"
	"strings"
	"testing"

	"kanarche.eu/internal/web"
)

var (
	crumbNav = regexp.MustCompile(`(?s)<nav class="breadcrumb"[^>]*>(.*?)</nav>`)
	crumbLI  = regexp.MustCompile(`(?s)<li[^>]*>(.*?)</li>`)
	anyTag   = regexp.MustCompile(`<[^>]*>`)
)

// crumbOLTexts returns the li texts of the page's breadcrumb ol.
func crumbOLTexts(t *testing.T, path, body string) []string {
	t.Helper()
	nav := crumbNav.FindStringSubmatch(body)
	if nav == nil {
		t.Fatalf("%s: no breadcrumb nav", path)
	}
	if !strings.Contains(nav[1], "<ol") {
		t.Fatalf("%s: breadcrumb is not an ol", path)
	}
	var names []string
	for _, m := range crumbLI.FindAllStringSubmatch(nav[1], -1) {
		names = append(names, strings.TrimSpace(html.UnescapeString(anyTag.ReplaceAllString(m[1], ""))))
	}
	if strings.Count(nav[1], "aria-current=\"page\"") != 1 || !strings.Contains(nav[1], "<span aria-current=\"page\">") {
		t.Errorf("%s: want exactly one aria-current span", path)
	}
	return names
}

func TestBreadcrumbOLMatchesBreadcrumbListJSONLD(t *testing.T) {
	area := renderer(t, jsonLDFixture(t))
	news := newsRenderer(t)
	cases := []struct {
		rr   *web.Renderer
		path string
		n    int
	}{
		{area, "/area/plovdiv-oblast", 3},
		{area, "/area/plovdiv", 4},
		{area, "/area/mladost", 5},
		{area, "/en/area/plovdiv-oblast", 3},
		{area, "/en/area/mladost", 5},
		{area, "/pm25", 2},
		{area, "/en/pm10", 2},
		{area, "/air-pollution", 2},
		{area, "/en/air-pollution", 2},
		{area, "/areas", 2},
		{area, "/en/areas", 2},
		{news, "/news", 2},
		{news, "/en/news", 2},
		{news, "/news/2026-09-sofia", 3},
		{news, "/en/news/2026-09-sofia", 3},
	}
	for _, c := range cases {
		body := fetch(t, c.rr, c.path).Body.String()
		names := crumbOLTexts(t, c.path, body)
		list := nodeOfType(jsonLD(t, c.path, body), "BreadcrumbList")
		if list == nil {
			t.Errorf("%s: no BreadcrumbList", c.path)
			continue
		}
		if len(names) != c.n || len(list.ItemListElement) != c.n {
			t.Errorf("%s: li=%d jsonld=%d, want %d", c.path, len(names), len(list.ItemListElement), c.n)
			continue
		}
		for i, item := range list.ItemListElement {
			if item.Name != names[i] {
				t.Errorf("%s: crumb %d name %q, visible %q", c.path, i, item.Name, names[i])
			}
			if !strings.HasPrefix(item.Item, "https://") || item.Position != i+1 {
				t.Errorf("%s: crumb %d = %+v", c.path, i, item)
			}
		}
	}
}

func TestAreasDirectoryBreadcrumbTrail(t *testing.T) {
	rr := renderer(t, jsonLDFixture(t))
	want := map[string][]string{
		"/areas":    {"Карта", "Райони"},
		"/en/areas": {"Map", "Areas"},
	}
	for path, names := range want {
		got := crumbOLTexts(t, path, fetch(t, rr, path).Body.String())
		if strings.Join(got, "|") != strings.Join(names, "|") {
			t.Errorf("%s: crumbs %v, want %v", path, got, names)
		}
	}
}

func TestBreadcrumbAriaLabelPerLanguage(t *testing.T) {
	rr := renderer(t, jsonLDFixture(t))
	for path, label := range map[string]string{"/pm25": "Вие сте тук", "/en/pm25": "Breadcrumb"} {
		body := fetch(t, rr, path).Body.String()
		if !strings.Contains(body, `<nav class="breadcrumb" aria-label="`+label+`">`) {
			t.Errorf("%s: want aria-label %q", path, label)
		}
	}
}
