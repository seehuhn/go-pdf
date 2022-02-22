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

	"seehuhn.de/go/pdf/font/sfnt/cmap"
	"seehuhn.de/go/pdf/font/sfnt/table"
)

// Info contains information from the "OS/2" table.
type Info struct {
	WeightClass Weight
	WidthClass  Width

	IsBold       bool
	IsItalic     bool
	HasUnderline bool
	IsOutlined   bool
	IsRegular    bool
	IsOblique    bool

	Ascent    int16
	Descent   int16 // as a negative number
	LineGap   int16
	XHeight   int16
	CapHeight int16

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

	FamilyClass int16     // https://docs.microsoft.com/en-us/typography/opentype/spec/ibmfc
	Panose      [10]byte  // https://monotype.github.io/panose/
	Vendor      table.Tag // https://docs.microsoft.com/en-us/typography/opentype/spec/os2#achvendid

	PermUse          Permissions
	PermNoSubsetting bool // the font may not be subsetted prior to embedding
	PermOnlyBitmap   bool // only bitmaps contained in the font may be embedded
}

// Weight indicates the visual weight (degree of blackness or thickness of
// strokes) of the characters in the font.  Values from 1 to 1000 are valid.
type Weight uint16

func (w Weight) String() string {
	switch w {
	case WeightThin:
		return "Thin"
	case WeightExtraLight:
		return "Extra Light"
	case WeightLight:
		return "Light"
	case WeightNormal:
		return "Normal"
	case WeightMedium:
		return "Medium"
	case WeightSemiBold:
		return "Semi Bold"
	case WeightBold:
		return "Bold"
	case WeightExtraBold:
		return "Extra Bold"
	case WeightBlack:
		return "Black"
	default:
		return fmt.Sprintf("%d", w)
	}
}

// Pre-defined weight classes.
// TODO(voss): move to a different package?
const (
	WeightThin       Weight = 100
	WeightExtraLight Weight = 200
	WeightLight      Weight = 300
	WeightNormal     Weight = 400
	WeightMedium     Weight = 500
	WeightSemiBold   Weight = 600
	WeightBold       Weight = 700
	WeightExtraBold  Weight = 800
	WeightBlack      Weight = 900
)

// Width indicates the aspect ratio (width to height ratio) as specified by a
// font designer for the glyphs in a font.
type Width uint16

// Valid width values.
const (
	WidthUltraCondensed Width = 1 // 50% of WidthNormal
	WidthExtraCondensed Width = 2 // 62.5% of WidthNormal
	WidthCondensed      Width = 3 // 75% of WidthNormal
	WidthSemiCondensed  Width = 4 // 87.5% of WidthNormal
	WidthNormal         Width = 5
	WidthSemiExpanded   Width = 6 // 112.5% of WidthNormal
	WidthExpanded       Width = 7 // 125% of WidthNormal
	WidthExtraExpanded  Width = 8 // 150% of WidthNormal
	WidthUltraExpanded  Width = 9 // 200% of WidthNormal
)

// Permissions describes rights to embed and use a font.
type Permissions int

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
		// Applications should ignore bits 7 to 15 in a font that has a version
		// 0 to version 3 OS/2 table.
		sel &= 0x007F
	}

	info := &Info{
		WeightClass:      Weight(v0.WeightClass),
		WidthClass:       Width(v0.WidthClass),
		PermUse:          permUse,
		PermNoSubsetting: permBits&0x0100 != 0,
		PermOnlyBitmap:   permBits&0x0200 != 0,

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

		Vendor: v0.VendID,

		IsItalic:     sel&0x0041 == 0x0001,
		HasUnderline: sel&0x0042 == 0x0002,
		IsOutlined:   sel&0x0048 == 0x0008,
		IsBold:       sel&0x0060 == 0x0020,
		IsRegular:    sel&0x0040 != 0,
		IsOblique:    sel&0x0200 != 0,
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
		info.Ascent = int16(v0ms.WinAscent)
		info.Descent = -int16(v0ms.WinDescent)
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
	info.XHeight = v2.XHeight
	info.CapHeight = v2.CapHeight

	return info, nil
}

// Encode converts the info to a "OS/2" table.
func (info *Info) Encode(cc cmap.Subtable) []byte {
	var avgCharWidth int16 // TODO(voss)

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

	var sel uint16
	if info.IsRegular {
		sel |= 0x0040
	} else {
		if info.IsItalic {
			sel |= 0x0001
		}
		if info.HasUnderline {
			sel |= 0x0002
		}
		if info.IsOutlined {
			sel |= 0x0008
		}
		if info.IsBold {
			sel |= 0x0020
		}
	}
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
		}
	}

	buf := &bytes.Buffer{}
	v0 := &v0Data{
		Version:            4,
		AvgCharWidth:       avgCharWidth,
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
		VendID:             info.Vendor,
		Selection:          sel,
		FirstCharIndex:     firstCharIndex,
		LastCharIndex:      lastCharIndex,
	}
	binary.Write(buf, binary.BigEndian, v0)

	v0ms := &v0MsData{
		TypoAscender:  info.Ascent,
		TypoDescender: info.Descent,
		TypoLineGap:   info.LineGap,
		WinAscent:     0, // TODO(voss)
		WinDescent:    0, // TODO(voss)
	}
	binary.Write(buf, binary.BigEndian, v0ms)

	var codePageRange uint64 // TODO(voss)
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
		XHeight:     info.XHeight,
		CapHeight:   info.CapHeight,
		DefaultChar: 0,
		BreakChar:   0x20, // TODO(voss)
		MaxContext:  0,    // TODO(voss)
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
	VendID             table.Tag
	Selection          uint16
	FirstCharIndex     uint16
	LastCharIndex      uint16
}

type v0MsData struct {
	TypoAscender  int16
	TypoDescender int16
	TypoLineGap   int16
	WinAscent     uint16
	WinDescent    uint16
}

type v2Data struct {
	XHeight     int16
	CapHeight   int16
	DefaultChar uint16
	BreakChar   uint16
	MaxContext  uint16
}
