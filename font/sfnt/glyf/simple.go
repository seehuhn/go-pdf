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
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/funit"
)

// SimpleGlyph is a simple glyph.
type SimpleGlyph struct {
	NumContours int16
	tail        []byte
}

// A Point is a point in a glyph outline
type Point struct {
	X, Y    funit.Int16
	OnCurve bool
}

// A Contour describes a connected part of a glyph outline.
type Contour []Point

// GlyphInfo contains the contours of a SimpleGlyph.
type GlyphInfo struct {
	Contours     []Contour
	Instructions []byte
}

// Decode returns the contours of a glyph.
func (glyph SimpleGlyph) Decode() (*GlyphInfo, error) {
	buf := glyph.tail

	numContours := int(glyph.NumContours)
	if len(buf) < 2*numContours+2 {
		return nil, errInvalidGlyphData
	}
	endPtsOfContours := make([]uint16, numContours)
	for i := 0; i < numContours; i++ {
		endPtsOfContours[i] = uint16(buf[2*i])<<8 | uint16(buf[2*i+1])
	}
	buf = buf[2*numContours:]
	numPoints := int(endPtsOfContours[numContours-1]) + 1

	instructionLength := int(buf[0])<<8 | int(buf[1])
	if len(buf) < 2+instructionLength {
		return nil, errInvalidGlyphData
	}
	instructions := buf[2 : 2+instructionLength]
	buf = buf[2+instructionLength:]

	// decode the flags
	ff := make([]byte, numPoints)
	i := 0
	for i < numPoints {
		if len(buf) < 1 {
			return nil, errInvalidGlyphData
		}
		flags := buf[0]
		buf = buf[1:]
		ff[i] = flags
		i++
		if flags&0x08 != 0 {
			if len(buf) < 1 {
				return nil, errInvalidGlyphData
			}
			count := buf[0]
			buf = buf[1:]
			for count > 0 && i < numPoints {
				ff[i] = flags
				i++
				count--
			}
		}
	}

	// decode the x-coordinates
	xx := make([]funit.Int16, numPoints)
	var x funit.Int16
	for i, flags := range ff {
		if flags&0x02 != 0 { // X_SHORT_VECTOR
			if len(buf) < 1 {
				return nil, errInvalidGlyphData
			}
			dx := funit.Int16(buf[0])
			buf = buf[1:]
			if flags&0x10 != 0 { // X_IS_SAME_OR_POSITIVE_X_SHORT_VECTOR
				x += dx
			} else {
				x -= dx
			}
		} else if flags&0x10 == 0 { // !X_IS_SAME_OR_POSITIVE_X_SHORT_VECTOR
			if len(buf) < 2 {
				return nil, errInvalidGlyphData
			}
			dx := funit.Int16(buf[0])<<8 | funit.Int16(buf[1])
			buf = buf[2:]
			x += dx
		}
		xx[i] = x
	}

	// decode the y-coordinates
	yy := make([]funit.Int16, numPoints)
	var y funit.Int16
	for i, flags := range ff {
		if flags&0x04 != 0 { // Y_SHORT_VECTOR
			if len(buf) < 1 {
				return nil, errInvalidGlyphData
			}
			dy := funit.Int16(buf[0])
			buf = buf[1:]
			if flags&0x20 != 0 { // Y_IS_SAME_OR_POSITIVE_Y_SHORT_VECTOR
				y += dy
			} else {
				y -= dy
			}
		} else if flags&0x20 == 0 { // !Y_IS_SAME_OR_POSITIVE_Y_SHORT_VECTOR
			if len(buf) < 2 {
				return nil, errInvalidGlyphData
			}
			dy := funit.Int16(buf[0])<<8 | funit.Int16(buf[1])
			buf = buf[2:]
			y += dy
		}
		yy[i] = y
	}

	cc := make([]Contour, numContours)
	start := 0
	for i := 0; i < numContours; i++ {
		end := int(endPtsOfContours[i]) + 1
		pp := make([]Point, end-start)
		for j := start; j < end; j++ {
			pp[j-start] = Point{xx[j], yy[j], ff[j]&0x01 != 0}
		}
		start = end

		cc[i] = pp
	}

	res := &GlyphInfo{
		Contours:     cc,
		Instructions: instructions,
	}

	return res, nil
}

var errInvalidGlyphData = &font.InvalidFontError{
	SubSystem: "sfnt/glyf",
	Reason:    "invalid glyph data",
}
