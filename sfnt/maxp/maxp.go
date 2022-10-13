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

// Package maxp reads and writes "maxp" tables.
// https://docs.microsoft.com/en-us/typography/opentype/spec/maxp
package maxp

import (
	"errors"
	"io"
)

// Info contains information from the "maxp" table.
type Info struct {
	// NumGlyphs is number of glyphs in the font, in the range 1, ..., 65535.
	NumGlyphs int

	// TTF contains additional information for TrueType fonts.
	// This must be nil for CFF-based fonts.
	TTF *TTFInfo
}

// TTFInfo contains TrueType-specific information from the "maxp" table.
type TTFInfo struct {
	MaxPoints             uint16
	MaxContours           uint16
	MaxCompositePoints    uint16
	MaxCompositeContours  uint16
	MaxZones              uint16
	MaxTwilightPoints     uint16
	MaxStorage            uint16
	MaxFunctionDefs       uint16
	MaxInstructionDefs    uint16
	MaxStackElements      uint16
	MaxSizeOfInstructions uint16
	MaxComponentElements  uint16
	MaxComponentDepth     uint16
}

// Read reads the "maxp" table.
func Read(r io.Reader) (*Info, error) {
	var buf [26]byte
	_, err := io.ReadFull(r, buf[:6])
	if err != nil {
		return nil, err
	}

	version := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
	if version != 0x00005000 && version != 0x00010000 {
		return nil, errors.New("sfnt/maxp: unknown version")
	}

	numGlyphs := int(buf[4])<<8 | int(buf[5])
	if numGlyphs == 0 {
		return nil, errors.New("sfnt/maxp: numGlyphs is zero")
	}
	info := &Info{
		NumGlyphs: numGlyphs,
	}
	if version == 0x00005000 {
		return info, nil
	}

	_, err = io.ReadFull(r, buf[:26])
	if err != nil {
		return nil, err
	}
	info.TTF = &TTFInfo{
		MaxPoints:             uint16(buf[0])<<8 | uint16(buf[1]),
		MaxContours:           uint16(buf[2])<<8 | uint16(buf[3]),
		MaxCompositePoints:    uint16(buf[4])<<8 | uint16(buf[5]),
		MaxCompositeContours:  uint16(buf[6])<<8 | uint16(buf[7]),
		MaxZones:              uint16(buf[8])<<8 | uint16(buf[9]),
		MaxTwilightPoints:     uint16(buf[10])<<8 | uint16(buf[11]),
		MaxStorage:            uint16(buf[12])<<8 | uint16(buf[13]),
		MaxFunctionDefs:       uint16(buf[14])<<8 | uint16(buf[15]),
		MaxInstructionDefs:    uint16(buf[16])<<8 | uint16(buf[17]),
		MaxStackElements:      uint16(buf[18])<<8 | uint16(buf[19]),
		MaxSizeOfInstructions: uint16(buf[20])<<8 | uint16(buf[21]),
		MaxComponentElements:  uint16(buf[22])<<8 | uint16(buf[23]),
		MaxComponentDepth:     uint16(buf[24])<<8 | uint16(buf[25]),
	}
	return info, nil
}

// Encode encodes the "maxp" table.
func (info *Info) Encode() []byte {
	numGlyphs := info.NumGlyphs
	if numGlyphs < 1 || numGlyphs >= 1<<16 {
		panic("sfnt/maxp: numGlyphs out of range")
	}
	if info.TTF == nil {
		buf := []byte{
			0x00, 0x00, 0x50, 0x00, byte(numGlyphs >> 8), byte(numGlyphs),
		}
		return buf
	}

	ttf := info.TTF
	buf := []byte{
		0x00, 0x01, 0x00, 0x00, // version
		byte(numGlyphs >> 8), byte(numGlyphs),
		byte(ttf.MaxPoints >> 8), byte(ttf.MaxPoints),
		byte(ttf.MaxContours >> 8), byte(ttf.MaxContours),
		byte(ttf.MaxCompositePoints >> 8), byte(ttf.MaxCompositePoints),
		byte(ttf.MaxCompositeContours >> 8), byte(ttf.MaxCompositeContours),
		byte(ttf.MaxZones >> 8), byte(ttf.MaxZones),
		byte(ttf.MaxTwilightPoints >> 8), byte(ttf.MaxTwilightPoints),
		byte(ttf.MaxStorage >> 8), byte(ttf.MaxStorage),
		byte(ttf.MaxFunctionDefs >> 8), byte(ttf.MaxFunctionDefs),
		byte(ttf.MaxInstructionDefs >> 8), byte(ttf.MaxInstructionDefs),
		byte(ttf.MaxStackElements >> 8), byte(ttf.MaxStackElements),
		byte(ttf.MaxSizeOfInstructions >> 8), byte(ttf.MaxSizeOfInstructions),
		byte(ttf.MaxComponentElements >> 8), byte(ttf.MaxComponentElements),
		byte(ttf.MaxComponentDepth >> 8), byte(ttf.MaxComponentDepth),
	}
	return buf
}
