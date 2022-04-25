// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package cmap

import (
	"bytes"
	"encoding/binary"
	"math/bits"

	"seehuhn.de/go/dijkstra"
	"seehuhn.de/go/pdf/font"
)

// Format4 represents a format 4 cmap subtable.
// https://docs.microsoft.com/en-us/typography/opentype/spec/cmap#format-4-segment-mapping-to-delta-values
type Format4 map[uint16]font.GlyphID

func decodeFormat4(in []byte, code2rune func(c int) rune) (Subtable, error) {
	if code2rune == nil {
		code2rune = unicode
	}

	if len(in)%2 != 0 || len(in) < 16 {
		return nil, errMalformedSubtable
	}

	segCountX2 := int(in[6])<<8 | int(in[7])
	if segCountX2%2 != 0 || 4*segCountX2+16 > len(in) {
		return nil, errMalformedSubtable
	}
	segCount := segCountX2 / 2

	// TODO(voss): decode words on-demand to avoid this allocation.
	words := make([]uint16, 0, (len(in)-14)/2)
	for i := 14; i < len(in); i += 2 {
		words = append(words, uint16(in[i])<<8|uint16(in[i+1]))
	}
	endCode := words[:segCount]
	// reservedPad omitted
	startCode := words[segCount+1 : 2*segCount+1]
	idDelta := words[2*segCount+1 : 3*segCount+1]
	idRangeOffset := words[3*segCount+1 : 4*segCount+1]
	glyphIDArray := words[4*segCount+1:]

	cmap := Format4{}
	prevEnd := uint32(0)
	for k := 0; k < segCount; k++ {
		start := uint32(startCode[k])
		end := uint32(endCode[k]) + 1
		if start < prevEnd || end <= start {
			return nil, errMalformedSubtable
		}
		prevEnd = end

		if idRangeOffset[k] == 0 {
			delta := idDelta[k]
			for idx := start; idx < end; idx++ {
				c := font.GlyphID(uint16(idx) + delta)
				if c != 0 {
					cmap[uint16(code2rune(int(idx)))] = c
				}
			}
		} else {
			d := int(idRangeOffset[k])/2 - (segCount - k)
			if d < 0 || d+int(end-start) > len(glyphIDArray) {
				if start == 0xFFFF {
					// some fonts seem to have invalid data for the last segment
					continue
				}
				return nil, errMalformedSubtable
			}
			for idx := start; idx < end; idx++ {
				c := font.GlyphID(glyphIDArray[d+int(idx-start)])
				if c != 0 {
					cmap[uint16(code2rune(int(idx)))] = c
				}
			}
		}
	}
	return cmap, nil
}

// Lookup implements the Subtable interface.
func (cmap Format4) Lookup(r rune) font.GlyphID {
	return cmap[uint16(r)]
}

// Encode encodes the subtable into a byte slice.
func (cmap Format4) Encode(language uint16) []byte {
	g := makeSegments(cmap)
	segments, err := dijkstra.ShortestPath[uint32, *segment, int](g, 0, 0x10000)
	if err != nil {
		panic(err)
	}

	var StartCode, EndCode, IDDelta, IDRangeOffsets, GlyphIDArray []uint16
	for i, s := range segments {
		StartCode = append(StartCode, s.first)
		EndCode = append(EndCode, s.last)
		IDDelta = append(IDDelta, s.delta)
		if !s.useValues {
			IDRangeOffsets = append(IDRangeOffsets, 0)
		} else {
			offs := 2 * (len(segments) - i + // remaining entries in IDRangeOffsets
				len(GlyphIDArray)) // any previous entries in GlyphIDArray
			if offs > 65535 {
				panic("too many mappings for a format 4 subtable")
			}
			IDRangeOffsets = append(IDRangeOffsets, uint16(offs))
			for c := uint32(s.first); c <= uint32(s.last); c++ {
				GlyphIDArray = append(GlyphIDArray, uint16(cmap[uint16(c)]))
			}
		}
	}

	// Encode the data in the binary format
	segCount := len(StartCode)
	sel := bits.Len(uint(segCount))
	data := &cmapFormat4{
		Format:        4,
		Length:        uint16(2 * (8 + 4*segCount + len(GlyphIDArray))),
		Language:      language,
		SegCountX2:    uint16(2 * segCount),
		SearchRange:   1 << sel,
		EntrySelector: uint16(sel - 1),
	}
	data.RangeShift = data.SegCountX2 - data.SearchRange

	EndCode = append(EndCode, 0) // add the ReservedPad field here

	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.BigEndian, data)
	for _, x := range [][]uint16{EndCode, StartCode, IDDelta, IDRangeOffsets, GlyphIDArray} {
		_ = binary.Write(buf, binary.BigEndian, x)
	}

	return buf.Bytes()
}

// CodeRange returns the smallest and largest code point in the subtable.
func (cmap Format4) CodeRange() (low, high rune) {
	if len(cmap) == 0 {
		return
	}
	low = 1<<31 - 1
	for k := range cmap {
		if rune(k) < low {
			low = rune(k)
		}
		if rune(k) > high {
			high = rune(k)
		}
	}
	return
}

type segment struct {
	first     uint16
	last      uint16
	delta     uint16
	useValues bool
}

type makeSegments map[uint16]font.GlyphID

func (ms makeSegments) Edges(v uint32) []*segment {
	if v > 0xFFFF {
		return nil
	}

	// skip leading .notdef mappings
	start := v
	var skip uint16
	for start < 0xFFFF && ms[uint16(start)] == 0 {
		start++
		skip++
	}

	// check whether this is the last, special segment
	delta := uint16(ms[uint16(start)]) - uint16(start)
	if start == 0xFFFF {
		return []*segment{
			{first: 0xFFFF, last: 0xFFFF, delta: delta},
		}
	}

	// try to use a delta offset
	end := start + 1
	for end < 0xFFFF && uint16(ms[uint16(end)])-uint16(end) == delta {
		end++
	}
	segs := []*segment{
		{
			first: uint16(start),
			last:  uint16(end - 1),
			delta: delta,
		},
	}
	if end-start >= 4 || start == 0xFFFE {
		return segs
	}

	// as a last resort, store GID values explicitly
	prevDelta := delta
	numDelta := 1
	numNotdef := 0
	end = start + 1
	for end < 0xFFFF {
		thisGid := ms[uint16(end)]

		thisDelta := uint16(thisGid) - uint16(end)
		if thisDelta == prevDelta {
			numDelta++
		} else {
			prevDelta = thisDelta
			numDelta = 1 + numNotdef
		}

		if thisGid == 0 {
			numNotdef++
		} else {
			numNotdef = 0
		}

		if numDelta == 5 || numNotdef == 5 {
			segs = append(segs, &segment{
				first:     uint16(start),
				last:      uint16(end - 5),
				useValues: true,
			})
			return segs
		}

		end++
	}

	segs = append(segs, &segment{
		first:     uint16(start),
		last:      uint16(end - uint32(numNotdef) - 1),
		useValues: true,
	})
	return segs
}

func (ms makeSegments) Length(e *segment) int {
	if e.useValues {
		return 4 + (int(e.last-e.first) + 1)
	}
	return 4
}

func (ms makeSegments) To(e *segment) uint32 {
	return uint32(e.last) + 1
}

type cmapFormat4 struct {
	Format        uint16
	Length        uint16
	Language      uint16
	SegCountX2    uint16
	SearchRange   uint16
	EntrySelector uint16
	RangeShift    uint16
	// EndCode        []uint16 // End characterCode for each segment, last=0xFFFF.
	// ReservedPad    uint16   // (0)
	// StartCode      []uint16 // Start character code for each segment.
	// IDDelta        []uint16 // Delta for all character codes in segment.
	// IDRangeOffsets []uint16 // Offsets into glyphIDArray or 0
	// GlyphIDArray   []uint16 // Glyph index array (arbitrary length)
}
