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
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/sfnt/glyph"
)

// TextStart starts a new text object.
func (p *Page) TextStart() {
	if !p.valid("TextStart", objPage) {
		return
	}
	p.currentObject = objText
	_, p.Err = fmt.Fprintln(p.Content, "BT")
}

// TextEnd ends the current text object.
func (p *Page) TextEnd() {
	if !p.valid("TextEnd", objText) {
		return
	}
	p.currentObject = objPage
	_, p.Err = fmt.Fprintln(p.Content, "ET")
}

// TextSetFont sets the font and font size.
//
// TODO(voss): Instead of font.Embedded, use only information which can
// be extracted from a PDF file?
func (p *Page) TextSetFont(font font.Embedded, size float64) {
	if !p.valid("TextSetFont", objText, objPage) {
		return
	}

	if p.Resources.Font == nil {
		p.Resources.Font = pdf.Dict{}
	}
	name := p.resourceName(font, p.Resources.Font, "F%d")

	if p.isSet(StateFont) && p.state.Font == name && p.state.FontSize == size {
		return
	}
	p.state.Font = name
	p.state.FontSize = size
	p.font = font
	p.set |= StateFont

	err := name.PDF(p.Content)
	if err != nil {
		p.Err = err
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, "", size, "Tf")
}

// TextFirstLine moves to the start of the next line of text.
func (p *Page) TextFirstLine(x, y float64) {
	if !p.valid("TextFirstLine", objText) {
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, p.coord(x), p.coord(y), "Td")
}

// TextSecondLine moves to the start of the next line of text and sets
// the leading.  Usually, dy is negative.
func (p *Page) TextSecondLine(dx, dy float64) {
	if !p.valid("TextSecondLine", objText) {
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, p.coord(dx), p.coord(dy), "TD")
}

// TextNextLine moves to the start of the next line of text.
func (p *Page) TextNextLine() {
	if !p.valid("TextNewLine", objText) {
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, "T*")
}

// TextLayout returns the glyph sequence for a string.
// The function panics if no font is set.
func (p *Page) TextLayout(s string) glyph.Seq {
	return p.font.Layout(s, p.state.FontSize)
}

// TextShow draws a string.
func (p *Page) TextShow(s string) float64 {
	if !p.valid("TextShow", objText) {
		return 0
	}
	if p.font == nil {
		p.Err = errors.New("no font set")
		return 0
	}
	gg := p.TextLayout(s)
	return p.showGlyphsWithMargins(gg, 0, 0)
}

// TextShowAligned draws a string and aligns it.
// The beginning is aligned in a space of width w.
// q=0 means left alignment, q=1 means right alignment
// and q=0.5 means center alignment.
func (p *Page) TextShowAligned(s string, w, q float64) {
	if !p.valid("TextShowAligned", objText) {
		return
	}
	if p.font == nil {
		p.Err = errors.New("no font set")
		return
	}
	p.showGlyphsAligned(p.TextLayout(s), w, q)
}

// TextShowGlyphs draws a sequence of glyphs.
func (p *Page) TextShowGlyphs(gg glyph.Seq) float64 {
	if !p.valid("TextShowGlyphs", objText) {
		return 0
	}
	if p.font == nil {
		p.Err = errors.New("no font set")
		return 0
	}

	return p.showGlyphsWithMargins(gg, 0, 0)
}

// TextShowGlyphsAligned draws a sequence of glyphs and aligns it.
func (p *Page) TextShowGlyphsAligned(gg glyph.Seq, w, q float64) {
	if !p.valid("TextShowGlyphsAligned", objText) {
		return
	}
	if p.font == nil {
		p.Err = errors.New("no font set")
		return
	}
	p.showGlyphsAligned(gg, w, q)
}

func (p *Page) showGlyphsAligned(gg glyph.Seq, w, q float64) {
	geom := p.font.GetGeometry()
	total := geom.ToPDF(p.state.FontSize, gg.AdvanceWidth())
	delta := w - total

	// we interpolate between the following:
	// q = 0: left = 0, right = delta
	// q = 1: left = delta, right = 0
	left := q * delta
	right := (1 - q) * delta

	p.showGlyphsWithMargins(gg, left*1000/p.state.FontSize, right*1000/p.state.FontSize)
}

func (p *Page) showGlyphsWithMargins(gg glyph.Seq, left, right float64) float64 {
	if len(gg) == 0 {
		return 0
	}

	var run pdf.String
	var out pdf.Array
	flush := func() {
		if len(run) > 0 {
			out = append(out, run)
			run = nil
		}
		if len(out) == 0 {
			return
		}

		if p.Err != nil {
			return
		}
		if len(out) == 1 {
			if s, ok := out[0].(pdf.String); ok {
				p.Err = s.PDF(p.Content)
				if p.Err != nil {
					return
				}
				_, p.Err = fmt.Fprintln(p.Content, " Tj")
				out = nil
				return
			}
		}

		p.Err = out.PDF(p.Content)
		if p.Err != nil {
			return
		}
		_, p.Err = fmt.Fprintln(p.Content, " TJ")
		out = nil
	}

	font := p.font
	geom := font.GetGeometry()
	widths := geom.Widths
	unitsPerEm := geom.UnitsPerEm
	q := 1000 / float64(unitsPerEm)

	// We track the actual and wanted x-position in PDF units,
	// relative to the initial x-position.
	xWanted := left
	xActual := 0.0
	for _, glyph := range gg {
		xWanted += float64(glyph.XOffset) * q
		xOffsetInt := pdf.Integer(math.Round(xWanted - xActual))
		if xOffsetInt != 0 {
			if len(run) > 0 {
				out = append(out, run)
				run = nil
			}
			out = append(out, -xOffsetInt)
			xActual += float64(xOffsetInt)
		}

		if newYPos := float64(glyph.YOffset) * q; p.set&StateTextRise == 0 || newYPos != p.state.TextRise {
			flush()
			p.state.TextRise = newYPos
			if p.Err != nil {
				return 0
			}
			p.Err = pdf.Number(p.state.TextRise).PDF(p.Content) // TODO(voss): rounding?
			if p.Err != nil {
				return 0
			}
			_, p.Err = fmt.Fprintln(p.Content, " Ts")
		}

		run = font.AppendEncoded(run, glyph.Gid, glyph.Text)

		var w funit.Int16
		if gid := glyph.Gid; int(gid) < len(widths) {
			w = widths[gid]
		}
		xActual += float64(w) * q
		xWanted += float64(-glyph.XOffset+glyph.Advance) * q
	}

	xWanted += right
	xOffsetInt := pdf.Integer(math.Round(xWanted - xActual))
	if xOffsetInt != 0 {
		if len(run) > 0 {
			out = append(out, run)
			run = nil
		}
		out = append(out, -xOffsetInt)
		xActual += float64(xOffsetInt)
	}

	flush()
	return xActual * p.state.FontSize / 1000
}
