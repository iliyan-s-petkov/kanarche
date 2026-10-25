package web

import "testing"

func TestBGOrdinalNeuterTable(t *testing.T) {
	want := map[int]string{
		1: "1-во", 2: "2-ро", 3: "3-то", 4: "4-то", 5: "5-то", 6: "6-то", 7: "7-мо", 8: "8-мо", 9: "9-то", 10: "10-то",
		11: "11-то", 12: "12-то", 13: "13-то", 14: "14-то", 15: "15-то", 16: "16-то", 17: "17-то", 18: "18-то", 19: "19-то",
		20: "20-то", 21: "21-во", 22: "22-ро", 23: "23-то", 24: "24-то", 25: "25-то", 26: "26-то", 27: "27-мо", 28: "28-мо",
		29: "29-то", 30: "30-то",
		100: "100-то", 101: "101-во", 102: "102-ро", 107: "107-мо", 111: "111-то", 112: "112-то", 119: "119-то", 121: "121-во",
	}
	for n, w := range want {
		if got := bgOrdinalNeuter(n); got != w {
			t.Errorf("bgOrdinalNeuter(%d) = %q, want %q", n, got, w)
		}
	}
}

func TestENOrdinalTable(t *testing.T) {
	want := map[int]string{
		1: "1st", 2: "2nd", 3: "3rd", 4: "4th", 9: "9th", 10: "10th", 11: "11th", 12: "12th", 13: "13th", 14: "14th",
		21: "21st", 22: "22nd", 23: "23rd", 24: "24th", 28: "28th", 30: "30th",
		100: "100th", 101: "101st", 111: "111th", 112: "112th", 113: "113th", 121: "121st",
	}
	for n, w := range want {
		if got := enOrdinal(n); got != w {
			t.Errorf("enOrdinal(%d) = %q, want %q", n, got, w)
		}
	}
}

func TestOrdinalByLanguage(t *testing.T) {
	if got := ordinal("bg", 7); got != "7-мо" {
		t.Errorf("bg = %q", got)
	}
	if got := ordinal("en", 7); got != "7th" {
		t.Errorf("en = %q", got)
	}
	if got := ordinal("de", 7); got != "7." {
		t.Errorf("fallback = %q", got)
	}
}

func TestPluralOf(t *testing.T) {
	for n, want := range map[int]string{0: "many", 1: "one", 2: "many", 11: "many", 21: "many"} {
		if got := pluralOf(n, "one", "many"); got != want {
			t.Errorf("pluralOf(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestFormatMeasure(t *testing.T) {
	cases := []struct {
		v    float64
		lang string
		want string
	}{
		{3.14, "bg", "3,1"}, {3.14, "en", "3.1"}, {0.3, "bg", "0,3"}, {9.94, "en", "9.9"},
		{9.96, "en", "10"}, {10, "bg", "10"}, {66.4, "bg", "66"}, {66.6, "en", "67"},
		{1043.9, "bg", "1 044"}, {1043.9, "en", "1,044"}, {2640.2, "bg", "2 640"},
		{999.4, "en", "999"}, {100000, "en", "100,000"}, {44.4, "bg", "44"},
	}
	for _, c := range cases {
		if got := formatMeasure(c.v, c.lang); got != c.want {
			t.Errorf("formatMeasure(%v, %s) = %q, want %q", c.v, c.lang, got, c.want)
		}
	}
}

func TestBearingSectorTable(t *testing.T) {
	// Each target sits one degree of latitude or longitude away from the origin on the named heading.
	cases := []struct {
		name       string
		lat2, lon2 float64
		want       int
		wantKey    string
	}{
		{"north", 43, 25, 0, "area.dir.n"},
		{"north-east", 42.7, 25.8, 1, "area.dir.ne"},
		{"east", 42, 26, 2, "area.dir.e"},
		{"south-east", 41.3, 25.8, 3, "area.dir.se"},
		{"south", 41, 25, 4, "area.dir.s"},
		{"south-west", 41.3, 24.2, 5, "area.dir.sw"},
		{"west", 42, 24, 6, "area.dir.w"},
		{"north-west", 42.7, 24.2, 7, "area.dir.nw"},
	}
	for _, c := range cases {
		got := bearingSector(42, 25, c.lat2, c.lon2)
		if got != c.want {
			t.Errorf("%s: sector %d, want %d", c.name, got, c.want)
		}
		if directionKeys[got] != c.wantKey {
			t.Errorf("%s: key %s, want %s", c.name, directionKeys[got], c.wantKey)
		}
	}
	// 22 degrees is still north, 23 is north-east.
	if got := bearingSector(0, 0, 1, 0.404); got != 0 {
		t.Errorf("22 degrees = %d, want 0", got)
	}
	if got := bearingSector(0, 0, 1, 0.425); got != 1 {
		t.Errorf("23 degrees = %d, want 1", got)
	}
}

func TestRankAscendingTiesShareTheLowerPlace(t *testing.T) {
	vs := []float64{10.0, 12.04, 12.0, 15.0, 9.0}
	// 12.04 and 12.0 are both 12.0 at one decimal.
	if got := rankAscending(12.0, vs); got != 3 {
		t.Errorf("12.0 = %d, want 3", got)
	}
	if got := rankAscending(12.04, vs); got != 3 {
		t.Errorf("12.04 = %d, want 3", got)
	}
	if got := rankAscending(9.0, vs); got != 1 {
		t.Errorf("9.0 = %d, want 1", got)
	}
	if got := rankAscending(15.0, vs); got != 5 {
		t.Errorf("15.0 = %d, want 5", got)
	}
}

func TestRankDescendingTiesShareTheLowerPlace(t *testing.T) {
	vs := []float64{100, 50.04, 50.0, 10}
	if got := rankDescending(100, vs); got != 1 {
		t.Errorf("100 = %d", got)
	}
	if got := rankDescending(50.0, vs); got != 2 {
		t.Errorf("50.0 = %d", got)
	}
	if got := rankDescending(50.04, vs); got != 2 {
		t.Errorf("50.04 = %d", got)
	}
	if got := rankDescending(10, vs); got != 4 {
		t.Errorf("10 = %d", got)
	}
}

func TestStationNormalisation(t *testing.T) {
	typeCases := map[string]string{
		"градска": "градска",
		"извънградска - близо до града": "извънградска",
		" отдалечена ":                  "отдалечена",
		"":                              "",
	}
	for in, want := range typeCases {
		if got := normStationType(in); got != want {
			t.Errorf("normStationType(%q) = %q, want %q", in, got, want)
		}
	}
	areaCases := map[string]string{
		"фонова":       "фонова",
		"промишлен ":   "промишлена", // BG0044A: truncated word plus a no-break space
		"промишлен":    "промишлена",
		"промишлена":   "промишлена",
		" транспортна": "транспортна",
	}
	for in, want := range areaCases {
		if got := normStationArea(in); got != want {
			t.Errorf("normStationArea(%q) = %q, want %q", in, got, want)
		}
	}
	for _, w := range []string{"градска", "извънградска", "отдалечена", "фонова", "транспортна", "промишлена"} {
		if stationWordIDs[w] == "" {
			t.Errorf("no id for %q", w)
		}
	}
}

func TestAOrAn(t *testing.T) {
	for w, want := range map[string]string{"urban": "an", "industrial": "an", "suburban": "a", "remote": "a", "traffic": "a", "": "a"} {
		if got := aOrAn(w); got != want {
			t.Errorf("aOrAn(%q) = %q, want %q", w, got, want)
		}
	}
}

func TestTransliterateBG(t *testing.T) {
	cases := map[string]string{
		"София - АИС Хиподрума":            "Sofia - AIS Hipodruma",
		"АИС Шумен":                        "AIS Shumen",
		"Девня - Изворите":                 "Devnya - Izvorite",
		"АИС Гълъбово":                     "AIS Galabovo",
		"Варна - АИС СОУ Ангел Кънчев":     "Varna - AIS SOU Angel Kanchev",
		"Бургас - АИС кв Долно Езерово":    "Burgas - AIS kv Dolno Ezerovo",
		"Стара Загора - АИС Зелен клин":    "Stara Zagora - AIS Zelen klin",
		"Пловдив - АИС ж.к. Тракия":        "Plovdiv - AIS zh.k. Trakia",
		"София - АИС ИАОС/Павлово":         "Sofia - AIS IAOS/Pavlovo",
		"Бургас - ДОАС РИОСВ":              "Burgas - DOAS RIOSV",
		"Рожен КФС":                        "Rozhen KFS",
		"Плевен - АИС НУ Патриарх Евтимий": "Pleven - AIS NU Patriarh Evtimiy",
		"АИС Видин 2":                      "AIS Vidin 2",
		"Щип Йордан Юнак Ябълка Цар Чугун": "Shtip Yordan Yunak Yabalka Tsar Chugun",
		"ЖК ЩАБ":            "ZHK SHTAB",
		"Ж":                 "Zh",
		"Мария, Юлия и Лия": "Maria, Yulia i Lia",
		"Кърджали - АИС Студен Кладенец": "Kardzhali - AIS Studen Kladenets",
		"Русе - АИС Възраждане":          "Ruse - AIS Vazrazhdane",
		"ь":                              "y",
		"":                               "",
		"Latin stays 123":                "Latin stays 123",
		"Димитровград - АИС Раковски": "Dimitrovgrad - AIS Rakovski",
		"Добрич - АИС ОУ Хан Аспарух": "Dobrich - AIS OU Han Asparuh",
		"Смолян - АИС Библиотеката":   "Smolyan - AIS Bibliotekata",
		"Враца - АИС жп гара":         "Vratsa - AIS zhp gara",
		"АИС Горна Оряховица":         "AIS Gorna Oryahovitsa",
		"Перник - АИС Център":         "Pernik - AIS Tsentar",
		"Варна - АИС Чайка":           "Varna - AIS Chayka",
		"София - АИС Копитото":        "Sofia - AIS Kopitoto",
		"Пловдив - АИС Каменица":      "Plovdiv - AIS Kamenitsa",
		"Бургас - АИС Меден Рудник":   "Burgas - AIS Meden Rudnik",
		"София - АИС Дружба":          "Sofia - AIS Druzhba",
		"София - АИС Надежда":         "Sofia - AIS Nadezhda",
		"София - АИС Младост":         "Sofia - AIS Mladost",
		"АИС Несебър":                 "AIS Nesebar",
		"АИС Ловеч":                   "AIS Lovech",
		"АИС Сливен":                  "AIS Sliven",
		"АИС Благоевград":             "AIS Blagoevgrad",
	}
	for in, want := range cases {
		if got := transliterateBG(in); got != want {
			t.Errorf("transliterateBG(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCapFirst(t *testing.T) {
	for in, want := range map[string]string{"в Шумен": "В Шумен", "във Враца": "Във Враца", "": "", "area": "Area"} {
		if got := capFirst(in); got != want {
			t.Errorf("capFirst(%q) = %q", in, got)
		}
	}
}
