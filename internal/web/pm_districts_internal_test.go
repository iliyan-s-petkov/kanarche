package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The Sofia city page carries the districts heading and intro with the district count; no other page does.
func TestSofiaDistrictsBlockOnlyOnSofiaCity(t *testing.T) {
	rr := acRenderer(t)
	get := func(path string) string {
		rec := httptest.NewRecorder()
		rr.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", path, rec.Code)
		}
		return rec.Body.String()
	}
	for _, c := range []struct{ prefix, heading, intro string }{
		{"", "Райони на София: качество на въздуха по квартали", "Всички 3 района на София"},
		{"/en", "Sofia districts: air quality by neighbourhood", "All 3 districts of Sofia"},
	} {
		body := get(c.prefix + "/area/" + sofiaHubSlug)
		if !strings.Contains(body, c.heading) || !strings.Contains(body, c.intro) {
			t.Errorf("%s Sofia page lacks districts heading or intro", c.prefix)
		}
		for _, other := range []string{"plovdiv", "mladost", "plovdiv-oblast", "sofia-grad-oblast"} {
			if strings.Contains(get(c.prefix+"/area/"+other), c.heading) {
				t.Errorf("%s/area/%s carries the Sofia districts heading", c.prefix, other)
			}
		}
	}
}
