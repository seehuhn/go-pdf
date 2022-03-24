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

// Package classdef reads and writes OpenType "Class Definition Tables".
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#classDefTbl
package classdef

import (
	"fmt"
	"io"

	"seehuhn.de/go/pdf/font"
)

// Info containst the information from an OpenType "Class Definition Table".
type Info map[font.GlyphID]uint16

type reader struct {
	r          io.Reader
	buf        []byte
	a, b       int
	readAtMost int
}

func (r *reader) ReadBytes(nBytes int) ([]byte, error) {
	if nBytes > len(r.buf) {
		panic("buffer too small")
	}
	for r.a+nBytes > r.b {
		if r.a > 0 && r.b > r.a {
			copy(r.buf, r.buf[r.a:r.b])
			r.b -= r.a
			r.a = 0
		}

		d := len(r.buf) - r.b
		if d > r.readAtMost {
			d = r.readAtMost
		}
		n, err := r.r.Read(r.buf[r.b : r.b+d])
		if n == 0 {
			if err == nil || err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return nil, err
		}
		r.b += n
		r.readAtMost -= n
	}
	res := r.buf[r.a : r.a+nBytes]
	r.a += nBytes
	return res, nil
}

func (r *reader) ReadUint16() (uint16, error) {
	data, err := r.ReadBytes(2)
	if err != nil {
		return 0, err
	}
	return uint16(data[0])<<8 | uint16(data[1]), nil
}

// Read reads and decodes an OpenType "Class Definition Table".
// The buffer `buf`, if non-nil, is used as scratch space.
func Read(r io.Reader, buf []byte) (Info, error) {
	if len(buf) < 16 {
		buf = make([]byte, 256)
	}
	tab := &reader{r: r, buf: buf}

	tab.readAtMost = 4
	version, err := tab.ReadUint16()
	if err != nil {
		return nil, err
	}
	switch version {
	case 1:
		tab.readAtMost += 2
		data, err := tab.ReadBytes(4)
		if err != nil {
			return nil, err
		}
		startGlyphID := font.GlyphID(data[0])<<8 | font.GlyphID(data[1])
		glyphCount := int(data[2])<<8 | int(data[3])
		if int(startGlyphID)+int(glyphCount)-1 > 0xFFFF {
			return nil, &font.InvalidFontError{
				SubSystem: "opentype/classdef",
				Reason:    "glyph count too large in class definition table",
			}
		}

		res := make(Info, glyphCount)
		tab.readAtMost += 2 * glyphCount

		for i := 0; i < glyphCount; i++ {
			classValue, err := tab.ReadUint16()
			if err != nil {
				return nil, err
			}
			if classValue != 0 {
				res[startGlyphID+font.GlyphID(i)] = classValue
			}
		}
		return res, nil

	case 2:
		classRangeCount, err := tab.ReadUint16()
		if err != nil {
			return nil, err
		}

		res := Info{}
		tab.readAtMost += 6 * int(classRangeCount)
		var prevEnd font.GlyphID
		for i := 0; i < int(classRangeCount); i++ {
			data, err := tab.ReadBytes(6)
			if err != nil {
				return nil, err
			}
			startGlyphID := font.GlyphID(data[0])<<8 | font.GlyphID(data[1])
			endGlyphID := font.GlyphID(data[2])<<8 | font.GlyphID(data[3])
			classValue := uint16(data[4])<<8 | uint16(data[5])

			if i > 0 && startGlyphID <= prevEnd {
				return nil, &font.InvalidFontError{
					SubSystem: "opentype/classdef",
					Reason:    "overlapping ranges in class definition table",
				}
			}
			prevEnd = endGlyphID

			if classValue != 0 {
				for j := int(startGlyphID); j <= int(endGlyphID); j++ {
					res[font.GlyphID(j)] = classValue
				}
			}
		}
		return res, nil

	default:
		return nil, &font.NotSupportedError{
			SubSystem: "opentype/classdef",
			Feature:   fmt.Sprintf("class definition table version %d", version),
		}
	}
}

// Encode converts the class definition table to binary format.
func (info Info) Encode() []byte {
	if len(info) == 0 {
		return []byte{0, 2, 0, 0}
	}

	minGid := font.GlyphID(0xFFFF)
	maxGid := font.GlyphID(0)
	for key := range info {
		if key < minGid {
			minGid = key
		}
		if key > maxGid {
			maxGid = key
		}
	}

	format1Size := 6 + 2*(int(maxGid)-int(minGid)+1)

	format2Size := 4
	segCount := 0
	segStart := -1
	var segClass uint16
	for i := int(minGid); i <= int(maxGid) && format2Size < format1Size; i++ {
		class := info[font.GlyphID(i)]

		if segStart >= 0 && class != segClass {
			format2Size += 6
			segCount++
			segStart = -1
		}
		if segStart == -1 {
			if class != 0 {
				segStart = i
				segClass = class
			}
		}
	}
	if segStart >= 0 {
		segCount++
		format2Size += 6
	}

	for format1Size <= format2Size {
		buf := make([]byte, format1Size)
		// buf[0] = 0
		buf[1] = 1
		buf[2] = byte(minGid >> 8)
		buf[3] = byte(minGid)
		count := maxGid - minGid + 1
		buf[4] = byte(count >> 8)
		buf[5] = byte(count)
		for i := 0; i < int(count); i++ {
			class := info[minGid+font.GlyphID(i)]
			buf[6+2*i] = byte(class >> 8)
			buf[6+2*i+1] = byte(class)
		}
		return buf
	}

	buf := make([]byte, format2Size)
	// buf[0] = 0
	buf[1] = 2
	buf[2] = byte(segCount >> 8)
	buf[3] = byte(segCount)
	pos := 4
	segStart = -1
	for i := int(minGid); i <= int(maxGid); i++ {
		class := info[font.GlyphID(i)]

		if segStart >= 0 && class != segClass {
			buf[pos] = byte(segStart >> 8)
			buf[pos+1] = byte(segStart)
			buf[pos+2] = byte((i - 1) >> 8)
			buf[pos+3] = byte(i - 1)
			buf[pos+4] = byte(segClass >> 8)
			buf[pos+5] = byte(segClass)

			pos += 6
			segStart = -1
		}
		if segStart == -1 {
			if class != 0 {
				segStart = i
				segClass = class
			}
		}
	}
	if segStart >= 0 {
		buf[pos] = byte(segStart >> 8)
		buf[pos+1] = byte(segStart)
		buf[pos+2] = byte(maxGid >> 8)
		buf[pos+3] = byte(maxGid)
		buf[pos+4] = byte(segClass >> 8)
		buf[pos+5] = byte(segClass)
	}
	return buf
}
