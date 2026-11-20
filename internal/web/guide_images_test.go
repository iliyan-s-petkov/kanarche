package web

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// webpSize reads the pixel size from a WebP header (lossy, lossless or extended).
func webpSize(t *testing.T, path string) (int, int) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 30 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
		t.Fatalf("%s: not a WebP file", path)
	}
	switch string(b[12:16]) {
	case "VP8 ":
		return int(binary.LittleEndian.Uint16(b[26:28]) & 0x3fff), int(binary.LittleEndian.Uint16(b[28:30]) & 0x3fff)
	case "VP8L":
		bits := binary.LittleEndian.Uint32(b[21:25])
		return int(bits&0x3fff) + 1, int((bits>>14)&0x3fff) + 1
	case "VP8X":
		w := int(b[24]) | int(b[25])<<8 | int(b[26])<<16
		h := int(b[27]) | int(b[28])<<8 | int(b[29])<<16
		return w + 1, h + 1
	}
	t.Fatalf("%s: unknown WebP chunk %q", path, b[12:16])
	return 0, 0
}

// The width and height written on each <img> are the real pixel size, so the browser reserves the right box.
func TestGuideImageSizesMatchTheFiles(t *testing.T) {
	for name, want := range guideImages {
		w, h := webpSize(t, filepath.Join("static", "build-a-sensor", name+".webp"))
		if w != want.W || h != want.H {
			t.Errorf("%s: file is %dx%d, guide.go says %dx%d", name, w, h, want.W, want.H)
		}
	}
	files, err := filepath.Glob(filepath.Join("static", "build-a-sensor", "*.webp"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".webp")
		if _, ok := guideImages[name]; !ok {
			t.Errorf("%s has no entry in guideImages", f)
		}
	}
}
