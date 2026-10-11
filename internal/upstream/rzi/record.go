// Package rzi ingests regional health inspectorate (RZI) bathing samples.
// This file is the outbox record contract written by cmd/rzi-extract.
package rzi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// SchemaVersion is the only outbox schema Decode accepts.
const SchemaVersion = 1

// MaxRecordBytes caps one outbox file.
const MaxRecordBytes = 2 << 20

// maxDepth is the deepest nesting the contract needs (file, blocks, block,
// rows, row, count) plus slack.
const maxDepth = 8

// maxKeyBytes caps one JSON object key.
const maxKeyBytes = 64

// Source ids, as stored in bathing_rzi_file.source.
const (
	SourceVarna   = "rzi_varna"
	SourceBurgas  = "rzi_burgas"
	SourceDobrich = "rzi_dobrich"
)

var (
	ErrRecord   = errors.New("rzi: malformed outbox record")
	ErrTooLarge = errors.New("rzi: outbox record over size cap")
	ErrSchema   = errors.New("rzi: unsupported outbox schema")
	errTooDeep  = errors.New("nesting too deep")
)

// File is one outbox record. Error is set by the extractor when it refused
// the input document; such a record carries no blocks.
type File struct {
	Schema           int      `json:"schema"`
	Source           string   `json:"source"`
	SHA256           string   `json:"sha256"`
	ExtractorVersion string   `json:"extractor_version"`
	Error            string   `json:"error,omitempty"`
	Blocks           []Block  `json:"blocks"`
	Warnings         []string `json:"warnings"`
}

// Block is one sampling point as printed in the document.
type Block struct {
	Point string  `json:"point"`
	Name  string  `json:"name"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
	Rows  []Row   `json:"rows"`
}

// Row is one sample. Dates are YYYY-MM-DD; Date is the actual sampling date.
// Remarks is untrusted inspector text, kept for review only.
type Row struct {
	Planned string `json:"planned"`
	Date    string `json:"date"`
	IE      Count  `json:"ie"`
	EC      Count  `json:"ec"`
	Unit    string `json:"unit"`
	Remarks string `json:"remarks"`
}

// Count is one indicator value. Op "<" means below detection.
type Count struct {
	Op    string `json:"op"`
	Value int64  `json:"value"`
}

// Decode reads one outbox record strictly: size cap, one JSON object,
// lowercase unique keys, no unknown fields, integer counts, schema 1.
func Decode(r io.Reader) (File, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxRecordBytes+1))
	if err != nil {
		return File{}, fmt.Errorf("%w: %v", ErrRecord, err)
	}
	if len(data) > MaxRecordBytes {
		return File{}, ErrTooLarge
	}
	if err := checkKeys(data); err != nil {
		return File{}, fmt.Errorf("%w: %w", ErrRecord, err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var f File
	if err := dec.Decode(&f); err != nil {
		return File{}, fmt.Errorf("%w: %v", ErrRecord, err)
	}
	if f.Schema != SchemaVersion {
		return File{}, fmt.Errorf("%w: %d", ErrSchema, f.Schema)
	}
	return f, nil
}

// frame is one open JSON object or array during checkKeys.
type frame struct {
	obj     bool
	wantKey bool
	keys    map[string]struct{}
}

// checkKeys walks the tokens once. encoding/json keeps the last of duplicate
// keys and matches names case-insensitively, so both are refused here. It
// also refuses trailing values and nesting deeper than maxDepth.
func checkKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var stack []frame
	values := 0
	// closeValue marks a finished value inside the current parent.
	closeValue := func() {
		if n := len(stack); n > 0 && stack[n-1].obj {
			stack[n-1].wantKey = true
		}
	}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(stack) == 0 {
			values++
			if values > 1 {
				return errors.New("trailing data after record")
			}
		}
		if n := len(stack); n > 0 && stack[n-1].obj && stack[n-1].wantKey {
			if d, ok := tok.(json.Delim); ok && d == '}' {
				stack = stack[:n-1]
				closeValue()
				continue
			}
			key, ok := tok.(string)
			if !ok || !validKey(key) {
				return errors.New("bad object key")
			}
			if _, dup := stack[n-1].keys[key]; dup {
				return fmt.Errorf("duplicate key %q", key)
			}
			stack[n-1].keys[key] = struct{}{}
			stack[n-1].wantKey = false
			continue
		}
		switch d, _ := tok.(json.Delim); d {
		case '{', '[':
			if len(stack) >= maxDepth {
				return errTooDeep
			}
			stack = append(stack, frame{obj: d == '{', wantKey: d == '{', keys: map[string]struct{}{}})
		case ']':
			stack = stack[:len(stack)-1]
			closeValue()
		default:
			closeValue()
		}
	}
	if values == 0 {
		return errors.New("empty record")
	}
	return nil
}

// validKey allows the contract's key alphabet only: [a-z0-9_], 1..64 bytes.
func validKey(k string) bool {
	if k == "" || len(k) > maxKeyBytes {
		return false
	}
	for i := 0; i < len(k); i++ {
		c := k[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}
