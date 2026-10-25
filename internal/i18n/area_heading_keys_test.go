package i18n_test

import (
	"strings"
	"testing"
)

// The four area page section headings are shared with the chart PR, so the key names and the copy are fixed here.
func TestAreaHeadingKeys(t *testing.T) {
	c := loaded(t)
	want := map[string][2]string{
		"area.heading.pollutants": {"Замърсители", "Pollutants"},
		"area.heading.weather":    {"Времето", "Weather"},
		"area.heading.history":    {"История", "History"},
		"area.heading.sensors":    {"Сензори", "Sensors"},
	}
	for key, langs := range want {
		for i, lang := range []string{"bg", "en"} {
			if !c.Has(lang, key) {
				t.Errorf("%s.json lacks %s", lang, key)
				continue
			}
			got := c.T(lang, key)
			if got != langs[i] {
				t.Errorf("%s %s = %q, want %q", lang, key, got, langs[i])
			}
			if strings.ContainsAny(got, ";—–") {
				t.Errorf("%s %s = %q holds a semicolon or dash", lang, key, got)
			}
		}
	}
}
