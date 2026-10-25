package web

import (
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"kanarche.eu/internal/i18n"
)

// nbsp keeps a number and its unit, or the groups of a number, on one line.
const nbsp = " "

// bgOrdinalNeuter is the digits-and-suffix ordinal for "място": 1-во, 2-ро, 7-мо, 8-мо, 11 to 19 -то, the rest -то.
func bgOrdinalNeuter(n int) string {
	suffix := "-то"
	if r := n % 100; r < 11 || r > 19 {
		switch n % 10 {
		case 1:
			suffix = "-во"
		case 2:
			suffix = "-ро"
		case 7, 8:
			suffix = "-мо"
		}
	}
	return strconv.Itoa(n) + suffix
}

// enOrdinal is 1st, 2nd, 3rd, 4th. 11 to 13 always take th.
func enOrdinal(n int) string {
	suffix := "th"
	if r := n % 100; r < 11 || r > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return strconv.Itoa(n) + suffix
}

// ordinal picks the language's rule. A language without one gets "n." so the sentence still reads.
func ordinal(lang string, n int) string {
	switch lang {
	case i18n.DefaultLang:
		return bgOrdinalNeuter(n)
	case "en":
		return enOrdinal(n)
	}
	return strconv.Itoa(n) + "."
}

// pluralOf is the one/other pair every count noun here uses.
func pluralOf(n int, one, other string) string {
	if n == 1 {
		return one
	}
	return other
}

// groupDigits writes the integer part with the language's group separator.
func groupDigits(digits, sep string) string {
	if len(digits) <= 3 {
		return digits
	}
	var b strings.Builder
	head := len(digits) % 3
	if head > 0 {
		b.WriteString(digits[:head])
	}
	for i := head; i < len(digits); i += 3 {
		if b.Len() > 0 {
			b.WriteString(sep)
		}
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}

// formatMeasure writes an area or a distance: one decimal below 10, a whole number from 10, grouped by thousands.
// Bulgarian groups with a no-break space and takes a comma decimal, English the reverse.
func formatMeasure(v float64, lang string) string {
	group, point := ",", "."
	if lang == i18n.DefaultLang {
		group, point = nbsp, ","
	}
	if v < 9.95 {
		s := strconv.FormatFloat(math.Round(v*10)/10, 'f', 1, 64)
		return strings.Replace(s, ".", point, 1)
	}
	return groupDigits(strconv.FormatInt(int64(math.Round(v)), 10), group)
}

// bearingSector is the 45 degree sector from the first point to the second: 0 north, then clockwise to 7 north-west.
func bearingSector(lat1, lon1, lat2, lon2 float64) int {
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLon := toRad(lon2 - lon1)
	y := math.Sin(dLon) * math.Cos(toRad(lat2))
	x := math.Cos(toRad(lat1))*math.Sin(toRad(lat2)) - math.Sin(toRad(lat1))*math.Cos(toRad(lat2))*math.Cos(dLon)
	deg := math.Mod(math.Atan2(y, x)*180/math.Pi+360, 360)
	return int(math.Floor((deg+22.5)/45)) % 8
}

// directionKeys are the catalogue keys of the eight sectors, in bearingSector order.
var directionKeys = [8]string{
	"area.dir.n", "area.dir.ne", "area.dir.e", "area.dir.se",
	"area.dir.s", "area.dir.sw", "area.dir.w", "area.dir.nw",
}

// rankAscending is the place of v among vs when 1 is the lowest. Values equal at one decimal share the lower place.
func rankAscending(v float64, vs []float64) int {
	r := func(x float64) float64 { return math.Round(x*10) / 10 }
	rank := 1
	for _, o := range vs {
		if r(o) < r(v) {
			rank++
		}
	}
	return rank
}

// rankDescending is the place of v among vs when 1 is the highest.
func rankDescending(v float64, vs []float64) int {
	r := func(x float64) float64 { return math.Round(x*10) / 10 }
	rank := 1
	for _, o := range vs {
		if r(o) > r(v) {
			rank++
		}
	}
	return rank
}

// capFirst upper-cases the first letter, for a phrase that starts a sentence.
func capFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

// stationWordIDs maps the Bulgarian classification words in bg_station_names.json to catalogue ids.
var stationWordIDs = map[string]string{
	"градска": "urban", "извънградска": "suburban", "отдалечена": "remote",
	"фонова": "background", "транспортна": "traffic", "промишлена": "industrial",
}

// normStationType cuts the qualifier after " - " ("извънградска - близо до града") and trims.
func normStationType(s string) string {
	head, _, _ := strings.Cut(s, " - ")
	return strings.TrimSpace(head)
}

// normStationArea trims the source's stray no-break space and restores the one truncated word ("промишлен").
func normStationArea(s string) string {
	s = strings.TrimSpace(s)
	if s == "промишлен" {
		return "промишлена"
	}
	return s
}

// aOrAn picks the English indefinite article by the first letter of the word that follows it.
func aOrAn(word string) string {
	if word != "" && strings.ContainsRune("aeiouAEIOU", rune(word[0])) {
		return "an"
	}
	return "a"
}

// streamlined is the Bulgarian Streamlined System (2009 law), letter by letter.
var streamlined = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ж': "zh", 'з': "z",
	'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p",
	'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "h", 'ц': "ts", 'ч': "ch",
	'ш': "sh", 'щ': "sht", 'ъ': "a", 'ь': "y", 'ю': "yu", 'я': "ya",
}

// transliterateBG writes a Bulgarian name in Latin letters. Case is kept: a capital before a lower-case letter gives a
// capitalised digraph ("Ж" is "Zh"), a capital inside an all-caps word gives caps ("ЖК" is "ZHK"). A word-final
// "ия" becomes "ia". Anything that is not a Cyrillic letter passes through.
func transliterateBG(s string) string {
	rs := []rune(s)
	isLetter := func(i int) bool { return i >= 0 && i < len(rs) && unicode.IsLetter(rs[i]) }
	var b strings.Builder
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		lower := unicode.ToLower(r)
		// Word-final "ия": the "и" is followed by "я" and then a non-letter or the end.
		if lower == 'и' && i+1 < len(rs) && unicode.ToLower(rs[i+1]) == 'я' && !isLetter(i+2) {
			if unicode.IsUpper(rs[i+1]) {
				b.WriteString("IA")
			} else if unicode.IsUpper(r) {
				b.WriteString("Ia")
			} else {
				b.WriteString("ia")
			}
			i++
			continue
		}
		latin, ok := streamlined[lower]
		if !ok {
			b.WriteRune(r)
			continue
		}
		if !unicode.IsUpper(r) {
			b.WriteString(latin)
			continue
		}
		inCaps := (isLetter(i+1) && unicode.IsUpper(rs[i+1])) || (isLetter(i-1) && unicode.IsUpper(rs[i-1]))
		if inCaps {
			b.WriteString(strings.ToUpper(latin))
		} else {
			b.WriteString(capFirst(latin))
		}
	}
	return b.String()
}
