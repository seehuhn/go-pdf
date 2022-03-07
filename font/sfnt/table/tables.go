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

const (
	ScalerTypeTrueType = 0x00010000
	ScalerTypeCFF      = 0x4F54544F
)

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
	if scalerType != ScalerTypeTrueType && scalerType != ScalerTypeCFF {
		return nil, &font.NotSupportedError{
			SubSystem: "sfnt/header",
			Feature:   fmt.Sprintf("scaler type %x", scalerType),
		}
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

// ReadTableBytes returns the body of an sfnt table.
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

// CMap is the Character To Glyph Index Mapping Table.
type CMap struct {
	Header struct {
		Version   uint16
		NumTables uint16
	}
	EncodingRecords []EncodingRecord
}

// ReadCMapTable reads the binary representation of a "cmap" table.
func ReadCMapTable(r io.ReadSeeker) (*CMap, error) {
	cmap := &CMap{}
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
func (ct *CMap) Find(plat, enc uint16) *EncodingRecord {
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

// LoadCMap reads a mapping from unicode runes to glyph indeces from a "cmap"
// table encoding record.
// The function does NOT check that glyph indices are valid.
func (encRec *EncodingRecord) LoadCMap(r io.ReadSeeker, i2r func(int) rune) (map[rune]font.GlyphID, error) {
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
			if total > 70_000 {
				// Fonts on my system have up to around 50,000 mappings,
				// a reasonable maximum is 65536.
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
					if r != unicode.ReplacementChar {
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
					if r != unicode.ReplacementChar {
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
				if r != unicode.ReplacementChar {
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

type NameHeader struct {
	Format uint16 // table version number
	Count  uint16 // number of name records
	Offset uint16 // offset to the beginning of strings (bytes)
}

type NameRecord struct {
	PlatformID uint16 // platform identifier code
	EncodingID uint16 // platform-specific encoding identifier
	LanguageID uint16 // language identifier
	NameID     uint16 // name identifier
	Length     uint16 // name string length in bytes
	Offset     uint16 // name string offset in bytes
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
	ItalicAngle        float64
	UnderlinePosition  int16
	UnderlineThickness int16
	IsFixedPitch       bool
}

type Glyf struct {
	Data []GlyphHeader
	// actual glyph descriptions omitted
}

// GlyphHeader is the structure at the beginning of a glyph description.
// https://docs.microsoft.com/en-us/typography/opentype/spec/glyf#glyph-headers
type GlyphHeader struct {
	NumberOfContours int16 // If the number of contours is greater than or equal to zero, this is a simple glyph. If negative, this is a composite glyph — the value -1 should be used for composite glyphs.
	XMin             int16 // Minimum x for coordinate data.
	YMin             int16 // Minimum y for coordinate data.
	XMax             int16 // Maximum x for coordinate data.
	YMax             int16 // Maximum y for coordinate data.
}

// Tag represents a tag string composed of 4 ASCII bytes
type Tag [4]byte

// MakeTag converts a string of length 4 bytes to a Tag.
func MakeTag(s string) Tag {
	if len(s) != 4 {
		panic("tag must be 4 bytes")
	}
	return Tag{s[0], s[1], s[2], s[3]}
}

func (tag Tag) String() string {
	return string(tag[:])
}
