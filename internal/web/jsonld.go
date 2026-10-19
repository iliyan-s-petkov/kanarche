package web

import (
	"encoding/json"
	"html/template"
)

// Structured data (OpenProject #607): schema.org JSON-LD emitted by base.gohtml.

const (
	schemaContext = "https://schema.org"
	sourceRepoURL = "https://github.com/iliyan-s-petkov/kanarche"
	odblURL       = "https://opendatacommons.org/licenses/odbl/1-0/"
	// Controller named in the privacy notice. A natural person: the project has no legal entity.
	controllerName  = "Iliyan Petkov"
	controllerEmail = "privacy@kanarche.eu"
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
	Author              ldRef  `json:"author"`
	IsBasedOn           ldRef  `json:"isBasedOn"`
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
	Creator             ldPerson          `json:"creator"`
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
func (ldWebSite) ldNode()        {}
func (ldWebApplication) ldNode() {}
func (ldDataset) ldNode()        {}
func (ldBreadcrumbList) ldNode() {}
func (ldPlace) ldNode()          {}
func (ldFAQPage) ldNode()        {}

// controller is the natural person behind the site; no node claims a legal entity.
func (p PageData) controller() ldPerson {
	return ldPerson{Type: "Person", ID: p.BaseURL + "/#controller", Name: controllerName, Email: controllerEmail}
}

// datasetID is language-independent: the Dataset lives on about-the-data and the home WebApplication references it.
func (p PageData) datasetID() string { return p.BaseURL + "/about-the-data#dataset" }

func (p PageData) homeJSONLD() (template.JS, error) {
	person := p.controller()
	site := ldWebSite{
		Type:          "WebSite",
		ID:            p.BaseURL + "/#website",
		Name:          p.T("seo.title_brand"),
		AlternateName: p.T("brand.wordmark"),
		URL:           p.CanonicalURL(),
		InLanguage:    p.Lang,
		Publisher:     ldRef{ID: person.ID},
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
		License:             odblURL,
		IsPartOf:            ldRef{ID: site.ID},
		Author:              ldRef{ID: person.ID},
		IsBasedOn:           ldRef{ID: p.datasetID()},
	}
	return encodeJSONLD(site, app, person)
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
	ds := ldDataset{
		Type:                "Dataset",
		ID:                  p.datasetID(),
		Name:                p.T("seo.dataset.name"),
		Description:         p.T("seo.dataset.description"),
		URL:                 p.CanonicalURL(),
		InLanguage:          p.Lang,
		License:             odblURL,
		IsAccessibleForFree: true,
		Creator:             p.controller(),
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
	return encodeJSONLD(ds)
}

// areaBreadcrumb maps the visible trail (p.AreaCrumbs, built by Breadcrumbs) one-to-one onto the BreadcrumbList.
func (rr *Renderer) areaBreadcrumb(p PageData) []ldListItem {
	items := make([]ldListItem, 0, len(p.AreaCrumbs))
	for _, c := range p.AreaCrumbs {
		items = append(items, ldListItem{Name: c.Name, Item: p.BaseURL + c.URL})
	}
	for i := range items {
		items[i].Type, items[i].Position = "ListItem", i+1
	}
	return items
}

// faqPage maps the rendered FAQ one-to-one onto a FAQPage, so markup and visible text share strings.
func (p PageData) faqPage() ldFAQPage {
	page := ldFAQPage{Type: "FAQPage"}
	for _, it := range p.AreaFAQ.Items {
		page.MainEntity = append(page.MainEntity, ldQuestion{
			Type: "Question", Name: it.Question,
			AcceptedAnswer: ldAnswer{Type: "Answer", Text: it.Answer},
		})
	}
	return page
}

// areaJSONLD is the breadcrumb trail, the FAQ, and a Place when a centroid exists.
func (rr *Renderer) areaJSONLD(p PageData, row AreaRow) (template.JS, error) {
	crumbs := ldBreadcrumbList{Type: "BreadcrumbList", ItemListElement: rr.areaBreadcrumb(p)}
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
