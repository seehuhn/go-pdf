package cmap

import (
	"seehuhn.de/go/pdf/font"
)

type format6 struct {
	FirstCode    uint16
	GlyphIDArray []font.GlyphID
}

func decodeFormat6(data []byte) (Subtable, error) {
	if len(data) < 10 {
		return nil, errMalformedCmap
	}
	firstCode := uint16(data[6])<<8 | uint16(data[7])
	count := int(data[8])<<8 | int(data[9])

	// some fonts have an excess 0x0000 at the end of the table
	if len(data) == 10+2*count+2 && data[10+2*count] == 0 && data[10+2*count+1] == 0 {
		data = data[:10+2*count]
	}

	if len(data) != 10+2*count {
		return nil, errMalformedCmap
	}

	res := &format6{
		FirstCode:    firstCode,
		GlyphIDArray: make([]font.GlyphID, count),
	}
	for i := 0; i < count; i++ {
		res.GlyphIDArray[i] = font.GlyphID(data[10+2*i])<<8 | font.GlyphID(data[11+2*i])
	}
	return res, nil
}

func (cmap *format6) Encode(language uint16) []byte {
	n := len(cmap.GlyphIDArray)
	length := 10 + 2*n
	res := make([]byte, length)
	copy(res, []byte{
		0, 6,
		byte(length >> 8), byte(length),
		byte(language >> 8), byte(language),
		byte(cmap.FirstCode >> 8), byte(cmap.FirstCode),
		byte(n >> 8), byte(n),
	})
	for i, id := range cmap.GlyphIDArray {
		res[10+2*i] = byte(id >> 8)
		res[11+2*i] = byte(id)
	}
	return res
}

func (cmap *format6) Lookup(code uint32) font.GlyphID {
	if code < uint32(cmap.FirstCode) {
		return 0
	}
	if code >= uint32(cmap.FirstCode)+uint32(len(cmap.GlyphIDArray)) {
		return 0
	}
	return cmap.GlyphIDArray[code-uint32(cmap.FirstCode)]
}

func (cmap *format6) CodeRange() (low, high uint32) {
	i := 0
	for i < len(cmap.GlyphIDArray) && cmap.GlyphIDArray[i] == 0 {
		i++
	}
	if i == len(cmap.GlyphIDArray) {
		return
	}
	low = uint32(cmap.FirstCode) + uint32(i)

	i = len(cmap.GlyphIDArray) - 1
	for cmap.GlyphIDArray[i] == 0 {
		i--
	}
	high = uint32(cmap.FirstCode) + uint32(i)
	return
}
