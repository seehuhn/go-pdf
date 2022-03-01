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
	"io"
	"math/bits"
	"sort"
	"time"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/head"
	"seehuhn.de/go/pdf/font/sfnt/table"
)

// ExportOptions provides options for the Font.Export() function.
type ExportOptions struct {
	IncludeTables map[string]bool   // include a subset of tables
	Replace       map[string][]byte // replace a table with a custom one
	IncludeGlyphs []font.GlyphID    // include a subset of glyphs
	SubsetMapping []font.CMapEntry  // include a generated cmap table
	ModTime       time.Time
}

// Export writes the font to the io.Writer w.
func (tt *Font) Export(w io.Writer, opt *ExportOptions) (int64, error) {
	var includeTables map[string]bool
	var includeGlyphs []font.GlyphID
	var subsetMapping []font.CMapEntry
	var modTime time.Time
	if opt != nil {
		includeTables = opt.IncludeTables
		includeGlyphs = opt.IncludeGlyphs
		subsetMapping = opt.SubsetMapping
		modTime = opt.ModTime
	}

	var subsetInfo *subsetInfo
	tableData := make(map[string][]byte)
	if includeGlyphs != nil {
		var err error
		subsetInfo, err = tt.getSubsetInfo(includeGlyphs)
		if err != nil {
			return 0, err
		}
		for name, blob := range subsetInfo.blobs {
			tableData[name] = blob
		}
		// fix up numGlyphs
		maxpBytes, err := tt.Header.ReadTableBytes(tt.Fd, "maxp")
		if err != nil {
			return 0, err
		}
		binary.BigEndian.PutUint16(maxpBytes[4:6], subsetInfo.numGlyphs)
		tableData["maxp"] = maxpBytes
	}

	if subsetMapping != nil && (includeTables == nil || includeTables["cmap"]) {
		cmapBytes, err := makeCMap(subsetMapping)
		if err != nil {
			return 0, err
		}
		tableData["cmap"] = cmapBytes
	}

	if includeTables["hhea"] || includeTables["hmtx"] {
		info := tt.HmtxInfo
		if subsetInfo != nil {
			i2 := *info
			info = &i2

			info.Width = subsetInfo.Width
			info.LSB = subsetInfo.LSB
			info.GlyphExtent = subsetInfo.GlyphExtent
		}
		hhea, hmtx := info.Encode()
		tableData["hhea"] = hhea
		tableData["hmtx"] = hmtx
	}

	tableNames := tt.selectTables(includeTables)

	if contains(tableNames, "head") {
		// write the head table
		// https://docs.microsoft.com/en-us/typography/opentype/spec/head
		if !modTime.IsZero() {
			tt.HeadInfo.Modified = modTime
		}

		if subsetInfo != nil {
			// TODO(voss): don't modify the original struct
			tt.HeadInfo.FontBBox.LLx = subsetInfo.xMin
			tt.HeadInfo.FontBBox.LLy = subsetInfo.yMin
			tt.HeadInfo.FontBBox.URx = subsetInfo.xMax
			tt.HeadInfo.FontBBox.URy = subsetInfo.yMax
			tt.HeadInfo.HasLongOffsets = subsetInfo.indexToLocFormat != 0
		}

		var err error
		tableData["head"], err = tt.HeadInfo.Encode()
		if err != nil {
			return 0, err
		}
	}

	if opt.Replace != nil {
		for name, repl := range opt.Replace {
			tableData[name] = repl
		}
	}

	for _, name := range tableNames {
		if _, ok := tableData[name]; !ok {
			blob, err := tt.Header.ReadTableBytes(tt.Fd, name)
			if err != nil {
				return 0, err
			}
			tableData[name] = blob
		}
	}

	return WriteTables(w, tt.Header.Offsets.ScalerType, tableNames, tableData)
}

type subsetInfo struct {
	numGlyphs uint16
	blobs     map[string][]byte

	// fields for the "hhea" and "hmtx" tables
	Width       []uint16
	GlyphExtent []font.Rect
	LSB         []int16

	// fields for the "head" table
	xMin             int16 // TODO(voss): are these needed?
	yMin             int16
	xMax             int16
	yMax             int16
	indexToLocFormat int16 // 0 for short offsets, 1 for long
}

func (tt *Font) getSubsetInfo(includeOnly []font.GlyphID) (*subsetInfo, error) {
	if includeOnly[0] != 0 {
		panic("missing .notdef glyph")
	}

	origNumGlyphs := tt.NumGlyphs()

	res := &subsetInfo{
		blobs: make(map[string][]byte),
		xMin:  32767,
		yMin:  32767,
		xMax:  -32768,
		yMax:  -32768,
	}

	res.Width = make([]uint16, len(includeOnly))
	if tt.HmtxInfo.LSB != nil {
		res.LSB = make([]int16, len(includeOnly))
	}
	if tt.HmtxInfo.GlyphExtent != nil {
		res.GlyphExtent = make([]font.Rect, len(includeOnly))
	}
	for i, gid := range includeOnly {
		res.Width[i] = tt.HmtxInfo.Width[gid]
		if tt.HmtxInfo.LSB != nil {
			res.LSB[i] = tt.HmtxInfo.LSB[gid]
		}
		if tt.HmtxInfo.GlyphExtent != nil {
			res.GlyphExtent[i] = tt.HmtxInfo.GlyphExtent[gid]
		}
	}

	origOffsets, err := tt.GetGlyfOffsets(origNumGlyphs)
	if err != nil {
		return nil, err
	}

	glyfFd, err := tt.GetTableReader("glyf", nil)
	if err != nil {
		return nil, err
	}

	// write the new "glyf" table
	var newOffsets []uint32
	buf := &bytes.Buffer{}
	newGid := 0
	// Elements may be appended to includeOnly during the loop, so we don't
	// use a range loop here.
	for newGid < len(includeOnly) {
		newOffsets = append(newOffsets, uint32(buf.Len()))

		origGid := includeOnly[newGid]
		start := origOffsets[origGid]
		end := origOffsets[origGid+1]
		length := end - start

		if length > 0 {
			_, err = glyfFd.Seek(int64(start), io.SeekStart)
			if err != nil {
				return nil, err
			}
			glyphHeader := &table.GlyphHeader{}
			err = binary.Read(glyfFd, binary.BigEndian, glyphHeader)
			if err != nil {
				return nil, err
			}
			err = binary.Write(buf, binary.BigEndian, glyphHeader)
			if err != nil {
				return nil, err
			}

			if glyphHeader.XMin < res.xMin {
				res.xMin = glyphHeader.XMin
			}
			if glyphHeader.YMin < res.yMin {
				res.yMin = glyphHeader.YMin
			}
			if glyphHeader.XMax > res.xMax {
				res.xMax = glyphHeader.XMax
			}
			if glyphHeader.YMax > res.yMax {
				res.yMax = glyphHeader.YMax
			}

			todo := int64(length) - 10
			if glyphHeader.NumberOfContours < 0 { // composite glyph
				// https://docs.microsoft.com/en-us/typography/opentype/spec/glyf#composite-glyph-description
				for {
					var compHead struct {
						Flags      uint16
						GlyphIndex uint16
					}
					err = binary.Read(glyfFd, binary.BigEndian, &compHead)
					if err != nil {
						return nil, err
					}

					// map the component gid to the new scheme
					origComponentGid := font.GlyphID(compHead.GlyphIndex)
					newComponentGid := -1
					for i, gid := range includeOnly {
						if gid == origComponentGid {
							newComponentGid = i
							break
						}
					}
					if newComponentGid < 0 {
						newComponentGid = len(includeOnly)
						includeOnly = append(includeOnly, origComponentGid)
					}
					compHead.GlyphIndex = uint16(newComponentGid)

					err = binary.Write(buf, binary.BigEndian, &compHead)
					if err != nil {
						return nil, err
					}
					todo -= 4

					if compHead.Flags&0x0020 == 0 { // no more components
						break
					}

					skip := int64(0)
					if compHead.Flags&0x0001 != 0 { // ARG_1_AND_2_ARE_WORDS
						skip += 4
					} else {
						skip += 2
					}
					if compHead.Flags&0x0008 != 0 { // WE_HAVE_A_SCALE
						skip += 2
					} else if compHead.Flags&0x0040 != 0 { // WE_HAVE_AN_X_AND_Y_SCALE
						skip += 4
					} else if compHead.Flags&0x0080 != 0 { // WE_HAVE_A_TWO_BY_TWO
						skip += 8
					}
					_, err = io.CopyN(buf, glyfFd, skip)
					if err != nil {
						return nil, err
					}
					todo -= skip
				}
			}

			_, err = io.CopyN(buf, glyfFd, todo)
			if err != nil {
				return nil, err
			}

			for length%4 != 0 {
				buf.WriteByte(0)
				length++
			}
		}

		newGid++
	}
	glyphEnd := buf.Len()
	newOffsets = append(newOffsets, uint32(glyphEnd))
	res.numGlyphs = uint16(len(includeOnly))
	res.blobs["glyf"] = buf.Bytes()

	// write the new "loca" table
	buf = &bytes.Buffer{}
	if glyphEnd < 1<<16 { // TODO(voss): should this be glyphEnd/2?
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
	res.blobs["loca"] = buf.Bytes()

	return res, nil
}

func (tt *Font) selectTables(include map[string]bool) []string {
	var names []string
	done := make(map[string]bool)

	// Fix the order of tables in the body of the files.
	// https://docs.microsoft.com/en-us/typography/opentype/spec/recom#optimized-table-ordering
	var candidates []string
	if tt.Header.Find("CFF ") != nil && (include == nil || include["CFF "]) {
		candidates = []string{
			"head", "hhea", "maxp", "OS/2", "name", "cmap", "post", "CFF ",
		}
	} else {
		candidates = []string{
			"head", "hhea", "maxp", "OS/2", "hmtx", "LTSH", "VDMX", "hdmx", "cmap",
			"fpgm", "prep", "cvt ", "loca", "glyf", "kern", "name", "post", "gasp",
		}
	}
	for _, name := range candidates {
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

func contains(ss []string, s string) bool {
	for _, si := range ss {
		if si == s {
			return true
		}
	}
	return false
}

// WriteTables writes an sfnt file containing the given tables.
// This changes the checksum in the "head" table in place.
func WriteTables(w io.Writer, scalerType uint32, tableNames []string, tableData map[string][]byte) (int64, error) {
	numTables := len(tableNames)

	// prepare the header
	sel := bits.Len(uint(numTables)) - 1
	offsets := &table.Offsets{
		ScalerType:    scalerType,
		NumTables:     uint16(numTables),
		SearchRange:   1 << (sel + 4),
		EntrySelector: uint16(sel),
		RangeShift:    uint16(16 * (numTables - 1<<sel)),
	}

	// temporarily clear the checksum in the "head" table
	if headData, ok := tableData["head"]; ok {
		head.ClearChecksum(headData)
	}

	var totalSum uint32
	offset := uint32(12 + 16*numTables)
	records := make([]table.Record, numTables)
	for i, name := range tableNames {
		body := tableData[name]
		length := uint32(len(body))
		checksum := Checksum(body)

		records[i].Tag = table.MakeTag(name)
		records[i].CheckSum = checksum
		records[i].Offset = offset
		records[i].Length = length

		totalSum += checksum
		offset += 4 * ((length + 3) / 4)
	}
	sort.Slice(records, func(i, j int) bool {
		return bytes.Compare(records[i].Tag[:], records[j].Tag[:]) < 0
	})

	buf := &bytes.Buffer{}
	binary.Write(buf, binary.BigEndian, offsets)
	binary.Write(buf, binary.BigEndian, records)
	headerBytes := buf.Bytes()
	totalSum += Checksum(headerBytes)

	// set the final checksum in the "head" table
	if headData, ok := tableData["head"]; ok {
		head.PatchChecksum(headData, totalSum)
	}

	// write the tables
	var totalSize int64
	n, err := w.Write(headerBytes)
	totalSize += int64(n)
	if err != nil {
		return totalSize, err
	}
	var pad [3]byte
	for _, name := range tableNames {
		body := tableData[name]
		n, err := w.Write(body)
		totalSize += int64(n)
		if err != nil {
			return totalSize, err
		}
		if k := n % 4; k != 0 {
			l, err := w.Write(pad[:4-k])
			totalSize += int64(l)
			if err != nil {
				return totalSize, err
			}
		}
	}
	return totalSize, nil
}
