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

// Package head reads and writes "head" tables.
// https://docs.microsoft.com/en-us/typography/opentype/spec/head
package head

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"time"

	"seehuhn.de/go/pdf/sfnt/funit"
	"seehuhn.de/go/pdf/sfnt/parser"
)

const headLength = 54

// Info represents the information in the 'head' table of an sfnt.
type Info struct {
	FontRevision Version // set by font manufacturer
	HasYBaseAt0  bool    // baseline for font at y=0
	HasXBaseAt0  bool    // left sidebearing point at x=0 (only for TrueType)
	IsNonlinear  bool    // outline/advance width may change nonlinearly
	UnitsPerEm   uint16  // font design units per em square
	Created      time.Time
	Modified     time.Time
	FontBBox     funit.Rect

	IsBold      bool
	IsItalic    bool
	HasShadow   bool
	IsCondensed bool
	IsExtended  bool

	LowestRecPPEM uint16 // smallest readable size in pixels
	LocaFormat    int16  // 0 for short offsets, 1 for long (TrueType only)
}

// Read reads and decodes the binary representation of the head table.
func Read(r io.Reader) (*Info, error) {
	enc := &binaryHead{}
	err := binary.Read(r, binary.BigEndian, enc)
	if err != nil {
		return nil, err
	}

	if enc.Version != 0x00010000 {
		return nil, &parser.NotSupportedError{
			SubSystem: "sfnt/head",
			Feature:   fmt.Sprintf("table version 0x%08x", enc.Version),
		}
	}
	if enc.MagicNumber != 0x5F0F3CF5 {
		return nil, &parser.InvalidFontError{
			SubSystem: "sfnt/head",
			Reason:    fmt.Sprintf("invalid magic number %08x", enc.MagicNumber),
		}
	}

	info := &Info{}

	info.FontRevision = Version(enc.FontRevision)

	flags := enc.Flags
	info.HasYBaseAt0 = flags&(1<<0) != 0
	info.HasXBaseAt0 = flags&(1<<1) != 0
	info.IsNonlinear = flags&(1<<2) != 0 || flags&(1<<4) != 0

	info.UnitsPerEm = enc.UnitsPerEm

	info.Created = decodeTime(enc.Created)
	info.Modified = decodeTime(enc.Modified)

	info.FontBBox = funit.Rect{
		LLx: enc.XMin,
		LLy: enc.YMin,
		URx: enc.XMax,
		URy: enc.YMax,
	}

	info.IsBold = enc.MacStyle&(1<<0) != 0
	info.IsItalic = enc.MacStyle&(1<<1) != 0
	// info.HasUnderline = enc.MacStyle&(1<<2) != 0
	// info.IsOutlined = enc.MacStyle&(1<<3) != 0
	info.HasShadow = enc.MacStyle&(1<<4) != 0
	info.IsCondensed = enc.MacStyle&(1<<5) != 0
	info.IsExtended = enc.MacStyle&(1<<6) != 0

	info.LowestRecPPEM = enc.LowestRecPPEM
	info.LocaFormat = enc.IndexToLocFormat

	return info, nil
}

// Encode returns the binary representation of the head table.
func (info *Info) Encode() []byte {
	var flags uint16
	if info.HasYBaseAt0 {
		flags |= 1 << 0
	}
	if info.HasXBaseAt0 {
		flags |= 1 << 1
	}
	if info.IsNonlinear {
		flags |= 1 << 2
		flags |= 1 << 4
	}
	flags |= 1 << 3
	flags |= 1 << 11
	flags |= 1 << 12
	flags |= 1 << 13

	var macStyle uint16
	if info.IsBold {
		macStyle |= 1 << 0
	}
	if info.IsItalic {
		macStyle |= 1 << 1
	}
	// if info.HasUnderline {
	// 	macStyle |= 1 << 2
	// }
	// if info.IsOutlined {
	// 	macStyle |= 1 << 3
	// }
	if info.HasShadow {
		macStyle |= 1 << 4
	}
	if info.IsCondensed {
		macStyle |= 1 << 5
	}
	if info.IsExtended {
		macStyle |= 1 << 6
	}

	rev := info.FontRevision
	tc := encodeTime(info.Created)
	tm := encodeTime(info.Modified)
	res := []byte{
		0, 1, 0, 0, // Version
		byte(rev >> 24), byte(rev >> 16), byte(rev >> 8), byte(rev), // FontRevision
		0, 0, 0, 0, // CheckSumAdjustment
		0x5F, 0x0F, 0x3C, 0xF5, // MagicNumber
		byte(flags >> 8), byte(flags), // Flags
		byte(info.UnitsPerEm >> 8), byte(info.UnitsPerEm), // UnitsPerEm
		byte(tc >> 56), byte(tc >> 48), byte(tc >> 40), byte(tc >> 32),
		byte(tc >> 24), byte(tc >> 16), byte(tc >> 8), byte(tc), // Created
		byte(tm >> 56), byte(tm >> 48), byte(tm >> 40), byte(tm >> 32),
		byte(tm >> 24), byte(tm >> 16), byte(tm >> 8), byte(tm), // Modified
		byte(info.FontBBox.LLx >> 8), byte(info.FontBBox.LLx), // XMin
		byte(info.FontBBox.LLy >> 8), byte(info.FontBBox.LLy), // YMin
		byte(info.FontBBox.URx >> 8), byte(info.FontBBox.URx), // XMax
		byte(info.FontBBox.URy >> 8), byte(info.FontBBox.URy), // YMax
		byte(macStyle >> 8), byte(macStyle), // MacStyle
		byte(info.LowestRecPPEM >> 8), byte(info.LowestRecPPEM), // LowestRecPPEM
		0, 2, // FontDirectionHint
		byte(info.LocaFormat >> 8), byte(info.LocaFormat), // IndexToLocFormat
		0, 0, // GlyphDataFormat
	}
	return res
}

type binaryHead struct {
	Version            uint32
	FontRevision       uint32
	CheckSumAdjustment uint32
	MagicNumber        uint32
	Flags              uint16
	UnitsPerEm         uint16
	Created            int64
	Modified           int64

	XMin funit.Int16
	YMin funit.Int16
	XMax funit.Int16
	YMax funit.Int16

	MacStyle uint16

	LowestRecPPEM     uint16
	FontDirectionHint int16

	IndexToLocFormat int16
	GlyphDataFormat  int16
}

// Version represents the font revision in 16.16 fixed point format.
type Version uint32

// VersionFromString parses a version string in the form "1.234" or "Version 1.234".
// String data after the last digit is ignored.
func VersionFromString(s string) (Version, error) {
	m := versionPat.FindStringSubmatch(s)
	if len(m) != 2 {
		return 0, errInvalidVersion
	}

	ver, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, errInvalidVersion
	}
	return Version(ver*65536 + 0.5), nil
}

func (v Version) String() string {
	return fmt.Sprintf("%.03f", float64(v)/65536)
}

// Round removes all information from v which is not visible in the string
// representation.
func (v Version) Round() Version {
	x := math.Round(float64(v)/65536*1000) / 1000
	return Version(math.Round(x * 65536))
}

var versionPat = regexp.MustCompile(`^(?:Version )?(\d+\.?\d+)`)

var errInvalidVersion = errors.New("invalid version")
