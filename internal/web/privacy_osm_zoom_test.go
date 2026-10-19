package web_test

import (
	"strings"
	"testing"
)

// TestPrivacyPageStatesOSMZoomCap pins the owner-approved privacy copy: the
// OpenStreetMap raster basemap is loaded only up to zoom 16, not at every zoom.
// The phrase is checked on both rendered privacy routes so a stale catalogue
// entry in either language fails the build.
func TestPrivacyPageStatesOSMZoomCap(t *testing.T) {
	rr := renderer(t, fixture(t))

	for _, tc := range []struct {
		path string
		want string
	}{
		{"/privacy", "до ниво на мащаб 16"},
		{"/en/privacy", "up to zoom level 16"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			body := fetch(t, rr, tc.path).Body.String()
			if !strings.Contains(body, tc.want) {
				t.Errorf("%s does not contain %q", tc.path, tc.want)
			}
		})
	}
}
