package truetype

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// offset subtable
type offsets struct {
	ScalerType    uint32
	NumTables     uint16
	SearchRange   uint16
	EntrySelector uint16
	RangeShift    uint16
}

type Header struct {
	ScalerType uint32
	Tables     map[string]*TableInfo
}

// table directory entry
type TableInfo struct {
	CheckSum uint32
	Offset   uint32
	Length   uint32
}

func ReadHeader(r io.Reader) (*Header, error) {
	offs := &offsets{}
	err := binary.Read(r, binary.BigEndian, offs)
	if err != nil {
		return nil, err
	}

	tag := offs.ScalerType
	if tag != 0x00010000 && tag != 0x4F54544F {
		return nil, errors.New("unsupported font type")
	}

	res := &Header{
		ScalerType: tag,
		Tables:     map[string]*TableInfo{},
	}
	for i := 0; i < int(offs.NumTables); i++ {
		var tag uint32
		err := binary.Read(r, binary.BigEndian, &tag)
		if err != nil {
			return nil, err
		}
		tagString := string([]byte{
			byte(tag >> 24),
			byte(tag >> 16),
			byte(tag >> 8),
			byte(tag)})
		info := &TableInfo{}
		err = binary.Read(r, binary.BigEndian, info)
		if err != nil {
			return nil, err
		}
		res.Tables[tagString] = info
	}

	return res, nil
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

func (header *Header) GetFontName(fd io.ReaderAt) (string, error) {
	info := header.Tables["name"]
	if info == nil {
		return "", errNoName
	}

	nameFd := io.NewSectionReader(fd, int64(info.Offset), int64(info.Length))

	nameHeader := &nameTableHeader{}
	err := binary.Read(nameFd, binary.BigEndian, nameHeader)
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
		case record.PlatformID == 1 && record.PlatformSpecificID == 0:
			nameFd.Seek(int64(nameHeader.Offset)+int64(record.Offset),
				io.SeekStart)
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
			nameFd.Seek(int64(nameHeader.Offset)+int64(record.Offset),
				io.SeekStart)
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

	return "", errNoName
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
	ItalicAngle        float64
	UnderlinePosition  int16
	UnderlineThickness int16
	IsFixedPitch       bool
}

func (header *Header) GetPostInfo(fd io.ReaderAt) (*postTableInfo, error) {
	info := header.Tables["post"]
	if info == nil {
		return nil, errors.New("missing post table")
	}

	postFd := io.NewSectionReader(fd, int64(info.Offset), int64(info.Length))

	postHeader := &postTableHeader{}
	err := binary.Read(postFd, binary.BigEndian, postHeader)
	if err != nil {
		return nil, err
	}

	// TODO(voss): check the format
	// fmt.Printf("format = 0x%08X\n", postHeader.Format)

	fmt.Println("xcv", postHeader.ItalicAngle)
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

func (header *Header) GetHeadInfo(fd io.ReaderAt) (*headTable, error) {
	info := header.Tables["head"]
	if info == nil {
		return nil, errors.New("missing head table")
	}
	headFd := io.NewSectionReader(fd, int64(info.Offset), int64(info.Length))

	head := &headTable{}
	err := binary.Read(headFd, binary.BigEndian, head)
	if err != nil {
		return nil, err
	}
	if head.MagicNumber != 0x5F0F3CF5 {
		return nil, errors.New("wrong magic number")
	}

	return head, nil
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

func (header *Header) GetOS2Info(fd io.ReaderAt) (*os2Table, error) {
	info := header.Tables["OS/2"]
	if info == nil {
		return nil, errors.New("missing head table")
	}
	os2Fd := io.NewSectionReader(fd, int64(info.Offset), int64(info.Length))

	os2 := &os2Table{}
	err := binary.Read(os2Fd, binary.BigEndian, &os2.V0)
	if err != nil {
		return nil, err
	}
	if os2.V0.Version > 0 || info.Length > 68 {
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
