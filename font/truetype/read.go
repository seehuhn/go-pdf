package truetype

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/truetype/table"
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

func (tt *Font) getCmapInfo() (*table.Cmap, *io.SectionReader, error) {
	cmap := &table.Cmap{}
	cmapFd, err := tt.Header.ReadTableHead(tt.Fd, "cmap", &cmap.Header)
	if err != nil {
		return nil, nil, err
	}

	cmap.EncodingRecords = make([]table.CmapRecord, cmap.Header.NumTables)
	err = binary.Read(cmapFd, binary.BigEndian, cmap.EncodingRecords)
	if err != nil {
		return nil, nil, err
	}

	return cmap, cmapFd, nil
}

func (tt *Font) load(fd *io.SectionReader, table *table.CmapRecord, i2r func(int) rune) (map[rune]font.GlyphIndex, error) {
	// The OpenType spec at
	// https://docs.microsoft.com/en-us/typography/opentype/spec/cmap
	// documents the following cmap subtable formats:
	//     Format 0: Byte encoding table
	//     Format 2: High-byte mapping through table
	//     Format 4: Segment mapping to delta values
	//     Format 6: Trimmed table mapping
	//     Format 8: mixed 16-bit and 32-bit coverage
	//     Format 10: Trimmed array
	//     Format 12: Segmented coverage
	//     Format 13: Many-to-one range mappings
	//     Format 14: Unicode Variation Sequences
	// For the *.ttf and *.otf files on my system, I have found the
	// following frequencies for these formats:
	//
	//     count | format
	//     ------+-----------
	//     21320 | Format 4
	//      5747 | Format 6
	//      3519 | Format 0
	//      3225 | Format 12
	//       143 | Format 2
	//       107 | Format 14
	//         4 | Format 13

	_, err := fd.Seek(int64(table.SubtableOffset), io.SeekStart)
	if err != nil {
		return nil, err
	}

	var format uint16
	err = binary.Read(fd, binary.BigEndian, &format)
	if err != nil {
		return nil, err
	}

	info := fmt.Sprintf("PlatID = %d, EncID = %d, Fmt = %d: ",
		table.PlatformID, table.EncodingID, format)

	cmap := make(map[rune]font.GlyphIndex)

	switch format {
	case 4: // Segment mapping to delta values
		type cmapFormat4 struct {
			Length        uint16
			Language      uint16
			SegCountX2    uint16
			SearchRange   uint16
			EntrySelector uint16
			RangeShift    uint16
		}
		data := &cmapFormat4{}
		err = binary.Read(fd, binary.BigEndian, data)
		if err != nil {
			return nil, err
		}
		if data.SegCountX2%2 != 0 {
			return nil, errors.New(info + "corrupted cmap subtable")
		}
		segCount := int(data.SegCountX2 / 2)
		buf := make([]uint16, 4*segCount+1)
		err = binary.Read(fd, binary.BigEndian, buf)
		if err != nil {
			return nil, err
		}
		startCode := buf[segCount+1 : 2*segCount+1]
		endCode := buf[:segCount]
		idDelta := buf[2*segCount+1 : 3*segCount+1]
		idRangeOffset := buf[3*segCount+1 : 4*segCount+1]
		glyphIDBase, err := fd.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, err
		}

		for k := 0; k < segCount; k++ {
			if idRangeOffset[k] == 0 {
				delta := idDelta[k]
				for idx := int(startCode[k]); idx <= int(endCode[k]); idx++ {
					c := int(uint16(idx) + delta)
					if c == 0 {
						continue
					}
					if c >= tt.NumGlyphs {
						return nil, errors.New(info + "glyph index " + strconv.Itoa(c) + " out of range")
					}
					r := i2r(idx)
					if unicode.IsGraphic(r) {
						cmap[r] = font.GlyphIndex(c)
					}
				}
			} else {
				d := int(idRangeOffset[k])/2 - (segCount - k)
				if d < 0 {
					return nil, errors.New(info + "corrupt cmap table")
				}
				tmp := make([]uint16, int(endCode[k]-startCode[k])+1)
				_, err = fd.Seek(glyphIDBase+2*int64(d), io.SeekStart)
				if err != nil {
					return nil, err
				}
				err = binary.Read(fd, binary.BigEndian, tmp)
				if err != nil {
					return nil, err
				}
				for idx := int(startCode[k]); idx <= int(endCode[k]); idx++ {
					c := int(tmp[int(idx)-int(startCode[k])])
					if c == 0 {
						continue
					}
					if c >= tt.NumGlyphs {
						return nil, errors.New(info + "glyph index " + strconv.Itoa(c) + " out of range")
					}
					r := i2r(idx)
					if unicode.IsGraphic(r) {
						cmap[r] = font.GlyphIndex(c)
					}
				}
			}
		}

	case 12: // Segmented coverage
		type cmapFormat12 struct {
			_         uint16 // reserved
			Length    uint32
			Language  uint32
			NumGroups uint32
		}
		data := &cmapFormat12{}
		err = binary.Read(fd, binary.BigEndian, data)
		if err != nil {
			return nil, err
		}

		type segment struct {
			StartCharCode uint32 //	First character code in this group
			EndCharCode   uint32 //	Last character code in this group
			StartGlyphID  uint32 //	Glyph index corresponding to the starting character code
		}
		for i := 0; i < int(data.NumGroups); i++ {
			seg := &segment{}
			err = binary.Read(fd, binary.BigEndian, seg)
			if err != nil {
				return nil, err
			}
			if seg.EndCharCode < seg.StartCharCode || seg.EndCharCode > 0x10FFFF {
				return nil, errors.New("invalid character code in font")
			}

			c := seg.StartGlyphID
			for idx := int(seg.StartCharCode); idx <= int(seg.EndCharCode); idx++ {
				r := i2r(idx)
				if unicode.IsGraphic(r) {
					cmap[r] = font.GlyphIndex(c)
				}
				c++
			}
		}

	default:
		return nil, errors.New(info + "unsupported cmap format " +
			strconv.Itoa(int(format)))
	}

	return cmap, nil
}

func (tt *Font) getFontName() (string, error) {
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

func (tt *Font) getPostInfo() (*table.PostInfo, error) {
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

func (tt *Font) getHeadInfo() (*table.Head, error) {
	head := &table.Head{}
	_, err := tt.Header.ReadTableHead(tt.Fd, "head", head)
	if err != nil {
		return nil, err
	}
	if head.MagicNumber != 0x5F0F3CF5 {
		return nil, errors.New("wrong magic number")
	}

	return head, nil
}

func (tt *Font) getHHeaInfo() (*table.Hhea, error) {
	hhea := &table.Hhea{}
	_, err := tt.Header.ReadTableHead(tt.Fd, "hhea", hhea)
	if err != nil {
		return nil, err
	}
	return hhea, nil
}

func (tt *Font) getHMtxInfo(NumOfLongHorMetrics uint16) (*table.Hmtx, error) {
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

func (tt *Font) getOS2Info() (*table.OS2, error) {
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

// read kerning information from the "kern" table
func (tt *Font) readKernInfo() (map[font.GlyphPair]int, error) {
	// factor for converting from TrueType FUnit to PDF glyph units
	q := 1000 / float64(tt.head.UnitsPerEm)

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
				font.GlyphIndex(buf[0]), font.GlyphIndex(buf[1])}
			kern := int16(buf[2])
			kerning[LR] = int(float64(kern)*q + 0.5)
			buf = buf[3:]
		}
	}

	return kerning, nil
}

func (tt *Font) getGlyfInfo() (*table.Glyf, error) {
	var err error
	offset := make([]uint32, tt.NumGlyphs+1)
	if tt.head.IndexToLocFormat == 0 {
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
	tableLen := tt.Header.Find("glyf").Length
	for i := 0; i < tt.NumGlyphs; i++ {
		offs := offset[i]
		if offs >= tableLen {
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

func readClassDefTable(r io.Reader) (map[font.GlyphIndex]uint16, error) {
	var format uint16
	err := binary.Read(r, binary.BigEndian, &format)
	if err != nil {
		return nil, err
	}

	res := make(map[font.GlyphIndex]uint16)
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
		base := font.GlyphIndex(firstGlyph)
		for i := 0; i < int(count); i++ {
			class := classValueArray[i]
			if class != 0 {
				res[base+font.GlyphIndex(i)] = class
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
			for idx := font.GlyphIndex(rec.StartGlyphID); idx <= font.GlyphIndex(rec.EndGlyphID); idx++ {
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

func (tt *Font) readGposLookups(langTag, scriptTag string) (*io.SectionReader, []*myLookupInfo, error) {
	GPOS := &table.GposHead{}
	fd, err := tt.Header.ReadTableHead(tt.Fd, "GPOS", &GPOS.V10)
	if err != nil {
		return nil, nil, err
	}
	if GPOS.V10.MajorVersion != 1 || GPOS.V10.MinorVersion > 1 {
		return nil, nil, fmt.Errorf("unsupported GPOS version %d.%d",
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

// readGposKernInfo reads kerning information from the "GPOS" table.
//
// A list of OpenType language tags is here:
// https://docs.microsoft.com/en-us/typography/opentype/spec/languagetags
//
// A list of OpenType script tags is here:
// https://docs.microsoft.com/en-us/typography/opentype/spec/scripttags
func (tt *Font) readGposKernInfo(langTag, scriptTag string) (map[font.GlyphPair]int, error) {
	// factor for converting from TrueType FUnit to PDF glyph units
	q := 1000 / float64(tt.head.UnitsPerEm)

	fd, allLookups, err := tt.readGposLookups(langTag, scriptTag)
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
				fmt.Println("  - PairPosFormat1 Subtable")
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
					a := font.GlyphIndex(coverage[k])
					for _, xi := range x.PairValueRecords {
						b := font.GlyphIndex(xi.SecondGlyph)
						// TODO(voss): scale this correctly
						d := int(float64(xi.ValueRecord1.XAdvance)*q + 0.5)
						res[font.GlyphPair{a, b}] = d
					}
				}
			case 2: // lookup type 2, format 2
				fmt.Println("  - PairPosFormat2 Subtable")
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
						c1 := classDef1[font.GlyphIndex(idx1)]
						c2 := classDef2[font.GlyphIndex(idx2)]
						k := c1*pairPos.Header.Class2Count + c2
						if int(k) >= len(pairPos.Records) {
							return nil, errors.New("GPOS/PairPos2: corrupt font")
						}
						rec := pairPos.Records[k]
						if rec.ValueRecord1 == nil || rec.ValueRecord1.XAdvance == 0 {
							continue
						}
						// fmt.Println(idx1, idx2, rec)
						a := font.GlyphIndex(idx1)
						b := font.GlyphIndex(idx2)
						d := int(float64(rec.ValueRecord1.XAdvance)*q + 0.5)
						res[font.GlyphPair{a, b}] = d
					}
				}
			default:
				fmt.Printf("  - unknown subtable format %d\n", format)
			}
		default:
			fmt.Printf("  - unknown lookup type %d\n", l.Type)
		}
	}

	return res, nil
}
