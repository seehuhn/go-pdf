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
	// bit 5 - This bit should be set in fonts that are intended to e laid out vertically, and in which the glyphs have been drawn such that an x-coordinate of 0 corresponds to the desired vertical baseline.
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

// A scriptList consists of a count of the scripts represented by the
// glyphs in the font and an array of records, one for each script for which
// the font defines script-specific features (a script without script-specific
// features does not need a ScriptRecord).
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#script-list-table-and-script-record
//
// Deprecated: convert to use sfnt/parser instead.
type scriptList struct {
	ScriptCount   uint16         // Number of ScriptRecords
	ScriptRecords []scriptRecord // Array of ScriptRecords, listed alphabetically by script tag
}

// A scriptRecord consists of a ScriptTag that identifies a script, and an
// offset into a Script table.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#script-list-table-and-script-record
//
// Deprecated: convert to use sfnt/parser instead.
type scriptRecord struct {
	ScriptTag    Tag    // Script tag identifier
	ScriptOffset uint16 // Offset to Script table, from beginning of ScriptList
}

// readScriptRecord returns the ScriptRecord with the given script tag.  If no
// record for tag is found, return the default record instead (or nil, of no
// default record is found).
//
// Deprecated: convert to use sfnt/parser instead.
func readScriptRecord(fd io.Reader, scriptTag string) (*scriptRecord, error) {
	data := &scriptList{}
	err := binary.Read(fd, binary.BigEndian, &data.ScriptCount)
	if err != nil {
		return nil, err
	}
	data.ScriptRecords = make([]scriptRecord, data.ScriptCount)
	err = binary.Read(fd, binary.BigEndian, data.ScriptRecords)
	if err != nil {
		return nil, err
	}

	var dfltScript *scriptRecord
	for i := range data.ScriptRecords {
		rec := &data.ScriptRecords[i]
		if i == 0 {
			// in case there is no default, use the first script record
			dfltScript = rec
		}
		switch rec.ScriptTag.String() {
		case scriptTag:
			return rec, nil
		case "DFLT":
			dfltScript = rec
		}
	}
	return dfltScript, nil
}

// script identifies each language system that defines how to use the
// glyphs in a script for a particular language.  It also references a default
// language system that defines how to use the script’s glyphs in the absence
// of language-specific knowledge.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#script-table-and-language-system-record
//
// Deprecated: convert to use sfnt/parser instead.
type script struct {
	Header struct {
		DefaultLangSysOffset uint16 // Offset to default LangSys table, from beginning of Script table — may be NULL
		LangSysCount         uint16 // Number of LangSysRecords for this script — excluding the default LangSys
	}
	LangSysRecords []langSysRecord // Array of LangSysRecords, listed alphabetically by LangSys tag
}

// A langSysRecord defines each language system (excluding the default) with an
// identification tag (LangSysTag) and an offset to a Language System table
// (LangSys).
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#script-table-and-language-system-record
//
// Deprecated: convert to use sfnt/parser instead.
type langSysRecord struct {
	LangSysTag    Tag    // LangSysTag identifier
	LangSysOffset uint16 // Offset to LangSys table, from beginning of Script table
}

// Deprecated: convert to use sfnt/parser instead.
type langSys struct {
	Header struct {
		_                    uint16 // (reserved for an offset to a reordering table)
		RequiredFeatureIndex uint16 // Index of a feature required for this language system; if no required features = 0xFFFF
		FeatureIndexCount    uint16 // Number of feature index values for this language system — excludes the required feature
	}
	FeatureIndices []uint16 // Array of indices into the FeatureList, in arbitrary order
}

// A featureList enumerates features in an array of records and specifies
// the total number of features.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#feature-list-table
//
// Deprecated: convert to use sfnt/parser instead.
type featureList struct {
	FeatureCount   uint16          // Number of FeatureRecords in this table
	FeatureRecords []FeatureRecord // Array of FeatureRecords — zero-based (first feature has FeatureIndex = 0), listed alphabetically by feature tag
}

// A FeatureRecord consists of a FeatureTag that identifies the feature and an
// offset to a Feature table.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#feature-list-table
//
// Deprecated: convert to use sfnt/parser instead.
type FeatureRecord struct {
	FeatureTag    Tag    // feature identification tag
	FeatureOffset uint16 // Offset to Feature table, from beginning of FeatureList
}

// Deprecated: convert to use sfnt/parser instead.
type Feature struct {
	Header struct {
		FeatureParamsOffset uint16 // Offset from start of Feature table to FeatureParams table, if defined for the feature and present, else NULL
		LookupIndexCount    uint16 // Number of LookupList indices for this feature
	}
	LookupListIndices []uint16 // Array of indices into the LookupList — zero-based (first lookup is LookupListIndex = 0)
}

// The LookupList table contains an array of offsets to Lookup tables.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#lookup-list-table
//
// Deprecated: convert to use sfnt/parser instead.
type LookupList struct {
	LookupCount   uint16   // Number of lookups in this table
	LookupOffsets []uint16 // Array of offsets to Lookup tables, from beginning of LookupList — zero based (first lookup is Lookup index = 0)
}

// ReadLookupList reads the binary representation of a LookupList
//
// Deprecated: convert to use sfnt/parser instead.
func ReadLookupList(r io.Reader) (*LookupList, error) {
	res := &LookupList{}
	err := binary.Read(r, binary.BigEndian, &res.LookupCount)
	if err != nil {
		return nil, err
	}

	res.LookupOffsets = make([]uint16, res.LookupCount)
	err = binary.Read(r, binary.BigEndian, res.LookupOffsets)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// Deprecated: convert to use sfnt/parser instead.
type Lookup struct {
	Header struct {
		LookupType    uint16 // Different enumerations for GSUB and GPOS
		LookupFlag    uint16 // Lookup qualifiers
		SubtableCount uint16 // Number of subtables for this lookup
	}
	SubtableOffsets  []uint16 // Array of offsets to lookup subtables, from beginning of Lookup table
	MarkFilteringSet uint16   // Index (base 0) into GDEF mark glyph sets structure. This field is only present if the USE_MARK_FILTERING_SET lookup flag is set.
}

// Deprecated: convert to use sfnt/parser instead.
func ReadLookup(r io.Reader) (*Lookup, error) {
	res := &Lookup{}
	err := binary.Read(r, binary.BigEndian, &res.Header)
	if err != nil {
		return nil, err
	}

	res.SubtableOffsets = make([]uint16, res.Header.SubtableCount)
	err = binary.Read(r, binary.BigEndian, &res.SubtableOffsets)
	if err != nil {
		return nil, err
	}

	if res.Header.LookupFlag&0x0010 != 0 {
		err = binary.Read(r, binary.BigEndian, &res.MarkFilteringSet)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

// Deprecated: convert to use sfnt/parser instead.
type ExtensionPosFormat1 struct {
	PosFormat           uint16 // Format identifier: format = 1
	ExtensionLookupType uint16 // Lookup type of subtable referenced by extensionOffset (i.e. the extension subtable).
	ExtensionOffset     uint32 // Offset to the extension subtable, of lookup type extensionLookupType, relative to the start of the ExtensionPosFormat1 subtable.
}

// Deprecated: convert to use sfnt/parser instead.
func ReadExtensionPos1(r io.Reader) (*ExtensionPosFormat1, error) {
	res := &ExtensionPosFormat1{}
	err := binary.Read(r, binary.BigEndian, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Deprecated: convert to use sfnt/parser instead.
type Coverage []uint16

// Deprecated: convert to use sfnt/parser instead.
func ReadCoverage(r io.Reader) (Coverage, error) {
	var version uint16
	err := binary.Read(r, binary.BigEndian, &version)
	if err != nil {
		return nil, err
	}

	var count uint16
	err = binary.Read(r, binary.BigEndian, &count)
	if err != nil {
		return nil, err
	}

	var res Coverage
	switch version {
	case 1:
		res = make(Coverage, count)
		err = binary.Read(r, binary.BigEndian, res)
		if err != nil {
			return nil, err
		}
	case 2:
		for i := 0; i < int(count); i++ {
			var buf struct {
				StartGlyphID       uint16 // First glyph ID in the range
				EndGlyphID         uint16 // Last glyph ID in the range
				StartCoverageIndex uint16 // Coverage Index of first glyph ID in range
			}
			err = binary.Read(r, binary.BigEndian, &buf)
			if err != nil {
				return nil, err
			}
			for j := int(buf.StartGlyphID); j <= int(buf.EndGlyphID); j++ {
				res = append(res, uint16(j))
			}
		}
	default:
		return nil, fmt.Errorf("unsupported coverage table vesrion %d", version)
	}
	return res, nil
}

// PairPosFormat1 is one of the two possible subtable formats for LookupType==2.
//
// Deprecated: convert to use sfnt/parser instead.
type PairPosFormat1 struct {
	// PosFormat uint16 omitted
	Header struct {
		CoverageOffset uint16 // Offset to Coverage table, from beginning of PairPos subtable.
		ValueFormat1   uint16 // Defines the types of data in valueRecord1 — for the first glyph in the pair (may be zero).
		ValueFormat2   uint16 // Defines the types of data in valueRecord2 — for the second glyph in the pair (may be zero).
		PairSetCount   uint16 // Number of PairSet tables
	}
	PairSetOffsets []uint16 // Array of offsets to PairSet tables. Offsets are from beginning of PairPos subtable, ordered by Coverage Index.
}

// ReadPairPosFormat1 reads the binary representation of PairPosFormat1
//
// Deprecated: convert to use sfnt/parser instead.
func ReadPairPosFormat1(r io.Reader) (*PairPosFormat1, error) {
	res := &PairPosFormat1{}
	err := binary.Read(r, binary.BigEndian, &res.Header)
	if err != nil {
		return nil, err
	}

	res.PairSetOffsets = make([]uint16, res.Header.PairSetCount)
	err = binary.Read(r, binary.BigEndian, res.PairSetOffsets)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// A PairSet table enumerates all the glyph pairs that begin with a covered
// glyph.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#pair-adjustment-positioning-format-1-adjustments-for-glyph-pairs
//
// Deprecated: convert to use sfnt/parser instead.
type PairSet struct {
	PairValueCount   uint16            // Number of PairValueRecords
	PairValueRecords []PairValueRecord // Array of PairValueRecords, ordered by glyph ID of the second glyph.
}

// A PairValueRecord specifies the second glyph in a pair and defines a
// ValueRecord for each glyph.
//
// Deprecated: convert to use sfnt/parser instead.
type PairValueRecord struct {
	SecondGlyph  uint16       // Glyph ID of second glyph in the pair (first glyph is listed in the Coverage table).
	ValueRecord1 *ValueRecord // Positioning data for the first glyph in the pair.
	ValueRecord2 *ValueRecord // Positioning data for the second glyph in the pair.
}

// ReadPairSet reads the binary representation of a PairSet.
//
// Deprecated: convert to use sfnt/parser instead.
func ReadPairSet(r io.Reader, ValueFormat1, ValueFormat2 uint16) (*PairSet, error) {
	res := &PairSet{}
	err := binary.Read(r, binary.BigEndian, &res.PairValueCount)
	if err != nil {
		return nil, err
	}

	res.PairValueRecords = make([]PairValueRecord, res.PairValueCount)
	for i := 0; i < int(res.PairValueCount); i++ {
		rec := &res.PairValueRecords[i]
		err = binary.Read(r, binary.BigEndian, &rec.SecondGlyph)
		if err != nil {
			return nil, err
		}
		rec.ValueRecord1, err = ReadValueRecord(r, ValueFormat1)
		if err != nil {
			return nil, err
		}
		rec.ValueRecord2, err = ReadValueRecord(r, ValueFormat2)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

// PairPosFormat2 is one of the two possible subtable formats for LookupType==2.
//
// Deprecated: convert to use sfnt/parser instead.
type PairPosFormat2 struct {
	// PosFormat uint16 omitted
	Header struct {
		CoverageOffset  uint16 // Offset to Coverage table, from beginning of PairPos subtable.
		ValueFormat1    uint16 // ValueRecord definition — for the first glyph of the pair (may be zero).
		ValueFormat2    uint16 // ValueRecord definition — for the second glyph of the pair (may be zero).
		ClassDef1Offset uint16 // Offset to ClassDef table, from beginning of PairPos subtable — for the first glyph of the pair.
		ClassDef2Offset uint16 // Offset to ClassDef table, from beginning of PairPos subtable — for the second glyph of the pair.
		Class1Count     uint16 // Number of classes in classDef1 table — includes Class 0.
		Class2Count     uint16 // Number of classes in classDef2 table — includes Class 0.
	}
	Records []Class2Record
}

// Deprecated: convert to use sfnt/parser instead.
type Class2Record struct {
	ValueRecord1 *ValueRecord // Positioning for first glyph — empty if valueFormat1 = 0.
	ValueRecord2 *ValueRecord // Positioning for second glyph — empty if valueFormat2 = 0.
}

// Deprecated: convert to use sfnt/parser instead.
func ReadPairPosFormat2(r io.Reader) (*PairPosFormat2, error) {
	res := &PairPosFormat2{}
	err := binary.Read(r, binary.BigEndian, &res.Header)
	if err != nil {
		return nil, err
	}

	n := int(res.Header.Class1Count) * int(res.Header.Class2Count)
	res.Records = make([]Class2Record, n)
	for i := 0; i < n; i++ {
		vr1, err := ReadValueRecord(r, res.Header.ValueFormat1)
		if err != nil {
			return nil, err
		}
		vr2, err := ReadValueRecord(r, res.Header.ValueFormat2)
		if err != nil {
			return nil, err
		}
		res.Records[i] = Class2Record{vr1, vr2}
	}

	return res, nil
}

// Deprecated: convert to use sfnt/parser instead.
type ClassRangeRecord struct {
	StartGlyphID uint16 // First glyph ID in the range
	EndGlyphID   uint16 // Last glyph ID in the range
	Class        uint16 // Applied to all glyphs in the range
}

// ValueRecord describes all the variables and values used to adjust the
// position of a glyph or set of glyphs.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#value-record
//
// Deprecated: convert to use sfnt/parser instead.
type ValueRecord struct {
	XPlacement       int16  // Horizontal adjustment for placement, in design units.
	YPlacement       int16  // Vertical adjustment for placement, in design units.
	XAdvance         int16  // Horizontal adjustment for advance, in design units — only used for horizontal layout.
	YAdvance         int16  // Vertical adjustment for advance, in design units — only used for vertical layout.
	XPlaDeviceOffset uint16 // Offset to Device table (non-variable font) / VariationIndex table (variable font) for horizontal placement, from beginning of the immediate parent table (SinglePos or PairPosFormat2 lookup subtable, PairSet table within a PairPosFormat1 lookup subtable) — may be NULL.
	YPlaDeviceOffset uint16 // Offset to Device table (non-variable font) / VariationIndex table (variable font) for vertical placement, from beginning of the immediate parent table (SinglePos or PairPosFormat2 lookup subtable, PairSet table within a PairPosFormat1 lookup subtable) — may be NULL.
	XAdvDeviceOffset uint16 // Offset to Device table (non-variable font) / VariationIndex table (variable font) for horizontal advance, from beginning of the immediate parent table (SinglePos or PairPosFormat2 lookup subtable, PairSet table within a PairPosFormat1 lookup subtable) — may be NULL.
	YAdvDeviceOffset uint16 // Offset to Device table (non-variable font) / VariationIndex table (variable font) for vertical advance, from beginning of the immediate parent table (SinglePos or PairPosFormat2 lookup subtable, PairSet table within a PairPosFormat1 lookup subtable) — may be NULL.
}

// ReadValueRecord reads the binary representation of a ValueRecord.  The given
// ValueFormat determines which fields are present in the binary
// representation.
//
// Deprecated: convert to use sfnt/parser instead.
func ReadValueRecord(r io.Reader, ValueFormat uint16) (*ValueRecord, error) {
	if ValueFormat == 0 {
		return nil, nil
	}
	res := &ValueRecord{}
	if ValueFormat&0x0001 != 0 {
		err := binary.Read(r, binary.BigEndian, &res.XPlacement)
		if err != nil {
			return nil, err
		}
	}
	if ValueFormat&0x0002 != 0 {
		err := binary.Read(r, binary.BigEndian, &res.YPlacement)
		if err != nil {
			return nil, err
		}
	}
	if ValueFormat&0x0004 != 0 {
		err := binary.Read(r, binary.BigEndian, &res.XAdvance)
		if err != nil {
			return nil, err
		}
	}
	if ValueFormat&0x0008 != 0 {
		err := binary.Read(r, binary.BigEndian, &res.YAdvance)
		if err != nil {
			return nil, err
		}
	}
	if ValueFormat&0x0010 != 0 {
		err := binary.Read(r, binary.BigEndian, &res.XPlaDeviceOffset)
		if err != nil {
			return nil, err
		}
	}
	if ValueFormat&0x0020 != 0 {
		err := binary.Read(r, binary.BigEndian, &res.YPlaDeviceOffset)
		if err != nil {
			return nil, err
		}
	}
	if ValueFormat&0x0040 != 0 {
		err := binary.Read(r, binary.BigEndian, &res.XAdvDeviceOffset)
		if err != nil {
			return nil, err
		}
	}
	if ValueFormat&0x0080 != 0 {
		err := binary.Read(r, binary.BigEndian, &res.YAdvDeviceOffset)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

// GposHead contains a version number for the GposHead table and offsets to
// locate the sub-tables.
//
// Deprecated: convert to use sfnt/parser instead.
type GposHead struct {
	V10 struct { // version 1.0
		MajorVersion      uint16 // Major version of the GPOS table
		MinorVersion      uint16 // Minor version of the GPOS table
		ScriptListOffset  uint16 // Offset to ScriptList table, from beginning of GPOS table
		FeatureListOffset uint16 // Offset to FeatureList table, from beginning of GPOS table
		LookupListOffset  uint16 // Offset to LookupList table, from beginning of GPOS table
	}
	V11 struct { // version 1.1
		FeatureVariationsOffset uint32 // Offset to FeatureVariations table, from beginning of GPOS table
	}
}

// Deprecated: convert to use sfnt/parser instead.
func (GPOS *GposHead) readLangSys(fd io.ReadSeeker, langSysTag, scriptTag string) (*langSys, error) {
	scriptListOffs := int64(GPOS.V10.ScriptListOffset)

	_, err := fd.Seek(scriptListOffs, io.SeekStart)
	if err != nil {
		return nil, err
	}
	scriptRecord, err := readScriptRecord(fd, scriptTag)
	if err != nil {
		return nil, err
	}
	if scriptRecord == nil {
		// TODO(voss): treat this error as if no GPOS/GSUB table is present?
		//     Or maybe search all of the feature list directly, instead?
		return nil, errors.New("no script record found")
	}
	scriptOffs := scriptListOffs + int64(scriptRecord.ScriptOffset)

	data := &script{}
	_, err = fd.Seek(scriptOffs, io.SeekStart)
	if err != nil {
		return nil, err
	}
	err = binary.Read(fd, binary.BigEndian, &data.Header)
	if err != nil {
		return nil, err
	}

	data.LangSysRecords = make([]langSysRecord, data.Header.LangSysCount)
	err = binary.Read(fd, binary.BigEndian, data.LangSysRecords)
	if err != nil {
		return nil, err
	}

	langSysOffs := int64(-1)
	for i := range data.LangSysRecords {
		rec := &data.LangSysRecords[i]
		if rec.LangSysTag.String() == langSysTag {
			langSysOffs = int64(rec.LangSysOffset)
			break
		}
	}
	if langSysOffs < 0 {
		if data.Header.DefaultLangSysOffset > 0 {
			langSysOffs = int64(data.Header.DefaultLangSysOffset)
		} else if len(data.LangSysRecords) > 0 {
			// missing default, take the first language
			langSysOffs = int64(data.LangSysRecords[0].LangSysOffset)
		} else {
			return nil, errors.New("no langSys record found")
		}
	}

	_, err = fd.Seek(scriptOffs+langSysOffs, io.SeekStart)
	if err != nil {
		return nil, err
	}
	ls := &langSys{}
	err = binary.Read(fd, binary.BigEndian, &ls.Header)
	if err != nil {
		return nil, err
	}
	ls.FeatureIndices = make([]uint16, ls.Header.FeatureIndexCount)
	err = binary.Read(fd, binary.BigEndian, &ls.FeatureIndices)
	if err != nil {
		return nil, err
	}

	return ls, nil
}

// Deprecated: convert to use sfnt/parser instead.
type featureInfo struct {
	Tag               string
	LookupListIndices []uint16
	Required          bool
}

// Deprecated: convert to use sfnt/parser instead.
func (GPOS *GposHead) ReadFeatureInfo(fd io.ReadSeeker, langTag, scriptTag string) ([]featureInfo, error) {
	langSys, err := GPOS.readLangSys(fd, langTag, scriptTag)
	if err != nil {
		return nil, err
	}
	type todoEntry struct {
		idx      uint16
		required bool
	}
	var todo []todoEntry
	if langSys.Header.RequiredFeatureIndex != 0xFFFF {
		todo = append(todo, todoEntry{langSys.Header.RequiredFeatureIndex, true})
	}
	for i := range langSys.FeatureIndices {
		todo = append(todo, todoEntry{langSys.FeatureIndices[i], false})
	}

	featureListBase := int64(GPOS.V10.FeatureListOffset)

	data := &featureList{}
	_, err = fd.Seek(featureListBase, io.SeekStart)
	if err != nil {
		return nil, err
	}
	err = binary.Read(fd, binary.BigEndian, &data.FeatureCount)
	if err != nil {
		return nil, err
	}
	data.FeatureRecords = make([]FeatureRecord, data.FeatureCount)
	err = binary.Read(fd, binary.BigEndian, data.FeatureRecords)
	if err != nil {
		return nil, err
	}
	for _, r := range data.FeatureRecords {
		fmt.Println(r)
	}

	var res []featureInfo
	for _, item := range todo {
		if item.idx >= data.FeatureCount {
			continue
		}
		tag := data.FeatureRecords[item.idx].FeatureTag.String()
		if tag == " RQD" {
			continue
		}

		fi := featureInfo{}
		fi.Tag = tag
		fi.Required = item.required

		offs := int64(data.FeatureRecords[item.idx].FeatureOffset)
		_, err = fd.Seek(featureListBase+offs, io.SeekStart)
		if err != nil {
			return nil, err
		}

		feature := &Feature{}
		err = binary.Read(fd, binary.BigEndian, &feature.Header)
		if err != nil {
			return nil, err
		}
		fi.LookupListIndices = make([]uint16, feature.Header.LookupIndexCount)
		err = binary.Read(fd, binary.BigEndian, fi.LookupListIndices)
		if err != nil {
			return nil, err
		}
		res = append(res, fi)
	}
	return res, nil
}

// Tag represents a tag string composed of 4 ASCII bytes
type Tag [4]byte

func (tag Tag) String() string {
	return string(tag[:])
}