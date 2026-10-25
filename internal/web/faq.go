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

// FAQBlock is a page's FAQ: a heading and its items.
type FAQBlock struct {
	Heading string
	Items   []FAQItem
}

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

// eaqiEdgePairs is the {p2_*} and {p1_*} placeholder pairs, shared by the About FAQ and the PM explainer pages.
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

// faqBlock is the area FAQ. Each answer is a facts block, so the FAQ never says more than the text above it.
// The rank question is left out when the rank block is.
func (p PageData) faqBlock(f *AreaFacts) *FAQBlock {
	block := &FAQBlock{Heading: p.factText("faq.heading")}
	add := func(qKey, answer string) {
		if answer != "" {
			block.Items = append(block.Items, newFAQItem(p.factText(qKey, "{kind_plain}", f.kindPlain), answer))
		}
	}
	if f.rankFAQ != "" {
		add("faq.q1", f.rankFAQ+" "+p.T("area.facts.rank.citizen"))
	}
	add("faq.q2", f.Network)
	add("faq.q3", f.Place)
	return block
}

// aboutFAQCount is the number of questions on the About the data page (about_faq.q1 to q17).
const aboutFAQCount = 17

// aboutFAQRZIAnswers are the About answers that end by pointing at the three RHI bathing pages.
var aboutFAQRZIAnswers = map[int]bool{10: true, 14: true}

// aboutFAQKeys maps About question n to its catalogue keys.
func aboutFAQKeys(n int) (q, a string) {
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
