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
	"sort"
	"unicode"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/funit"
)

const (
	ScalerTypeTrueType = 0x00010000
	ScalerTypeCFF      = 0x4F54544F
	ScalerTypeApple    = 0x74727565
)

type Header struct {
	ScalerType uint32
	Toc        map[string]Record
}

type Record struct {
	Offset uint32
	Length uint32
}

// ReadHeader reads the file header of an sfnt font file.
func ReadHeader(r io.ReaderAt) (*Header, error) {
	var buf [16]byte
	_, err := r.ReadAt(buf[:6], 0)
	if err != nil {
		return nil, err
	}
	scalerType := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
	numTables := int(buf[4])<<8 | int(buf[5])

	if scalerType != ScalerTypeTrueType &&
		scalerType != ScalerTypeCFF &&
		scalerType != ScalerTypeApple {
		return nil, &font.NotSupportedError{
			SubSystem: "sfnt/header",
			Feature:   fmt.Sprintf("scaler type 0x%x", scalerType),
		}
	}
	if numTables > 280 {
		// the largest value observed on my laptop is 28
		return nil, errors.New("sfnt/header: too many tables")
	}

	h := &Header{
		ScalerType: scalerType,
		Toc:        make(map[string]Record),
	}
	type alloc struct {
		Start uint32
		End   uint32
	}
	var coverage []alloc
	for i := 0; i < numTables; i++ {
		_, err := r.ReadAt(buf[:], int64(12+i*16))
		if err != nil {
			return nil, err
		}
		name := string(buf[:4])
		offset := uint32(buf[8])<<24 + uint32(buf[9])<<16 + uint32(buf[10])<<8 + uint32(buf[11])
		length := uint32(buf[12])<<24 + uint32(buf[13])<<16 + uint32(buf[14])<<8 + uint32(buf[15])
		if !isKnownTable[name] {
			continue
		}
		h.Toc[name] = Record{
			Offset: offset,
			Length: length,
		}
		coverage = append(coverage, alloc{
			Start: offset,
			End:   offset + length,
		})
	}
	if len(h.Toc) == 0 {
		return nil, errors.New("sfnt/header: no tables found")
	}

	// perform some sanity checks
	sort.Slice(coverage, func(i, j int) bool {
		if coverage[i].Start != coverage[j].Start {
			return coverage[i].Start < coverage[j].Start
		}
		return coverage[i].End < coverage[j].End
	})
	if coverage[0].Start < 12 {
		return nil, errors.New("sfnt/header: invalid table offset")
	}
	for i := 1; i < len(coverage); i++ {
		if coverage[i-1].End > coverage[i].Start {
			return nil, errors.New("sfnt/header: overlapping tables")
		}
	}
	_, err = r.ReadAt(buf[:1], int64(coverage[len(coverage)-1].End)-1)
	if err == io.EOF {
		return nil, errors.New("sfnt/header: table extends beyond EOF")
	} else if err != nil {
		return nil, err
	}

	return h, nil
}

func (h *Header) Has(names ...string) bool {
	for _, name := range names {
		if _, ok := h.Toc[name]; !ok {
			return false
		}
	}
	return true
}

func (h *Header) Find(tableName string) (Record, error) {
	rec, ok := h.Toc[tableName]
	if !ok {
		return rec, &ErrNoTable{Name: tableName}
	}
	return rec, nil
}

func (h *Header) ReadTableBytes(r io.ReaderAt, tableName string) ([]byte, error) {
	rec, err := h.Find(tableName)
	if err != nil {
		return nil, err
	}
	res := make([]byte, rec.Length)
	n, err := r.ReadAt(res, int64(rec.Offset))
	if n < len(res) && err != nil {
		return nil, err
	}
	return res[:n], nil
}

var isKnownTable = map[string]bool{
	"BASE": true,
	"CBDT": true,
	"CBLC": true,
	"CFF ": true,
	"cmap": true,
	"cvt ": true,
	"DSIG": true,
	"feat": true,
	"FFTM": true,
	"fpgm": true,
	"fvar": true,
	"gasp": true,
	"GDEF": true,
	"glyf": true,
	"GPOS": true,
	"GSUB": true,
	"gvar": true,
	"hdmx": true,
	"head": true,
	"hhea": true,
	"hmtx": true,
	"HVAR": true,
	"kern": true,
	"loca": true,
	"LTSH": true,
	"maxp": true,
	"meta": true,
	"morx": true,
	"name": true,
	"OS/2": true,
	"post": true,
	"prep": true,
	"STAT": true,
	"VDMX": true,
	"vhea": true,
	"vmtx": true,
	"VORG": true,
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

type Glyf struct {
	Data []GlyphHeader
	// actual glyph descriptions omitted
}

// GlyphHeader is the structure at the beginning of a glyph description.
// https://docs.microsoft.com/en-us/typography/opentype/spec/glyf#glyph-headers
type GlyphHeader struct {
	NumberOfContours int16       // If the number of contours is greater than or equal to zero, this is a simple glyph. If negative, this is a composite glyph — the value -1 should be used for composite glyphs.
	XMin             funit.Int16 // Minimum x for coordinate data.
	YMin             funit.Int16 // Minimum y for coordinate data.
	XMax             funit.Int16 // Maximum x for coordinate data.
	YMax             funit.Int16 // Maximum y for coordinate data.
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
