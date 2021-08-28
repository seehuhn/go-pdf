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

// ExportOptions provides options for the Font.Export() function.
type ExportOptions struct {
	Include map[string]bool // select a subset of tables
	Subset  []font.GlyphID  // select a subset of glyphs
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
	// debug, err := os.Create("debug.ttf")
	// if err != nil {
	// 	return 0, err
	// }
	// defer debug.Close()
	// w = io.MultiWriter(w, debug)

	if opt == nil {
		opt = &ExportOptions{}
	} else if opt.Subset != nil {
		opt.Include["cmap"] = true
	}
	tableNames := tt.selectTables(opt)

	replTab := make(map[string][]byte)
	replSum := make(map[string]uint32)

	indexToLocFormat := -1
	if opt.Subset != nil {
		includeOnly := []font.GlyphID{0}
		for _, origGid := range opt.Subset {
			if origGid != 0 {
				includeOnly = append(includeOnly, origGid)
			}
		}

		cmapBytes, err := makeSimpleCmap(opt.Subset)
		if err != nil {
			return 0, err
		}
		replTab["cmap"] = cmapBytes
		replSum["cmap"] = checksum(cmapBytes)

		subsetInfo, err := tt.makeSubset(includeOnly)
		if err != nil {
			return 0, err
		}
		replTab["glyf"] = subsetInfo.glyfBytes
		replSum["glyf"] = checksum(subsetInfo.glyfBytes)
		replTab["loca"] = subsetInfo.locaBytes
		replSum["loca"] = checksum(subsetInfo.locaBytes)
		indexToLocFormat = subsetInfo.indexToLocFormat
		replTab["hmtx"] = subsetInfo.hmtxBytes
		replSum["hmtx"] = checksum(subsetInfo.hmtxBytes)
		replTab["hhea"] = subsetInfo.hheaBytes
		replSum["hhea"] = checksum(subsetInfo.hheaBytes)

		// fix up numGlyphs
		maxpBytes, err := tt.Header.ReadTableBytes(tt.Fd, "maxp")
		if err != nil {
			return 0, err
		}
		binary.BigEndian.PutUint16(maxpBytes[4:6], subsetInfo.numGlyphs)
		replTab["maxp"] = maxpBytes
		replSum["maxp"] = checksum(maxpBytes)
	}

	hasHead := contains(tableNames, "head")
	if hasHead {
		// Copy and modify the "head" table.
		// https://docs.microsoft.com/en-us/typography/opentype/spec/head
		headTable := &table.Head{}
		*headTable = *tt.Head
		headTable.CheckSumAdjustment = 0
		ttZeroTime := time.Date(1904, time.January, 1, 0, 0, 0, 0, time.UTC)
		headTable.Modified = int64(time.Since(ttZeroTime).Seconds())
		if indexToLocFormat >= 0 {
			headTable.IndexToLocFormat = int16(indexToLocFormat)
		}

		buf := &bytes.Buffer{}
		_ = binary.Write(buf, binary.BigEndian, headTable)
		headBytes := buf.Bytes()
		replTab["head"] = headBytes
		replSum["head"] = checksum(headBytes)
	}

	var totalSize int64
	var totalSum uint32

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
	buf := &bytes.Buffer{}
	err := binary.Write(buf, binary.BigEndian, header.Offsets)
	if err != nil {
		return 0, err
	}
	err = binary.Write(buf, binary.BigEndian, header.Records)
	if err != nil {
		return 0, err
	}
	headerBytes := buf.Bytes()
	_, err = w.Write(headerBytes)
	if err != nil {
		return 0, err
	}
	totalSize += int64(len(headerBytes))
	totalSum += checksum(headerBytes)

	if hasHead {
		// fix the checksum in the "head" table
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
		totalSize += n
		if k := n % 4; k != 0 {
			l, err := w.Write(pad[:4-k])
			if err != nil {
				return 0, err
			}
			totalSize += int64(l)
		}
	}

	return totalSize, nil
}

type subsetInfo struct {
	numGlyphs        uint16
	glyfBytes        []byte
	locaBytes        []byte
	indexToLocFormat int // 0 for short offsets, 1 for long
	hmtxBytes        []byte
	hheaBytes        []byte
}

func (tt *Font) makeSubset(includeOnly []font.GlyphID) (*subsetInfo, error) {
	oldOffsets, err := tt.GetGlyfOffsets()
	if err != nil {
		return nil, err
	}

	glyfFd, err := tt.Header.ReadTableHead(tt.Fd, "glyf", nil)
	if err != nil {
		return nil, err
	}

	hheaInfo, err := tt.GetHHeaInfo()
	if err != nil {
		return nil, err
	}
	hmtx, err := tt.GetHMtxInfo(hheaInfo.NumOfLongHorMetrics)
	if err != nil {
		return nil, err
	}

	res := &subsetInfo{
		numGlyphs: uint16(len(includeOnly)),
	}

	// write the new "glyf" table
	var newOffsets []uint32
	var newHMetrics []table.LongHorMetric
	var advanceWidthMax uint16
	var minLeftSideBearing int16 = 32767
	buf := &bytes.Buffer{}
	for _, origGid := range includeOnly {
		start := oldOffsets[origGid]
		end := oldOffsets[origGid+1]
		length := end - start

		newOffsets = append(newOffsets, uint32(buf.Len()))
		if length > 0 {
			_, err = glyfFd.Seek(int64(start), io.SeekStart)
			if err != nil {
				return nil, err
			}
			_, err = io.CopyN(buf, glyfFd, int64(length))
			if err != nil {
				return nil, err
			}
		}

		advanceWidth := hmtx.GetAdvanceWidth(int(origGid))
		leftSideBearing := hmtx.GetLSB(int(origGid))
		newHMetrics = append(newHMetrics, table.LongHorMetric{
			AdvanceWidth:    advanceWidth,
			LeftSideBearing: leftSideBearing,
		})
		if advanceWidth > advanceWidthMax {
			advanceWidthMax = advanceWidth
		}
		if length > 0 && leftSideBearing < minLeftSideBearing {
			minLeftSideBearing = leftSideBearing
		}
	}
	newOffsets = append(newOffsets, uint32(buf.Len()))
	res.glyfBytes = buf.Bytes()

	// write the new "loca" table
	buf = &bytes.Buffer{}
	if buf.Len() < 1<<16 {
		res.indexToLocFormat = 0
		shortOffsets := make([]uint16, len(newOffsets))
		for i, offs := range newOffsets {
			shortOffsets[i] = uint16(offs / 2)
		}
		err = binary.Write(buf, binary.BigEndian, shortOffsets)
	} else {
		res.indexToLocFormat = 1
		err = binary.Write(buf, binary.BigEndian, newOffsets)
	}
	if err != nil {
		return nil, err
	}
	res.locaBytes = buf.Bytes()

	// write the new "hmtx" table
	n := len(newHMetrics)
	numOfLongHorMetrics := n
	for i := n - 1; i > 0; i-- {
		if newHMetrics[i] != newHMetrics[i-1] {
			break
		}
		numOfLongHorMetrics--
	}
	newLSB := make([]int16, n-numOfLongHorMetrics)
	for i, hm := range newHMetrics[numOfLongHorMetrics:] {
		newLSB[i] = hm.LeftSideBearing
	}
	buf = &bytes.Buffer{}
	err = binary.Write(buf, binary.BigEndian, newHMetrics[:numOfLongHorMetrics])
	if err != nil {
		return nil, err
	}
	err = binary.Write(buf, binary.BigEndian, newLSB)
	if err != nil {
		return nil, err
	}
	res.hmtxBytes = buf.Bytes()

	// write the new "hhea" table
	newHhea := &table.Hhea{}
	*newHhea = *hheaInfo // copy the old data
	newHhea.AdvanceWidthMax = advanceWidthMax
	newHhea.MinLeftSideBearing = minLeftSideBearing
	newHhea.NumOfLongHorMetrics = uint16(numOfLongHorMetrics)
	buf = &bytes.Buffer{}
	err = binary.Write(buf, binary.BigEndian, newHhea)
	if err != nil {
		return nil, err
	}
	res.hheaBytes = buf.Bytes()

	return res, nil
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

// Write a cmap with just a 1,0,4 subtable to map character indices to glyph
// indices in a subset, simple font.
func makeSimpleCmap(subset []font.GlyphID) ([]byte, error) {
	n := len(subset)
	if n > 256 {
		return nil, errors.New("too many characters for a simple font")
	}

	// Every non-zero entry in subset corresponds to a glyph included in the
	// subset.  In the subsetted font these glyphs will be numbered
	// consecutively, starting at glyph ID 1.  Thus, there are no contiguous
	// character code segments mapping to non-consecutive glyph IDs and we will
	// not need to use the glyphIdArray.

	var StartCode, EndCode, IDDelta, IDRangeOffsets []uint16
	prevC := 999 // impossible value
	newGid := uint16(0)
	for c, origGid := range subset {
		if origGid == 0 {
			continue
		}
		newGid++

		if c == prevC+1 {
			EndCode[len(EndCode)-1]++
		} else {
			c16 := uint16(c)
			StartCode = append(StartCode, c16)
			EndCode = append(EndCode, c16)
			IDDelta = append(IDDelta, newGid-c16)
			IDRangeOffsets = append(IDRangeOffsets, 0)
		}
		prevC = c
	}
	// add the required final segment
	StartCode = append(StartCode, 0xFFFF)
	EndCode = append(EndCode, 0xFFFF)
	IDDelta = append(IDDelta, 0x0001)
	IDRangeOffsets = append(IDRangeOffsets, 0)

	// Encode the data in the binary format described at
	// https://docs.microsoft.com/en-us/typography/opentype/spec/cmap#format-4-segment-mapping-to-delta-values
	data := &simpleCmapTableHead{
		NumTables:      1,
		PlatformID:     1,
		EncodingID:     0,
		SubtableOffset: 12,
		Format:         4,
	}
	segCount := len(StartCode)
	data.Length = uint16(2 * (8 + 4*segCount))
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
	for _, x := range [][]uint16{EndCode, StartCode, IDDelta, IDRangeOffsets} {
		err := binary.Write(buf, binary.BigEndian, x)
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}
