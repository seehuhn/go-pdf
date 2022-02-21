package cmap

import (
	"fmt"

	"seehuhn.de/go/pdf/font"
)

// https://docs.microsoft.com/en-us/typography/opentype/spec/cmap#format-0-byte-encoding-table

type format0 struct {
	glyphIDArray [256]uint8
}

func decodeFormat0(data []byte) (Subtable, error) {
	data = data[6:]
	if len(data) != 256 {
		return nil, fmt.Errorf("cmap: format 0: expected 256 bytes, got %d", len(data))
	}
	res := &format0{}
	copy(res.glyphIDArray[:], data)
	return res, nil
}

func (cmap *format0) Lookup(code uint32) font.GlyphID {
	if code < 256 {
		return font.GlyphID(cmap.glyphIDArray[code])
	}
	return 0
}

func (cmap *format0) Encode(language uint16) []byte {
	return append([]byte{0, 0, 1, 6, byte(language >> 8), byte(language)},
		cmap.glyphIDArray[:]...)
}

func (cmap *format0) CodeRange() (low, high uint32) {
	for i, c := range cmap.glyphIDArray {
		if c == 0 {
			continue
		}
		if low == 0 {
			low = uint32(i)
		}
		high = uint32(i)
	}
	return
}
