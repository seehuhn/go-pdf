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

// Glyph represents a single glyph in a TrueType font.
type Glyph struct {
	funit.Rect
	data interface{} // either GlyphSimple or GlyphComposite
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
		Rect: funit.Rect{
			LLx: funit.Int16(data[2])<<8 | funit.Int16(data[3]),
			LLy: funit.Int16(data[4])<<8 | funit.Int16(data[5]),
			URx: funit.Int16(data[6])<<8 | funit.Int16(data[7]),
			URy: funit.Int16(data[8])<<8 | funit.Int16(data[9]),
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

func (g *Glyph) encodeLen() int {
	if g == nil {
		return 0
	}

	total := 10
	switch d := g.data.(type) {
	case GlyphSimple:
		total += len(d.tail)
	case GlyphComposite:
		for _, comp := range d.Components {
			total += 4 + len(comp.Args)
		}
		total += len(d.Commands)
	default:
		panic("unexpected glyph type")
	}
	for total%4 != 0 {
		total++
	}
	return total
}

func (g *Glyph) append(buf []byte) []byte {
	if g == nil {
		return buf
	}

	var numContours int16
	switch g0 := g.data.(type) {
	case GlyphSimple:
		numContours = g0.numContours
	case GlyphComposite:
		numContours = -1
	default:
		panic("unexpected glyph type")
	}

	buf = append(buf,
		byte(numContours>>8),
		byte(numContours),
		byte(g.LLx>>8),
		byte(g.LLx),
		byte(g.LLy>>8),
		byte(g.LLy),
		byte(g.URx>>8),
		byte(g.URx),
		byte(g.URy>>8),
		byte(g.URy))

	switch d := g.data.(type) {
	case GlyphSimple:
		buf = append(buf, d.tail...)
	case GlyphComposite:
		for _, comp := range d.Components {
			buf = append(buf,
				byte(comp.Flags>>8), byte(comp.Flags),
				byte(comp.GlyphIndex>>8), byte(comp.GlyphIndex))
			buf = append(buf, comp.Args...)
		}
		buf = append(buf, d.Commands...)
	default:
		panic("unexpected glyph type")
	}

	for len(buf)%4 != 0 {
		buf = append(buf, 0)
	}

	return buf
}

// Components returns the components of a composite glyph, or nil if the glyph
// is simple.
func (g *Glyph) Components() []font.GlyphID {
	if g == nil {
		return nil
	}
	switch d := g.data.(type) {
	case GlyphSimple:
		return nil
	case GlyphComposite:
		res := make([]font.GlyphID, len(d.Components))
		for i, comp := range d.Components {
			res[i] = comp.GlyphIndex
		}
		return res
	default:
		panic("unexpected glyph type")
	}
}

// FixComponents changes the glyph component IDs of a composite glyph.
func (g *Glyph) FixComponents(newGid map[font.GlyphID]font.GlyphID) *Glyph {
	if g == nil {
		return nil
	}
	switch d := g.data.(type) {
	case GlyphSimple:
		return g
	case GlyphComposite:
		d2 := GlyphComposite{
			Components: make([]GlyphComponent, len(d.Components)),
			Commands:   d.Commands,
		}
		for i, c := range d.Components {
			d2.Components[i] = GlyphComponent{
				Flags:      c.Flags,
				GlyphIndex: newGid[c.GlyphIndex],
				Args:       c.Args,
			}
		}
		g2 := &Glyph{
			Rect: g.Rect,
			data: d2,
		}
		return g2
	default:
		panic("unexpected glyph type")
	}
}

var errIncompleteGlyph = &font.InvalidFontError{
	SubSystem: "sfnt/glyf",
	Reason:    "incomplete glyph",
}
