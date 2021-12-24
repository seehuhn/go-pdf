// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package sfnt

import (
	"bytes"
	"encoding/binary"
	"math/bits"

	"seehuhn.de/go/pdf/dijkstra"
	"seehuhn.de/go/pdf/font"
)

type cmapTableHead struct {
	// header
	Version   uint16 // Table version number (0)
	NumTables uint16 // Number of encoding tables that follow (1)

	// encoding records (array of length 1)
	PlatformID     uint16 // Platform ID (1)
	EncodingID     uint16 // Platform-specific encoding ID (0)
	SubtableOffset uint32 // Byte offset to the subtable (12)

	// format 4 subtable
	Format        uint16 // Format number (4)
	Length        uint16 // Length in bytes of the subtable.
	Language      uint16 // (0)
	SegCountX2    uint16 // 2 × segCount.
	SearchRange   uint16 // ...
	EntrySelector uint16 // ...
	RangeShift    uint16 // ...
	// EndCode        []uint16 // End characterCode for each segment, last=0xFFFF.
	// ReservedPad    uint16   // (0)
	// StartCode      []uint16 // Start character code for each segment.
	// IDDelta        []uint16 // Delta for all character codes in segment.
	// IDRangeOffsets []uint16 // Offsets into glyphIDArray or 0
	// GlyphIDArray   []uint16 // Glyph index array (arbitrary length)
}

// makeCMap writes a cmap containing a 1,0,4 subtable to map character indices
// to glyph indices in a subsetted font.  The slice `mapping` must be sorted in
// order of increasing CharCode values.
func makeCMap(mapping []font.CMapEntry) ([]byte, error) {
	if len(mapping) == 0 {
		return nil, nil
	}

	var finalGID uint16
	if n := len(mapping); mapping[n-1].CharCode == 0xFFFF {
		finalGID = uint16(mapping[n-1].GID)
		mapping = mapping[:n-1]
	}

	var StartCode, EndCode, IDDelta, IDRangeOffsets, GlyphIDArray []uint16
	segments := findSegments(mapping)
	for i := 1; i < len(segments); i++ {
		start := segments[i-1]
		end := segments[i]

		charCode := mapping[start].CharCode
		gid := uint16(mapping[start].GID)
		delta := gid - charCode
		canUseDelta := true
		for i := start + 1; i < end; i++ {
			thisCharCode := mapping[i].CharCode
			thisGid := uint16(mapping[i].GID)
			thisDelta := thisGid - thisCharCode
			if thisDelta != delta {
				canUseDelta = false
				break
			}
		}

		StartCode = append(StartCode, charCode)
		EndCode = append(EndCode, mapping[end-1].CharCode)
		if canUseDelta {
			IDDelta = append(IDDelta, delta)
			IDRangeOffsets = append(IDRangeOffsets, 0)
		} else {
			IDDelta = append(IDDelta, 0)
			offs := 2 * (len(segments) - i + // remaining entries in IDRangeOffsets
				1 + // the final segment
				len(GlyphIDArray)) // any previous entries in GlyphIDArray
			if offs > 65535 {
				panic("too many mappings for a 1,0,4 subtable")
			}
			IDRangeOffsets = append(IDRangeOffsets, uint16(offs))
			pos := start
			for c := charCode; c <= mapping[end-1].CharCode; c++ {
				var val uint16
				if mapping[pos].CharCode == c {
					val = uint16(mapping[pos].GID)
					pos++
				}
				GlyphIDArray = append(GlyphIDArray, val)
			}
		}
	}
	// add the required final segment
	StartCode = append(StartCode, 0xFFFF)
	EndCode = append(EndCode, 0xFFFF)
	IDDelta = append(IDDelta, finalGID-0xFFFF)
	IDRangeOffsets = append(IDRangeOffsets, 0)

	// Encode the data in the binary format described at
	// https://docs.microsoft.com/en-us/typography/opentype/spec/cmap#format-4-segment-mapping-to-delta-values
	data := &cmapTableHead{
		NumTables:      1,
		PlatformID:     1,
		EncodingID:     0,
		SubtableOffset: 12,
		Format:         4,
	}
	segCount := len(StartCode)
	data.Length = uint16(2 * (8 + 4*segCount + len(GlyphIDArray)))
	data.SegCountX2 = uint16(2 * segCount)
	sel := bits.Len(uint(segCount))
	data.SearchRange = 1 << sel
	data.EntrySelector = uint16(sel - 1)
	data.RangeShift = data.SegCountX2 - data.SearchRange

	EndCode = append(EndCode, 0) // add the ReservedPad field here

	buf := &bytes.Buffer{}
	err := binary.Write(buf, binary.BigEndian, data)
	if err != nil {
		return nil, err
	}
	for _, x := range [][]uint16{EndCode, StartCode, IDDelta, IDRangeOffsets, GlyphIDArray} {
		err := binary.Write(buf, binary.BigEndian, x)
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func findSegments(mapping []font.CMapEntry) []int {
	// There are two different ways to encode GID values for a segment
	// of CharCode values:
	//
	//   - If GID-CharCode is constant over the range, IDDelta can be used.
	//     This requires 4 words of storage.
	//     The range can contain unmapped character indices.
	//   - Otherwise, GlyphIDArray can be used.  This requires
	//     4 + (EndCode - StartCode + 1) words of storage.
	//
	// Example:
	//     charCode:  1  2  5 |  6  7  8  ->  4 + 7 = 11 words
	//     gid:       1  2  5 | 10 11  6
	//
	//     charCode:  1  2  5  6  7  8  ->  12 words
	//     gid:       1  2  5 10 11  6
	//
	//     charCode:  1  2  5 |  6  7 | 8  ->  4 + 4 + 5 = 13 words
	//     gid:       1  2  5 | 10 11 | 6

	cost := func(k, l int) int {
		delta := uint16(mapping[k].GID) - mapping[k].CharCode
		for i := k + 1; i < l; i++ {
			deltaI := uint16(mapping[i].GID) - mapping[i].CharCode
			if deltaI != delta {
				// we have to use GlyphIDArray
				return 4 + int(mapping[l-1].CharCode) - int(mapping[k].CharCode) + 1
			}
		}
		return 4 // we can use IDDelta
	}

	n := len(mapping)

	// Use Dijkstra's algorithm to find the best splits between segments.
	_, path := dijkstra.ShortestPath(cost, n)
	return path
}
