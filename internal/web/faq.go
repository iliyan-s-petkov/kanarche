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

// FAQBlock is the area page's FAQ: a heading and seven items.
type FAQBlock struct {
	Heading string
	Items   []FAQItem
}

// faqCount is the number of questions the catalogue holds (faq.q1 to faq.q7).
const faqCount = 7

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
	p2, p1 := pm25Edges(), pm10Edges()
	return strings.NewReplacer(
		"{area}", p.faqArea(row),
		"{area_in}", p.areaIn(),
		"{minutes}", strconv.Itoa(p.pollMinutes),
		"{p2_good}", p2[0], "{p2_fair}", p2[1], "{p2_moderate}", p2[2], "{p2_poor}", p2[3], "{p2_verypoor}", p2[4],
		"{p1_good}", p1[0], "{p1_fair}", p1[1], "{p1_moderate}", p1[2], "{p1_poor}", p1[3], "{p1_verypoor}", p1[4],
	)
}

// faqBlock renders the area FAQ. faq.a5_official joins answer 5 only when the area has EEA data, the same rule as the area description.
func (p PageData) faqBlock(row AreaRow) *FAQBlock {
	fill := p.faqReplacer(row)
	aboutLabel := p.T("about.title")
	block := &FAQBlock{Heading: fill.Replace(p.T("faq.heading"))}
	for n := 1; n <= faqCount; n++ {
		num := strconv.Itoa(n)
		answer := p.T("faq.a" + num)
		if n == 5 && row.hasEEA() {
			answer += " " + p.T("faq.a5_official")
		}
		answer = fill.Replace(answer)
		item := FAQItem{Question: fill.Replace(p.T("faq.q" + num)), Answer: answer}
		item.AnswerHTML = template.HTML(html.EscapeString(answer))
		// Answer 6 names the About page in quotes; the name becomes the link.
		if n == 6 {
			if i := strings.Index(answer, aboutLabel); i >= 0 {
				link := `<a class="link" href="` + html.EscapeString(p.Path("/about-the-data")) + `">` + html.EscapeString(aboutLabel) + `</a>`
				item.AnswerHTML = template.HTML(html.EscapeString(answer[:i]) + link + html.EscapeString(answer[i+len(aboutLabel):]))
			}
		}
		block.Items = append(block.Items, item)
	}
	return block
}
