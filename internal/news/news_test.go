package news_test

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"kanarche.eu/internal/news"
)

// The shipped drafts must stay valid so promoting one is a status edit.
func TestShippedDraftsParseAndStayInvisible(t *testing.T) {
	set, err := news.Load(os.DirFS("../../content/news/_drafts"), news.Options{})
	if err != nil {
		t.Fatalf("Load drafts: %v", err)
	}
	if got := len(set.Visible(time.Now())); got != 0 {
		t.Errorf("status draft posts must not be visible, got %d", got)
	}
}

const okPost = `---
title: Air in Varna in September 2026
description: Monthly PM summary for Varna.
date: 2026-09-30
type: monthly
areas: [varna]
status: published
---
Intro paragraph.

## Details

Some **bold** text and a [link](https://example.org/).
`

func file(body string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(body)} }

func mustLoad(t *testing.T, fsys fstest.MapFS, opt news.Options) *news.Set {
	t.Helper()
	set, err := news.Load(fsys, opt)
	if err != nil {
		t.Fatalf("Load error = %v, want nil", err)
	}
	return set
}

var now = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

func TestLoadParsesFrontMatterAndRendersMarkdown(t *testing.T) {
	set := mustLoad(t, fstest.MapFS{"2026-09-varna.en.md": file(okPost)}, news.Options{})
	p, ok := set.Lookup(now, "en", "2026-09-varna")
	if !ok {
		t.Fatal("post not found")
	}
	if p.Title != "Air in Varna in September 2026" || p.Type != "monthly" || p.Lang != "en" {
		t.Errorf("post = %+v", p)
	}
	if len(p.Areas) != 1 || p.Areas[0] != "varna" {
		t.Errorf("Areas = %v", p.Areas)
	}
	if got := p.Date.Format("2006-01-02"); got != "2026-09-30" {
		t.Errorf("Date = %s", got)
	}
	html := string(p.HTML)
	for _, want := range []string{"<h2", "<strong>bold</strong>", `<a href="https://example.org/">link</a>`} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing %q: %s", want, html)
		}
	}
}

func TestDraftsDirectoryIsNeverLoaded(t *testing.T) {
	fsys := fstest.MapFS{
		"2026-09-varna.en.md":         file(okPost),
		"_drafts/2026-09-varna.bg.md": file("not even front matter"),
	}
	set := mustLoad(t, fsys, news.Options{})
	if _, ok := set.Lookup(now, "bg", "2026-09-varna"); ok {
		t.Error("post from _drafts is visible")
	}
	if n := len(set.Visible(now)); n != 1 {
		t.Errorf("Visible = %d posts, want 1", n)
	}
}

func TestStatusDraftIsNotVisible(t *testing.T) {
	body := strings.Replace(okPost, "status: published", "status: draft", 1)
	set := mustLoad(t, fstest.MapFS{"2026-09-varna.en.md": file(body)}, news.Options{})
	if n := len(set.Visible(now)); n != 0 {
		t.Errorf("Visible = %d, want 0 for status draft", n)
	}
	if _, ok := set.Lookup(now, "en", "2026-09-varna"); ok {
		t.Error("Lookup returns a status draft")
	}
}

func TestFutureDatedPostIsNotVisibleUntilItsDay(t *testing.T) {
	body := strings.Replace(okPost, "date: 2026-09-30", "date: 2026-11-01", 1)
	set := mustLoad(t, fstest.MapFS{"2026-11-varna.en.md": file(body)}, news.Options{})
	if n := len(set.Visible(now)); n != 0 {
		t.Errorf("Visible before date = %d, want 0", n)
	}
	if _, ok := set.Lookup(now, "en", "2026-11-varna"); ok {
		t.Error("Lookup returns a future post")
	}
	// 00:30 Sofia on the day is already the day, though still the evening before in UTC.
	sofia, err := time.LoadLocation("Europe/Sofia")
	if err != nil {
		t.Skipf("no tzdata: %v", err)
	}
	set = mustLoad(t, fstest.MapFS{"2026-11-varna.en.md": file(body)}, news.Options{Location: sofia})
	if n := len(set.Visible(time.Date(2026, 11, 1, 0, 30, 0, 0, sofia))); n != 1 {
		t.Errorf("Visible on the day = %d, want 1", n)
	}
	if n := len(set.Visible(time.Date(2026, 10, 31, 23, 30, 0, 0, sofia))); n != 0 {
		t.Errorf("Visible the evening before = %d, want 0", n)
	}
}

func TestBadFrontMatterIsRejectedWithTheFileName(t *testing.T) {
	cases := map[string]string{
		"no front matter":     "just text",
		"unterminated":        "---\ntitle: x\n",
		"missing title":       strings.Replace(okPost, "title: Air in Varna in September 2026\n", "", 1),
		"missing description": strings.Replace(okPost, "description: Monthly PM summary for Varna.\n", "", 1),
		"bad date":            strings.Replace(okPost, "date: 2026-09-30", "date: 30.09.2026", 1),
		"bad type":            strings.Replace(okPost, "type: monthly", "type: gossip", 1),
		"bad status":          strings.Replace(okPost, "status: published", "status: maybe", 1),
		"missing status":      strings.Replace(okPost, "status: published\n", "", 1),
		"unknown field":       strings.Replace(okPost, "type: monthly", "type: monthly\nfoo: bar", 1),
		"lang mismatch":       strings.Replace(okPost, "type: monthly", "type: monthly\nlang: bg", 1),
		"slug mismatch":       strings.Replace(okPost, "type: monthly", "type: monthly\nslug: other", 1),
		"updated before date": strings.Replace(okPost, "type: monthly", "type: monthly\nupdated: 2026-09-01", 1),
		"h1 in body":          okPost + "\n# Second title\n",
		"empty body":          strings.Split(okPost, "Intro paragraph.")[0],
		"bad area syntax":     strings.Replace(okPost, "[varna]", "[\"Var na\"]", 1),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := news.Load(fstest.MapFS{"2026-09-varna.en.md": file(body)}, news.Options{})
			if err == nil {
				t.Fatal("Load error = nil, want an error")
			}
			if !strings.Contains(err.Error(), "2026-09-varna.en.md") {
				t.Errorf("error %q does not name the file", err)
			}
		})
	}
}

func TestBadFileNamesAreRejected(t *testing.T) {
	for _, name := range []string{"2026-09-varna.md", "Bad_Slug.en.md", "x.EN.md", "a.b.en.md"} {
		if _, err := news.Load(fstest.MapFS{name: file(okPost)}, news.Options{}); err == nil {
			t.Errorf("Load accepted file name %q", name)
		}
	}
}

func TestNonMarkdownFilesAreIgnored(t *testing.T) {
	set := mustLoad(t, fstest.MapFS{"README.txt": file("notes"), "2026-09-varna.en.md": file(okPost)}, news.Options{})
	if n := len(set.Visible(now)); n != 1 {
		t.Errorf("Visible = %d, want 1", n)
	}
}

func TestUnknownAreaSlugIsRejected(t *testing.T) {
	known := func(slug string) bool { return slug == "sofia" }
	_, err := news.Load(fstest.MapFS{"2026-09-varna.en.md": file(okPost)}, news.Options{KnownArea: known})
	if err == nil || !strings.Contains(err.Error(), "varna") {
		t.Fatalf("Load error = %v, want one naming the unknown area", err)
	}
	known = func(slug string) bool { return slug == "varna" }
	mustLoad(t, fstest.MapFS{"2026-09-varna.en.md": file(okPost)}, news.Options{KnownArea: known})
}

func TestRawHTMLAndScriptURLsAreNeutralised(t *testing.T) {
	body := okPost + "\n<script>alert(1)</script>\n\n<img src=x onerror=alert(2)>\n\n[bad](javascript:alert(3))\n\nInline <b onclick=x>raw</b> html.\n"
	set := mustLoad(t, fstest.MapFS{"2026-09-varna.en.md": file(body)}, news.Options{})
	p, _ := set.Lookup(now, "en", "2026-09-varna")
	html := string(p.HTML)
	for _, bad := range []string{"<script", "<img", "onerror", "onclick", "javascript:", "<b "} {
		if strings.Contains(html, bad) {
			t.Errorf("HTML contains %q: %s", bad, html)
		}
	}
}

func TestVisibleIsNewestFirstAndLangsListsOnlyExistingLanguages(t *testing.T) {
	older := strings.Replace(okPost, "date: 2026-09-30", "date: 2026-08-31", 1)
	fsys := fstest.MapFS{
		"2026-09-varna.en.md": file(okPost),
		"2026-09-varna.bg.md": file(okPost),
		"2026-08-varna.en.md": file(older),
	}
	set := mustLoad(t, fsys, news.Options{})
	en := set.ForLang(now, "en")
	if len(en) != 2 || en[0].Slug != "2026-09-varna" || en[1].Slug != "2026-08-varna" {
		t.Errorf("ForLang(en) order = %v", slugs(en))
	}
	if got := set.Langs(now, "2026-09-varna"); strings.Join(got, ",") != "bg,en" {
		t.Errorf("Langs = %v, want bg,en", got)
	}
	if got := set.Langs(now, "2026-08-varna"); strings.Join(got, ",") != "en" {
		t.Errorf("Langs = %v, want en", got)
	}
	if got := set.Langs(now, "nope"); len(got) != 0 {
		t.Errorf("Langs(unknown) = %v", got)
	}
}

func slugs(ps []news.Post) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Slug
	}
	return out
}

func TestUpdatedDefaultsToDate(t *testing.T) {
	set := mustLoad(t, fstest.MapFS{"2026-09-varna.en.md": file(okPost)}, news.Options{})
	p, _ := set.Lookup(now, "en", "2026-09-varna")
	if !p.Updated.Equal(p.Date) || p.HasUpdate() {
		t.Errorf("Updated = %v, Date = %v, HasUpdate = %v", p.Updated, p.Date, p.HasUpdate())
	}
	body := strings.Replace(okPost, "type: monthly", "type: monthly\nupdated: 2026-10-02", 1)
	set = mustLoad(t, fstest.MapFS{"2026-09-varna.en.md": file(body)}, news.Options{})
	p, _ = set.Lookup(now, "en", "2026-09-varna")
	if !p.HasUpdate() || p.Updated.Format("2006-01-02") != "2026-10-02" {
		t.Errorf("Updated = %v", p.Updated)
	}
}
