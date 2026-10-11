package burgas

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

// CFB sector markers and layout constants.
const (
	sectEnd     = 0xFFFFFFFE
	sectMaxReg  = 0xFFFFFFFA
	dirEntrySz  = 128
	headerSz    = 512
	difatInHdr  = 109
	maxDirEntry = MaxInputBytes / dirEntrySz // the input cap bounds the entry count
	miniCutoff  = 4096
)

var (
	ErrCycle     = errors.New("burgas: sector chain loops")
	ErrChain     = errors.New("burgas: bad sector chain")
	ErrContainer = errors.New("burgas: bad OLE2 container")
)

var cfbMagic = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

// cfbMap is the part of the container the chain check needs.
type cfbMap struct {
	data    []byte
	secSize int
	nSec    int
	fat     []uint32
}

// walkChain follows a sector chain and returns the sectors in order. The
// visited set is what stops a looping FAT; it also bounds the length.
func walkChain(fat []uint32, start uint32, nSec int) ([]uint32, error) {
	var out []uint32
	seen := make(map[uint32]bool)
	for s := start; s != sectEnd; {
		if s > sectMaxReg || int(s) >= nSec || int(s) >= len(fat) {
			return nil, fmt.Errorf("%w: sector %d out of range", ErrChain, s)
		}
		if seen[s] {
			return nil, ErrCycle
		}
		seen[s] = true
		out = append(out, s)
		s = fat[s]
	}
	return out, nil
}

// checkChains validates every sector chain of the container before mscfb
// sees it: the directory, the mini FAT and each stream. Every read is
// bounded by the input length.
func checkChains(data []byte) error {
	if len(data) < headerSz || !bytes.Equal(data[:8], cfbMagic) {
		return fmt.Errorf("%w: magic", ErrContainer)
	}
	shift := binary.LittleEndian.Uint16(data[30:32])
	if shift != 9 && shift != 12 {
		return fmt.Errorf("%w: sector size", ErrContainer)
	}
	m := &cfbMap{data: data, secSize: 1 << shift}
	if len(data) < m.secSize {
		return fmt.Errorf("%w: short", ErrContainer)
	}
	m.nSec = (len(data) - m.secSize) / m.secSize
	nFat := int(binary.LittleEndian.Uint32(data[44:48]))
	dirFrom := binary.LittleEndian.Uint32(data[48:52])
	miniFrom := binary.LittleEndian.Uint32(data[60:64])
	nextDifat := binary.LittleEndian.Uint32(data[68:72])
	if nFat < 1 || nFat > difatInHdr || nFat > m.nSec {
		return fmt.Errorf("%w: FAT count", ErrContainer)
	}
	if nextDifat != sectEnd {
		return fmt.Errorf("%w: extended DIFAT", ErrContainer)
	}
	per := m.secSize / 4
	for i := 0; i < nFat; i++ {
		sec, ok := m.sector(binary.LittleEndian.Uint32(data[76+4*i:]))
		if !ok {
			return fmt.Errorf("%w: FAT sector", ErrContainer)
		}
		for j := 0; j < per; j++ {
			m.fat = append(m.fat, binary.LittleEndian.Uint32(sec[4*j:]))
		}
	}
	dirChain, err := walkChain(m.fat, dirFrom, m.nSec)
	if err != nil {
		return err
	}
	dirs, err := m.join(dirChain)
	if err != nil {
		return err
	}
	miniChain, err := walkChain(m.fat, miniFrom, m.nSec)
	if err != nil {
		return err
	}
	miniBytes, err := m.join(miniChain)
	if err != nil {
		return err
	}
	mini := make([]uint32, 0, len(miniBytes)/4)
	for i := 0; i+4 <= len(miniBytes); i += 4 {
		mini = append(mini, binary.LittleEndian.Uint32(miniBytes[i:]))
	}
	for off := 0; off+dirEntrySz <= len(dirs); off += dirEntrySz {
		e := dirs[off : off+dirEntrySz]
		typ := e[66]
		if typ != 2 && typ != 5 { // stream or root
			continue
		}
		start := binary.LittleEndian.Uint32(e[116:120])
		size := binary.LittleEndian.Uint64(e[120:128])
		if typ == 2 && size == 0 {
			continue
		}
		fat, n := m.fat, m.nSec
		if typ == 2 && size < miniCutoff {
			fat, n = mini, len(mini)
		}
		if _, err := walkChain(fat, start, n); err != nil {
			return err
		}
	}
	return nil
}

// sector returns the bytes of sector s, or false when it is out of range.
func (m *cfbMap) sector(s uint32) ([]byte, bool) {
	if s > sectMaxReg || int(s) >= m.nSec {
		return nil, false
	}
	off := (int(s) + 1) * m.secSize
	return m.data[off : off+m.secSize], true
}

func (m *cfbMap) join(chain []uint32) ([]byte, error) {
	var out []byte
	for _, s := range chain {
		b, ok := m.sector(s)
		if !ok {
			return nil, fmt.Errorf("%w: sector %d", ErrChain, s)
		}
		out = append(out, b...)
	}
	return out, nil
}
