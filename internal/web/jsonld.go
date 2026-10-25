package web

import (
	"encoding/json"
	"html/template"
	"time"

	"kanarche.eu/internal/i18n"
)

// Structured data (OpenProject #607): schema.org JSON-LD emitted by base.gohtml.

const (
	schemaContext = "https://schema.org"
	sourceRepoURL = "https://github.com/iliyan-s-petkov/kanarche"
	odblURL       = "https://opendatacommons.org/licenses/odbl/1-0/"
	mitURL        = "https://opensource.org/license/mit"
	// Privacy controller: a natural person, kept apart from the Organization publisher.
	controllerName  = "Iliyan Petkov"
	controllerEmail = "privacy@kanarche.eu"
	// First day of the hourly series on prod; reading_hourly keeps two years (migration 00003).
	dataCoverageStart  = "2026-08-31"
	dataRetentionYears = 2
	// Square logo shipped in static/, used as the Organization logo.
	logoStatic, logoSize = "apple-touch-icon.png", 180
	// Bulgaria's extent from data/boundaries/bulgaria.geojson, "south west north east".
	bulgariaBox = "41.2381 22.3450 44.2284 28.6035"
)

type ldGraph struct {
	Context string            `json:"@context"`
	Graph   []json.RawMessage `json:"@graph"`
}

type ldRef struct {
	ID string `json:"@id"`
}

type ldPerson struct {
	Type  string `json:"@type"`
	ID    string `json:"@id,omitempty"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ldImageObject struct {
	Type   string `json:"@type"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type ldOrganization struct {
	Type          string        `json:"@type"`
	ID            string        `json:"@id"`
	Name          string        `json:"name"`
	AlternateName string        `json:"alternateName"`
	URL           string        `json:"url"`
	Logo          ldImageObject `json:"logo"`
	SameAs        []string      `json:"sameAs"`
}

type ldWebSite struct {
	Type          string   `json:"@type"`
	ID            string   `json:"@id"`
	Name          string   `json:"name"`
	AlternateName string   `json:"alternateName,omitempty"`
	URL           string   `json:"url"`
	InLanguage    string   `json:"inLanguage"`
	Publisher     ldRef    `json:"publisher"`
	SameAs        []string `json:"sameAs,omitempty"`
}

type ldGeoShape struct {
	Type string `json:"@type"`
	Box  string `json:"box"`
}

type ldGeoCoordinates struct {
	Type      string  `json:"@type"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type ldPlace struct {
	Type             string          `json:"@type"`
	Name             string          `json:"name"`
	URL              string          `json:"url,omitempty"`
	Geo              json.RawMessage `json:"geo,omitempty"`
	ContainedInPlace *ldPlace        `json:"containedInPlace,omitempty"`
}

type ldWebApplication struct {
	Type                string `json:"@type"`
	ID                  string `json:"@id"`
	Name                string `json:"name"`
	Description         string `json:"description"`
	URL                 string `json:"url"`
	InLanguage          string `json:"inLanguage"`
	ApplicationCategory string `json:"applicationCategory"`
	OperatingSystem     string `json:"operatingSystem"`
	IsAccessibleForFree bool   `json:"isAccessibleForFree"`
	License             string `json:"license"`
	IsPartOf            ldRef  `json:"isPartOf"`
	IsBasedOn           ldRef  `json:"isBasedOn"`
	Publisher           ldRef  `json:"publisher"`
	Creator             ldRef  `json:"creator"`
}

type ldPropertyValue struct {
	Type     string `json:"@type"`
	Name     string `json:"name"`
	UnitText string `json:"unitText,omitempty"`
}

type ldDataDownload struct {
	Type           string `json:"@type"`
	Name           string `json:"name"`
	EncodingFormat string `json:"encodingFormat"`
	ContentURL     string `json:"contentUrl"`
}

type ldDataset struct {
	Type                string            `json:"@type"`
	ID                  string            `json:"@id"`
	Name                string            `json:"name"`
	Description         string            `json:"description"`
	URL                 string            `json:"url"`
	InLanguage          string            `json:"inLanguage"`
	License             string            `json:"license"`
	IsAccessibleForFree bool              `json:"isAccessibleForFree"`
	Creator             ldRef             `json:"creator"`
	Publisher           ldRef             `json:"publisher"`
	TemporalCoverage    string            `json:"temporalCoverage"`
	SpatialCoverage     ldPlace           `json:"spatialCoverage"`
	VariableMeasured    []ldPropertyValue `json:"variableMeasured"`
	Distribution        []ldDataDownload  `json:"distribution"`
	DateModified        string            `json:"dateModified,omitempty"`
}

type ldListItem struct {
	Type     string `json:"@type"`
	Position int    `json:"position"`
	Name     string `json:"name"`
	Item     string `json:"item"`
}

type ldAnswer struct {
	Type string `json:"@type"`
	Text string `json:"text"`
}

type ldQuestion struct {
	Type           string   `json:"@type"`
	Name           string   `json:"name"`
	AcceptedAnswer ldAnswer `json:"acceptedAnswer"`
}

type ldFAQPage struct {
	Type       string       `json:"@type"`
	MainEntity []ldQuestion `json:"mainEntity"`
}

type ldArticle struct {
	Type             string `json:"@type"`
	Headline         string `json:"headline"`
	Description      string `json:"description"`
	InLanguage       string `json:"inLanguage"`
	MainEntityOfPage string `json:"mainEntityOfPage"`
	Author           ldRef  `json:"author"`
	Publisher        ldRef  `json:"publisher"`
	DatePublished    string `json:"datePublished,omitempty"`
	DateModified     string `json:"dateModified,omitempty"`
	Image            string `json:"image,omitempty"`
}

type ldBreadcrumbList struct {
	Type            string       `json:"@type"`
	ItemListElement []ldListItem `json:"itemListElement"`
}

// encodeJSONLD wraps nodes in one @graph; json.Marshal escapes <, > and & so no value can close the script.
func encodeJSONLD(nodes ...interface{ ldNode() }) (template.JS, error) {
	g := ldGraph{Context: schemaContext}
	for _, n := range nodes {
		raw, err := json.Marshal(n)
		if err != nil {
			return "", err
		}
		g.Graph = append(g.Graph, raw)
	}
	out, err := json.Marshal(g)
	if err != nil {
		return "", err
	}
	return template.JS(out), nil
}

func (ldPerson) ldNode()         {}
func (ldOrganization) ldNode()   {}
func (ldWebSite) ldNode()        {}
func (ldWebApplication) ldNode() {}
func (ldDataset) ldNode()        {}
func (ldBreadcrumbList) ldNode() {}
func (ldPlace) ldNode()          {}
func (ldFAQPage) ldNode()        {}
func (ldArticle) ldNode()        {}

// controller is the natural person named in the privacy notice.
func (p PageData) controller() ldPerson {
	return ldPerson{Type: "Person", ID: p.BaseURL + "/#controller", Name: controllerName, Email: controllerEmail}
}

// organization is the publisher of the graph; sameAs is GitHub plus any configured social profiles.
func (p PageData) organization() ldOrganization {
	other := "en"
	if p.Lang != i18n.DefaultLang {
		other = i18n.DefaultLang
	}
	sameAs := []string{sourceRepoURL}
	for _, u := range []string{p.FacebookURL, p.LinkedInURL} {
		if u != "" {
			sameAs = append(sameAs, u)
		}
	}
	return ldOrganization{
		Type: "Organization", ID: p.BaseURL + "/#org", Name: p.T("seo.title_brand"),
		AlternateName: p.cat.T(other, "seo.title_brand"), URL: p.BaseURL + "/",
		Logo:   ldImageObject{Type: "ImageObject", URL: p.BaseURL + p.Static(logoStatic), Width: logoSize, Height: logoSize},
		SameAs: sameAs,
	}
}

// temporalCoverage runs from the first stored day, or two years before the snapshot once older data is gone.
func (p PageData) temporalCoverage() string {
	start, err := time.Parse("2006-01-02", dataCoverageStart)
	if err != nil {
		return ""
	}
	if cut := p.GeneratedAt.UTC().AddDate(-dataRetentionYears, 0, 0); !p.GeneratedAt.IsZero() && cut.After(start) {
		start = cut
	}
	return start.Format("2006-01-02") + "/.."
}

// datasetID is language-independent: the Dataset lives on about-the-data and the home WebApplication references it.
func (p PageData) datasetID() string { return p.BaseURL + "/about-the-data#dataset" }

func (p PageData) homeJSONLD() (template.JS, error) {
	person, org := p.controller(), p.organization()
	site := ldWebSite{
		Type:          "WebSite",
		ID:            p.BaseURL + "/#website",
		Name:          p.T("seo.title_brand"),
		AlternateName: p.T("brand.wordmark"),
		URL:           p.CanonicalURL(),
		InLanguage:    p.Lang,
		Publisher:     ldRef{ID: org.ID},
		SameAs:        []string{sourceRepoURL},
	}
	app := ldWebApplication{
		Type:                "WebApplication",
		ID:                  p.BaseURL + "/#webapp",
		Name:                p.T("seo.title_brand"),
		Description:         p.T("seo.home.description"),
		URL:                 p.CanonicalURL(),
		InLanguage:          p.Lang,
		ApplicationCategory: "UtilitiesApplication",
		OperatingSystem:     "Any",
		IsAccessibleForFree: true,
		License:             mitURL,
		IsPartOf:            ldRef{ID: site.ID},
		IsBasedOn:           ldRef{ID: p.datasetID()},
		Publisher:           ldRef{ID: org.ID},
		Creator:             ldRef{ID: org.ID},
	}
	return encodeJSONLD(site, app, org, person)
}

func (p PageData) datasetJSONLD() (template.JS, error) {
	vars := make([]ldPropertyValue, len(p.Metrics))
	for i := range p.Metrics {
		vars[i] = ldPropertyValue{Type: "PropertyValue", Name: p.MetricLabels[i], UnitText: p.MetricUnits[i]}
	}
	geo, err := json.Marshal(ldGeoShape{Type: "GeoShape", Box: bulgariaBox})
	if err != nil {
		return "", err
	}
	api := p.BaseURL + "/api/v1/"
	org := p.organization()
	ds := ldDataset{
		Type:                "Dataset",
		ID:                  p.datasetID(),
		Name:                p.T("seo.dataset.name"),
		Description:         p.T("seo.dataset.description"),
		URL:                 p.CanonicalURL(),
		InLanguage:          p.Lang,
		License:             odblURL,
		IsAccessibleForFree: true,
		Creator:             ldRef{ID: org.ID},
		Publisher:           ldRef{ID: org.ID},
		TemporalCoverage:    p.temporalCoverage(),
		SpatialCoverage:     ldPlace{Type: "Place", Name: p.T("seo.dataset.place"), Geo: geo},
		VariableMeasured:    vars,
		Distribution: []ldDataDownload{
			{Type: "DataDownload", Name: "/api/v1/areas", EncodingFormat: "application/json", ContentURL: api + "areas"},
			{Type: "DataDownload", Name: "/api/v1/overview", EncodingFormat: "application/json", ContentURL: api + "overview"},
		},
	}
	if !p.GeneratedAt.IsZero() {
		ds.DateModified = p.GeneratedAtISO()
	}
	nodes := []interface{ ldNode() }{ds, org}
	// A page built without the About FAQ carries the Dataset and Organization alone.
	if p.AboutFAQ != nil {
		nodes = append(nodes, faqPageOf(p.AboutFAQ))
	}
	return encodeJSONLD(nodes...)
}

// breadcrumbLD maps the visible trail (p.Crumbs) one-to-one onto a BreadcrumbList with absolute URLs.
func (p PageData) breadcrumbLD() ldBreadcrumbList {
	items := make([]ldListItem, 0, len(p.Crumbs))
	for i, c := range p.Crumbs {
		items = append(items, ldListItem{Type: "ListItem", Position: i + 1, Name: c.Name, Item: p.BaseURL + c.URL})
	}
	return ldBreadcrumbList{Type: "BreadcrumbList", ItemListElement: items}
}

// faqPage maps the rendered FAQ one-to-one onto a FAQPage, so markup and visible text share strings.
func (p PageData) faqPage() ldFAQPage { return faqPageOf(p.AreaFAQ) }

// faqPageOf maps any FAQBlock onto a FAQPage, for the area and About pages alike.
func faqPageOf(block *FAQBlock) ldFAQPage {
	page := ldFAQPage{Type: "FAQPage"}
	for _, it := range block.Items {
		page.MainEntity = append(page.MainEntity, ldQuestion{
			Type: "Question", Name: it.Question,
			AcceptedAnswer: ldAnswer{Type: "Answer", Text: it.Answer},
		})
	}
	return page
}

// areaJSONLD is the breadcrumb trail, the FAQ, and a Place when a centroid exists.
func (rr *Renderer) areaJSONLD(p PageData, row AreaRow) (template.JS, error) {
	crumbs := p.breadcrumbLD()
	faq := p.faqPage()

	if row.Lat == 0 && row.Lon == 0 {
		return encodeJSONLD(crumbs, faq)
	}
	geo, err := json.Marshal(ldGeoCoordinates{Type: "GeoCoordinates", Latitude: row.Lat, Longitude: row.Lon})
	if err != nil {
		return "", err
	}
	place := ldPlace{Type: "Place", Name: rr.areaCrumbName(row, p.Lang), URL: p.CanonicalURL(), Geo: geo}
	if p.AreaParentBlock != nil {
		place.ContainedInPlace = rr.containedIn(p, p.AreaParentBlock.Items)
	}
	return encodeJSONLD(crumbs, place, faq)
}

// containedIn nests the ancestor chain (immediate parent first) into containedInPlace Places.
func (rr *Renderer) containedIn(p PageData, chain []AreaRow) *ldPlace {
	if len(chain) == 0 {
		return nil
	}
	a := chain[0]
	return &ldPlace{Type: "Place", Name: rr.areaCrumbName(a, p.Lang), URL: p.BaseURL + p.Path("/area/"+a.Slug), ContainedInPlace: rr.containedIn(p, chain[1:])}
}

// areaCrumbName labels an oblast as a province so it does not read as its capital city.
func (rr *Renderer) areaCrumbName(a AreaRow, lang string) string {
	if a.Kind == "oblast" {
		return rr.oblastForm(a.Slug, lang, a.Name, "seo.oblast.label")
	}
	return a.Name
}

// mustJSONLD drops the graph on a marshal error; the page still renders without it.
func (rr *Renderer) mustJSONLD(js template.JS, err error) template.JS {
	if err != nil {
		return ""
	}
	return js
}
