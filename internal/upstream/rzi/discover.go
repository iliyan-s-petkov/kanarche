package rzi

import (
	"errors"
	"html"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

var (
	ErrNoMatch   = errors.New("rzi: no document link matches the source pattern")
	ErrAmbiguous = errors.New("rzi: more than one document link matches the source pattern")
	ErrHost      = errors.New("rzi: host or scheme not on the source allow-list")
)

// Kind is the document type a source serves.
type Kind struct {
	Ext          string
	Magic        []byte
	ContentTypes []string
}

var (
	kindPDF  = Kind{Ext: "pdf", Magic: []byte("%PDF-"), ContentTypes: []string{"application/pdf"}}
	kindXLS  = Kind{Ext: "xls", Magic: []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, ContentTypes: []string{"application/vnd.ms-excel"}}
	kindXLSX = Kind{Ext: "xlsx", Magic: []byte("PK\x03\x04"), ContentTypes: []string{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"}}
)

// Spec is the fixed, in-code description of one source. Hosts is the exact
// allow-list for every request and redirect of that source.
type Spec struct {
	ID    string
	Hosts []string
	Kind  Kind
	// pattern matches the URL path of the current-year document link.
	pattern func(year string) *regexp.Regexp
	// rewrite maps a link as published to the URL that serves the file.
	rewrite func(*url.URL)
}

var specs = []Spec{
	{
		ID:    SourceVarna,
		Hosts: []string{"rzi-varna.com", "www.rzi-varna.com"},
		Kind:  kindPDF,
		// The schedule PDF is grafik_mvodi-<year>.pdf: the name must start the segment.
		pattern: func(y string) *regexp.Regexp { return regexp.MustCompile(`(^|/)mvodi-` + y + `\.pdf$`) },
	},
	{
		ID:    SourceBurgas,
		Hosts: []string{"rzi-burgas.bg"},
		Kind:  kindXLS,
		// The suffix after the year varies by month: -20249.xls, -2023-1.xls.
		pattern: func(y string) *regexp.Regexp {
			return regexp.MustCompile(`(?i)(^|/)tablici-monitoring-more-mz-` + y + `[^/]*\.xls$`)
		},
	},
	{
		ID:    SourceDobrich,
		Hosts: []string{"rzi-dobrich.egov.bg"},
		Kind:  kindXLSX,
		// Path keeps '+' literally.
		pattern: func(y string) *regexp.Regexp {
			return regexp.MustCompile(`(^|/)[^/]*morski\+vodi\+` + y + `[^/]*\.xlsx$`)
		},
		// The page links /wcm/myconnect/, which redirects to an internal login host.
		// /wcm/connect/ serves the file.
		rewrite: func(u *url.URL) {
			u.Path = strings.Replace(u.Path, "/wcm/myconnect/", "/wcm/connect/", 1)
			u.RawPath = ""
			u.RawQuery = "MOD=AJPERES"
		},
	},
}

func specFor(id string) (Spec, bool) {
	for _, s := range specs {
		if s.ID == id {
			return s, true
		}
	}
	return Spec{}, false
}

func (s Spec) allows(u *url.URL) bool {
	return u.Scheme == "https" && slices.Contains(s.Hosts, u.Host)
}

var hrefPattern = regexp.MustCompile(`(?i)\bhref\s*=\s*(?:"([^"]*)"|'([^']*)')`)

// discover finds the one current-year document link in a section page.
// Links are only read, never followed; the result is on the source's allow-list.
// Errors are fixed strings and carry nothing from the page.
func discover(page []byte, base *url.URL, spec Spec, year string) (*url.URL, error) {
	re := spec.pattern(year)
	var found []*url.URL
	for _, m := range hrefPattern.FindAllSubmatch(page, -1) {
		raw := string(m[1])
		if raw == "" {
			raw = string(m[2])
		}
		u, err := base.Parse(html.UnescapeString(raw))
		if err != nil || !re.MatchString(u.Path) {
			continue
		}
		u.Fragment = ""
		if spec.rewrite != nil {
			spec.rewrite(u)
		}
		if !spec.allows(u) {
			return nil, ErrHost
		}
		if !slices.ContainsFunc(found, func(o *url.URL) bool { return o.String() == u.String() }) {
			found = append(found, u)
		}
	}
	switch len(found) {
	case 0:
		return nil, ErrNoMatch
	case 1:
		return found[0], nil
	}
	return nil, ErrAmbiguous
}
