package datahub

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// VerifyFile applies the Fetch pins to a local file: size first, then sha256.
// Callers parse only after it returns nil.
func VerifyFile(path, sha string, size int64) (Meta, error) {
	if !pinPattern.MatchString(sha) {
		return Meta{}, ErrBadPin
	}
	if size <= 0 {
		return Meta{}, ErrBadLimits
	}
	f, err := os.Open(path)
	if err != nil {
		return Meta{}, fmt.Errorf("datahub: %w", err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return Meta{}, fmt.Errorf("datahub: %w", err)
	}
	if !st.Mode().IsRegular() || st.Size() != size {
		return Meta{}, fmt.Errorf("%w: file has %d bytes, pinned %d", ErrSize, st.Size(), size)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return Meta{}, fmt.Errorf("datahub: %w", err)
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if sum != sha {
		return Meta{}, fmt.Errorf("%w: observed %s, pinned %s", ErrSHA256, sum, sha)
	}
	return Meta{Path: path, SHA256: sum, Size: st.Size()}, nil
}
