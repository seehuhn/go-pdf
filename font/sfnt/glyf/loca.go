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

package glyf

import (
	"fmt"

	"seehuhn.de/go/pdf/font"
)

func decodeLoca(enc *Encoded) ([]int, error) {
	var offs []int
	switch enc.LocaFormat {
	case 0:
		n := len(enc.LocaData)
		if n < 4 || n%2 != 0 {
			return nil, &font.InvalidFontError{
				SubSystem: "sfnt/loca",
				Reason:    "invalid table length",
			}
		}
		offs = make([]int, n/2)
		prev := 0
		for i := range offs {
			x := int(enc.LocaData[2*i])<<8 + int(enc.LocaData[2*i+1])
			pos := 2 * x
			if pos < prev || pos > len(enc.GlyfData) {
				return nil, &font.InvalidFontError{
					SubSystem: "sfnt/loca",
					Reason:    fmt.Sprintf("invalid offset %d", pos),
				}
			}
			offs[i] = pos
			prev = pos
		}
	case 1:
		n := len(enc.LocaData)
		if n < 8 || n%4 != 0 {
			return nil, &font.InvalidFontError{
				SubSystem: "sfnt/loca",
				Reason:    "invalid table length",
			}
		}
		offs = make([]int, len(enc.LocaData)/4)
		prev := 0
		for i := range offs {
			pos := int(enc.LocaData[4*i])<<24 + int(enc.LocaData[4*i+1])<<16 +
				int(enc.LocaData[4*i+2])<<8 + int(enc.LocaData[4*i+3])
			if pos < prev || pos > len(enc.GlyfData) {
				return nil, &font.InvalidFontError{
					SubSystem: "sfnt/loca",
					Reason:    "invalid offset",
				}
			}
			offs[i] = pos
			prev = pos
		}
	default:
		return nil, &font.NotSupportedError{
			SubSystem: "sfnt/loca",
			Feature:   fmt.Sprintf("loca table format %d", enc.LocaFormat),
		}
	}
	return offs, nil
}

func encodeLoca(offs []int) ([]byte, int16) {
	var locaData []byte
	var locaFormat int16
	if offs[len(offs)-1] <= 0xffff {
		locaFormat = 0
		locaData = make([]byte, 2*len(offs))
		for i, off := range offs {
			x := off / 2
			locaData[2*i] = byte(x >> 8)
			locaData[2*i+1] = byte(x)
		}
	} else {
		locaFormat = 1
		locaData = make([]byte, 4*len(offs))
		for i, off := range offs {
			locaData[4*i] = byte(off >> 24)
			locaData[4*i+1] = byte(off >> 16)
			locaData[4*i+2] = byte(off >> 8)
			locaData[4*i+3] = byte(off)
		}
	}
	return locaData, locaFormat
}
