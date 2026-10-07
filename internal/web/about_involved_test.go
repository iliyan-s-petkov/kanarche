package web

import (
	"html/template"
	"strings"
	"testing"
)

// involvedParts splits the translated sentence on {repo} and {issues}. Text
// stays a plain string, so html/template escapes it when the page renders.
func TestInvolvedPartsSplitOnPlaceholders(t *testing.T) {
	links := map[string]involvedLink{
		"repo":   {Href: "https://example.org/r", Label: "star <it>"},
		"issues": {Href: "https://example.org/r/issues", Label: "issues"},
	}
	parts := involvedParts(`Open <b>& free. A {repo} helps. Send {issues}.`, links)

	var text string
	var hrefs []string
	for _, p := range parts {
		var s string = p.Text // must stay a string, never template.HTML
		text += s
		if p.Href != "" {
			hrefs = append(hrefs, p.Href)
		}
	}
	if len(hrefs) != 2 || hrefs[0] != "https://example.org/r" || hrefs[1] != "https://example.org/r/issues" {
		t.Fatalf("hrefs = %v", hrefs)
	}
	if strings.Contains(text, "{repo}") || strings.Contains(text, "{issues}") {
		t.Errorf("placeholders left in text: %q", text)
	}

	// Rendered through the same shape the page template uses, hostile text
	// and labels come out escaped and the only live tags are the two anchors.
	tpl := template.Must(template.New("p").Parse(
		`{{range .}}{{if .Href}}<a href="{{.Href}}">{{.Text}}</a>{{else}}{{.Text}}{{end}}{{end}}`))
	var sb strings.Builder
	if err := tpl.Execute(&sb, parts); err != nil {
		t.Fatal(err)
	}
	out := sb.String()
	for _, want := range []string{"Open &lt;b&gt;&amp; free.", "star &lt;it&gt;"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks escaped %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "<a ") != 2 || strings.Contains(out, "<b>") || strings.Contains(out, "<it>") {
		t.Errorf("unexpected live markup:\n%s", out)
	}
}

func TestInvolvedPartsKeepsUnknownPlaceholdersAsText(t *testing.T) {
	parts := involvedParts("a {nope} b", map[string]involvedLink{})
	var text string
	for _, p := range parts {
		text += p.Text
	}
	if text != "a {nope} b" {
		t.Errorf("text = %q", text)
	}
}
