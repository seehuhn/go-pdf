package cmap

import (
	"sort"

	"seehuhn.de/go/pdf/font"
)

// format12 represents a format 12 cmap subtable.
// https://docs.microsoft.com/en-us/typography/opentype/spec/cmap#format-12-segmented-coverage
type format12 []format12segment

type format12segment struct {
	startCharCode uint32
	endCharCode   uint32
	startGlyphID  uint32
}

func decodeFormat12(data []byte) (Subtable, error) {
	if len(data) < 16 {
		return nil, errMalformedCmap
	}

	nSegments := uint32(data[12])<<24 | uint32(data[13])<<16 | uint32(data[14])<<8 | uint32(data[15])
	if len(data) != 16+int(nSegments)*12 || nSegments > 1e6 {
		return nil, errMalformedCmap
	}

	segments := make(format12, nSegments)
	var prevEnd uint32
	for i := uint32(0); i < nSegments; i++ {
		base := 16 + i*12
		segments[i].startCharCode = uint32(data[base])<<24 | uint32(data[base+1])<<16 | uint32(data[base+2])<<8 | uint32(data[base+3])
		segments[i].endCharCode = uint32(data[base+4])<<24 | uint32(data[base+5])<<16 | uint32(data[base+6])<<8 | uint32(data[base+7])
		segments[i].startGlyphID = uint32(data[base+8])<<24 | uint32(data[base+9])<<16 | uint32(data[base+10])<<8 | uint32(data[base+11])

		if (i > 0 && segments[i].startCharCode <= prevEnd) ||
			segments[i].endCharCode < segments[i].startCharCode {
			return nil, errMalformedCmap
		}
		prevEnd = segments[i].endCharCode
	}

	return segments, nil
}

func (cmap format12) Encode(language uint16) []byte {
	nSegments := len(cmap)
	l := uint32(16 + nSegments*12)
	out := make([]byte, l)
	copy(out, []byte{
		0, 12, 0, 0,
		byte(l >> 24), byte(l >> 16), byte(l >> 8), byte(l),
		0, 0, byte(language >> 8), byte(language),
		byte(nSegments >> 24), byte(nSegments >> 16), byte(nSegments >> 8), byte(nSegments),
	})
	for i := 0; i < nSegments; i++ {
		base := 16 + i*12
		out[base] = byte(cmap[i].startCharCode >> 24)
		out[base+1] = byte(cmap[i].startCharCode >> 16)
		out[base+2] = byte(cmap[i].startCharCode >> 8)
		out[base+3] = byte(cmap[i].startCharCode)
		out[base+4] = byte(cmap[i].endCharCode >> 24)
		out[base+5] = byte(cmap[i].endCharCode >> 16)
		out[base+6] = byte(cmap[i].endCharCode >> 8)
		out[base+7] = byte(cmap[i].endCharCode)
		out[base+8] = byte(cmap[i].startGlyphID >> 24)
		out[base+9] = byte(cmap[i].startGlyphID >> 16)
		out[base+10] = byte(cmap[i].startGlyphID >> 8)
		out[base+11] = byte(cmap[i].startGlyphID)
	}
	return out
}

func (cmap format12) Lookup(code uint32) font.GlyphID {
	idx := sort.Search(len(cmap), func(i int) bool {
		return code <= cmap[i].endCharCode
	})
	if idx == len(cmap) || cmap[idx].startCharCode > code {
		return 0
	}
	return font.GlyphID(cmap[idx].startGlyphID + code - cmap[idx].startCharCode)
}

func (cmap format12) CodeRange() (low, high uint32) {
	if len(cmap) == 0 {
		return 0, 0
	}
	return cmap[0].startCharCode, cmap[len(cmap)-1].endCharCode
}
