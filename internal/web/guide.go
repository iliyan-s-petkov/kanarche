package web

import (
	"html"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// guideImage is the pixel size of a file under static/build-a-sensor. The size goes on the <img> so the
// page does not shift while the image loads. guide_images_test.go checks it against the WebP header.
type guideImage struct{ W, H int }

var guideImages = map[string]guideImage{
	"parts":                   {900, 506},
	"solder-parts":            {900, 506},
	"solder-pins":             {900, 506},
	"wiring-diagram":          {900, 1118},
	"wiring-diagram-large":    {1600, 1988},
	"wiring-photo":            {900, 505},
	"tie-board":               {900, 600},
	"tube-on-sensor":          {900, 675},
	"bme-on-tube":             {900, 675},
	"sensor-in-bend":          {900, 675},
	"bme-position":            {900, 675},
	"flasher-select-firmware": {493, 461},
	"flasher-finished":        {493, 460},
	"usb-board":               {150, 200},
	"station-home":            {900, 721},
	"station-values":          {900, 700},
	"station-wifi":            {900, 813},
	"station-more":            {900, 1069},
	"station-sensors":         {900, 932},
	"station-apis":            {900, 1019},
}

// extLink is a {placeholder} that renders as an anchor to another site.
type extLink struct{ Placeholder, Label, Href string }

const (
	adoptURL      = "https://airbg.info/zaqwi-uchastie/"
	guideURLBG    = "https://sensor.community/bg/sensors/airrohr/"
	guideURLEN    = "https://sensor.community/en/sensors/airrohr/"
	guideLicence  = "https://github.com/opendata-stuttgart/sensor.community/blob/develop/LICENSE"
	guideImageDir = "build-a-sensor/"
)

var literalRE = regexp.MustCompile("`([^`]+)`")

// GuideText escapes the copy under key, fills {minutes}, swaps each external-link placeholder for an
// anchor and turns `literals` into <code>. The escaping runs first, so the markup comes only from here.
func (p PageData) GuideText(key string, links ...extLink) template.HTML {
	out := html.EscapeString(strings.NewReplacer("{minutes}", strconv.Itoa(p.pollMinutes)).Replace(p.T(key)))
	out = literalRE.ReplaceAllString(out, "<code>$1</code>")
	for _, l := range links {
		anchor := `<a class="link" href="` + html.EscapeString(l.Href) + `" rel="noopener noreferrer">` + html.EscapeString(l.Label) + `</a>`
		out = strings.ReplaceAll(out, l.Placeholder, anchor)
	}
	return template.HTML(out)
}

// BuildIntro is the opening paragraph with the link to the adoptable stations.
func (p PageData) BuildIntro() template.HTML {
	return p.GuideText("build.intro", extLink{"{adopt_link}", p.T("build.adopt_label"), adoptURL})
}

// BuildCredit is the small-type credit line with the original guide and licence links.
func (p PageData) BuildCredit() template.HTML {
	guide := guideURLEN
	if p.Lang == "bg" {
		guide = guideURLBG
	}
	return p.GuideText("build.credit",
		extLink{"{guide_link}", p.T("build.credit.guide_label"), guide},
		extLink{"{licence_link}", p.T("build.credit.licence_label"), guideLicence})
}

// GuideKeys lists the catalogue keys prefix1 to prefixN, in order.
func (p PageData) GuideKeys(prefix string, n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = prefix + strconv.Itoa(i+1)
	}
	return keys
}

// GuideCells splits a table row stored as "a|b|c" in one catalogue key.
func (p PageData) GuideCells(key string) []string { return strings.Split(p.T(key), "|") }

// GuideTable renders a pin table: caption, header row and rows 1 to n stored under key.r1..rn.
func (p PageData) GuideTable(key string, n int) template.HTML {
	var b strings.Builder
	b.WriteString(`<table class="table guide-table"><caption>` + html.EscapeString(p.T(key+".caption")) + `</caption><thead><tr>`)
	for _, c := range p.GuideCells(key + ".head") {
		b.WriteString(`<th scope="col">` + string(p.GuideCell(c)) + `</th>`)
	}
	b.WriteString(`</tr></thead><tbody>`)
	for _, rk := range p.GuideKeys(key+".r", n) {
		b.WriteString(`<tr>`)
		for j, c := range p.GuideCells(rk) {
			tag := "td"
			if j == 0 {
				tag = "th scope=\"row\""
			}
			b.WriteString("<" + tag + ">" + string(p.GuideCell(c)) + "</" + strings.Fields(tag)[0] + ">")
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return template.HTML(b.String())
}

// GuideCell is one cell of a table row as inline copy.
func (p PageData) GuideCell(cell string) template.HTML {
	return template.HTML(literalRE.ReplaceAllString(html.EscapeString(cell), "<code>$1</code>"))
}

// GuidePartText is a parts-list line split after its first sentence: the part name, then the detail.
type GuidePartText struct{ Name, Detail string }

// GuidePart splits the parts line under key.
func (p PageData) GuidePart(key string) GuidePartText {
	text := p.T(key)
	if i := strings.Index(text, ". "); i >= 0 {
		return GuidePartText{Name: text[:i+1], Detail: text[i+2:]}
	}
	return GuidePartText{Name: text}
}

// GuideImageURL is the static URL of a guide image.
func (p PageData) GuideImageURL(name string) string {
	return p.Static(guideImageDir + name + ".webp")
}

// GuideFig renders a captioned figure. The alt text is the caption, so the caption is hidden from
// screen readers rather than read twice.
func (p PageData) GuideFig(name, altKey string) template.HTML {
	return p.guideFig(name, altKey, "")
}

// GuideFigZoom is GuideFig with the picture linked to the larger file.
func (p PageData) GuideFigZoom(name, large, altKey string) template.HTML {
	return p.guideFig(name, altKey, p.GuideImageURL(large))
}

func (p PageData) guideFig(name, altKey, zoomURL string) template.HTML {
	img, ok := guideImages[name]
	if !ok {
		return ""
	}
	alt := html.EscapeString(p.T(altKey))
	tag := `<img src="` + html.EscapeString(p.GuideImageURL(name)) + `" alt="` + alt + `" width="` + strconv.Itoa(img.W) +
		`" height="` + strconv.Itoa(img.H) + `" loading="lazy" decoding="async">`
	if zoomURL != "" {
		tag = `<a class="guide-fig__zoom" href="` + html.EscapeString(zoomURL) + `">` + tag + `</a>`
	}
	return template.HTML(`<figure class="guide-fig">` + tag + `<figcaption aria-hidden="true">` + alt + `</figcaption></figure>`)
}

// Coastal reports whether the area page is a Black Sea coast city or province, which links the bathing page.
func (p PageData) Coastal() bool {
	if p.Area == nil {
		return false
	}
	return coastalAreaSlugs[p.Area.Slug]
}

// coastalAreaSlugs are the areas served by the three regional health inspectorates that publish bathing samples.
var coastalAreaSlugs = map[string]bool{
	"varna": true, "varna-oblast": true,
	"burgas": true, "burgas-oblast": true,
	"dobrich": true, "dobrich-oblast": true,
}

// SeaMapPath is the home map opened with the bathing water layer on.
func (p PageData) SeaMapPath() string { return p.Path("/") + "#layers=sea" }

// buildFAQBlock is the sensor page FAQ.
func (p PageData) buildFAQBlock() *FAQBlock {
	block := &FAQBlock{Heading: p.T("build.faq.heading")}
	fill := strings.NewReplacer("{minutes}", strconv.Itoa(p.pollMinutes))
	for n := 1; n <= buildFAQCount; n++ {
		num := strconv.Itoa(n)
		block.Items = append(block.Items, newFAQItem(fill.Replace(p.T("build.faq.q"+num)), fill.Replace(p.T("build.faq.a"+num))))
	}
	return block
}

// buildFAQCount is the number of build.faq.q/a pairs in the catalogue.
const buildFAQCount = 4

// bathingFAQNumbers are the about_faq questions that moved to the bathing page. The catalogue keys keep their numbers.
var bathingFAQNumbers = []int{10, 11, 12, 13, 14}

func isBathingFAQ(n int) bool { return n >= 10 && n <= 14 }

// bathingFAQBlock is the bathing page FAQ: the five questions that used to sit on /about-the-data, with the same RHI links and references.
func (p PageData) bathingFAQBlock() *FAQBlock {
	block := &FAQBlock{Heading: p.T("build.faq.heading")}
	fill := p.aboutFAQFill()
	for _, n := range bathingFAQNumbers {
		block.Items = append(block.Items, p.aboutFAQItem(n, fill))
	}
	return block
}

func (rr *Renderer) guidePage(w http.ResponseWriter, r *http.Request, name string) {
	var generatedAt time.Time
	if snap := rr.holder.Load(); snap != nil {
		generatedAt = snap.GeneratedAt
	}
	lang, path := rr.cat.LangFromPath(r.URL.Path)
	data := rr.newPageData(lang, path, generatedAt)
	data.Title = composeTitle(rr.cat.T(lang, "seo."+name+".title"), rr.cat.T(lang, "seo.title_brand"))
	data.Description = rr.cat.T(lang, "seo."+name+".description")
	route := "/build-a-sensor"
	if name == "bathing" {
		route = "/bathing-water"
		data.BathingFAQ = data.bathingFAQBlock()
	} else {
		data.BuildFAQ = data.buildFAQBlock()
	}
	data.Crumbs = []Crumb{
		{Name: data.T("area.breadcrumb.home"), URL: data.Path("/")},
		{Name: rr.cat.T(lang, "seo."+name+".title"), URL: data.Path(route), Current: true},
	}
	faq := data.BuildFAQ
	if faq == nil {
		faq = data.BathingFAQ
	}
	data.JSONLD = rr.mustJSONLD(encodeJSONLD(data.breadcrumbLD(), faqPageOf(faq), data.organization(), data.controller()))
	templateName := "build"
	if name == "bathing" {
		templateName = "bathing"
	}
	rr.render(w, r, http.StatusOK, templateName, data)
}

// handleBuildASensor serves the build-a-sensor guide.
func (rr *Renderer) handleBuildASensor(w http.ResponseWriter, r *http.Request) {
	rr.guidePage(w, r, "build")
}

// handleBathingWater serves the bathing water explainer.
func (rr *Renderer) handleBathingWater(w http.ResponseWriter, r *http.Request) {
	rr.guidePage(w, r, "bathing")
}
