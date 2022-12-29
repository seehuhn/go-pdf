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
	"seehuhn.de/go/pdf/sfnt/glyph"
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
	oldFont, ok := p.resources.Font[font.InstName].(*pdf.Reference)
	if ok && *oldFont != *font.Ref {
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
	p.ShowTextAligned(s, 0, 0)
}

// ShowTextAligned draws a string and aligns it.
// The beginning of the string is shifted right by a*w+b, where w
// is the width of the string.
func (p *Page) ShowTextAligned(s string, a, b float64) {
	if !p.valid("ShowTextAligned", stateText) {
		return
	}

	font := p.font
	if font == nil {
		p.err = errors.New("no font set")
		return
	}
	gg := font.Typeset(s, p.fontSize)
	p.ShowGlyphsAligned(gg, a, b)
}

// ShowGlyphs draws a sequence of glyphs.
func (p *Page) ShowGlyphs(gg []glyph.Info) {
	p.ShowGlyphsAligned(gg, 0, 0)
}

// ShowGlyphsAligned draws a sequence of glyphs and aligns it.
// The beginning of the string is shifted right by a*w+b, where w
// is the width of the string.
func (p *Page) ShowGlyphsAligned(gg []glyph.Info, a, b float64) {
	if !p.valid("ShowGlyphsAligned", stateText) {
		return
	}
	font := p.font
	if font == nil {
		p.err = errors.New("no font set")
		return
	}

	if len(gg) == 0 {
		return
	}

	q := 1000 / float64(font.UnitsPerEm)

	leftOffset := 1000 * b / p.fontSize
	if a != 0 {
		totalWidth := 0.0
		for _, g := range gg {
			totalWidth += float64(g.Advance)
		}
		leftOffset += a * totalWidth * q
	}

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

	xOffset := leftOffset
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

		xOffset += (float64(glyph.Advance) - float64(glyph.XOffset) - float64(font.Widths[gid])) * q
	}
	flush()
}
