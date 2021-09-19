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
	"seehuhn.de/go/pdf/font/sfnt/table"
)

// ExportOptions provides options for the Font.Export() function.
type ExportOptions struct {
	Include map[string]bool // select a subset of tables
	Cid2Gid []font.GlyphID  // select a subset of glyphs
}

// Export writes the font to the io.Writer w.
func (tt *Font) Export(w io.Writer, opt *ExportOptions) (int64, error) {
	if opt == nil {
		opt = &ExportOptions{}
	} else if opt.Cid2Gid != nil {
		opt.Include["cmap"] = true
	}
	tableNames := tt.selectTables(opt)

	replTab := make(map[string][]byte)

	var subsetInfo *subsetInfo
	if opt.Cid2Gid != nil {
		var includeOnly []font.GlyphID
		includeOnly = append(includeOnly, 0) // always include ".notdef"
		for _, origGid := range opt.Cid2Gid {
			if origGid != 0 {
				includeOnly = append(includeOnly, origGid)
			}
		}

		cmapBytes, err := MakeCMap(opt.Cid2Gid)
		if err != nil {
			return 0, err
		}
		replTab["cmap"] = cmapBytes

		subsetInfo, err = tt.getSubsetInfo(includeOnly)
		if err != nil {
			return 0, err
		}
		for name, blob := range subsetInfo.blobs {
			replTab[name] = blob
		}
		// fix up numGlyphs
		maxpBytes, err := tt.Header.ReadTableBytes(tt.Fd, "maxp")
		if err != nil {
			return 0, err
		}
		binary.BigEndian.PutUint16(maxpBytes[4:6], subsetInfo.numGlyphs)
		replTab["maxp"] = maxpBytes
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
		if subsetInfo != nil {
			headTable.XMin = subsetInfo.xMin
			headTable.YMin = subsetInfo.yMin
			headTable.XMax = subsetInfo.xMax
			headTable.YMax = subsetInfo.yMax
			headTable.IndexToLocFormat = int16(subsetInfo.indexToLocFormat)
		}

		buf := &bytes.Buffer{}
		_ = binary.Write(buf, binary.BigEndian, headTable)
		headBytes := buf.Bytes()
		replTab["head"] = headBytes
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
		var newChecksum uint32
		var length uint32
		if body, ok := replTab[name]; ok {
			newChecksum = Checksum(body)
			length = uint32(len(body))
		} else {
			newChecksum = old.CheckSum
			length = old.Length
		}
		header.Records[i].CheckSum = newChecksum
		header.Records[i].Length = length
		totalSum += newChecksum
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
	totalSum += Checksum(headerBytes)

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
	numGlyphs uint16
	blobs     map[string][]byte

	// fields for the "head" table
	xMin             int16
	yMin             int16
	xMax             int16
	yMax             int16
	indexToLocFormat int16 // 0 for short offsets, 1 for long
}

func (tt *Font) getSubsetInfo(includeOnly []font.GlyphID) (*subsetInfo, error) {
	// TODO(voss): make better use of the data stored in tt.

	NumGlyphs := len(tt.Width)

	origOffsets, err := tt.getGlyfOffsets(NumGlyphs)
	if err != nil {
		return nil, err
	}

	hheaInfo, err := tt.getHHeaInfo()
	if err != nil {
		return nil, err
	}
	hmtx, err := tt.getHMtxInfo(NumGlyphs, int(hheaInfo.NumOfLongHorMetrics))
	if err != nil {
		return nil, err
	}

	glyfFd, err := tt.Header.ReadTableHead(tt.Fd, "glyf", nil)
	if err != nil {
		return nil, err
	}

	res := &subsetInfo{
		blobs: make(map[string][]byte),
		xMin:  32767,
		yMin:  32767,
		xMax:  -32768,
		yMax:  -32768,
	}

	// write the new "glyf" table
	var newOffsets []uint32
	var newHMetrics []table.LongHorMetric
	var advanceWidthMax uint16
	var minLeftSideBearing int16 = 32767
	var minRightSideBearing int16 = 32767
	var xMaxExtent int16
	buf := &bytes.Buffer{}
	newGid := 0
	for newGid < len(includeOnly) {
		newOffsets = append(newOffsets, uint32(buf.Len()))

		origGid := includeOnly[newGid]
		start := origOffsets[origGid]
		end := origOffsets[origGid+1]
		length := end - start

		advanceWidth := hmtx.GetAdvanceWidth(int(origGid))
		if advanceWidth > advanceWidthMax {
			advanceWidthMax = advanceWidth
		}

		leftSideBearing := hmtx.GetLSB(int(origGid))
		if length > 0 && leftSideBearing < minLeftSideBearing {
			minLeftSideBearing = leftSideBearing
		}

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

			xExtent := (int16(leftSideBearing) + glyphHeader.XMax - glyphHeader.XMin)
			if xExtent > xMaxExtent {
				xMaxExtent = xExtent
			}
			rightSideBearing := int16(advanceWidth) - xExtent
			if rightSideBearing < minRightSideBearing {
				minRightSideBearing = rightSideBearing
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
					origComponetGid := font.GlyphID(compHead.GlyphIndex)
					newComponentGid := -1
					for i, gid := range includeOnly {
						if gid == origComponetGid {
							newComponentGid = i
							break
						}
					}
					if newComponentGid < 0 {
						newComponentGid = len(includeOnly)
						includeOnly = append(includeOnly, origComponetGid)
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

		newHMetrics = append(newHMetrics, table.LongHorMetric{
			AdvanceWidth:    advanceWidth,
			LeftSideBearing: leftSideBearing,
		})

		newGid++
	}
	glyphEnd := buf.Len()
	newOffsets = append(newOffsets, uint32(glyphEnd))
	res.numGlyphs = uint16(len(includeOnly))
	res.blobs["glyf"] = buf.Bytes()

	// write the new "loca" table
	buf = &bytes.Buffer{}
	if glyphEnd < 1<<16 {
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
	res.blobs["hmtx"] = buf.Bytes()

	// write the new "hhea" table
	newHhea := &table.Hhea{}
	*newHhea = *hheaInfo // copy the old data
	newHhea.AdvanceWidthMax = advanceWidthMax
	newHhea.MinLeftSideBearing = minLeftSideBearing
	newHhea.MinRightSideBearing = minRightSideBearing
	newHhea.XMaxExtent = xMaxExtent
	newHhea.NumOfLongHorMetrics = uint16(numOfLongHorMetrics)
	buf = &bytes.Buffer{}
	err = binary.Write(buf, binary.BigEndian, newHhea)
	if err != nil {
		return nil, err
	}
	res.blobs["hhea"] = buf.Bytes()

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

func contains(ss []string, s string) bool {
	for _, si := range ss {
		if si == s {
			return true
		}
	}
	return false
}
