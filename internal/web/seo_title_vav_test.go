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
		{"/area/varna", "Качество на въздуха във Варна сега: ФПЧ2.5 и ФПЧ10"},
		{"/area/vratsa", "Качество на въздуха във Враца сега: ФПЧ2.5 и ФПЧ10"},
		{"/area/veliko-tarnovo", "Качество на въздуха във Велико Търново сега: ФПЧ2.5 и ФПЧ10"},
		{"/area/plovdiv", "Качество на въздуха в Пловдив сега: ФПЧ2.5 и ФПЧ10"},
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
