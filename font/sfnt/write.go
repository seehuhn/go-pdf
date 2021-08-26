// seehuhn.de/go/pdf - support for reading and writing PDF files
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
	"errors"
	"io"
	"math/bits"
	"sort"
	"time"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/table"
)

type ExportOptions struct {
	Include            map[string]bool // select a subset of tables
	Subset             []font.GlyphID  // select a subset of glyphs
	GenerateSimpleCmap bool
}

func contains(ss []string, s string) bool {
	for _, si := range ss {
		if si == s {
			return true
		}
	}
	return false
}

// Export writes the font to the io.Writer w.
func (tt *Font) Export(w io.Writer, opt *ExportOptions) (int64, error) {
	if opt == nil {
		opt = &ExportOptions{}
	}

	var total int64

	tableNames := tt.selectTables(opt)

	replTab := make(map[string][]byte)
	replSum := make(map[string]uint32)
	buf := &bytes.Buffer{}

	cc := &check{}

	hasHead := contains(tableNames, "head")
	if hasHead {
		// Copy and modify the "head" table.
		// https://docs.microsoft.com/en-us/typography/opentype/spec/head
		headTable := &table.Head{}
		*headTable = *tt.Head
		headTable.CheckSumAdjustment = 0
		ttZeroTime := time.Date(1904, time.January, 1, 0, 0, 0, 0, time.UTC)
		headTable.Modified = int64(time.Since(ttZeroTime).Seconds())
		_ = binary.Write(buf, binary.BigEndian, headTable)
		replTab["head"] = append([]byte{}, buf.Bytes()...) // make a copy
		_, _ = buf.WriteTo(cc)
		replSum["head"] = cc.Sum()
	}

	// generate and write the new file header
	numTables := len(tableNames)
	sel := bits.Len(uint(numTables)) - 1
	header := &table.Header{
		Offsets: table.Offsets{
			ScalerType:    tt.Header.Offsets.ScalerType,
			NumTables:     uint16(numTables),
			SearchRange:   1 << (sel + 4),
			EntrySelector: uint16(sel),
			RangeShift:    uint16(16 * (numTables - 1<<sel)),
		},
		Records: make([]table.Record, len(tableNames)),
	}
	offset := uint32(12 + 16*numTables)
	var totalSum uint32
	for i, name := range tableNames {
		old := tt.Header.Find(name)
		header.Records[i].Tag = old.Tag
		header.Records[i].Offset = offset
		var checksum uint32
		var length uint32
		if body, ok := replTab[name]; ok {
			checksum = replSum[name]
			length = uint32(len(body))
		} else {
			checksum = old.CheckSum // TODO(voss): recalculate?
			length = old.Length
		}
		header.Records[i].CheckSum = checksum
		header.Records[i].Length = length
		totalSum += checksum
		offset += 4 * ((length + 3) / 4)
	}
	sort.Slice(header.Records, func(i, j int) bool {
		return bytes.Compare(header.Records[i].Tag[:], header.Records[j].Tag[:]) < 0
	})
	err := binary.Write(w, binary.BigEndian, header.Offsets)
	if err != nil {
		return 0, err
	}
	total += 12
	err = binary.Write(w, binary.BigEndian, header.Records)
	if err != nil {
		return 0, err
	}
	total += 16 * int64(len(header.Records))

	// fix the checksum in the "head" table
	if hasHead {
		cc.Reset()
		headerChecksum, _ := checksumOld(buf, false)
		totalSum += headerChecksum
		binary.BigEndian.PutUint32(replTab["head"][8:12], 0xB1B0AFBA-totalSum)
	}

	// write the tables
	var pad [3]byte
	for _, name := range tableNames {
		var n int64
		if body, ok := replTab[name]; ok {
			n32, e2 := w.Write(body)
			n = int64(n32)
			err = e2
		} else {
			table := tt.Header.Find(name)
			tableFd := io.NewSectionReader(tt.Fd, int64(table.Offset), int64(table.Length))
			n, err = io.Copy(w, tableFd)
		}
		if err != nil {
			return 0, err
		}
		total += n
		if k := n % 4; k != 0 {
			l, err := w.Write(pad[:4-k])
			if err != nil {
				return 0, err
			}
			total += int64(l)
		}
	}

	return total, nil
}

func (tt *Font) selectTables(opt *ExportOptions) []string {
	var names []string
	done := make(map[string]bool)
	include := opt.Include

	// Fix the order of tables in the body of the files.
	// https://docs.microsoft.com/en-us/typography/opentype/spec/recom#optimized-table-ordering
	for _, name := range []string{
		"head", "hhea", "maxp", "OS/2", "hmtx", "LTSH", "VDMX", "hdmx", "cmap",
		"fpgm", "prep", "cvt ", "loca", "glyf", "kern", "name", "post", "gasp",
	} {
		done[name] = true
		if tt.Header.Find(name) != nil {
			if include == nil || include[name] {
				names = append(names, name)
			}
		}
	}

	// Pre-existing digital signatures will no longer be valid after we
	// re-arranged the tables:
	done["DSIG"] = true

	extraPos := len(names)
	for i := 0; i < int(tt.Header.Offsets.NumTables); i++ {
		name := tt.Header.Records[i].Tag.String()
		if done[name] {
			continue
		}
		if include == nil || include[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names[extraPos:])

	return names
}

type simpleCmapTableHead struct {
	// header
	Version   uint16 // Table version number (0)
	NumTables uint16 // Number of encoding tables that follow (1)

	// encoding records (array of length 1)
	PlatformID     uint16 // Platform ID (3)
	EncodingID     uint16 // Platform-specific encoding ID (0)
	SubtableOffset uint32 // Byte offset to the subtable (10)

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

// write a cmap with just a 3,0,4 subtable for a simple font
func writeSimpleCmap(w io.Writer, cmap map[rune]font.GlyphID) (func(...font.GlyphID) []byte, error) {
	var used []rune
	for r, idx := range cmap {
		if idx != 0 {
			used = append(used, r)
		}
	}
	sort.Slice(used, func(i, j int) bool {
		return cmap[used[i]] < cmap[used[j]]
	})
	n := len(used)
	if n > 256 {
		return nil, errors.New("too many characters for a simple font")
	}

	// Covering k glyphs as a segment with non-consecutive character indices
	// uses 4+k uint16 values.  Covering l glyphs as a segment with consecutive
	// character indices uses 4 uint16 values.  Thus, we only use segments with
	// consecutive character indices, if l >= 4.
	type segment struct {
		start  int // first index into used
		end    int // last index into used
		target font.GlyphID
	}
	var ss []*segment
	var strays []rune

	first := true
	var prev font.GlyphID
	runLength := 0
	pos := 0
	for i := 0; i < n; i++ {
		idx := cmap[used[i]]
		if first || idx == prev+1 {
			runLength++
		} else {
			if runLength >= 4 {
				if pos < i-runLength {
					strays = append(strays, used[pos:i-runLength]...)
				}
				ss = append(ss, &segment{
					start:  i - runLength,
					end:    i,
					target: cmap[used[i-runLength]],
				})
				pos = i
			}
			runLength = 1
		}

		first = false
		prev = idx
	}
	if runLength >= 4 {
		if pos < n-runLength {
			strays = append(strays, used[pos:n-runLength]...)
		}
		ss = append(ss, &segment{
			start:  n - runLength,
			end:    n,
			target: cmap[used[n-runLength]],
		})
	} else {
		strays = append(strays, used[pos:]...)
	}

	var base uint16
	switch {
	case 33+n <= 256:
		base = 33
	default:
		base = uint16(256 - n)
	}
	next := 0xF000 + base

	g2c := make(map[font.GlyphID]byte)

	// Construct the cmap table
	data := &simpleCmapTableHead{
		NumTables:      1,
		PlatformID:     3,
		EncodingID:     0,
		SubtableOffset: 12,
		Format:         4,
	}
	var StartCode, EndCode, IDDelta, IDRangeOffsets, GlyphIDArray []uint16
	for _, r := range strays {
		GlyphIDArray = append(GlyphIDArray, uint16(cmap[r]))
	}
	for _, seg := range ss {
		length := uint16(seg.end - seg.start)
		StartCode = append(StartCode, next)
		EndCode = append(EndCode, next+length-1)
		IDDelta = append(IDDelta, uint16(seg.target)-next)
		IDRangeOffsets = append(IDRangeOffsets, 0)
		for i := 0; i < int(length); i++ {
			g2c[seg.target+font.GlyphID(i)] = byte(next + uint16(i))
		}
		next += length
	}
	if len(GlyphIDArray) > 0 {
		length := uint16(len(GlyphIDArray))
		StartCode = append(StartCode, next)
		EndCode = append(EndCode, next+length-1)
		if length == 1 {
			// a continuous segment of length one is shorter
			IDDelta = append(IDDelta, GlyphIDArray[0]-next)
			IDRangeOffsets = append(IDRangeOffsets, 0)
			GlyphIDArray = nil
		} else {
			IDDelta = append(IDDelta, 0)
			// There will be one more segment, so we are using the second slot
			// from the end, i.e. k = segCount-2.  We want to point at index 0
			// in the GlyphIDArray, so we need
			// 0 == idRangeOffset[k]/2 - (segCount - k).
			IDRangeOffsets = append(IDRangeOffsets, 4)
		}
		for i := 0; i < int(length); i++ {
			g2c[font.GlyphID(GlyphIDArray[i])] = byte(next + uint16(i))
		}
	}
	// add the required final segment
	StartCode = append(StartCode, 0xFFFF)
	EndCode = append(EndCode, 0xFFFF)
	IDDelta = append(IDDelta, 0x0001)
	IDRangeOffsets = append(IDRangeOffsets, 0)

	segCount := len(StartCode)
	data.Length = uint16(2 * (8 + 4*(segCount+1) + len(GlyphIDArray)))
	data.SegCountX2 = uint16(2 * segCount)
	sel := bits.Len(uint(segCount))
	data.SearchRange = 1 << sel
	data.EntrySelector = uint16(sel - 1)
	data.RangeShift = data.SegCountX2 - data.SearchRange

	EndCode = append(EndCode, 0) // add the ReservedPad field here

	err := binary.Write(w, binary.BigEndian, data)
	if err != nil {
		return nil, err
	}
	for _, buf := range [][]uint16{EndCode, StartCode, IDDelta, IDRangeOffsets, GlyphIDArray} {
		err := binary.Write(w, binary.BigEndian, buf)
		if err != nil {
			return nil, err
		}
	}

	enc := func(ii ...font.GlyphID) []byte {
		var res []byte
		for _, i := range ii {
			res = append(res, g2c[i])
		}
		return res
	}

	return enc, nil
}
