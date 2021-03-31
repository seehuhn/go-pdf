package table

import (
	"encoding/binary"
	"io"
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

// A Record is part of the Header.  It contains data about a single sfnt table.
type Record struct {
	Tag      Tag
	CheckSum uint32
	Offset   uint32
	Length   uint32
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

// Find returns the table record for the table with the given name.
// If no such table exists, nil is returned.
func (h *Header) Find(name string) *Record {
	for i := 0; i < int(h.Offsets.NumTables); i++ {
		if string(h.Records[i].Tag[:]) == name {
			return &h.Records[i]
		}
	}
	return nil
}

type MaxpHead struct {
	Version   int32  //	0x00005000 or 0x00010000
	NumGlyphs uint16 //	the number of glyphs in the font
}

type Cmap struct {
	Header struct {
		Version   uint16
		NumTables uint16
	}
	EncodingRecords []CmapRecord
}

// Find locates an encoding record in the cmap table.
func (ct *Cmap) Find(plat, enc uint16) *CmapRecord {
	for i := range ct.EncodingRecords {
		table := &ct.EncodingRecords[i]
		if table.PlatformID == plat && table.EncodingID == enc {
			return table
		}
	}
	return nil
}

type CmapRecord struct {
	PlatformID     uint16
	EncodingID     uint16
	SubtableOffset uint32
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

type Head struct {
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
type scriptList struct {
	ScriptCount   uint16         // Number of ScriptRecords
	ScriptRecords []scriptRecord // Array of ScriptRecords, listed alphabetically by script tag
}

// A scriptRecord consists of a ScriptTag that identifies a script, and an
// offset into a Script table.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#script-list-table-and-script-record
type scriptRecord struct {
	ScriptTag    Tag    // Script tag identifier
	ScriptOffset uint16 // Offset to Script table, from beginning of ScriptList
}

// readScriptRecord returns the ScriptRecord with the given script tag.  If no
// record for tag is found, return the default record instead (or nil, of no
// default record is found).
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
type langSysRecord struct {
	LangSysTag    Tag    // LangSysTag identifier
	LangSysOffset uint16 // Offset to LangSys table, from beginning of Script table
}

type langSys struct {
	Header struct {
		_                    uint16 // (reserved for an offset to a reordering table)
		RequiredFeatureIndex uint16 // Index of a feature required for this language system; if no required features = 0xFFFF
		FeatureIndexCount    uint16 // Number of feature index values for this language system — excludes the required feature
	}
	FeatureIndices []uint16 // Array of indices into the FeatureList, in arbitrary order
}

// A FeatureList enumerates features in an array of records and specifies
// the total number of features.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#feature-list-table
type FeatureList struct {
	FeatureCount   uint16          // Number of FeatureRecords in this table
	FeatureRecords []FeatureRecord // Array of FeatureRecords — zero-based (first feature has FeatureIndex = 0), listed alphabetically by feature tag
}

// A FeatureRecord consists of a FeatureTag that identifies the feature and an
// offset to a Feature table.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#feature-list-table
type FeatureRecord struct {
	FeatureTag    Tag    // feature identification tag
	FeatureOffset uint16 // Offset to Feature table, from beginning of FeatureList
}

type Feature struct {
	Header struct {
		FeatureParamsOffset uint16 // Offset from start of Feature table to FeatureParams table, if defined for the feature and present, else NULL
		LookupIndexCount    uint16 // Number of LookupList indices for this feature
	}
	LookupListIndices []uint16 // Array of indices into the LookupList — zero-based (first lookup is LookupListIndex = 0)
}

// The LookupList table contains an array of offsets to Lookup tables.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#lookup-list-table
type LookupList struct {
	LookupCount   uint16   // Number of lookups in this table
	LookupOffsets []uint16 // Array of offsets to Lookup tables, from beginning of LookupList — zero based (first lookup is Lookup index = 0)
}

// GposHead contains a version number for the GposHead table and offsets to
// locate the sub-tables.
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
	LookupList LookupList
	// FeatureVariations FeatureVariationsTable
}

func (GPOS *GposHead) ReadLangSys(fd io.ReadSeeker, langSysTag, scriptTag string) (*langSys, error) {
	scriptListOffs := int64(GPOS.V10.ScriptListOffset)

	_, err := fd.Seek(scriptListOffs, io.SeekStart)
	if err != nil {
		return nil, err
	}
	scriptRecord, err := readScriptRecord(fd, scriptTag)
	if err != nil {
		return nil, err
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
		if data.Header.DefaultLangSysOffset == 0 {
			return nil, nil
		}
		langSysOffs = int64(data.Header.DefaultLangSysOffset)
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

type FeatureInfo struct {
	Tag               string
	LookupListIndices []uint16
	Required          bool
}

func (GPOS *GposHead) ReadFeatureInfo(fd io.ReadSeeker, langSysTag, scriptTag string) ([]FeatureInfo, error) {
	langSys, err := GPOS.ReadLangSys(fd, langSysTag, scriptTag)
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

	data := &FeatureList{}
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

	res := make([]FeatureInfo, len(todo))
	for i, item := range todo {
		if item.idx >= data.FeatureCount {
			continue
		}
		res[i].Tag = data.FeatureRecords[item.idx].FeatureTag.String()
		res[i].Required = item.required

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
		res[i].LookupListIndices = make([]uint16, feature.Header.LookupIndexCount)
		err = binary.Read(fd, binary.BigEndian, res[i].LookupListIndices)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

// Tag represents a tag string composed of 4 ASCII bytes
type Tag [4]byte

func (tag Tag) String() string {
	return string(tag[:])
}
