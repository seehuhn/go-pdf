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
	"sort"

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

// CMapEntry describes the association between a character index and
// a glyph ID.
type CMapEntry struct {
	CID uint16
	GID font.GlyphID
}

// MakeSubset converts a mapping from a full font to a subsetted font.
// It also returns the list of original glyphs to include in the subset.
func MakeSubset(origMapping []CMapEntry) ([]CMapEntry, []font.GlyphID) {
	var newMapping []CMapEntry
	for _, m := range origMapping {
		if m.GID != 0 {
			newMapping = append(newMapping, m)
		}
	}
	sort.Slice(newMapping, func(i, j int) bool {
		return newMapping[i].CID < newMapping[j].CID
	})

	newToOrigGid := []font.GlyphID{0}
	for i, m := range newMapping {
		newGid := font.GlyphID(i + 1)
		newToOrigGid = append(newToOrigGid, m.GID)
		newMapping[i].GID = newGid
	}

	return newMapping, newToOrigGid
}

// MakeCMap writes a cmap with just a 1,0,4 subtable to map character indices
// to glyph indices in a subsetted font. The slice `mapping` must be sorted in
// order of increasing CID values.
func MakeCMap(mapping []CMapEntry) ([]byte, error) {
	if len(mapping) == 0 {
		return nil, nil
	}

	var finalGID uint16
	if n := len(mapping); mapping[n-1].CID == 0xFFFF {
		finalGID = uint16(mapping[n-1].GID)
		mapping = mapping[:n-1]
	}

	var StartCode, EndCode, IDDelta, IDRangeOffsets, GlyphIDArray []uint16
	segments := findSegments(mapping)
	for i := 1; i < len(segments); i++ {
		start := segments[i-1]
		end := segments[i]

		cid := mapping[start].CID
		gid := uint16(mapping[start].GID)
		delta := gid - cid
		canUseDelta := true
		for i := start + 1; i < end; i++ {
			thisCid := mapping[i].CID
			thisGid := uint16(mapping[i].GID)
			thisDelta := thisGid - thisCid
			if thisDelta != delta {
				canUseDelta = false
				break
			}
		}

		StartCode = append(StartCode, cid)
		EndCode = append(EndCode, mapping[end-1].CID)
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
			for c := cid; c <= mapping[end-1].CID; c++ {
				var val uint16
				if mapping[pos].CID == c {
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

func findSegments(mapping []CMapEntry) []int {
	// There are two different ways to encode GID values for a segment
	// of CID values:
	//
	//   - If GID-CID is constant over the range, IDDelta can be used.
	//     This requires 4 words of storage.
	//     The range can contain unmapped character indices.
	//   - Otherwise, GlyphIDArray can be used.  This requires
	//     4 + (EndCode - StartCode + 1) words of storage.
	//
	// Example:
	//     cid:  1  2  5 |  6  7  8  ->  4 + 7 = 11 words
	//     gid:  1  2  5 | 10 11  6
	//
	//     cid:  1  2  5  6  7  8  ->  12 words
	//     gid:  1  2  5 10 11  6
	//
	//     cid:  1  2  5 |  6  7 | 8  ->  4 + 4 + 5 = 13 words
	//     gid:  1  2  5 | 10 11 | 6

	cost := func(k, l int) int {
		delta := uint16(mapping[k].GID) - mapping[k].CID
		for i := k + 1; i < l; i++ {
			deltaI := uint16(mapping[i].GID) - mapping[i].CID
			if deltaI != delta {
				// we have to use GlyphIDArray
				return 4 + int(mapping[l-1].CID) - int(mapping[k].CID) + 1
			}
		}
		return 4 // we can use IDDelta
	}

	// Use Dijkstra's algorithm to find the best splits between segments.
	// https://en.wikipedia.org/wiki/Dijkstra%27s_algorithm
	//     vertices: 0, 1, ..., n, start at 0, end at n
	//     edges: (k, l) with 0 <= k < l <= n
	n := len(mapping)
	dist := make([]int, n)
	to := make([]int, n)
	for i := 0; i < n; i++ {
		dist[i] = cost(i, n)
		to[i] = n
	}

	pos := n
	for pos > 0 {
		bestNode, bestDist := 0, dist[0]
		for i := 1; i < pos; i++ {
			if dist[i] < bestDist {
				bestNode = i
				bestDist = dist[i]
			}
		}
		pos = bestNode

		for i := 0; i < pos; i++ {
			alt := bestDist + cost(i, pos)
			if alt < dist[i] {
				dist[i] = alt
				to[i] = pos
			}
		}
	}

	res := []int{0}
	pos = 0
	for pos < n {
		pos = to[pos]
		res = append(res, pos)
	}
	return res
}
