// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

// Package os2 has code for reading and wrinting the "OS/2" table.
// https://docs.microsoft.com/en-us/typography/opentype/spec/os2
package os2

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/funit"
	"seehuhn.de/go/pdf/font/sfnt/cmap"
)

// Info contains information from the "OS/2" table.
type Info struct {
	WeightClass font.Weight
	WidthClass  font.Width

	IsBold    bool
	IsItalic  bool
	IsRegular bool
	IsOblique bool

	Ascent    funit.Int16
	Descent   funit.Int16 // as a negative number
	LineGap   funit.Int16
	CapHeight funit.Int16
	XHeight   funit.Int16

	AvgGlyphWidth int16 // arithmetic average of the width of all non-zero width glyphs

	SubscriptXSize     int16
	SubscriptYSize     int16
	SubscriptXOffset   int16
	SubscriptYOffset   int16
	SuperscriptXSize   int16
	SuperscriptYSize   int16
	SuperscriptXOffset int16
	SuperscriptYOffset int16
	StrikeoutSize      int16
	StrikeoutPosition  int16

	FamilyClass int16    // https://docs.microsoft.com/en-us/typography/opentype/spec/ibmfc
	Panose      [10]byte // https://monotype.github.io/panose/
	Vendor      string   // https://docs.microsoft.com/en-us/typography/opentype/spec/os2#achvendid

	PermUse          Permissions
	PermNoSubsetting bool // the font may not be subsetted prior to embedding
	PermOnlyBitmap   bool // only bitmaps contained in the font may be embedded
}

// Permissions describes rights to embed and use a font.
type Permissions int

func (perm Permissions) String() string {
	switch perm {
	case PermInstall:
		return "can install"
	case PermEdit:
		return "can edit"
	case PermView:
		return "can view"
	case PermRestricted:
		return "restricted"
	default:
		return fmt.Sprintf("Permissions(%d)", perm)
	}
}

// The possible permission values.
const (
	PermInstall    Permissions = iota // bits 0-3 unset
	PermEdit                          // bit 3
	PermView                          // bit 2
	PermRestricted                    // bit 1
)

// Read reads the "OS/2" table from r.
func Read(r io.Reader) (*Info, error) {
	v0 := &v0Data{}
	err := binary.Read(r, binary.BigEndian, v0)
	if err != nil {
		return nil, err
	} else if v0.Version > 5 {
		return nil, errors.New("OS/2: unsupported version")
	}

	var permUse Permissions
	permBits := v0.Type
	if v0.Version == 0 {
		permBits &= 0xF
	}
	if permBits&8 != 0 {
		permUse = PermEdit
	} else if permBits&4 != 0 {
		permUse = PermView
	} else if permBits&2 != 0 {
		permUse = PermRestricted
	} else {
		permUse = PermInstall
	}

	sel := v0.Selection
	if v0.Version <= 3 {
		// Applications should ignore bits 7 to 15 in a font that has a
		// version 0 to version 3 OS/2 table.
		sel &= 0x007F
	}

	info := &Info{
		WeightClass: font.Weight(v0.WeightClass),
		WidthClass:  font.Width(v0.WidthClass),

		IsBold:   sel&0x0060 == 0x0020,
		IsItalic: sel&0x0041 == 0x0001,
		// HasUnderline: sel&0x0042 == 0x0002,
		// IsOutlined:   sel&0x0048 == 0x0008,
		IsRegular: sel&0x0040 != 0,
		IsOblique: sel&0x0200 != 0,

		AvgGlyphWidth: v0.AvgCharWidth,

		SubscriptXSize:     v0.SubscriptXSize,
		SubscriptYSize:     v0.SubscriptYSize,
		SubscriptXOffset:   v0.SubscriptXOffset,
		SubscriptYOffset:   v0.SubscriptYOffset,
		SuperscriptXSize:   v0.SuperscriptXSize,
		SuperscriptYSize:   v0.SuperscriptYSize,
		SuperscriptXOffset: v0.SuperscriptXOffset,
		SuperscriptYOffset: v0.SuperscriptYOffset,
		StrikeoutSize:      v0.StrikeoutSize,
		StrikeoutPosition:  v0.StrikeoutPosition,

		FamilyClass: v0.FamilyClass,
		Panose:      v0.Panose,
		Vendor:      string(v0.VendID[:]),

		PermUse:          permUse,
		PermNoSubsetting: permBits&0x0100 != 0,
		PermOnlyBitmap:   permBits&0x0200 != 0,
	}

	v0ms := &v0MsData{}
	err = binary.Read(r, binary.BigEndian, v0ms)
	if err == io.EOF {
		return info, nil
	} else if err != nil {
		return nil, err
	}
	if sel&0x0080 != 0 {
		info.Ascent = v0ms.TypoAscender
		info.Descent = v0ms.TypoDescender
	} else {
		info.Ascent = funit.Int16(v0ms.WinAscent)
		info.Descent = -funit.Int16(v0ms.WinDescent)
	}
	info.LineGap = v0ms.TypoLineGap

	if v0.Version < 2 {
		return info, nil
	}

	var codePageRange [8]byte
	err = binary.Read(r, binary.BigEndian, codePageRange[:])
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}

	v2 := &v2Data{}
	err = binary.Read(r, binary.BigEndian, v2)
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	if v2.XHeight > 0 {
		info.XHeight = v2.XHeight
	}
	if v2.CapHeight > 0 {
		info.CapHeight = v2.CapHeight
	}

	return info, nil
}

// Encode converts the info to a "OS/2" table.
func (info *Info) Encode(cc cmap.Subtable) []byte {
	var permBits uint16
	switch info.PermUse {
	case PermRestricted:
		permBits |= 2
	case PermView:
		permBits |= 4
	case PermEdit:
		permBits |= 8
	}
	if info.PermNoSubsetting {
		permBits |= 0x0100
	}
	if info.PermOnlyBitmap {
		permBits |= 0x0200
	}

	var unicodeRange [4]uint32 // TODO(voss)
	setUniBit := func(b int) {
		w := b / 32
		b = b % 32
		unicodeRange[w] |= 1 << b
	}

	// setUniBit(0) // Basic Latin

	var sel uint16
	if info.IsRegular {
		sel |= 0x0040
	} else {
		if info.IsItalic {
			sel |= 0x0001
		}
		if info.IsBold {
			sel |= 0x0020
		}
	}
	// if info.HasUnderline {
	// 	sel |= 0x0002
	// }
	// if info.IsOutlined {
	// 	sel |= 0x0008
	// }
	if info.IsOblique {
		sel |= 0x0200
	}
	sel |= 0x0080 // always use Typo{A,De}scender

	var firstCharIndex, lastCharIndex uint16
	if cc != nil {
		low, high := cc.CodeRange()
		firstCharIndex = uint16(low)
		if low > 0xFFFF {
			firstCharIndex = 0xFFFF
		}
		lastCharIndex = uint16(high)
		if high > 0xFFFF {
			lastCharIndex = 0xFFFF
			setUniBit(57)
		}
	}

	vendor := [4]byte{' ', ' ', ' ', ' '}
	if len(info.Vendor) == 4 {
		copy(vendor[:], info.Vendor)
	}

	buf := &bytes.Buffer{}
	v0 := &v0Data{
		Version:            4,
		AvgCharWidth:       info.AvgGlyphWidth,
		WeightClass:        uint16(info.WeightClass),
		WidthClass:         uint16(info.WidthClass),
		Type:               permBits,
		SubscriptXSize:     info.SubscriptXSize,
		SubscriptYSize:     info.SubscriptYSize,
		SubscriptXOffset:   info.SubscriptXOffset,
		SubscriptYOffset:   info.SubscriptYOffset,
		SuperscriptXSize:   info.SuperscriptXSize,
		SuperscriptYSize:   info.SuperscriptYSize,
		SuperscriptXOffset: info.SuperscriptXOffset,
		SuperscriptYOffset: info.SuperscriptYOffset,
		StrikeoutSize:      info.StrikeoutSize,
		StrikeoutPosition:  info.StrikeoutPosition,
		FamilyClass:        info.FamilyClass,
		Panose:             info.Panose,
		UnicodeRange:       unicodeRange,
		VendID:             vendor,
		Selection:          sel,
		FirstCharIndex:     firstCharIndex,
		LastCharIndex:      lastCharIndex,
	}
	_ = binary.Write(buf, binary.BigEndian, v0)

	v0ms := &v0MsData{
		TypoAscender:  info.Ascent,
		TypoDescender: info.Descent,
		TypoLineGap:   info.LineGap,
		WinAscent:     info.Ascent,   // TODO(voss)
		WinDescent:    -info.Descent, // TODO(voss)
	}
	_ = binary.Write(buf, binary.BigEndian, v0ms)

	var codePageRange uint64 // TODO(voss)
	// setCodePageBit := func(b int) {
	// 	codePageRange |= 1 << b
	// }
	// setCodePageBit(0) // Latin 1

	buf.Write([]byte{
		byte(codePageRange >> 24),
		byte(codePageRange >> 16),
		byte(codePageRange >> 8),
		byte(codePageRange),
		byte(codePageRange >> 56),
		byte(codePageRange >> 48),
		byte(codePageRange >> 40),
		byte(codePageRange >> 32),
	})

	v2 := &v2Data{
		XHeight:   info.XHeight,
		CapHeight: info.CapHeight,
		// DefaultChar: 0, // TODO(voss)
		// BreakChar:   0, // TODO(voss)
		// MaxContext:  0, // TODO(voss)
	}
	binary.Write(buf, binary.BigEndian, v2)

	return buf.Bytes()
}

type v0Data struct {
	Version            uint16
	AvgCharWidth       int16
	WeightClass        uint16
	WidthClass         uint16
	Type               uint16
	SubscriptXSize     int16
	SubscriptYSize     int16
	SubscriptXOffset   int16
	SubscriptYOffset   int16
	SuperscriptXSize   int16
	SuperscriptYSize   int16
	SuperscriptXOffset int16
	SuperscriptYOffset int16
	StrikeoutSize      int16
	StrikeoutPosition  int16
	FamilyClass        int16
	Panose             [10]byte
	UnicodeRange       [4]uint32
	VendID             [4]byte
	Selection          uint16
	FirstCharIndex     uint16
	LastCharIndex      uint16
}

type v0MsData struct {
	TypoAscender  funit.Int16
	TypoDescender funit.Int16
	TypoLineGap   funit.Int16
	WinAscent     funit.Int16
	WinDescent    funit.Int16 // positive
}

type v2Data struct {
	XHeight     funit.Int16
	CapHeight   funit.Int16
	DefaultChar uint16
	BreakChar   uint16
	MaxContext  uint16
}
