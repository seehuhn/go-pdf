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

package table

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"unicode"

	"seehuhn.de/go/pdf/font"
)

// Header describes the start of a TrueType/OpenType file.  The structure
// contains information required to access the tables in the file.
type Header struct {
	Offsets Offsets
	Records []Record
}

// The Offsets sub-table forms the first part of Header.
type Offsets struct {
	ScalerType    uint32
	NumTables     uint16
	SearchRange   uint16
	EntrySelector uint16
	RangeShift    uint16
}

// A Record is part of the file Header.  It contains data about a single sfnt
// table.
type Record struct {
	Tag      Tag
	CheckSum uint32
	Offset   uint32
	Length   uint32
}

// ReadHeader reads the file header of an sfnt font file.
func ReadHeader(r io.Reader) (*Header, error) {
	res := &Header{}
	err := binary.Read(r, binary.BigEndian, &res.Offsets)
	if err != nil {
		return nil, err
	}

	scalerType := res.Offsets.ScalerType
	if scalerType != 0x00010000 && scalerType != 0x4F54544F {
		return nil, fmt.Errorf("unsupported sfnt type 0x%8X", scalerType)
	}
	if res.Offsets.NumTables > 280 {
		// the largest value observed on my laptop is 28
		return nil, errors.New("too many sfnt tables in font")
	}

	res.Records = make([]Record, res.Offsets.NumTables)
	err = binary.Read(r, binary.BigEndian, &res.Records)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// Find returns the table record for the table with the given name.
// If no such table exists, nil is returned.
func (h *Header) Find(name string) *Record {
	for i := 0; i < int(h.Offsets.NumTables); i++ {
		if h.Records[i].Tag.String() == name {
			return &h.Records[i]
		}
	}
	return nil
}

// ReadTableHead can be used to read the initial, fixed-size portion of a sfnt
// table,  It returns an io.SectionReader which can be used to read the rest of
// the table data.
func (h *Header) ReadTableHead(r io.ReaderAt, name string, head interface{}) (*io.SectionReader, error) {
	table := h.Find(name)
	if table == nil {
		return nil, &ErrNoTable{name}
	}
	tableFd := io.NewSectionReader(r, int64(table.Offset), int64(table.Length))

	if head != nil {
		err := binary.Read(tableFd, binary.BigEndian, head)
		if err != nil {
			return nil, err
		}
	}

	return tableFd, nil
}

// ReadTableBytes returns the body of a sfnt table.
func (h *Header) ReadTableBytes(r io.ReaderAt, name string) ([]byte, error) {
	table := h.Find(name)
	if table == nil {
		return nil, &ErrNoTable{name}
	}

	buf := make([]byte, table.Length)
	_, err := r.ReadAt(buf, int64(table.Offset))
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// --------------------------------------------------------------------------

// The Head table contains global information about a font.
type Head struct {
	Version            uint32 // 0x00010000 = version 1.0
	FontRevision       uint32 // set by font manufacturer
	CheckSumAdjustment uint32
	MagicNumber        uint32 // set to 0x5F0F3CF5

	// bit 0 - y value of 0 specifies baseline
	// bit 1 - x position of left most black bit is LSB
	// bit 2 - scaled point size and actual point size will differ (i.e. 24 point glyph differs from 12 point glyph scaled by factor of 2)
	// bit 3 - use integer scaling instead of fractional
	// bit 4 - (used by the Microsoft implementation of the TrueType scaler)
	// bit 5 - This bit should be set in fonts that are intended to be laid out vertically, and in which the glyphs have been drawn such that an x-coordinate of 0 corresponds to the desired vertical baseline.
	// bit 6 - This bit must be set to zero.
	// bit 7 - This bit should be set if the font requires layout for correct linguistic rendering (e.g. Arabic fonts).
	// bit 8 - This bit should be set for an AAT font which has one or more metamorphosis effects designated as happening by default.
	// bit 9 - This bit should be set if the font contains any strong right-to-left glyphs.
	// bit 10 - This bit should be set if the font contains Indic-style rearrangement effects.
	// bits 11-13 - Defined by Adobe.
	// bit 14 - This bit should be set if the glyphs in the font are simply generic symbols for code point ranges, such as for a last resort font.
	Flags uint16

	UnitsPerEm uint16 // range from 64 to 16384

	// Number of seconds since 12:00 midnight that started January 1st 1904 in
	// GMT/UTC time zone.
	Created  int64
	Modified int64

	XMin int16 // for all glyph bounding boxes
	YMin int16 // for all glyph bounding boxes
	XMax int16 // for all glyph bounding boxes
	YMax int16 // for all glyph bounding boxes

	// bit 0 bold
	// bit 1 italic
	// bit 2 underline
	// bit 3 outline
	// bit 4 shadow
	// bit 5 condensed (narrow)
	// bit 6 extended
	MacStyle uint16

	LowestRecPPEM uint16 //	smallest readable size in pixels

	// Deprecated (Set to 2).
	// 0 Mixed directional glyphs
	// 1 Only strongly left to right glyphs
	// 2 Like 1 but also contains neutrals
	// -1 Only strongly right to left glyphs
	// -2 Like -1 but also contains neutrals
	FontDirectionHint int16

	IndexToLocFormat int16 // 0 for short offsets, 1 for long
	GlyphDataFormat  int16 // 0 for current format
}

// --------------------------------------------------------------------------

// Cmap is the Character To Glyph Index Mapping Table.
type Cmap struct {
	Header struct {
		Version   uint16
		NumTables uint16
	}
	EncodingRecords []EncodingRecord
}

// ReadCmapTable reads the binary representation of a "cmap" table.
func ReadCmapTable(r io.ReadSeeker) (*Cmap, error) {
	cmap := &Cmap{}
	err := binary.Read(r, binary.BigEndian, &cmap.Header)
	if err != nil {
		return nil, err
	}

	if cmap.Header.NumTables > 100 {
		// The largest number of cmap tables in the fonts on my laptop is 9.
		return nil, errors.New("sfnt/cmap: too many encoding records")
	}

	cmap.EncodingRecords = make([]EncodingRecord, cmap.Header.NumTables)
	err = binary.Read(r, binary.BigEndian, cmap.EncodingRecords)
	if err != nil {
		return nil, err
	}

	return cmap, nil
}

// Find locates an encoding record in the cmap table.
func (ct *Cmap) Find(plat, enc uint16) *EncodingRecord {
	for i := range ct.EncodingRecords {
		table := &ct.EncodingRecords[i]
		if table.PlatformID == plat && table.EncodingID == enc {
			return table
		}
	}
	return nil
}

// An EncodingRecord specifies a particular encoding and the offset to the
// subtable for this encoding.
type EncodingRecord struct {
	PlatformID     uint16 // Platform ID.
	EncodingID     uint16 // Platform-specific encoding ID.
	SubtableOffset uint32 // Byte offset from beginning of table to the subtable for this encoding.
}

type cmapFormat4 struct {
	// Format uint16 omitted
	Length        uint16
	Language      uint16
	SegCountX2    uint16
	SearchRange   uint16
	EntrySelector uint16
	RangeShift    uint16
}

type cmapFormat12 struct {
	// Format uint16 omitted
	_         uint16 // reserved
	Length    uint32
	Language  uint32
	NumGroups uint32
}

// used for cmap formats 8 and 12
type sequentialMapGroup struct {
	StartCharCode uint32 //	First character code in this group
	EndCharCode   uint32 //	Last character code in this group
	StartGlyphID  uint32 //	Glyph index corresponding to the starting character code
}

// LoadCmap reads a mapping from unicode runes to glyph indeces from a "cmap"
// table encoding record.
// The function does NOT check that glyph indices are valid.
func (encRec *EncodingRecord) LoadCmap(r io.ReadSeeker, i2r func(int) rune) (map[rune]font.GlyphID, error) {
	// The OpenType spec at
	// https://docs.microsoft.com/en-us/typography/opentype/spec/cmap
	// documents the following cmap subtable formats:
	//
	//     Format 0: Byte encoding table
	//     Format 2: High-byte mapping through table
	//     Format 4: Segment mapping to delta values
	//     Format 6: Trimmed table mapping
	//     Format 8: mixed 16-bit and 32-bit coverage
	//     Format 10: Trimmed array
	//     Format 12: Segmented coverage
	//     Format 13: Many-to-one range mappings
	//     Format 14: Unicode Variation Sequences
	//
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

	_, err := r.Seek(int64(encRec.SubtableOffset), io.SeekStart)
	if err != nil {
		return nil, err
	}

	var format uint16
	err = binary.Read(r, binary.BigEndian, &format)
	if err != nil {
		return nil, err
	}

	errPfx := fmt.Sprintf("sfnt/cmap/%d,%d,%d: ",
		encRec.PlatformID, encRec.EncodingID, format)

	cmap := make(map[rune]font.GlyphID)

	switch format {
	case 4: // Segment mapping to delta values
		data := &cmapFormat4{}
		err = binary.Read(r, binary.BigEndian, data)
		if err != nil {
			return nil, err
		}

		// Read endCode, reservedPad, startCode, idDelta, and idRangeOffsets.
		// Since all of these are (arrays of) uint16 values, we just use a
		// single read command.
		if data.SegCountX2%2 != 0 {
			return nil, errors.New(errPfx + "table corrupted")
		}
		segCount := int(data.SegCountX2 / 2)
		if segCount > 100_000 {
			// fonts on my system have up to around 10,000 segments
			return nil, errors.New(errPfx + "too many segments")
		}
		buf := make([]uint16, 4*segCount+1)
		err = binary.Read(r, binary.BigEndian, buf)
		if err != nil {
			return nil, err
		}
		endCode := buf[:segCount]
		// reservedPad omitted
		startCode := buf[segCount+1 : 2*segCount+1]
		idDelta := buf[2*segCount+1 : 3*segCount+1]
		idRangeOffset := buf[3*segCount+1 : 4*segCount+1]
		glyphIDBase, err := r.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, err
		}

		total := 0
		for k := 0; k < segCount; k++ {
			a := int(startCode[k])
			b := int(endCode[k])
			if b < a {
				return nil, errors.New(errPfx + "table corrupted")
			}
			total += b - a + 1
			if total > 500_000 {
				// fonts on my system have up to around 50,000 mappings
				return nil, errors.New(errPfx + "too many mappings")
			}

			if idRangeOffset[k] == 0 {
				delta := idDelta[k]
				for idx := a; idx <= b; idx++ {
					c := int(uint16(idx) + delta)
					if c == 0 {
						continue
					}
					r := i2r(idx)
					if unicode.IsGraphic(r) {
						cmap[r] = font.GlyphID(c)
					}
				}
			} else {
				d := int(idRangeOffset[k])/2 - (segCount - k)
				if d < 0 {
					return nil, errors.New(errPfx + "table corrupted")
				}
				tmp := make([]uint16, b-a+1)
				_, err = r.Seek(glyphIDBase+2*int64(d), io.SeekStart)
				if err != nil {
					return nil, err
				}
				err = binary.Read(r, binary.BigEndian, tmp)
				if err != nil {
					return nil, err
				}
				for idx := a; idx <= b; idx++ {
					c := int(tmp[idx-a])
					if c == 0 {
						continue
					}
					r := i2r(idx)
					if unicode.IsGraphic(r) {
						cmap[r] = font.GlyphID(c)
					}
				}
			}
		}

	case 12: // Segmented coverage
		data := &cmapFormat12{}
		err = binary.Read(r, binary.BigEndian, data)
		if err != nil {
			return nil, err
		}
		if data.NumGroups > 200_000 {
			// fonts on my system have up to around 20,000 groups
			return nil, errors.New(errPfx + "too many groups")
		}

		total := 0
		for i := 0; i < int(data.NumGroups); i++ {
			seg := &sequentialMapGroup{}
			err = binary.Read(r, binary.BigEndian, seg)
			if err != nil {
				return nil, err
			}

			a := int(seg.StartCharCode)
			b := int(seg.EndCharCode)
			if b < a || b > 0x10FFFF {
				return nil, errors.New(errPfx + "invalid character code")
			}
			total += b - a + 1
			if total > 500_000 {
				// fonts on my system have up to around 50,000 mappings
				return nil, errors.New(errPfx + "too many mappings")
			}

			c := font.GlyphID(seg.StartGlyphID)
			for idx := a; idx <= b; idx++ {
				r := i2r(idx)
				if unicode.IsGraphic(r) {
					cmap[r] = c
				}
				c++
			}
		}

	default:
		return nil, fmt.Errorf("%sunsupported cmap format %d", errPfx, format)
	}

	return cmap, nil
}

// --------------------------------------------------------------------------

type MaxpHead struct {
	Version   int32  //	0x00005000 or 0x00010000
	NumGlyphs uint16 //	the number of glyphs in the font
}

type NameHeader struct {
	Format uint16 // table version number
	Count  uint16 // number of name records
	Offset uint16 // offset to the beginning of strings (bytes)
}

type NameRecord struct {
	PlatformID         uint16 // platform identifier code
	PlatformSpecificID uint16 // platform-specific encoding identifier
	LanguageID         uint16 // language identifier
	NameID             uint16 // name identifier
	Length             uint16 // name string length in bytes
	Offset             uint16 // name string offset in bytes
}

type PostHeader struct {
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

type PostInfo struct {
	ItalicAngle        float64 // TODO(voss): use the in-table representation here
	UnderlinePosition  int16
	UnderlineThickness int16
	IsFixedPitch       bool
}

type Hhea struct {
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

type Hmtx struct {
	HMetrics        []LongHorMetric
	LeftSideBearing []int16
}

type LongHorMetric struct {
	AdvanceWidth    uint16
	LeftSideBearing int16
}

// GetAdvanceWidth returns the advance width of a glyph, in font design units.
func (h *Hmtx) GetAdvanceWidth(gid int) uint16 {
	if gid >= len(h.HMetrics) {
		return h.HMetrics[len(h.HMetrics)-1].AdvanceWidth
	}
	return h.HMetrics[gid].AdvanceWidth
}

// GetLSB returns the left side bearing width of a glyph, in font design units.
func (h *Hmtx) GetLSB(gid int) int16 {
	if gid < len(h.HMetrics) {
		return h.HMetrics[gid].LeftSideBearing
	}
	gid -= len(h.HMetrics)
	if gid < len(h.LeftSideBearing) {
		return h.LeftSideBearing[gid]
	}
	return 0
}

type OS2 struct {
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

type Glyf struct {
	Data []GlyphHeader
	// actual glyph descriptions omitted
}

type GlyphHeader struct {
	_    int16 // If the number of contours is greater than or equal to zero, this is a simple glyph. If negative, this is a composite glyph — the value -1 should be used for composite glyphs.
	XMin int16 // Minimum x for coordinate data.
	YMin int16 // Minimum y for coordinate data.
	XMax int16 // Maximum x for coordinate data.
	YMax int16 // Maximum y for coordinate data.
}

// Tag represents a tag string composed of 4 ASCII bytes
type Tag [4]byte

func (tag Tag) String() string {
	return string(tag[:])
}
