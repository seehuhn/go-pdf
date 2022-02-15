package cmap

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/bits"
	"sort"

	"seehuhn.de/go/pdf/dijkstra"
	"seehuhn.de/go/pdf/font"
)

// https://docs.microsoft.com/en-us/typography/opentype/spec/cmap#format-4-segment-mapping-to-delta-values

type format4 map[uint32]font.GlyphID

func decodeFormat4(in []byte) (format4, error) {
	if len(in)%2 != 0 || len(in) < 16 {
		return nil, errMalformedCmap
	}

	segCountX2 := int(in[6])<<8 | int(in[7])
	if segCountX2%2 != 0 || 4*segCountX2+16 > len(in) {
		return nil, errMalformedCmap
	}
	segCount := segCountX2 / 2

	// TODO(voss): decode words on-demand to avoid this allocation.
	words := make([]uint16, 0, (len(in)-14)/2)
	for i := 14; i < len(in); i += 2 {
		words = append(words, uint16(in[i])<<8+uint16(in[i+1]))
	}
	endCode := words[:segCount]
	// reservedPad omitted
	startCode := words[segCount+1 : 2*segCount+1]
	idDelta := words[2*segCount+1 : 3*segCount+1]
	idRangeOffset := words[3*segCount+1 : 4*segCount+1]
	glyphIDArray := words[4*segCount+1:]

	fmt.Println()
	fmt.Printf("startCode: %x\n", startCode)
	fmt.Printf("endCode:   %x\n", endCode)
	fmt.Printf("idDelta:   %x\n", idDelta)
	fmt.Printf("idRngOffs: %x\n", idRangeOffset)
	fmt.Printf("glyphID:   %x\n", glyphIDArray)

	cmap := format4{}
	prevEnd := uint32(0)
	for k := 0; k < segCount; k++ {
		start := uint32(startCode[k])
		end := uint32(endCode[k]) + 1
		if start < prevEnd || end <= start {
			return nil, errMalformedCmap
		}
		prevEnd = end

		if idRangeOffset[k] == 0 {
			delta := idDelta[k]
			for idx := start; idx < end; idx++ {
				c := font.GlyphID(uint16(idx) + delta)
				if c != 0 {
					cmap[idx] = c
				}
			}
		} else {
			d := int(idRangeOffset[k])/2 - (segCount - k)
			if d < 0 || d+int(end-start) > len(glyphIDArray) {
				if start == 0xFFFF {
					// some fonts seem to have invalid data for the last segment
					continue
				}
				return nil, errMalformedCmap
			}
			for idx := start; idx < end; idx++ {
				c := font.GlyphID(glyphIDArray[d+int(idx-start)])
				if c != 0 {
					cmap[idx] = c
				}
			}
		}
	}
	return cmap, nil
}

func (cmap format4) Lookup(code uint32) font.GlyphID {
	return cmap[code]
}

func (cmap format4) Encode() []byte {
	mapping := make([]font.CMapEntry, 0, len(cmap))
	for code, gid := range cmap {
		c := uint16(code)
		if uint32(c) != code {
			continue
		}
		mapping = append(mapping, font.CMapEntry{
			CharCode: c,
			GID:      gid,
		})
	}
	sort.Slice(mapping, func(i, j int) bool {
		return mapping[i].CharCode < mapping[j].CharCode
	})

	// The final segment must be 0xFFFF-0xFFFF.  We map to 0, if there
	// is no preexisting mapping.
	var finalGID uint16
	if n := len(mapping); n > 0 && mapping[n-1].CharCode == 0xFFFF {
		finalGID = uint16(mapping[n-1].GID)
		mapping = mapping[:n-1]
	}

	segments := findSegments(mapping)

	var StartCode, EndCode, IDDelta, IDRangeOffsets, GlyphIDArray []uint16
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
				panic("too many mappings for a format 4 subtable")
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

	// Encode the data in the binary format
	data := &cmapFormat4{
		Format: 4,
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
	binary.Write(buf, binary.BigEndian, data)
	for _, x := range [][]uint16{EndCode, StartCode, IDDelta, IDRangeOffsets, GlyphIDArray} {
		binary.Write(buf, binary.BigEndian, x)
	}

	return buf.Bytes()
}

func findSegments(mapping []font.CMapEntry) []int {
	// There are two different ways to encode GID values for a segment
	// of CharCode values:
	//
	//   - If GID-CharCode is constant over the range, IDDelta can be used.
	//     This requires 4 words of storage.
	//
	//   - Otherwise, GlyphIDArray can be used.  This requires
	//     4 + (EndCode - StartCode + 1) words of storage.

	cost := func(k, l int) int {
		delta := uint16(mapping[k].GID) - mapping[k].CharCode
		for i := k + 1; i < l; i++ {
			deltaI := uint16(mapping[i].GID) - mapping[i].CharCode
			if mapping[i].GID != mapping[i-1].GID+1 || deltaI != delta {
				// we have to use GlyphIDArray
				return 4 + int(mapping[l-1].CharCode) - int(mapping[k].CharCode) + 1
			}
		}
		return 4 // we can use IDDelta
	}

	// Use Dijkstra's algorithm to find the best splits between segments.
	_, path := dijkstra.ShortestPath(cost, len(mapping))
	return path
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
