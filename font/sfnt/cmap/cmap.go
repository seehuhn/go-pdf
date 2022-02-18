// Package cmap has code for reading and writing cmap tables.
// https://docs.microsoft.com/en-us/typography/opentype/spec/cmap
package cmap

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"sort"

	"golang.org/x/exp/slices"
	"seehuhn.de/go/pdf/font"
)

// SubtableData represents an encoded cmap subtable.
type SubtableData struct {
	PlatformID uint16 // Platform ID.
	EncodingID uint16 // Platform-specific encoding ID.
	Language   uint16
	Data       []byte
}

// Subtables represents the collection of all subtables in a cmap table.
type Subtables []SubtableData

// LocateSubtables returns all subtables of the given "cmap" table.
// The returned subtables are guaranteed to be at least 10 bytes long
// and to have a valid format value (0, 2, 4, 6, 8, 10, 12, 13 or 14)
// in the first two bytes.
func LocateSubtables(data []byte) (Subtables, error) {
	const minLength = 10 // length of an empty format 6 subtable

	if len(data) < 4 || len(data) > math.MaxUint32 {
		return nil, errMalformedCmap
	}
	version := uint16(data[0])<<8 | uint16(data[1])
	if version != 0 {
		return nil, fmt.Errorf("cmap: unknown version %d", version)
	}
	numTables := int(data[2])<<8 | int(data[3])
	if len(data) < 4+8*numTables {
		return nil, errMalformedCmap
	}

	endOfHeader := uint32(4 + 8*numTables)
	endOfData := uint32(len(data))

	type seg struct {
		start, end uint32
	}
	var segs []seg

	res := make([]SubtableData, numTables)
	for i := 0; i < numTables; i++ {
		platformID := uint16(data[4+i*8])<<8 | uint16(data[5+i*8])
		if platformID > 4 {
			return nil, errMalformedCmap
		}
		encodingID := uint16(data[6+i*8])<<8 | uint16(data[7+i*8])

		o := uint32(data[8+i*8])<<24 |
			uint32(data[9+i*8])<<16 |
			uint32(data[10+i*8])<<8 |
			uint32(data[11+i*8])
		if o < endOfHeader || o > endOfData-minLength {
			return nil, errMalformedCmap
		}

		var language uint16
		var length uint32
		format := uint16(data[o])<<8 | uint16(data[o+1])
		checkLength := uint32(minLength)
		switch format {
		case 0, 2, 4, 6:
			length = uint32(data[o+2])<<8 | uint32(data[o+3])
			language = uint16(data[o+4])<<8 | uint16(data[o+5])
		case 8, 10, 12, 13:
			checkLength = 12
			if o > endOfData-checkLength {
				return nil, errMalformedCmap
			}
			length = uint32(data[o+4])<<24 |
				uint32(data[o+5])<<16 |
				uint32(data[o+6])<<8 |
				uint32(data[o+7])
			language = uint16(data[o+10])<<8 | uint16(data[o+11])
		case 14:
			length = uint32(data[o+2])<<24 |
				uint32(data[o+3])<<16 |
				uint32(data[o+4])<<8 |
				uint32(data[o+5])
		default:
			return nil, errMalformedCmap
		}
		if length < checkLength || length > endOfData-o {
			return nil, errMalformedCmap
		}

		// check that subtables are either disjoint or identical
		idx := sort.Search(len(segs), func(i int) bool {
			return o <= segs[i].start
		})
		if idx == len(segs) || o != segs[idx].start {
			if idx > 0 && o < segs[idx-1].end ||
				idx < len(segs) && o+length > segs[idx].start {
				return nil, errMalformedCmap
			}
			segs = slices.Insert(segs, idx, seg{o, o + length})
		}

		res[i].PlatformID = platformID
		res[i].EncodingID = encodingID
		res[i].Language = language
		res[i].Data = data[o : o+length]
	}

	return res, nil
}

func (ss Subtables) Write(w io.Writer) error {
	type extended struct {
		Data       []byte
		Offs       uint32
		PlatformID uint16
		EncodingID uint16
		Language   uint16
	}
	ext := make([]extended, len(ss))
	for i, s := range ss {
		ext[i].Data = s.Data
		ext[i].PlatformID = s.PlatformID
		ext[i].EncodingID = s.EncodingID
		ext[i].Language = s.Language
	}
	sort.Slice(ext, func(i, j int) bool {
		if ext[i].PlatformID != ext[j].PlatformID {
			return ext[i].PlatformID < ext[j].PlatformID
		}
		if ext[i].EncodingID != ext[j].EncodingID {
			return ext[i].EncodingID < ext[j].EncodingID
		}
		return ext[i].Language < ext[j].Language
	})

	numTables := len(ext)
	endOfHeader := uint32(4 + 8*numTables)

	pos := endOfHeader
offsLoop:
	for i, e := range ext {
		for j := 0; j < i; j++ {
			if bytes.Equal(e.Data, ext[j].Data) {
				ext[i].Offs = ext[j].Offs
				ext[i].Data = nil
				continue offsLoop
			}
		}
		ext[i].Offs = pos
		pos += uint32(len(e.Data))
	}

	header := make([]byte, endOfHeader)
	// header[0] = 0
	// header[1] = 0
	header[2] = byte(numTables >> 8)
	header[3] = byte(numTables)
	for i, e := range ext {
		header[4+i*8] = byte(e.PlatformID >> 8)
		header[5+i*8] = byte(e.PlatformID)
		header[6+i*8] = byte(e.EncodingID >> 8)
		header[7+i*8] = byte(e.EncodingID)
		header[8+i*8] = byte(e.Offs >> 24)
		header[9+i*8] = byte(e.Offs >> 16)
		header[10+i*8] = byte(e.Offs >> 8)
		header[11+i*8] = byte(e.Offs)
	}
	_, err := w.Write(header)
	if err != nil {
		return err
	}
	for _, e := range ext {
		_, err = w.Write(e.Data)
		if err != nil {
			return err
		}
	}

	return nil
}

// Subtable represents a decoded cmap subtable.
type Subtable interface {
	Lookup(code uint32) font.GlyphID
	Encode() []byte
}

type format2 struct {
}

func decodeFormat2(data []byte) (*format2, error) {
	panic("not implemented")
}

func (cmap *format2) Lookup(code uint32) font.GlyphID {
	panic("not implemented")
}

func (cmap *format2) Encode() []byte {
	panic("not implemented")
}

type format6 struct {
}

func decodeFormat6(data []byte) (*format6, error) {
	panic("not implemented")
}

func (cmap *format6) Lookup(code uint32) font.GlyphID {
	panic("not implemented")
}

func (cmap *format6) Encode() []byte {
	panic("not implemented")
}

type format8 struct {
}

func decodeFormat8(data []byte) (*format8, error) {
	panic("not implemented")
}

func (cmap *format8) Lookup(code uint32) font.GlyphID {
	panic("not implemented")
}

func (cmap *format8) Encode() []byte {
	panic("not implemented")
}

type format10 struct {
}

func decodeFormat10(data []byte) (*format10, error) {
	panic("not implemented")
}

func (cmap *format10) Lookup(code uint32) font.GlyphID {
	panic("not implemented")
}

func (cmap *format10) Encode() []byte {
	panic("not implemented")
}

type format12 struct {
}

func decodeFormat12(data []byte) (*format12, error) {
	panic("not implemented")
}

func (cmap *format12) Lookup(code uint32) font.GlyphID {
	panic("not implemented")
}

func (cmap *format12) Encode() []byte {
	panic("not implemented")
}

type format13 struct {
}

func decodeFormat13(data []byte) (*format13, error) {
	panic("not implemented")
}

func (cmap *format13) Lookup(code uint32) font.GlyphID {
	panic("not implemented")
}

func (cmap *format13) Encode() []byte {
	panic("not implemented")
}

type format14 struct {
}

func decodeFormat14(data []byte) (*format14, error) {
	panic("not implemented")
}

func (cmap *format14) Lookup(code uint32) font.GlyphID {
	panic("not implemented")
}

func (cmap *format14) Encode() []byte {
	panic("not implemented")
}

var errMalformedCmap = fmt.Errorf("malformed cmap table")
