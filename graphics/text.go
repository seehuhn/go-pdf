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

package graphics

import (
	"errors"
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/sfnt/glyph"
)

// BeginText starts a new text object.
func (p *Page) BeginText() {
	if !p.valid("BeginText", objPage) {
		return
	}
	p.currentObject = objText
	_, p.Err = fmt.Fprintln(p.Content, "BT")
}

// EndText ends the current text object.
func (p *Page) EndText() {
	if !p.valid("EndText", objText) {
		return
	}
	p.currentObject = objPage
	p.font = nil
	_, p.Err = fmt.Fprintln(p.Content, "ET")
}

// SetFont sets the font and font size.
func (p *Page) SetFont(font font.Embedded, size float64) {
	if !p.valid("SetFont", objText, objPage) {
		return
	}

	if p.Resources == nil {
		p.Resources = &pdf.Resources{}
	}
	if p.Resources.Font == nil {
		p.Resources.Font = pdf.Dict{}
	}
	name := p.resourceName(font, p.Resources.Font, "F%d")

	p.font = font
	p.fontSize = size

	err := name.PDF(p.Content)
	if err != nil {
		p.Err = err
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, "", size, "Tf")
}

// StartLine moves to the start of the next line of text.
func (p *Page) StartLine(x, y float64) {
	if !p.valid("StartLine", objText) {
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, p.coord(x), p.coord(y), "Td")
}

// StartNextLine moves to the start of the next line of text and sets
// the leading.  Usually, dy is negative.
func (p *Page) StartNextLine(dx, dy float64) {
	if !p.valid("StartNextLine", objText) {
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, p.coord(dx), p.coord(dy), "TD")
}

// NewLine moves to the start of the next line of text.
func (p *Page) NewLine() {
	if !p.valid("NewLine", objText) {
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, "T*")
}

// ShowText draws a string.
func (p *Page) ShowText(s string) {
	if !p.valid("ShowText", objText) {
		return
	}
	if p.font == nil {
		p.Err = errors.New("no font set")
		return
	}
	p.showGlyphsWithMargins(p.font.Layout(s, p.fontSize), 0, 0)
}

// ShowTextAligned draws a string and aligns it.
// The beginning is aligned in a space of width w.
// q=0 means left alignment, q=1 means right alignment
// and q=0.5 means center alignment.
func (p *Page) ShowTextAligned(s string, w, q float64) {
	if !p.valid("ShowTextAligned", objText) {
		return
	}
	if p.font == nil {
		p.Err = errors.New("no font set")
		return
	}
	p.showGlyphsAligned(p.font.Layout(s, p.fontSize), w, q)
}

// ShowGlyphs draws a sequence of glyphs.
func (p *Page) ShowGlyphs(gg glyph.Seq) {
	if !p.valid("ShowGlyphs", objText) {
		return
	}
	if p.font == nil {
		p.Err = errors.New("no font set")
		return
	}

	p.showGlyphsWithMargins(gg, 0, 0)
}

// ShowGlyphsAligned draws a sequence of glyphs and aligns it.
// The beginning of the string is shifted right by a*w+b, where w
// is the width of the string.
func (p *Page) ShowGlyphsAligned(gg glyph.Seq, w, q float64) {
	if !p.valid("ShowGlyphsAligned", objText) {
		return
	}
	if p.font == nil {
		p.Err = errors.New("no font set")
		return
	}
	p.showGlyphsAligned(gg, w, q)
}

func (p *Page) showGlyphsAligned(gg glyph.Seq, w, q float64) {
	advanceWidth := gg.AdvanceWidth()
	unitsPerEm := p.font.GetGeometry().UnitsPerEm
	total := float64(advanceWidth) * p.fontSize / float64(unitsPerEm)
	delta := w - total

	// we interpolate between the following:
	// q = 0: left = 0, right = delta
	// q = 1: left = delta, right = 0
	left := q * delta
	right := (1 - q) * delta

	p.showGlyphsWithMargins(gg, left*1000/p.fontSize, right*1000/p.fontSize)
}

func (p *Page) showGlyphsWithMargins(gg glyph.Seq, left, right float64) {
	if len(gg) == 0 {
		return
	}

	font := p.font
	geom := font.GetGeometry()
	widths := geom.Widths
	unitsPerEm := geom.UnitsPerEm
	q := 1000 / float64(unitsPerEm)

	var out pdf.Array
	var run pdf.String

	flush := func() {
		if len(run) > 0 {
			out = append(out, run)
			run = nil
		}
		if len(out) == 0 {
			return
		}
		if len(out) == 1 {
			if s, ok := out[0].(pdf.String); ok {
				if p.Err != nil {
					return
				}
				p.Err = s.PDF(p.Content)
				if p.Err != nil {
					return
				}
				_, p.Err = fmt.Fprintln(p.Content, " Tj")
				out = nil
				return
			}
		}

		if p.Err != nil {
			return
		}
		p.Err = out.PDF(p.Content)
		if p.Err != nil {
			return
		}
		_, p.Err = fmt.Fprintln(p.Content, " TJ")
		if p.Err != nil {
			return
		}
		out = nil
	}

	xOffset := left
	for _, glyph := range gg {
		xOffset += float64(glyph.XOffset) * q
		xOffsetInt := pdf.Integer(math.Round(xOffset))
		if xOffsetInt != 0 {
			if len(run) > 0 {
				out = append(out, run)
				run = nil
			}
			out = append(out, -xOffsetInt)
			xOffset -= float64(xOffsetInt)
		}

		if newYPos := pdf.Integer(math.Round(float64(glyph.YOffset) * q)); newYPos != p.textRise {
			flush()
			p.textRise = newYPos
			if p.Err != nil {
				return
			}
			p.Err = p.textRise.PDF(p.Content)
			if p.Err != nil {
				return
			}
			_, p.Err = fmt.Fprintln(p.Content, " Ts")
		}

		gid := glyph.Gid
		if int(gid) > len(widths) {
			// TODO(voss): Is this the right thing to do?
			gid = 0
		}
		run = font.AppendEncoded(run, glyph.Gid, glyph.Text)

		xOffset += float64(glyph.Advance-glyph.XOffset-widths[gid]) * q
	}

	xOffset += right
	xOffsetInt := pdf.Integer(math.Round(xOffset))
	if xOffsetInt != 0 {
		if len(run) > 0 {
			out = append(out, run)
			run = nil
		}
		out = append(out, -xOffsetInt)
	}

	flush()
}
