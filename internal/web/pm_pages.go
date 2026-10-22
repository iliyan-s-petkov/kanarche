package web

import (
	"html/template"
	"net/http"
	"strings"
	"time"
)

// PMPage is what the pm template needs beyond PageData: the catalogue key
// prefix of the page, its filled bands paragraph and the sibling page.
type PMPage struct {
	Key       string
	Bands     string
	OtherPath string
	OtherKey  string
}

// handlePM serves one explainer page; key is "pm25" or "pm10", other the sibling.
func (rr *Renderer) handlePM(key, other string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var generatedAt time.Time
		if snap := rr.holder.Load(); snap != nil {
			generatedAt = snap.GeneratedAt
		}
		lang, path := rr.cat.LangFromPath(r.URL.Path)
		data := rr.newPageData(lang, path, generatedAt)
		seoKey := "seo." + key
		data.Title = composeTitle(rr.cat.T(lang, seoKey+".title"), rr.cat.T(lang, "seo.title_brand"))
		data.Description = rr.cat.T(lang, seoKey+".description")
		// Band edges come from the same replacer the area FAQ uses.
		fill := strings.NewReplacer(eaqiEdgePairs()...)
		data.PM = &PMPage{
			Key:       key,
			Bands:     fill.Replace(data.T(key + ".bands.body")),
			OtherPath: data.Path("/" + other),
			OtherKey:  "guide.links." + other,
		}
		data.JSONLD = rr.mustJSONLD(data.pmJSONLD())
		rr.render(w, r, http.StatusOK, "pm", data)
	}
}

// pmJSONLD is the breadcrumb trail plus an Article for the explainer.
func (p PageData) pmJSONLD() (template.JS, error) {
	person, org := p.controller(), p.organization()
	article := ldArticle{
		Type:             "Article",
		Headline:         p.T(p.PM.Key + ".h1"),
		Description:      p.Description,
		InLanguage:       p.Lang,
		MainEntityOfPage: p.CanonicalURL(),
		Author:           ldRef{ID: person.ID},
		Publisher:        ldRef{ID: org.ID},
	}
	crumbs := ldBreadcrumbList{Type: "BreadcrumbList", ItemListElement: []ldListItem{
		{Type: "ListItem", Position: 1, Name: p.T("area.breadcrumb.home"), Item: p.BaseURL + p.Path("/")},
		{Type: "ListItem", Position: 2, Name: p.T(p.PM.Key + ".h1"), Item: p.CanonicalURL()},
	}}
	return encodeJSONLD(crumbs, article, org, person)
}
