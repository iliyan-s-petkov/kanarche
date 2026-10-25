package web

import (
	"html"
	"html/template"
	"strconv"
	"strings"
)

// FAQItem is one question and answer. Answer is the plain text that FAQPage JSON-LD carries, AnswerHTML its escaped form with the About link.
type FAQItem struct {
	Question   string
	Answer     string
	AnswerHTML template.HTML
}

// FAQBlock is the area page's FAQ: a heading and eleven items.
type FAQBlock struct {
	Heading string
	Items   []FAQItem
}

// faqCount is the number of questions the catalogue holds (faq.q1 to faq.q11).
const faqCount = 11

// areaIn is the BG preposition and name ("в Смолян", "във Враца", "в район Младост", "в област Пловдив"), cut out of the h1 so the grammar rule stays in one place.
func (p PageData) areaIn() string {
	h := p.AreaHeading()
	for _, sep := range []string{" във ", " в "} {
		if i := strings.Index(h, sep); i >= 0 {
			return h[i+1:]
		}
	}
	return h
}

// faqArea is {area} in the middle of a sentence: an oblast takes its inline form ("област Смолян"), not the capitalised crumb label.
func (p PageData) faqArea(row AreaRow) string {
	if row.Kind == "oblast" {
		return p.oblastFormKey(row, "seo.oblast.inline")
	}
	return row.Name
}

// faqReplacer fills every FAQ placeholder: the area, the h1's preposition form, the poll interval and the EAQI edges from api.Scales().
func (p PageData) faqReplacer(row AreaRow) *strings.Replacer {
	pairs := []string{
		"{area}", p.faqArea(row),
		"{area_in}", p.areaIn(),
		"{minutes}", strconv.Itoa(p.pollMinutes),
		"{air_pollution_link}", p.T("faq.air_pollution_link"),
	}
	return strings.NewReplacer(append(pairs, eaqiEdgePairs()...)...)
}

// eaqiEdgePairs is the {p2_*} and {p1_*} placeholder pairs, shared by the FAQ and the PM explainer pages.
func eaqiEdgePairs() []string {
	p2, p1 := pm25Edges(), pm10Edges()
	return []string{
		"{p2_good}", p2[0], "{p2_fair}", p2[1], "{p2_moderate}", p2[2], "{p2_poor}", p2[3], "{p2_verypoor}", p2[4],
		"{p1_good}", p1[0], "{p1_fair}", p1[1], "{p1_moderate}", p1[2], "{p1_poor}", p1[3], "{p1_verypoor}", p1[4],
	}
}

// faqLink names a phrase inside an answer that becomes an anchor in the visible HTML. JSON-LD keeps the plain text.
type faqLink struct{ Label, Href string }

// newFAQItem escapes the answer and turns the first occurrence of each link label into an anchor.
func newFAQItem(question, answer string, links ...faqLink) FAQItem {
	out := html.EscapeString(answer)
	for _, l := range links {
		label := html.EscapeString(l.Label)
		if label == "" || !strings.Contains(out, label) {
			continue
		}
		anchor := `<a class="link" href="` + html.EscapeString(l.Href) + `">` + label + `</a>`
		out = strings.Replace(out, label, anchor, 1)
	}
	return FAQItem{Question: question, Answer: answer, AnswerHTML: template.HTML(out)}
}

// faqBlock renders the area FAQ. faq.a5_official joins answer 5 only when the area has EEA data, the same rule as the area description.
func (p PageData) faqBlock(row AreaRow) *FAQBlock {
	fill := p.faqReplacer(row)
	aboutLink := faqLink{Label: p.T("about.title"), Href: p.Path("/about-the-data")}
	block := &FAQBlock{Heading: fill.Replace(p.T("faq.heading"))}
	for n := 1; n <= faqCount; n++ {
		num := strconv.Itoa(n)
		answer := p.T("faq.a" + num)
		if n == 5 && row.hasEEA() {
			answer += " " + p.T("faq.a5_official")
		}
		var links []faqLink
		// Answer 6 names the About page in quotes; the name becomes the link.
		if n == 6 {
			links = append(links, aboutLink)
		}
		// Answer 8 ends by pointing at the explainer; the filled placeholder text becomes the link.
		if n == 8 {
			links = append(links, faqLink{Label: p.T("faq.air_pollution_link"), Href: p.Path("/air-pollution")})
		}
		block.Items = append(block.Items, newFAQItem(fill.Replace(p.T("faq.q"+num)), fill.Replace(answer), links...))
	}
	return block
}

// aboutFAQCount is the number of questions on the About the data page (about_faq.q1 to q17).
const aboutFAQCount = 17

// aboutFAQRZIAnswers are the About answers that end by pointing at the three RHI bathing pages.
var aboutFAQRZIAnswers = map[int]bool{10: true, 14: true}

// aboutFAQKeys maps About question n to its catalogue keys. Questions 3 and 4 reuse the area FAQ's PM and colour entries.
func aboutFAQKeys(n int) (q, a string) {
	switch n {
	case 3:
		return "faq.q2", "faq.a2"
	case 4:
		return "faq.q3", "faq.a3"
	}
	num := strconv.Itoa(n)
	return "about_faq.q" + num, "about_faq.a" + num
}

// aboutFAQBlock renders the About page FAQ. No area is involved, so only the poll interval and EAQI edges are filled.
func (p PageData) aboutFAQBlock() *FAQBlock {
	pairs := append([]string{"{minutes}", strconv.Itoa(p.pollMinutes)}, eaqiEdgePairs()...)
	fill := strings.NewReplacer(pairs...)
	links := []faqLink{
		{Label: p.T("about.sources.heading"), Href: "#sources"},
		{Label: p.T("licences.title"), Href: p.Path("/licences")},
	}
	block := &FAQBlock{Heading: p.T("about_faq.heading")}
	for n := 1; n <= aboutFAQCount; n++ {
		qKey, aKey := aboutFAQKeys(n)
		item := newFAQItem(fill.Replace(p.T(qKey)), fill.Replace(p.T(aKey)), links...)
		if aboutFAQRZIAnswers[n] {
			item = p.withRZILinks(item)
		}
		block.Items = append(block.Items, item)
	}
	return block
}

// withRZILinks appends the RHI pages after an answer that ends in a colon. The page gets labelled anchors
// separated like /licences; the JSON-LD text stays plain, so it gets the bare URLs instead.
func (p PageData) withRZILinks(item FAQItem) FAQItem {
	var anchors, urls []string
	for _, page := range p.RZIPages() {
		if page.URL == "" {
			continue
		}
		anchors = append(anchors, `<a href="`+html.EscapeString(page.URL)+`" rel="noopener noreferrer">`+html.EscapeString(p.T(page.LabelKey))+`</a>`)
		urls = append(urls, page.URL)
	}
	if len(urls) == 0 {
		return item
	}
	item.Answer += " " + strings.Join(urls, " ")
	item.AnswerHTML += template.HTML(" " + strings.Join(anchors, " · "))
	return item
}
