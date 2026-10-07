package datahub

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
)

// Header records which pinned file the classes came from. It carries no
// timestamp, so the same file always encodes to the same bytes.
type Header struct {
	SourceURL string `json:"source_url"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
	Edition   string `json:"edition"`
	Published string `json:"published"`
	Licence   string `json:"licence"`
	Country   string `json:"country"`
}

// File is the committed snapshot document.
type File struct {
	Header  Header  `json:"header"`
	Classes []Class `json:"classes"`
}

// NewFile copies s and sorts the copy by site then season.
func NewFile(h Header, s Snapshot) File {
	classes := slices.Clone(s.Classes)
	slices.SortFunc(classes, func(a, b Class) int {
		if c := strings.Compare(a.SiteID, b.SiteID); c != 0 {
			return c
		}
		return a.Season - b.Season
	})
	return File{Header: h, Classes: classes}
}

// Encode renders the file with one class per line, so a yearly diff reads as
// added lines. The layout is fixed: two-space indent, trailing newline.
func (f File) Encode() ([]byte, error) {
	head, err := json.MarshalIndent(f.Header, "  ", "  ")
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteString("{\n  \"header\": ")
	b.Write(head)
	b.WriteString(",\n  \"classes\": [")
	for i, c := range f.Classes {
		line, err := json.Marshal(c)
		if err != nil {
			return nil, err
		}
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString("\n    ")
		b.Write(line)
	}
	if len(f.Classes) > 0 {
		b.WriteString("\n  ")
	}
	b.WriteString("]\n}\n")
	return b.Bytes(), nil
}
