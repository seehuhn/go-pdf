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
	"bytes"

	"seehuhn.de/go/pdf/font"
)

// Glyph represents a single glyph in a TrueType font.
type Glyph struct {
	font.Rect
	data interface{} // either GlyphSimple or GlyphComposite
}

func decodeGlyph(data []byte) (*Glyph, error) {
	if len(data) == 0 {
		return nil, nil
	} else if len(data) < 10 {
		return nil, &font.InvalidFontError{
			SubSystem: "sfnt/glyf",
			Reason:    "incomplete glyph header",
		}
	}

	g := &Glyph{
		Rect: font.Rect{
			LLx: int16(data[2])<<8 | int16(data[3]),
			LLy: int16(data[4])<<8 | int16(data[5]),
			URx: int16(data[6])<<8 | int16(data[7]),
			URy: int16(data[8])<<8 | int16(data[9]),
		},
	}

	numCont := int16(data[0])<<8 | int16(data[1])
	if numCont >= 0 {
		g.data = GlyphSimple{
			numContours: numCont,
			tail:        data[10:],
		}
	} else {
		comp, err := decodeGlyphComposite(data[10:])
		if err != nil {
			return nil, err
		}
		g.data = *comp
	}

	return g, nil
}

// GlyphSimple is a simple glyph.
type GlyphSimple struct {
	numContours int16
	tail        []byte
}

// GlyphComposite is a composite glyph.
type GlyphComposite struct {
	Components []GlyphComponent
	Commands   []byte
}

// GlyphComponent is a single component of a composite glyph.
type GlyphComponent struct {
	Flags      uint16
	GlyphIndex font.GlyphID
	Args       []byte
}

func decodeGlyphComposite(data []byte) (*GlyphComposite, error) {
	var components []GlyphComponent
	done := false
	for !done {
		if len(data) < 4 {
			return nil, errIncompleteGlyph
		}

		flags := uint16(data[0])<<8 | uint16(data[1])
		glyphIndex := uint16(data[2])<<8 | uint16(data[3])
		data = data[4:]

		skip := 0
		if flags&0x0001 != 0 { // ARG_1_AND_2_ARE_WORDS
			skip += 4
		} else {
			skip += 2
		}
		if flags&0x0008 != 0 { // WE_HAVE_A_SCALE
			skip += 2
		} else if flags&0x0040 != 0 { // WE_HAVE_AN_X_AND_Y_SCALE
			skip += 4
		} else if flags&0x0080 != 0 { // WE_HAVE_A_TWO_BY_TWO
			skip += 8
		}
		if len(data) < skip {
			return nil, errIncompleteGlyph
		}
		args := data[:skip]
		data = data[skip:]

		components = append(components, GlyphComponent{
			Flags:      flags,
			GlyphIndex: font.GlyphID(glyphIndex),
			Args:       args,
		})

		done = flags&0x0020 == 0 // MORE_COMPONENTS
	}

	res := &GlyphComposite{
		Components: components,
		Commands:   data,
	}
	return res, nil
}

func (g *Glyph) encode() []byte {
	if g == nil {
		return nil
	}

	var numContours int16
	var tail []byte
	switch g0 := g.data.(type) {
	case GlyphSimple:
		numContours = g0.numContours
		tail = g0.tail

	case GlyphComposite:
		numContours = -1

		buf := bytes.Buffer{} // TODO(voss): avoid allocation/copying
		for _, comp := range g0.Components {
			buf.Write([]byte{
				byte(comp.Flags >> 8), byte(comp.Flags),
				byte(comp.GlyphIndex >> 8), byte(comp.GlyphIndex),
			})
			buf.Write(comp.Args)
		}
		buf.Write(g0.Commands)
		tail = buf.Bytes()

	default:
		panic("unexpected glyph type")
	}

	data := make([]byte, 10+len(tail))
	data[0] = byte(numContours >> 8)
	data[1] = byte(numContours)
	data[2] = byte(g.LLx >> 8)
	data[3] = byte(g.LLx)
	data[4] = byte(g.LLy >> 8)
	data[5] = byte(g.LLy)
	data[6] = byte(g.URx >> 8)
	data[7] = byte(g.URx)
	data[8] = byte(g.URy >> 8)
	data[9] = byte(g.URy)
	copy(data[10:], tail)
	return data
}

var errIncompleteGlyph = &font.InvalidFontError{
	SubSystem: "sfnt/glyf",
	Reason:    "incomplete glyph",
}
