package truetype

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode"

	"seehuhn.de/go/pdf/font"
)

type offsetsTable struct {
	ScalerType    uint32
	NumTables     uint16
	SearchRange   uint16
	EntrySelector uint16
	RangeShift    uint16
}

type tableRecord struct {
	Tag      uint32
	CheckSum uint32
	Offset   uint32
	Length   uint32
}

type maxpTableHead struct {
	Version   int32  //	0x00005000 or 0x00010000
	NumGlyphs uint16 //	the number of glyphs in the font
}

func (tt *Font) getMaxpInfo() (*maxpTableHead, error) {
	maxp := &maxpTableHead{}
	_, err := tt.readTableHead("maxp", maxp)
	if err != nil {
		return nil, err
	}
	if maxp.Version != 0x00005000 && maxp.Version != 0x00010000 {
		return nil, errors.New("unknown maxp version 0x" +
			strconv.FormatInt(int64(maxp.Version), 16))
	}
	return maxp, nil
}

type cmapTable struct {
	Header struct {
		Version   uint16
		NumTables uint16
	}
	EncodingRecords []cmapRecord
}

type cmapRecord struct {
	PlatformID     uint16
	EncodingID     uint16
	SubtableOffset uint32
}

func (tt *Font) getCmapInfo() (*cmapTable, *io.SectionReader, error) {
	cmap := &cmapTable{}
	cmapFd, err := tt.readTableHead("cmap", &cmap.Header)
	if err != nil {
		return nil, nil, err
	}

	cmap.EncodingRecords = make([]cmapRecord, cmap.Header.NumTables)
	err = binary.Read(cmapFd, binary.BigEndian, cmap.EncodingRecords)
	if err != nil {
		return nil, nil, err
	}

	return cmap, cmapFd, nil
}

func (ct *cmapTable) find(plat, enc uint16) *cmapRecord {
	for i := range ct.EncodingRecords {
		table := &ct.EncodingRecords[i]
		if table.PlatformID == plat && table.EncodingID == enc {
			return table
		}
	}
	return nil
}

func (tt *Font) load(fd *io.SectionReader, table *cmapRecord, i2r func(int) rune) (map[rune]font.GlyphIndex, error) {
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

type nameTableHeader struct {
	Format uint16 // table version number
	Count  uint16 // number of name records
	Offset uint16 // offset to the beginning of strings (bytes)
}

type nameTableRecord struct {
	PlatformID         uint16 // platform identifier code
	PlatformSpecificID uint16 // platform-specific encoding identifier
	LanguageID         uint16 // language identifier
	NameID             uint16 // name identifier
	Length             uint16 // name string length in bytes
	Offset             uint16 // name string offset in bytes
}

func (tt *Font) getFontName() (string, error) {
	nameHeader := &nameTableHeader{}
	nameFd, err := tt.readTableHead("name", nameHeader)
	if err != nil {
		return "", err
	}

	record := &nameTableRecord{}
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

type postTableHeader struct {
	Format             uint32 // Format of this table
	ItalicAngle        int32  // Italic angle in degrees
	UnderlinePosition  int16  // Underline position
	UnderlineThickness int16  // Underline thickness
	IsFixedPitch       uint32 // Font is monospaced; set to 1 if the font is monospaced and 0 otherwise (N.B., to maintain compatibility with older versions of the TrueType spec, accept any non-zero value as meaning that the font is monospaced)
	MinMemType42       uint32 // Minimum memory usage when a TrueType font is downloaded as a Type 42 font
	MaxMemType42       uint32 // Maximum memory usage when a TrueType font is downloaded as a Type 42 font
	MinMemType1        uint32 // Minimum memory usage when a TrueType font is downloaded as a Type 1 font
	MaxMemType1        uint32 // Maximum memory usage when a TrueType font is downloaded as a Type 1 font
}

type postTableInfo struct {
	ItalicAngle        float64 // TODO(voss): use the in-table representation here
	UnderlinePosition  int16
	UnderlineThickness int16
	IsFixedPitch       bool
}

func (tt *Font) getPostInfo() (*postTableInfo, error) {
	postHeader := &postTableHeader{}
	_, err := tt.readTableHead("post", postHeader)
	if err != nil {
		return nil, err
	}

	// TODO(voss): check the format
	// fmt.Printf("format = 0x%08X\n", postHeader.Format)

	// TODO(voss): make this more similar to the other functions in this file.
	res := &postTableInfo{
		ItalicAngle:        float64(postHeader.ItalicAngle) / 65536,
		UnderlinePosition:  postHeader.UnderlinePosition,
		UnderlineThickness: postHeader.UnderlineThickness,
		IsFixedPitch:       postHeader.IsFixedPitch != 0,
	}
	return res, nil
}

type headTable struct {
	Version            uint32 // 0x00010000 = version 1.0
	FontRevision       uint32 // set by font manufacturer
	CheckSumAdjustment uint32
	MagicNumber        uint32 // set to 0x5F0F3CF5
	Flags              uint16 //
	// bit 0 - y value of 0 specifies baseline
	// bit 1 - x position of left most black bit is LSB
	// bit 2 - scaled point size and actual point size will differ (i.e. 24 point glyph differs from 12 point glyph scaled by factor of 2)
	// bit 3 - use integer scaling instead of fractional
	// bit 4 - (used by the Microsoft implementation of the TrueType scaler)
	// bit 5 - This bit should be set in fonts that are intended to e laid out vertically, and in which the glyphs have been drawn such that an x-coordinate of 0 corresponds to the desired vertical baseline.
	// bit 6 - This bit must be set to zero.
	// bit 7 - This bit should be set if the font requires layout for correct linguistic rendering (e.g. Arabic fonts).
	// bit 8 - This bit should be set for an AAT font which has one or more metamorphosis effects designated as happening by default.
	// bit 9 - This bit should be set if the font contains any strong right-to-left glyphs.
	// bit 10 - This bit should be set if the font contains Indic-style rearrangement effects.
	// bits 11-13 - Defined by Adobe.
	// bit 14 - This bit should be set if the glyphs in the font are simply generic symbols for code point ranges, such as for a last resort font.
	UnitsPerEm uint16 // range from 64 to 16384
	Created    int64  // international date
	Modified   int64  // international date
	XMin       int16  // for all glyph bounding boxes
	YMin       int16  // for all glyph bounding boxes
	XMax       int16  // for all glyph bounding boxes
	YMax       int16  // for all glyph bounding boxes
	MacStyle   uint16
	// bit 0 bold
	// bit 1 italic
	// bit 2 underline
	// bit 3 outline
	// bit 4 shadow
	// bit 5 condensed (narrow)
	// bit 6 extended
	LowestRecPPEM     uint16 //	smallest readable size in pixels
	FontDirectionHint int16
	// 0 Mixed directional glyphs
	// 1 Only strongly left to right glyphs
	// 2 Like 1 but also contains neutrals
	// -1 Only strongly right to left glyphs
	// -2 Like -1 but also contains neutrals
	IndexToLocFormat int16 // 0 for short offsets, 1 for long
	GlyphDataFormat  int16 // 0 for current format
}

func (tt *Font) getHeadInfo() (*headTable, error) {
	head := &headTable{}
	_, err := tt.readTableHead("head", head)
	if err != nil {
		return nil, err
	}
	if head.MagicNumber != 0x5F0F3CF5 {
		return nil, errors.New("wrong magic number")
	}

	return head, nil
}

type hheaTable struct {
	Version             uint32 // 0x00010000 (1.0)
	Ascent              int16  // Distance from baseline of highest ascender
	Descent             int16  // Distance from baseline of lowest descender
	LineGap             int16  // typographic line gap
	AdvanceWidthMax     uint16 // must be consistent with horizontal metrics
	MinLeftSideBearing  int16  // must be consistent with horizontal metrics
	MinRightSideBearing int16  // must be consistent with horizontal metrics
	XMaxExtent          int16  // max(lsb + (xMax-xMin))
	CaretSlopeRise      int16  // used to calculate the slope of the caret (rise/run) set to 1 for vertical caret
	CaretSlopeRun       int16  // 0 for vertical
	CaretOffset         int16  // set value to 0 for non-slanted fonts
	_                   int16  // set value to 0
	_                   int16  // set value to 0
	_                   int16  // set value to 0
	_                   int16  // set value to 0
	MetricDataFormat    int16  // 0 for current format
	NumOfLongHorMetrics uint16 // number of advance widths in metrics table
}

func (tt *Font) getHHeaInfo() (*hheaTable, error) {
	hhea := &hheaTable{}
	_, err := tt.readTableHead("hhea", hhea)
	if err != nil {
		return nil, err
	}
	return hhea, nil
}

type hmtxTable struct {
	HMetrics        []longHorMetric
	LeftSideBearing []int16
}

type longHorMetric struct {
	AdvanceWidth    uint16
	LeftSideBearing int16
}

func (tt *Font) getHMtxInfo(NumOfLongHorMetrics uint16) (*hmtxTable, error) {
	hmtx := &hmtxTable{
		HMetrics:        make([]longHorMetric, NumOfLongHorMetrics),
		LeftSideBearing: make([]int16, tt.NumGlyphs-int(NumOfLongHorMetrics)),
	}
	fd, err := tt.readTableHead("hmtx", hmtx.HMetrics)
	if err != nil {
		return nil, err
	}
	err = binary.Read(fd, binary.BigEndian, hmtx.LeftSideBearing)
	if err != nil {
		return nil, err
	}
	return hmtx, nil
}

type os2Table struct {
	V0 struct {
		Version            uint16    // table version number (set to 0)
		AvgCharWidth       int16     // average weighted advance width of lower case letters and space
		WeightClass        uint16    // visual weight (degree of blackness or thickness) of stroke in glyphs
		WidthClass         uint16    // relative change from the normal aspect ratio (width to height ratio) as specified by a font designer for the glyphs in the font
		Type               int16     // characteristics and properties of this font (set undefined bits to zero)
		SubscriptXSize     int16     // recommended horizontal size in pixels for subscripts
		SubscriptYSize     int16     // recommended vertical size in pixels for subscripts
		SubscriptXOffset   int16     // recommended horizontal offset for subscripts
		SubscriptYOffset   int16     // recommended vertical offset form the baseline for subscripts
		SuperscriptXSize   int16     // recommended horizontal size in pixels for superscripts
		SuperscriptYSize   int16     // recommended vertical size in pixels for superscripts
		SuperscriptXOffset int16     // recommended horizontal offset for superscripts
		SuperscriptYOffset int16     // recommended vertical offset from the baseline for superscripts
		StrikeoutSize      int16     // width of the strikeout stroke
		StrikeoutPosition  int16     // position of the strikeout stroke relative to the baseline
		FamilyClass        int16     // classification of font-family design.
		Panose             [10]byte  // series of number used to describe the visual characteristics of a given typeface
		UnicodeRange       [4]uint32 // Field is split into two bit fields of 96 and 36 bits each. The low 96 bits are used to specify the Unicode blocks encompassed by the font file. The high 32 bits are used to specify the character or script sets covered by the font file. Bit assignments are pending. Set to 0
		VendID             [4]byte   // four character identifier for the font vendor
		Selection          uint16    // 2-byte bit field containing information concerning the nature of the font patterns
		FirstCharIndex     uint16    // The minimum Unicode index in this font.
		LastCharIndex      uint16    // The maximum Unicode index in this font.
	}
	V0MSValid bool
	V0MS      struct {
		TypoAscender  int16  // The typographic ascender for this font. This is not necessarily the same as the ascender value in the 'hhea' table.
		TypoDescender int16  // The typographic descender for this font. This is not necessarily the same as the descender value in the 'hhea' table.
		TypoLineGap   int16  // The typographic line gap for this font. This is not necessarily the same as the line gap value in the 'hhea' table.
		WinAscent     uint16 // The ascender metric for Windows. WinAscent is computed as the yMax for all characters in the Windows ANSI character set.
		WinDescent    uint16 // The descender metric for Windows. WinDescent is computed as the -yMin for all characters in the Windows ANSI character set.
	}
	V1 struct {
		CodePageRange1 uint32 // Bits 0-31
		CodePageRange2 uint32 // Bits 32-63
	}
	V4 struct {
		XHeight     int16  // The distance between the baseline and the approximate height of non-ascending lowercase letters measured in FUnits.
		CapHeight   int16  // The distance between the baseline and the approximate height of uppercase letters measured in FUnits.
		DefaultChar uint16 // The default character displayed by Windows to represent an unsupported character. (Typically this should be 0.)
		BreakChar   uint16 // The break character used by Windows.
		MaxContext  uint16 // The maximum length of a target glyph OpenType context for any feature in this font.
	}
	V5 struct {
		LowerPointSize uint16 // The lowest size (in twentieths of a typographic point), at which the font starts to be used. This is an inclusive value.
		UpperPointSize uint16 // The highest size (in twentieths of a typographic point), at which the font starts to be used. This is an exclusive value. Use 0xFFFFU to indicate no upper limit.
	}
}

func (tt *Font) getOS2Info() (*os2Table, error) {
	os2 := &os2Table{}
	os2Fd, err := tt.readTableHead("OS/2", &os2.V0)
	if err != nil {
		return nil, err
	}

	if os2.V0.Version > 0 || tt.tables["OS/2"].Length > 68 {
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

func (tt *Font) getKernInfo(q float64) (map[font.GlyphPair]int, error) {
	var Header struct {
		Version   uint16
		NumTables uint16
	}
	kernFd, err := tt.readTableHead("kern", &Header)
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

type glyfTable struct {
	Data []glyphHeader
	// actual glyph descriptions omitted
}

type glyphHeader struct {
	_    int16 // If the number of contours is greater than or equal to zero, this is a simple glyph. If negative, this is a composite glyph — the value -1 should be used for composite glyphs.
	XMin int16 // Minimum x for coordinate data.
	YMin int16 // Minimum y for coordinate data.
	XMax int16 // Maximum x for coordinate data.
	YMax int16 // Maximum y for coordinate data.
}

func (tt *Font) getGlyfInfo() (*glyfTable, error) {
	var err error
	offset := make([]uint32, tt.NumGlyphs+1)
	if tt.head.IndexToLocFormat == 0 {
		short := make([]uint16, tt.NumGlyphs+1)
		_, err = tt.readTableHead("loca", short)
		for i, x := range short {
			offset[i] = uint32(x)
		}
	} else {
		_, err = tt.readTableHead("loca", offset)
	}
	if err != nil {
		return nil, err
	}

	res := &glyfTable{
		Data: make([]glyphHeader, tt.NumGlyphs),
	}
	glyfFd, err := tt.readTableHead("glyf", nil)
	if err != nil {
		return nil, err
	}
	for i := 0; i < tt.NumGlyphs; i++ {
		_, err := glyfFd.Seek(int64(offset[i]), io.SeekStart)
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

func (tt *Font) readTableHead(name string, head interface{}) (*io.SectionReader, error) {
	table := tt.tables[name]
	if table == nil {
		return nil, errNoTable
	}
	tableFd := io.NewSectionReader(tt.fd, int64(table.Offset), int64(table.Length))

	if head != nil {
		err := binary.Read(tableFd, binary.BigEndian, head)
		if err != nil {
			return nil, err
		}
	}

	return tableFd, nil
}
