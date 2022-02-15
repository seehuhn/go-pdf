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

// Package head supports reading and writing the HEAD table.
// https://docs.microsoft.com/en-us/typography/opentype/spec/head
package head

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"seehuhn.de/go/pdf/font"
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
	FontBBox     font.Rect

	IsBold       bool
	IsItalic     bool
	HasUnderline bool
	IsOutlined   bool
	HasShadow    bool
	IsCondensed  bool
	IsExtended   bool

	LowestRecPPEM  uint16 // smallest readable size in pixels
	HasLongOffsets bool   // 'loca' table uses 32 bit offsets (TrueType only)
}

// Read reads and  decodes the binary representation of the head table.
func Read(r io.Reader) (*Info, error) {
	enc := &binaryHead{}
	err := binary.Read(r, binary.BigEndian, enc)
	if err != nil {
		return nil, err
	}

	if enc.Version != 0x00010000 {
		return nil, fmt.Errorf("sfnt/head: unsupported table version %08x", enc.Version)
	}
	if enc.MagicNumber != 0x5F0F3CF5 {
		return nil, fmt.Errorf("sfnt/head: invalid magic number %08x", enc.MagicNumber)
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

	info.FontBBox = font.Rect{
		LLx: enc.XMin,
		LLy: enc.YMin,
		URx: enc.XMax,
		URy: enc.YMax,
	}

	info.IsBold = enc.MacStyle&(1<<0) != 0
	info.IsItalic = enc.MacStyle&(1<<1) != 0
	info.HasUnderline = enc.MacStyle&(1<<2) != 0
	info.IsOutlined = enc.MacStyle&(1<<3) != 0
	info.HasShadow = enc.MacStyle&(1<<4) != 0
	info.IsCondensed = enc.MacStyle&(1<<5) != 0
	info.IsExtended = enc.MacStyle&(1<<6) != 0

	info.LowestRecPPEM = enc.LowestRecPPEM
	info.HasLongOffsets = enc.IndexToLocFormat != 0

	return info, nil
}

// Encode returns the binary representation of the head table.
func (info *Info) Encode() (data []byte, err error) {
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
	if info.HasUnderline {
		macStyle |= 1 << 2
	}
	if info.IsOutlined {
		macStyle |= 1 << 3
	}
	if info.HasShadow {
		macStyle |= 1 << 4
	}
	if info.IsCondensed {
		macStyle |= 1 << 5
	}
	if info.IsExtended {
		macStyle |= 1 << 6
	}

	enc := &binaryHead{
		Version:           0x00010000,
		FontRevision:      uint32(info.FontRevision),
		MagicNumber:       0x5F0F3CF5,
		Flags:             flags,
		UnitsPerEm:        info.UnitsPerEm,
		Created:           encodeTime(info.Created),
		Modified:          encodeTime(info.Modified),
		XMin:              int16(info.FontBBox.LLx),
		YMin:              int16(info.FontBBox.LLy),
		XMax:              int16(info.FontBBox.URx),
		YMax:              int16(info.FontBBox.URy),
		MacStyle:          macStyle,
		LowestRecPPEM:     info.LowestRecPPEM,
		FontDirectionHint: 2,
	}

	if info.HasLongOffsets {
		enc.IndexToLocFormat = 1
	}

	buf := bytes.NewBuffer(make([]byte, 0, headLength))
	_ = binary.Write(buf, binary.BigEndian, enc)
	return buf.Bytes(), nil
}

// PatchChecksum updates the checksum of the head table.
// The argument is the checksum of the entire font before patching.
func PatchChecksum(head []byte, checksum uint32) {
	binary.BigEndian.PutUint32(head[8:12], 0xB1B0AFBA-checksum)
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

	XMin int16
	YMin int16
	XMax int16
	YMax int16

	MacStyle uint16

	LowestRecPPEM     uint16
	FontDirectionHint int16

	IndexToLocFormat int16
	GlyphDataFormat  int16
}

// Version represents the font revision in 16.16 fixed point format.
type Version uint32

func (v Version) String() string {
	return fmt.Sprintf("%.03f", float32(v)/65536)
}