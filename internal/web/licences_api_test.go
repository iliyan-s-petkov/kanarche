package web_test

import (
	"strings"
	"testing"

	"kanarche.eu/internal/i18n"
)

func TestLicencesPointsAtTheTermsForTheAPI(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct{ sentence, href string }{
		"/licences": {
			"Публичният API на /api/v1 предоставя същите данни като картата. Всеки източник запазва своя лиценз. Ако използвате данните, посочете всеки източник, както е описано по-долу. Данните се предоставят такива, каквито са, без гаранции, както е описано в ",
			"/terms",
		},
		"/en/licences": {
			"The public API at /api/v1 serves the same data as the map. Each source keeps its own licence. If you reuse the data, credit each source as listed below. The data comes as it is, with no warranty, as set out in the ",
			"/en/terms",
		},
	}
	for path, w := range want {
		body := fetch(t, rziRenderer(t, true), path).Body.String()
		link := `<a href="` + w.href + `">` + cat.T(rziLangs[path], "licences.api.terms") + `</a>`
		if !strings.Contains(body, w.sentence+link+".") {
			t.Errorf("%s lacks the API sentence ending in %s", path, link)
		}
		intro := strings.Index(body, cat.T(rziLangs[path], "licences.intro"))
		api := strings.Index(body, w.sentence)
		if intro < 0 || api < intro {
			t.Errorf("%s: API sentence must follow the intro", path)
		}
	}
}
