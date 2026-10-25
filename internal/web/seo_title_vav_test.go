package web_test

import (
	"strings"
	"testing"
)

// TestCityTitleTakesVavLikeTheH1: the Bulgarian <title> and og:title use
// "във" before В/Ф names, the same rule as the h1, and keep "в" elsewhere.
func TestCityTitleTakesVavLikeTheH1(t *testing.T) {
	rr := renderer(t, h1Fixture(t))
	cases := []struct {
		path, want string
	}{
		{"/area/varna", "Индекс за качество на въздуха във Варна (AQI) сега"},
		{"/area/vratsa", "Индекс за качество на въздуха във Враца (AQI) сега"},
		{"/area/veliko-tarnovo", "Индекс за качество на въздуха във Велико Търново (AQI) сега"},
		{"/area/plovdiv", "Индекс за качество на въздуха в Пловдив (AQI) сега"},
	}
	for _, c := range cases {
		body := fetch(t, rr, c.path).Body.String()
		if got := pageTitle(t, body); !strings.HasPrefix(got, c.want) {
			t.Errorf("%s: title = %q, want prefix %q", c.path, got, c.want)
		}
		if got := tagAttr(t, body, `<meta property="og:title" content="`); !strings.HasPrefix(got, c.want) {
			t.Errorf("%s: og:title = %q, want prefix %q", c.path, got, c.want)
		}
	}
	// EN has no _vav key and is untouched.
	if got := pageTitle(t, fetch(t, rr, "/en/area/varna").Body.String()); strings.Contains(got, "във") {
		t.Errorf("EN title = %q", got)
	}
}
