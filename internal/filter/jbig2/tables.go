// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package jbig2

import (
	"cmp"
	"encoding/binary"
	"fmt"
	"math"
	"slices"
)

// parseCustomHuffTable parses a type-53 segment's data into a huffTable.
// The format is defined in ITU-T T.88, Annex B.2.
func parseCustomHuffTable(data []byte) (*huffTable, error) {
	if len(data) < 9 {
		return nil, fmt.Errorf("custom Huffman table too short: %d bytes", len(data))
	}

	// flags (1 byte)
	flags := data[0]
	htOOB := flags&1 != 0
	htPS := int((flags>>1)&7) + 1 // prefix-length field width
	htRS := int((flags>>4)&7) + 1 // range-length field width

	// HTLOW and HTHIGH (4 bytes each, signed big-endian)
	htLow := int32(binary.BigEndian.Uint32(data[1:5]))
	htHigh := int32(binary.BigEndian.Uint32(data[5:9]))

	if htLow > htHigh {
		return nil, fmt.Errorf("custom table HTLOW (%d) > HTHIGH (%d)", htLow, htHigh)
	}

	hr := newHuffReader(data[9:])
	var lines []huffLine

	checkPrefLen := func(pl int) error {
		if pl > 32 {
			return fmt.Errorf("custom table prefix length %d exceeds 32", pl)
		}
		return nil
	}

	// normal lines (use int64 to avoid overflow with large RANGELEN)
	curRangeLow := int64(htLow)
	for curRangeLow < int64(htHigh) {
		prefLen := int(hr.readBits(htPS))
		rangeLen := int(hr.readBits(htRS))
		if hr.err != nil {
			return nil, hr.err
		}
		if err := checkPrefLen(prefLen); err != nil {
			return nil, err
		}
		lines = append(lines, huffLine{
			RangeLow: int32(curRangeLow),
			PrefLen:  prefLen,
			RangeLen: rangeLen,
		})
		curRangeLow += int64(1) << rangeLen
		if len(lines) > 1000 {
			return nil, fmt.Errorf("custom table too many lines")
		}
	}

	// lower range line
	prefLen := int(hr.readBits(htPS))
	rangeLen := int(hr.readBits(htRS))
	if hr.err != nil {
		return nil, hr.err
	}
	if err := checkPrefLen(prefLen); err != nil {
		return nil, err
	}
	lines = append(lines, huffLine{
		RangeLow: htLow - 1,
		PrefLen:  prefLen,
		RangeLen: rangeLen,
		IsLower:  true,
	})

	// upper range line
	prefLen = int(hr.readBits(htPS))
	rangeLen = int(hr.readBits(htRS))
	if hr.err != nil {
		return nil, hr.err
	}
	if err := checkPrefLen(prefLen); err != nil {
		return nil, err
	}
	if curRangeLow > math.MaxInt32 || curRangeLow < math.MinInt32 {
		return nil, fmt.Errorf("custom table upper range overflow")
	}
	lines = append(lines, huffLine{
		RangeLow: int32(curRangeLow),
		PrefLen:  prefLen,
		RangeLen: rangeLen,
	})

	// OOB line
	if htOOB {
		prefLen = int(hr.readBits(htPS))
		if hr.err != nil {
			return nil, hr.err
		}
		if err := checkPrefLen(prefLen); err != nil {
			return nil, err
		}
		lines = append(lines, huffLine{
			PrefLen: prefLen,
			IsOOB:   true,
		})
	}

	t := &huffTable{Lines: lines}
	t.assignCodes()
	return t, nil
}

// encodeCustomHuffTable encodes a huffTable as type-53 segment data.
// Returns an error if the table cannot be represented in the format
// (e.g. ranges overflow int32).
func encodeCustomHuffTable(t *huffTable) ([]byte, error) {
	// first pass: collect all non-special lines and compute HTLOW/HTHIGH
	var candidates []huffLine
	var lowerLine *huffLine
	var oobLine *huffLine

	for i := range t.Lines {
		l := &t.Lines[i]
		switch {
		case l.IsOOB:
			oobLine = l
		case l.IsLower:
			lowerLine = l
		default:
			candidates = append(candidates, *l)
		}
	}

	// sort candidates by RangeLow and walk ranges to separate normal
	// lines from the upper range line
	slices.SortFunc(candidates, func(a, b huffLine) int {
		return cmp.Compare(a.RangeLow, b.RangeLow)
	})

	htLow := int32(0)
	htHigh := int32(0)
	var normalLines []huffLine
	var upperLine *huffLine

	if len(candidates) > 0 {
		htLow = candidates[0].RangeLow

		// walk contiguous ranges from HTLOW
		curRangeLow := int64(htLow)
		for _, l := range candidates {
			if int64(l.RangeLow) != curRangeLow {
				// this candidate doesn't continue the range — it's
				// the upper range line
				upperLine = &l
				break
			}
			normalLines = append(normalLines, l)
			curRangeLow += int64(1) << l.RangeLen
		}
		// if all candidates were contiguous (no break), the last one
		// is the upper range line
		if upperLine == nil && len(normalLines) > 0 {
			upperLine = &normalLines[len(normalLines)-1]
			normalLines = normalLines[:len(normalLines)-1]
			if len(normalLines) > 0 {
				last := normalLines[len(normalLines)-1]
				curRangeLow = int64(last.RangeLow) + int64(1)<<last.RangeLen
			} else {
				curRangeLow = int64(htLow)
			}
		}

		if curRangeLow > math.MaxInt32 || curRangeLow < math.MinInt32 {
			return nil, fmt.Errorf("custom table range overflow")
		}
		htHigh = int32(curRangeLow)
	}

	// compute HTPS and HTRS (minimum field widths)
	maxPrefLen := 0
	maxRangeLen := 0
	for _, l := range t.Lines {
		if l.PrefLen > maxPrefLen {
			maxPrefLen = l.PrefLen
		}
		if !l.IsOOB && l.RangeLen > maxRangeLen {
			maxRangeLen = l.RangeLen
		}
	}
	htPS := 1
	for (1 << htPS) <= maxPrefLen {
		htPS++
	}
	if htPS > 8 {
		htPS = 8
	}
	htRS := 1
	for (1 << htRS) <= maxRangeLen {
		htRS++
	}
	if htRS > 8 {
		htRS = 8
	}

	// flags byte
	var flags byte
	if oobLine != nil {
		flags |= 1
	}
	flags |= byte(htPS-1) << 1
	flags |= byte(htRS-1) << 4

	var buf []byte
	buf = append(buf, flags)
	buf = binary.BigEndian.AppendUint32(buf, uint32(htLow))
	buf = binary.BigEndian.AppendUint32(buf, uint32(htHigh))

	// write table lines
	w := &bitWriter{}

	// normal lines
	for _, l := range normalLines {
		w.writeBits(uint32(l.PrefLen), htPS)
		w.writeBits(uint32(l.RangeLen), htRS)
	}

	// lower range line
	if lowerLine != nil {
		w.writeBits(uint32(lowerLine.PrefLen), htPS)
		w.writeBits(uint32(lowerLine.RangeLen), htRS)
	} else {
		w.writeBits(0, htPS) // PrefLen=0 (no code)
		w.writeBits(0, htRS)
	}

	// upper range line
	if upperLine != nil {
		w.writeBits(uint32(upperLine.PrefLen), htPS)
		w.writeBits(uint32(upperLine.RangeLen), htRS)
	} else {
		w.writeBits(0, htPS)
		w.writeBits(0, htRS)
	}

	// OOB line
	if oobLine != nil {
		w.writeBits(uint32(oobLine.PrefLen), htPS)
	}

	w.align()
	buf = append(buf, w.bytes()...)
	return buf, nil
}
