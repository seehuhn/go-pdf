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
	Data interface{} // either SimpleGlyph or CompositeGlyph
}

// CompositeGlyph is a composite glyph.
type CompositeGlyph struct {
	Components   []GlyphComponent
	Instructions []byte
}

// GlyphComponent is a single component of a composite glyph.
type GlyphComponent struct {
	Flags      uint16
	GlyphIndex font.GlyphID
	Args       []byte
}

// Note that decodeGlyph retains sub-slices of data.
func decodeGlyph(data []byte) (*Glyph, error) {
	if len(data) == 0 {
		return nil, nil
	} else if len(data) < 10 {
		return nil, &font.InvalidFontError{
			SubSystem: "sfnt/glyf",
			Reason:    "incomplete glyph header",
		}
	}

	var glyphData interface{}
	numCont := int16(data[0])<<8 | int16(data[1])
	if numCont >= 0 {
		simple := SimpleGlyph{
			NumContours: numCont,
			Tail:        data[10:],
		}
		err := simple.removePadding()
		if err != nil {
			return nil, err
		}
		glyphData = simple
	} else {
		comp, err := decodeGlyphComposite(data[10:])
		if err != nil {
			return nil, err
		}
		glyphData = *comp
	}

	g := &Glyph{
		Rect: funit.Rect{
			LLx: funit.Int16(data[2])<<8 | funit.Int16(data[3]),
			LLy: funit.Int16(data[4])<<8 | funit.Int16(data[5]),
			URx: funit.Int16(data[6])<<8 | funit.Int16(data[7]),
			URy: funit.Int16(data[8])<<8 | funit.Int16(data[9]),
		},
		Data: glyphData,
	}
	return g, nil
}

func decodeGlyphComposite(data []byte) (*CompositeGlyph, error) {
	var components []GlyphComponent
	done := false
	weHaveInstructions := false
	for !done {
		if len(data) < 4 {
			return nil, errIncompleteGlyph
		}

		flags := uint16(data[0])<<8 | uint16(data[1])
		glyphIndex := uint16(data[2])<<8 | uint16(data[3])
		data = data[4:]

		if flags&0x0100 != 0 { // WE_HAVE_INSTRUCTIONS
			weHaveInstructions = true
		}

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

	if weHaveInstructions && len(data) >= 2 {
		L := int(data[0])<<8 | int(data[1])
		data = data[2:]
		if len(data) > L {
			data = data[:L]
		}
	} else {
		data = nil
	}

	res := &CompositeGlyph{
		Components:   components,
		Instructions: data,
	}
	return res, nil
}

func (g *Glyph) encodeLen() int {
	if g == nil {
		return 0
	}

	total := 10
	switch d := g.Data.(type) {
	case SimpleGlyph:
		total += len(d.Tail)
	case CompositeGlyph:
		for _, comp := range d.Components {
			total += 4 + len(comp.Args)
		}
		if d.Instructions != nil {
			total += 2 + len(d.Instructions)
		}
	default:
		panic("unexpected glyph type")
	}
	for total%glyfAlign != 0 {
		total++
	}
	return total
}

func (g *Glyph) append(buf []byte) []byte {
	if g == nil {
		return buf
	}

	var numContours int16
	switch g0 := g.Data.(type) {
	case SimpleGlyph:
		numContours = g0.NumContours
	case CompositeGlyph:
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

	switch d := g.Data.(type) {
	case SimpleGlyph:
		buf = append(buf, d.Tail...)
	case CompositeGlyph:
		for _, comp := range d.Components {
			buf = append(buf,
				byte(comp.Flags>>8), byte(comp.Flags),
				byte(comp.GlyphIndex>>8), byte(comp.GlyphIndex))
			buf = append(buf, comp.Args...)
		}
		if d.Instructions != nil {
			L := len(d.Instructions)
			buf = append(buf, byte(L>>8), byte(L))
			buf = append(buf, d.Instructions...)
		}
	default:
		panic("unexpected glyph type")
	}

	for len(buf)%glyfAlign != 0 {
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
	switch d := g.Data.(type) {
	case SimpleGlyph:
		return nil
	case CompositeGlyph:
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
	switch d := g.Data.(type) {
	case SimpleGlyph:
		return g
	case CompositeGlyph:
		d2 := CompositeGlyph{
			Components:   make([]GlyphComponent, len(d.Components)),
			Instructions: d.Instructions,
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
			Data: d2,
		}
		return g2
	default:
		panic("unexpected glyph type")
	}
}

const glyfAlign = 2

var errIncompleteGlyph = &font.InvalidFontError{
	SubSystem: "sfnt/glyf",
	Reason:    "incomplete glyph",
}
