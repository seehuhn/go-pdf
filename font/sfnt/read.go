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
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/table"
)

// TODO(voss): add better protections against malicious font files

func (tt *Font) getMaxpInfo() (*table.MaxpHead, error) {
	maxp := &table.MaxpHead{}
	_, err := tt.Header.ReadTableHead(tt.Fd, "maxp", maxp)
	if err != nil {
		return nil, err
	}
	if maxp.Version != 0x00005000 && maxp.Version != 0x00010000 {
		return nil, errors.New("unknown maxp version 0x" +
			strconv.FormatInt(int64(maxp.Version), 16))
	}
	return maxp, nil
}

func (tt *Font) GetFontName() (string, error) {
	// TODO(voss): if FontName == "", invent a name: The name must be no
	// longer than 63 characters and restricted to the printable ASCII
	// subset, codes 33 to 126, except for the 10 characters '[', ']', '(',
	// ')', '{', '}', '<', '>', '/', '%'.

	nameHeader := &table.NameHeader{}
	nameFd, err := tt.Header.ReadTableHead(tt.Fd, "name", nameHeader)
	if err != nil {
		return "", err
	}

	record := &table.NameRecord{}
	for i := 0; i < int(nameHeader.Count); i++ {
		err := binary.Read(nameFd, binary.BigEndian, record)
		if err != nil {
			return "", err
		}
		if record.NameID != 6 {
			continue
		}

		switch {
		case record.PlatformID == 1 && record.PlatformSpecificID == 0 &&
			record.LanguageID == 0:
			_, err = nameFd.Seek(int64(nameHeader.Offset)+int64(record.Offset),
				io.SeekStart)
			if err != nil {
				return "", err
			}
			buf := make([]byte, record.Length)
			_, err := io.ReadFull(nameFd, buf)
			if err != nil {
				return "", err
			}
			rr := make([]rune, len(buf))
			for i, c := range buf {
				rr[i] = macintosh[c]
			}
			return string(rr), nil
		case record.PlatformID == 3 && record.PlatformSpecificID == 1:
			_, err = nameFd.Seek(int64(nameHeader.Offset)+int64(record.Offset),
				io.SeekStart)
			if err != nil {
				return "", err
			}
			buf := make([]uint16, record.Length/2)
			err := binary.Read(nameFd, binary.BigEndian, buf)
			if err != nil {
				return "", err
			}
			rr := make([]rune, len(buf))
			for i, c := range buf {
				rr[i] = rune(c)
			}
			return string(rr), nil
		}
	}

	return "", errors.New("no usable font name found")
}

func (tt *Font) GetPostInfo() (*table.PostInfo, error) {
	postHeader := &table.PostHeader{}
	_, err := tt.Header.ReadTableHead(tt.Fd, "post", postHeader)
	if err != nil {
		return nil, err
	}

	// TODO(voss): check the format
	// fmt.Printf("format = 0x%08X\n", postHeader.Format)

	// TODO(voss): make this more similar to the other functions in this file.
	res := &table.PostInfo{
		ItalicAngle:        float64(postHeader.ItalicAngle) / 65536,
		UnderlinePosition:  postHeader.UnderlinePosition,
		UnderlineThickness: postHeader.UnderlineThickness,
		IsFixedPitch:       postHeader.IsFixedPitch != 0,
	}
	return res, nil
}

func (tt *Font) GetHHeaInfo() (*table.Hhea, error) {
	hhea := &table.Hhea{}
	_, err := tt.Header.ReadTableHead(tt.Fd, "hhea", hhea)
	if err != nil {
		return nil, err
	}
	return hhea, nil
}

func (tt *Font) GetHMtxInfo(NumOfLongHorMetrics uint16) (*table.Hmtx, error) {
	hmtx := &table.Hmtx{
		HMetrics:        make([]table.LongHorMetric, NumOfLongHorMetrics),
		LeftSideBearing: make([]int16, tt.NumGlyphs-int(NumOfLongHorMetrics)),
	}
	fd, err := tt.Header.ReadTableHead(tt.Fd, "hmtx", hmtx.HMetrics)
	if err != nil {
		return nil, err
	}
	err = binary.Read(fd, binary.BigEndian, hmtx.LeftSideBearing)
	if err != nil {
		return nil, err
	}
	return hmtx, nil
}

func (tt *Font) GetOS2Info() (*table.OS2, error) {
	os2 := &table.OS2{}
	os2Fd, err := tt.Header.ReadTableHead(tt.Fd, "OS/2", &os2.V0)
	if err != nil {
		return nil, err
	}

	if os2.V0.Version > 0 || tt.Header.Find("OS/2").Length > 68 {
		os2.V0MSValid = true
		err := binary.Read(os2Fd, binary.BigEndian, &os2.V0MS)
		if err != nil {
			return nil, err
		}
	}
	if os2.V0.Version >= 1 {
		err := binary.Read(os2Fd, binary.BigEndian, &os2.V1)
		if err != nil {
			return nil, err
		}
	}
	if os2.V0.Version >= 4 {
		err := binary.Read(os2Fd, binary.BigEndian, &os2.V4)
		if err != nil {
			return nil, err
		}
	}
	if os2.V0.Version >= 5 {
		err := binary.Read(os2Fd, binary.BigEndian, &os2.V5)
		if err != nil {
			return nil, err
		}
	}
	return os2, nil
}

// ReadKernInfo reads kerning information from the "kern" table.
func (tt *Font) ReadKernInfo() (map[font.GlyphPair]int, error) {
	// factor for converting from TrueType FUnit to PDF glyph units
	q := 1000 / float64(tt.Head.UnitsPerEm)

	var Header struct {
		Version   uint16
		NumTables uint16
	}
	kernFd, err := tt.Header.ReadTableHead(tt.Fd, "kern", &Header)
	if err != nil {
		return nil, err
	} else if Header.Version != 0 {
		return nil, errors.New("unsupported kern table version")
	}

	kerning := make(map[font.GlyphPair]int)
	for i := 0; i < int(Header.NumTables); i++ {
		var subHeader struct {
			Version  uint16
			Length   uint16
			Coverage uint16
		}
		err = binary.Read(kernFd, binary.BigEndian, &subHeader)
		if err != nil {
			return nil, err
		}
		if subHeader.Version != 0 ||
			subHeader.Coverage != 1 ||
			subHeader.Length < 6+8 ||
			subHeader.Length%2 != 0 {
			// skip unsupported and mal-formed subtables
			_, err = io.CopyN(io.Discard, kernFd, int64(subHeader.Length-6))
			if err != nil {
				return nil, err
			}
			continue
		}

		buf := make([]uint16, (subHeader.Length-6)/2)
		err = binary.Read(kernFd, binary.BigEndian, buf)
		if err != nil {
			return nil, err
		}

		nPairs := int(buf[0])
		buf = buf[4:] // skip the header
		for nPairs > 0 && len(buf) >= 3 {
			LR := font.GlyphPair{
				font.GlyphID(buf[0]), font.GlyphID(buf[1])}
			kern := int16(buf[2])
			kerning[LR] = int(float64(kern)*q + 0.5)
			buf = buf[3:]
		}
	}

	return kerning, nil
}

func (tt *Font) GetGlyfInfo() (*table.Glyf, error) {
	var err error
	offset := make([]uint32, tt.NumGlyphs+1)
	if tt.Head.IndexToLocFormat == 0 {
		short := make([]uint16, tt.NumGlyphs+1)
		_, err = tt.Header.ReadTableHead(tt.Fd, "loca", short)
		for i, x := range short {
			offset[i] = uint32(x) * 2
		}
	} else {
		_, err = tt.Header.ReadTableHead(tt.Fd, "loca", offset)
	}
	if err != nil {
		return nil, err
	}

	res := &table.Glyf{
		Data: make([]table.GlyphHeader, tt.NumGlyphs),
	}
	glyfFd, err := tt.Header.ReadTableHead(tt.Fd, "glyf", nil)
	if err != nil {
		return nil, err
	}
	for i := 0; i < tt.NumGlyphs; i++ {
		offs := offset[i]
		if offs == offset[i+1] {
			continue
		}
		_, err := glyfFd.Seek(int64(offs), io.SeekStart)
		if err != nil {
			return nil, err
		}
		err = binary.Read(glyfFd, binary.BigEndian, &res.Data[i])
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func readClassDefTable(r io.Reader) (map[font.GlyphID]uint16, error) {
	var format uint16
	err := binary.Read(r, binary.BigEndian, &format)
	if err != nil {
		return nil, err
	}

	res := make(map[font.GlyphID]uint16)
	switch format {
	case 1:
		var firstGlyph uint16
		err := binary.Read(r, binary.BigEndian, &firstGlyph)
		if err != nil {
			return nil, err
		}
		var count uint16
		err = binary.Read(r, binary.BigEndian, &count)
		if err != nil {
			return nil, err
		}
		classValueArray := make([]uint16, count)
		err = binary.Read(r, binary.BigEndian, classValueArray)
		if err != nil {
			return nil, err
		}
		base := font.GlyphID(firstGlyph)
		for i := 0; i < int(count); i++ {
			class := classValueArray[i]
			if class != 0 {
				res[base+font.GlyphID(i)] = class
			}
		}
	case 2:
		var count uint16
		err := binary.Read(r, binary.BigEndian, &count)
		if err != nil {
			return nil, err
		}
		var rec table.ClassRangeRecord
		for i := uint16(0); i < count; i++ {
			err := binary.Read(r, binary.BigEndian, &rec)
			if err != nil {
				return nil, err
			}
			for idx := font.GlyphID(rec.StartGlyphID); idx <= font.GlyphID(rec.EndGlyphID); idx++ {
				if rec.Class != 0 {
					res[idx] = rec.Class
				}
			}
		}
	default:
		return nil, fmt.Errorf("unsupported ClassDef table format %d", format)
	}
	return res, nil
}

type myLookupInfo struct {
	Tag  string
	Type uint16
	Flag uint16
	Pos  int64
}

// A list of OpenType language tags is here:
// https://docs.microsoft.com/en-us/typography/opentype/spec/languagetags
//
// A list of OpenType script tags is here:
// https://docs.microsoft.com/en-us/typography/opentype/spec/scripttags
func (tt *Font) readGtabLookups(tableName string, langTag, scriptTag string) (*io.SectionReader, []*myLookupInfo, error) {
	GPOS := &table.GposHead{}
	fd, err := tt.Header.ReadTableHead(tt.Fd, tableName, &GPOS.V10)
	if err != nil {
		return nil, nil, err
	}
	if GPOS.V10.MajorVersion != 1 || GPOS.V10.MinorVersion > 1 {
		return nil, nil, fmt.Errorf("sfnt/"+tableName+": unsupported version %d.%d",
			GPOS.V10.MajorVersion, GPOS.V10.MinorVersion)
	}
	if GPOS.V10.MinorVersion > 0 {
		err = binary.Read(fd, binary.BigEndian, &GPOS.V11)
		if err != nil {
			return nil, nil, err
		}
	}

	featureList, err := GPOS.ReadFeatureInfo(fd, langTag, scriptTag)
	if err != nil {
		return nil, nil, err
	}

	lookupBase := int64(GPOS.V10.LookupListOffset)
	_, err = fd.Seek(lookupBase, io.SeekStart)
	if err != nil {
		return nil, nil, err
	}
	lookupList, err := table.ReadLookupList(fd)
	if err != nil {
		return nil, nil, err
	}

	var allLookups []*myLookupInfo
	for _, feature := range featureList {
		for _, idx := range feature.LookupListIndices {
			lookupTableBase := lookupBase + int64(lookupList.LookupOffsets[idx])
			_, err = fd.Seek(lookupTableBase, io.SeekStart)
			if err != nil {
				return nil, nil, err
			}
			lookupTable, err := table.ReadLookup(fd)
			if err != nil {
				return nil, nil, err
			}

			lookupType := lookupTable.Header.LookupType
			if lookupType != 9 {
				for _, offs := range lookupTable.SubtableOffsets {
					allLookups = append(allLookups, &myLookupInfo{
						Tag:  feature.Tag,
						Type: lookupType,
						Flag: lookupTable.Header.LookupFlag,
						Pos:  lookupTableBase + int64(offs),
					})
				}
			} else {
				for _, offs := range lookupTable.SubtableOffsets {
					extensionPosTableBase := lookupTableBase + int64(offs)
					_, err = fd.Seek(extensionPosTableBase, io.SeekStart)
					if err != nil {
						return nil, nil, err
					}
					extPos, err := table.ReadExtensionPos1(fd)
					if err != nil {
						return nil, nil, err
					}
					if extPos.PosFormat != 1 {
						return nil, nil, fmt.Errorf(
							"Extension Positioning Subtable format %d not supported",
							extPos.PosFormat)
					}
					allLookups = append(allLookups, &myLookupInfo{
						Tag:  feature.Tag,
						Type: extPos.ExtensionLookupType,
						Flag: lookupTable.Header.LookupFlag,
						Pos:  extensionPosTableBase + int64(extPos.ExtensionOffset),
					})
				}
			}
		}
	}
	return fd, allLookups, nil
}

func (tt *Font) ReadGsubLigInfo(langTag, scriptTag string) (map[font.GlyphPair]int, error) {
	fd, allLookups, err := tt.readGtabLookups("GSUB", langTag, scriptTag)
	if err != nil {
		return nil, err
	}

	for _, l := range allLookups {
		// https://docs.microsoft.com/en-us/typography/opentype/spec/features_ae#ccmp
		if l.Tag != "ccmp" {
			continue
		}

		switch l.Type {
		case 6:
			_, err := fd.Seek(l.Pos, io.SeekStart)
			if err != nil {
				return nil, err
			}
			var format uint16
			err = binary.Read(fd, binary.BigEndian, &format)
			if err != nil {
				return nil, err
			}
			switch format {
			default:
				fmt.Printf("  - unsupported GSUB lookup type 6.%d\n", format)
			}
		default:
			fmt.Printf("  - unsupported GSUB lookup type %d\n", l.Type)
		}

		// TODO(voss): also handle "liga"
	}

	return nil, nil
}

// ReadGposKernInfo reads kerning information from the "GPOS" table.
func (tt *Font) ReadGposKernInfo(langTag, scriptTag string) (map[font.GlyphPair]int, error) {
	fd, allLookups, err := tt.readGtabLookups("GPOS", langTag, scriptTag)
	if err != nil {
		return nil, err
	}

	// TODO(voss): In lookupTable.Header.LookupFlag,
	// IGNORE_BASE_GLYPHS, IGNORE_LIGATURES, or IGNORE_MARKS refer to
	// base glyphs, ligatures and marks as defined in the Glyph Class
	// Definition Table in the GDEF table.  If any of these flags are
	// set, a Glyph Class Definition Table must be present. If any of
	// these bits is set, then lookups must ignore glyphs of the
	// respective type; that is, the other glyphs must be processed
	// just as though these glyphs were not present.

	// factor for converting from TrueType FUnit to PDF glyph units
	q := 1000 / float64(tt.Head.UnitsPerEm)

	res := make(map[font.GlyphPair]int)
	for _, l := range allLookups {
		if l.Tag != "kern" {
			continue
		}
		switch l.Type {
		case 2:
			pairPosTableBase := l.Pos
			_, err = fd.Seek(pairPosTableBase, io.SeekStart)
			if err != nil {
				return nil, err
			}
			var format uint16
			err = binary.Read(fd, binary.BigEndian, &format)
			if err != nil {
				return nil, err
			}
			switch format {
			case 1: // lookup type 2, format 1
				pairPos, err := table.ReadPairPosFormat1(fd)
				if err != nil {
					return nil, err
				}

				_, err = fd.Seek(pairPosTableBase+int64(pairPos.Header.CoverageOffset),
					io.SeekStart)
				if err != nil {
					return nil, err
				}
				coverage, err := table.ReadCoverage(fd)
				if err != nil {
					return nil, err
				}
				if len(coverage) != int(pairPos.Header.PairSetCount) {
					return nil, errors.New("GPOS/PairPos1: corrupted PairPos table")
				}

				for k, offs := range pairPos.PairSetOffsets {
					_, err = fd.Seek(pairPosTableBase+int64(offs), io.SeekStart)
					if err != nil {
						return nil, err
					}
					x, err := table.ReadPairSet(fd,
						pairPos.Header.ValueFormat1,
						pairPos.Header.ValueFormat2)
					if err != nil {
						return nil, err
					}
					// fmt.Println("      -", coverage[k], x)
					a := font.GlyphID(coverage[k])
					for _, xi := range x.PairValueRecords {
						b := font.GlyphID(xi.SecondGlyph)
						// TODO(voss): scale this correctly
						d := int(float64(xi.ValueRecord1.XAdvance)*q + 0.5)
						res[font.GlyphPair{a, b}] = d
					}
				}
			case 2: // lookup type 2, format 2
				pairPos, err := table.ReadPairPosFormat2(fd)
				if err != nil {
					return nil, err
				}

				_, err = fd.Seek(pairPosTableBase+int64(pairPos.Header.CoverageOffset),
					io.SeekStart)
				if err != nil {
					return nil, err
				}
				firstGlyphs, err := table.ReadCoverage(fd)
				if err != nil {
					return nil, err
				}

				offs := pairPos.Header.ClassDef1Offset
				_, err = fd.Seek(pairPosTableBase+int64(offs), io.SeekStart)
				if err != nil {
					return nil, err
				}
				classDef1, err := readClassDefTable(fd)
				if err != nil {
					return nil, err
				}

				offs = pairPos.Header.ClassDef2Offset
				_, err = fd.Seek(pairPosTableBase+int64(offs), io.SeekStart)
				if err != nil {
					return nil, err
				}
				classDef2, err := readClassDefTable(fd)
				if err != nil {
					return nil, err
				}

				for _, idx1 := range firstGlyphs {
					for idx2 := 0; idx2 < tt.NumGlyphs; idx2++ {
						c1 := classDef1[font.GlyphID(idx1)]
						c2 := classDef2[font.GlyphID(idx2)]
						k := c1*pairPos.Header.Class2Count + c2
						if int(k) >= len(pairPos.Records) {
							return nil, errors.New("GPOS/PairPos2: corrupt font")
						}
						rec := pairPos.Records[k]
						if rec.ValueRecord1 == nil || rec.ValueRecord1.XAdvance == 0 {
							continue
						}
						// fmt.Println(idx1, idx2, rec)
						a := font.GlyphID(idx1)
						b := font.GlyphID(idx2)
						d := int(float64(rec.ValueRecord1.XAdvance)*q + 0.5)
						res[font.GlyphPair{a, b}] = d
					}
				}
			default:
				fmt.Printf("  - unknown subtable format %d\n", format)
			}
		default:
			fmt.Printf("  - unsupported GPOS lookup type %d\n", l.Type)
		}
	}

	return res, nil
}
