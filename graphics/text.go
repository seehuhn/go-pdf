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
	if !p.valid("BeginText", stateGlobal) {
		return
	}
	p.state = stateText
	_, p.err = fmt.Fprintln(p.content, "BT")
}

// EndText ends the current text object.
func (p *Page) EndText() {
	if !p.valid("EndText", stateText) {
		return
	}
	p.state = stateGlobal
	p.font = nil
	_, p.err = fmt.Fprintln(p.content, "ET")
}

// StartLine moves to the start of the next line of text.
func (p *Page) StartLine(x, y float64) {
	if !p.valid("StartLine", stateText) {
		return
	}
	_, p.err = fmt.Fprintln(p.content, p.coord(x), p.coord(y), "Td")
}

// StartNextLine moves to the start of the next line of text and sets
// the leading.  Usually, dy is negative.
func (p *Page) StartNextLine(dx, dy float64) {
	if !p.valid("StartNextLine", stateText) {
		return
	}
	_, p.err = fmt.Fprintln(p.content, p.coord(dx), p.coord(dy), "TD")
}

// NewLine moves to the start of the next line of text.
func (p *Page) NewLine() {
	if !p.valid("NewLine", stateText) {
		return
	}
	_, p.err = fmt.Fprintln(p.content, "T*")
}

// SetFont sets the font and font size.
func (p *Page) SetFont(font *font.Font, size float64) {
	if !p.valid("SetFont", stateText) {
		return
	}

	if p.resources == nil {
		p.resources = &pdf.Resources{}
	}
	if p.resources.Font == nil {
		p.resources.Font = pdf.Dict{}
	}
	prevFont, ok := p.resources.Font[font.InstName].(*pdf.Reference)
	if ok && *prevFont != *font.Ref {
		p.err = fmt.Errorf("font %q already defined", font.InstName)
		return
	}
	p.resources.Font[font.InstName] = font.Ref

	p.font = font
	p.fontSize = size
	err := font.InstName.PDF(p.content)
	if err != nil {
		p.err = err
		return
	}
	_, p.err = fmt.Fprintln(p.content, "", size, "Tf")
}

// ShowText draws a string.
func (p *Page) ShowText(s string) {
	if !p.valid("ShowText", stateText) {
		return
	}
	if p.font == nil {
		p.err = errors.New("no font set")
		return
	}
	p.showGlyphsWithMargins(p.font.Typeset(s, p.fontSize), 0, 0)
}

// ShowTextAligned draws a string and aligns it.
// The beginning is aligned in a space of width w.
// q=0 means left alignment, q=1 means right alignment
// and q=0.5 means center alignment.
func (p *Page) ShowTextAligned(s string, w, q float64) {
	if !p.valid("ShowTextAligned", stateText) {
		return
	}
	if p.font == nil {
		p.err = errors.New("no font set")
		return
	}
	p.showGlyphsAligned(p.font.Typeset(s, p.fontSize), w, q)
}

// ShowGlyphs draws a sequence of glyphs.
func (p *Page) ShowGlyphs(gg glyph.Seq) {
	if !p.valid("ShowGlyphs", stateText) {
		return
	}
	if p.font == nil {
		p.err = errors.New("no font set")
		return
	}

	p.showGlyphsWithMargins(gg, 0, 0)
}

// ShowGlyphsAligned draws a sequence of glyphs and aligns it.
// The beginning of the string is shifted right by a*w+b, where w
// is the width of the string.
func (p *Page) ShowGlyphsAligned(gg glyph.Seq, w, q float64) {
	if !p.valid("ShowGlyphsAligned", stateText) {
		return
	}
	if p.font == nil {
		p.err = errors.New("no font set")
		return
	}
	p.showGlyphsAligned(gg, w, q)
}

func (p *Page) showGlyphsAligned(gg glyph.Seq, w, q float64) {
	total := p.font.ToPDF(p.fontSize, gg.AdvanceWidth())
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
				if p.err != nil {
					return
				}
				p.err = s.PDF(p.content)
				if p.err != nil {
					return
				}
				_, p.err = fmt.Fprintln(p.content, " Tj")
				out = nil
				return
			}
		}

		if p.err != nil {
			return
		}
		p.err = out.PDF(p.content)
		if p.err != nil {
			return
		}
		_, p.err = fmt.Fprintln(p.content, " TJ")
		if p.err != nil {
			return
		}
		out = nil
	}

	xOffset := left
	for _, glyph := range gg {
		xOffset += font.ToPDF16(1000, glyph.XOffset)
		xOffsetInt := pdf.Integer(math.Round(xOffset))
		if xOffsetInt != 0 {
			if len(run) > 0 {
				out = append(out, run)
				run = nil
			}
			out = append(out, -xOffsetInt)
			xOffset -= float64(xOffsetInt)
		}

		if newYPos := pdf.Integer(math.Round(font.ToPDF16(1000, glyph.YOffset))); newYPos != p.textRise {
			flush()
			p.textRise = newYPos
			if p.err != nil {
				return
			}
			p.err = p.textRise.PDF(p.content)
			if p.err != nil {
				return
			}
			_, p.err = fmt.Fprintln(p.content, " Ts")
		}

		gid := glyph.Gid
		if int(gid) >= len(font.Widths) {
			gid = 0
		}
		run = append(run, font.Enc(gid)...)

		xOffset += font.ToPDF16(1000, glyph.Advance-glyph.XOffset-font.Widths[gid])
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
