package eea

import "testing"

func TestLookupStationInfo(t *testing.T) {
	got, ok := LookupStationInfo("BG0038A")
	if !ok {
		t.Fatal("BG0038A missing")
	}
	if got.Name != "АИС Шумен" || got.Type != "градска" || got.Area != "фонова" {
		t.Errorf("got %+v", got)
	}
	if _, ok := LookupStationInfo("BG9999Z"); ok {
		t.Error("unknown code found")
	}
}

func TestLookupStationInfoKeepsTheSourceQuirks(t *testing.T) {
	// The caller normalises these, so the lookup hands them over untouched.
	got, _ := LookupStationInfo("BG0044A")
	if got.Area != "промишлен " {
		t.Errorf("BG0044A area = %q", got.Area)
	}
	got, _ = LookupStationInfo("BG0070A")
	if got.Type != "извънградска - близо до града" {
		t.Errorf("BG0070A type = %q", got.Type)
	}
}
