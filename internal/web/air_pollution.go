package web

import (
	"html"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// airPollutionFAQCount is the number of air_pollution.faq.q/a pairs in the catalogue.
const airPollutionFAQCount = 8

// phLink is a {placeholder} in catalogue copy that renders as an anchor.
type phLink struct{ Placeholder, Label, Href string }

// linkedText fills the poll interval and EAQI edges, escapes the copy, then swaps each link placeholder for an anchor.
func (p PageData) linkedText(key string, links ...phLink) template.HTML {
	pairs := append([]string{"{minutes}", strconv.Itoa(p.pollMinutes)}, eaqiEdgePairs()...)
	out := html.EscapeString(strings.NewReplacer(pairs...).Replace(p.T(key)))
	for _, l := range links {
		anchor := `<a class="link" href="` + html.EscapeString(l.Href) + `">` + html.EscapeString(l.Label) + `</a>`
		out = strings.ReplaceAll(out, l.Placeholder, anchor)
	}
	return template.HTML(out)
}

// APText renders one air_pollution body key with its links.
func (p PageData) APText(key string) template.HTML {
	return p.linkedText(key,
		phLink{"{pm25_link}", p.T("air_pollution.pm.link25"), p.Path("/pm25")},
		phLink{"{pm10_link}", p.T("air_pollution.pm.link10"), p.Path("/pm10")},
		phLink{"{areas_link}", p.T("air_pollution.read.areas_link"), p.Path("/areas")},
		phLink{"{data_link}", p.T("air_pollution.read.data_link"), p.Path("/about-the-data")},
	)
}

// HomeExplainerLine is the home page's pointer to the explainer and the data FAQ.
func (p PageData) HomeExplainerLine() template.HTML {
	return p.linkedText("home.explainer.line",
		phLink{"{explainer_link}", p.T("home.explainer.explainer_link"), p.Path("/air-pollution")},
		phLink{"{faq_link}", p.T("home.explainer.faq_link"), p.Path("/about-the-data") + "#faq"},
	)
}

// AreaExplainerLine is the pointer under the area FAQ.
func (p PageData) AreaExplainerLine() template.HTML {
	return p.linkedText("faq.explainer_line",
		phLink{"{link}", p.T("faq.explainer_link"), p.Path("/air-pollution")})
}

// airPollutionFAQ builds the explainer's FAQ once, so the markup and the FAQPage JSON-LD share strings.
func (p PageData) airPollutionFAQ() *FAQBlock {
	fill := strings.NewReplacer(append([]string{"{minutes}", strconv.Itoa(p.pollMinutes)}, eaqiEdgePairs()...)...)
	block := &FAQBlock{Heading: p.T("air_pollution.faq.heading")}
	for n := 1; n <= airPollutionFAQCount; n++ {
		num := strconv.Itoa(n)
		block.Items = append(block.Items, newFAQItem(
			fill.Replace(p.T("air_pollution.faq.q"+num)), fill.Replace(p.T("air_pollution.faq.a"+num))))
	}
	return block
}

// handleAirPollution serves the air pollution explainer.
func (rr *Renderer) handleAirPollution(w http.ResponseWriter, r *http.Request) {
	var generatedAt time.Time
	if snap := rr.holder.Load(); snap != nil {
		generatedAt = snap.GeneratedAt
	}
	lang, path := rr.cat.LangFromPath(r.URL.Path)
	data := rr.newPageData(lang, path, generatedAt)
	data.Title = composeTitle(rr.cat.T(lang, "seo.air_pollution.title"), rr.cat.T(lang, "seo.title_brand"))
	data.Description = rr.cat.T(lang, "seo.air_pollution.description")
	data.AirPollutionFAQ = data.airPollutionFAQ()
	data.JSONLD = rr.mustJSONLD(data.airPollutionJSONLD())
	rr.render(w, r, http.StatusOK, "air_pollution", data)
}

// airPollutionJSONLD is the breadcrumb trail, an Article and the FAQPage built from the visible FAQ.
func (p PageData) airPollutionJSONLD() (template.JS, error) {
	person, org := p.controller(), p.organization()
	article := ldArticle{
		Type:             "Article",
		Headline:         p.T("air_pollution.h1"),
		Description:      p.Description,
		InLanguage:       p.Lang,
		MainEntityOfPage: p.CanonicalURL(),
		Author:           ldRef{ID: person.ID},
		Publisher:        ldRef{ID: org.ID},
	}
	crumbs := ldBreadcrumbList{Type: "BreadcrumbList", ItemListElement: []ldListItem{
		{Type: "ListItem", Position: 1, Name: p.T("area.breadcrumb.home"), Item: p.BaseURL + p.Path("/")},
		{Type: "ListItem", Position: 2, Name: p.T("air_pollution.breadcrumb"), Item: p.CanonicalURL()},
	}}
	return encodeJSONLD(crumbs, article, faqPageOf(p.AirPollutionFAQ), org, person)
}
