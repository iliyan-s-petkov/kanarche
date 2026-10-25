// Package news loads the Markdown posts of the /news section.
//
// Posts are files named <slug>.<lang>.md with YAML front matter. A post is
// public only when it sits outside a _drafts directory, has status published
// and its date is not in the future. The same slug in several languages is one
// post that exists in each of those languages.
package news

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

// Post types and statuses accepted in front matter.
const (
	StatusPublished = "published"
	StatusDraft     = "draft"
)

var postTypes = map[string]bool{"monthly": true, "episode": true, "pollen": true, "update": true}

var (
	slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	langRe = regexp.MustCompile(`^[a-z]{2,3}$`)
)

// Options configures Load.
type Options struct {
	// Location is the zone post dates are calendar days in. Nil means UTC.
	Location *time.Location
	// KnownArea reports whether an area slug exists. Nil skips the check and
	// only the slug syntax is validated.
	KnownArea func(slug string) bool
}

// Post is one language version of one post.
type Post struct {
	Slug        string
	Lang        string
	Title       string
	Description string
	Type        string
	Status      string
	Areas       []string
	// Date and Updated are midnight of the calendar day in Options.Location.
	// Updated equals Date when the front matter has no updated field.
	Date    time.Time
	Updated time.Time
	// HTML is the rendered body. Raw HTML in the source is dropped by the
	// renderer, so it is safe to emit unescaped.
	HTML template.HTML
}

// HasUpdate reports whether the post was edited after its publication day.
func (p Post) HasUpdate() bool { return p.Updated.After(p.Date) }

// Set is every post that loaded, in all states.
type Set struct {
	posts []Post
}

type frontMatter struct {
	Slug        string   `yaml:"slug"`
	Lang        string   `yaml:"lang"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Date        string   `yaml:"date"`
	Updated     string   `yaml:"updated"`
	Type        string   `yaml:"type"`
	Areas       []string `yaml:"areas"`
	Status      string   `yaml:"status"`
}

var markdown = goldmark.New(
	goldmark.WithExtensions(extension.Table),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	// No html.WithUnsafe: raw HTML and javascript: links are dropped.
)

// Load reads every post under fsys. Directories whose name starts with "_" or
// "." are skipped, so _drafts never loads. All problems are reported together,
// each naming its file.
func Load(fsys fs.FS, opt Options) (*Set, error) {
	loc := opt.Location
	if loc == nil {
		loc = time.UTC
	}
	var posts []Post
	var errs []error
	seen := map[string]bool{}

	walkErr := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != "." && (strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".")) {
				return fs.SkipDir
			}
			return nil
		}
		if path.Ext(name) != ".md" {
			return nil
		}
		raw, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		post, err := parsePost(name, raw, loc, opt.KnownArea)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p, err))
			return nil
		}
		key := post.Lang + "/" + post.Slug
		if seen[key] {
			errs = append(errs, fmt.Errorf("%s: duplicate post %s in language %s", p, post.Slug, post.Lang))
			return nil
		}
		seen[key] = true
		posts = append(posts, post)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	// Newest first, ties broken by slug then language so output is stable.
	sort.Slice(posts, func(i, j int) bool {
		a, b := posts[i], posts[j]
		if !a.Date.Equal(b.Date) {
			return a.Date.After(b.Date)
		}
		if a.Slug != b.Slug {
			return a.Slug < b.Slug
		}
		return a.Lang < b.Lang
	})
	return &Set{posts: posts}, nil
}

func parsePost(filename string, raw []byte, loc *time.Location, known func(string) bool) (Post, error) {
	stem := strings.TrimSuffix(filename, ".md")
	dot := strings.LastIndex(stem, ".")
	if dot < 0 {
		return Post{}, errors.New("file name must be <slug>.<lang>.md")
	}
	slug, lang := stem[:dot], stem[dot+1:]
	if !slugRe.MatchString(slug) {
		return Post{}, fmt.Errorf("slug %q must be lowercase letters, digits and hyphens", slug)
	}
	if !langRe.MatchString(lang) {
		return Post{}, fmt.Errorf("language %q in the file name must be 2 or 3 lowercase letters", lang)
	}

	fm, body, err := splitFrontMatter(raw)
	if err != nil {
		return Post{}, err
	}
	if fm.Slug != "" && fm.Slug != slug {
		return Post{}, fmt.Errorf("front matter slug %q does not match the file name", fm.Slug)
	}
	if fm.Lang != "" && fm.Lang != lang {
		return Post{}, fmt.Errorf("front matter lang %q does not match the file name", fm.Lang)
	}
	if strings.TrimSpace(fm.Title) == "" {
		return Post{}, errors.New("title is required")
	}
	if strings.TrimSpace(fm.Description) == "" {
		return Post{}, errors.New("description is required")
	}
	if !postTypes[fm.Type] {
		return Post{}, fmt.Errorf("type %q must be one of monthly, episode, pollen, update", fm.Type)
	}
	if fm.Status != StatusPublished && fm.Status != StatusDraft {
		return Post{}, fmt.Errorf("status %q must be published or draft", fm.Status)
	}
	date, err := time.ParseInLocation("2006-01-02", fm.Date, loc)
	if err != nil {
		return Post{}, fmt.Errorf("date %q must be YYYY-MM-DD", fm.Date)
	}
	updated := date
	if fm.Updated != "" {
		updated, err = time.ParseInLocation("2006-01-02", fm.Updated, loc)
		if err != nil {
			return Post{}, fmt.Errorf("updated %q must be YYYY-MM-DD", fm.Updated)
		}
		if updated.Before(date) {
			return Post{}, errors.New("updated is before date")
		}
	}
	for _, a := range fm.Areas {
		if !slugRe.MatchString(a) {
			return Post{}, fmt.Errorf("area %q is not a valid slug", a)
		}
		if known != nil && !known(a) {
			return Post{}, fmt.Errorf("unknown area %q", a)
		}
	}

	html, err := renderBody(body)
	if err != nil {
		return Post{}, err
	}
	return Post{
		Slug: slug, Lang: lang, Title: strings.TrimSpace(fm.Title), Description: strings.TrimSpace(fm.Description),
		Type: fm.Type, Status: fm.Status, Areas: fm.Areas, Date: date, Updated: updated, HTML: html,
	}, nil
}

// splitFrontMatter cuts the leading --- block off and decodes it strictly, so a
// misspelt field is an error instead of a silently ignored one.
func splitFrontMatter(raw []byte) (frontMatter, []byte, error) {
	src := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	const fence = "---\n"
	if !bytes.HasPrefix(src, []byte(fence)) {
		return frontMatter{}, nil, errors.New("missing front matter, the file must start with ---")
	}
	rest := src[len(fence):]
	end := bytes.Index(rest, []byte("\n"+fence))
	if end < 0 {
		return frontMatter{}, nil, errors.New("front matter is not closed with ---")
	}
	dec := yaml.NewDecoder(bytes.NewReader(rest[:end+1]))
	dec.KnownFields(true)
	var fm frontMatter
	if err := dec.Decode(&fm); err != nil {
		return frontMatter{}, nil, fmt.Errorf("front matter: %w", err)
	}
	return fm, rest[end+1+len(fence):], nil
}

// renderBody renders Markdown and rejects a level 1 heading, because the page
// template already prints the title as the only h1.
func renderBody(body []byte) (template.HTML, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return "", errors.New("body is empty")
	}
	doc := markdown.Parser().Parse(text.NewReader(body))
	var h1 bool
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if h, ok := n.(*ast.Heading); ok && entering && h.Level == 1 {
			h1 = true
		}
		return ast.WalkContinue, nil
	})
	if h1 {
		return "", errors.New("body has a level 1 heading, start at level 2")
	}
	var buf bytes.Buffer
	if err := markdown.Renderer().Render(&buf, body, doc); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

// Visible returns the posts public at now, newest first, all languages.
func (s *Set) Visible(now time.Time) []Post {
	var out []Post
	for _, p := range s.posts {
		if p.visible(now) {
			out = append(out, p.at(now))
		}
	}
	return out
}

// at caps Updated at now, so an updated date in the future never reaches a page, feed or sitemap.
func (p Post) at(now time.Time) Post {
	if p.Updated.After(now) {
		p.Updated = now.In(p.Updated.Location())
	}
	return p
}

func (p Post) visible(now time.Time) bool {
	return p.Status == StatusPublished && !p.Date.After(now)
}

// ForLang returns the public posts in one language, newest first.
func (s *Set) ForLang(now time.Time, lang string) []Post {
	var out []Post
	for _, p := range s.Visible(now) {
		if p.Lang == lang {
			out = append(out, p)
		}
	}
	return out
}

// Lookup finds one public post.
func (s *Set) Lookup(now time.Time, lang, slug string) (Post, bool) {
	for _, p := range s.posts {
		if p.Lang == lang && p.Slug == slug && p.visible(now) {
			return p.at(now), true
		}
	}
	return Post{}, false
}

// Langs lists, sorted, the languages in which the slug is public.
func (s *Set) Langs(now time.Time, slug string) []string {
	var out []string
	for _, p := range s.posts {
		if p.Slug == slug && p.visible(now) {
			out = append(out, p.Lang)
		}
	}
	sort.Strings(out)
	return out
}
