package web

import (
	"html"
	"html/template"
	"strings"
)

// officialDoc is a primary official document an FAQ answer rests on. The label is the catalogue key
// "faq.ref.<id>"; the URL is per language where the publisher has a Bulgarian version.
type officialDoc struct{ bg, en string }

// officialDocs is the registry of FAQ references, keyed by id. Every URL is https on an official domain.
var officialDocs = map[string]officialDoc{
	"who2021": {"https://www.who.int/publications/i/item/9789240034228", "https://www.who.int/publications/i/item/9789240034228"},
	"dir2008": {"https://eur-lex.europa.eu/legal-content/BG/TXT/?uri=CELEX:32008L0050", "https://eur-lex.europa.eu/legal-content/EN/TXT/?uri=CELEX:32008L0050"},
	"dir2024": {"https://eur-lex.europa.eu/legal-content/BG/TXT/?uri=CELEX:32024L2881", "https://eur-lex.europa.eu/legal-content/EN/TXT/?uri=CELEX:32024L2881"},
	"dir2006": {"https://eur-lex.europa.eu/legal-content/BG/TXT/?uri=CELEX:32006L0007", "https://eur-lex.europa.eu/legal-content/EN/TXT/?uri=CELEX:32006L0007"},
	"ord12":   {"https://lex.bg/laws/ldoc/2135691821", "https://lex.bg/laws/ldoc/2135691821"},
	"eaqi":    {"https://airindex.eea.europa.eu/AQI/index.html", "https://airindex.eea.europa.eu/AQI/index.html"},
	"eeabw":   {"https://www.eea.europa.eu/en/topics/in-depth/bathing-water", "https://www.eea.europa.eu/en/topics/in-depth/bathing-water"},
	"iaos":    {"https://eea.government.bg/", "https://eea.government.bg/"},
	"cams":    {"https://atmosphere.copernicus.eu/", "https://atmosphere.copernicus.eu/"},
	"ecmwf":   {"https://www.ecmwf.int/en/forecasts/datasets/open-data", "https://www.ecmwf.int/en/forecasts/datasets/open-data"},
}

// faqRefIDs lists the official documents behind each FAQ answer. An answer not listed has none.
var (
	aboutFAQRefs = map[int][]string{
		1: {"iaos"}, 4: {"eaqi"}, 10: {"eeabw", "dir2006"}, 11: {"dir2006", "eeabw"},
		12: {"eeabw"}, 14: {"eeabw"}, 15: {"ecmwf"}, 16: {"cams"},
	}
	airPollutionFAQRefs = map[int][]string{
		2: {"eaqi"}, 3: {"who2021", "dir2008", "dir2024", "ord12"}, 6: {"eaqi"}, 7: {"eaqi"}, 10: {"cams"}, 15: {"dir2024"},
	}
)

// FAQRef is one rendered source link.
type FAQRef struct{ Label, Href string }

// withRefs sets the item's references from the registry ids.
func (p PageData) withRefs(item FAQItem, ids ...string) FAQItem {
	for _, id := range ids {
		doc := officialDocs[id]
		href := doc.en
		if p.Lang == "bg" {
			href = doc.bg
		}
		item.Refs = append(item.Refs, FAQRef{Label: p.T("faq.ref." + id), Href: href})
	}
	return item
}

// SourceLine is the "Source:" line under an answer, or empty when the answer has no references.
// It is not part of Answer, so the FAQPage JSON-LD stays the plain answer text.
func (p PageData) SourceLine(item FAQItem) template.HTML {
	if len(item.Refs) == 0 {
		return ""
	}
	label := p.T("faq.ref.one")
	if len(item.Refs) > 1 {
		label = p.T("faq.ref.many")
	}
	anchors := make([]string, 0, len(item.Refs))
	for _, r := range item.Refs {
		anchors = append(anchors, `<a class="link" href="`+html.EscapeString(r.Href)+`" rel="noopener noreferrer">`+html.EscapeString(r.Label)+`</a>`)
	}
	return template.HTML(html.EscapeString(label) + " " + strings.Join(anchors, ", "))
}
