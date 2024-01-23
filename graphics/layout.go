// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/sfnt/glyph"
)

func (w *Writer) TextShowString(s string) {
	F := w.State.TextFont.(font.Layouter)
	gg := F.Layout(s)
	_, pdfGlyphs := convertGlyphs(gg, F.FontMatrix(), w.State.TextFontSize)
	w.TextShowGlyphsRaw(pdfGlyphs)
}

// TextLayout returns the glyph sequence for a string.
// The function panics if no font is set.
func (w *Writer) TextLayout(s string) glyph.Seq {
	st := w.State
	return st.TextFont.(font.Layouter).Layout(s)
}

// TextShow draws a string.
func (w *Writer) TextShow(s string) float64 {
	if !w.isValid("TextShow", objText) {
		return 0
	}
	if w.State.TextFont == nil {
		w.Err = errors.New("no font set")
		return 0
	}
	gg := w.TextLayout(s)
	return w.showGlyphsWithMargins(gg, 0, 0)
}

// TextShowAligned draws a string and aligns it.
// The beginning is aligned in a space of the given width.
// q=0 means left alignment, q=1 means right alignment
// and q=0.5 means center alignment.
func (w *Writer) TextShowAligned(s string, width, q float64) {
	if !w.isValid("TextShowAligned", objText) {
		return
	}
	if w.State.TextFont == nil {
		w.Err = errors.New("no font set")
		return
	}
	w.showGlyphsAligned(w.TextLayout(s), width, q)
}

// TextShowGlyphsOld draws a sequence of glyphs.
func (w *Writer) TextShowGlyphsOld(gg glyph.Seq) float64 {
	if !w.isValid("TextShowGlyphs", objText) {
		return 0
	}
	if w.State.TextFont == nil {
		w.Err = errors.New("no font set")
		return 0
	}

	return w.showGlyphsWithMargins(gg, 0, 0)
}

// TextShowGlyphsAligned draws a sequence of glyphs and aligns it.
func (w *Writer) TextShowGlyphsAligned(gg glyph.Seq, width, q float64) {
	if !w.isValid("TextShowGlyphsAligned", objText) {
		return
	}
	if w.State.TextFont == nil {
		w.Err = errors.New("no font set")
		return
	}
	w.showGlyphsAligned(gg, width, q)
}

func (w *Writer) showGlyphsAligned(gg glyph.Seq, width, q float64) {
	geom := w.State.TextFont.(font.Layouter).GetGeometry()
	total := geom.ToPDF(w.State.TextFontSize, gg.AdvanceWidth())
	delta := width - total

	// we interpolate between the following:
	// q = 0: left = 0, right = delta
	// q = 1: left = delta, right = 0
	left := q * delta
	right := (1 - q) * delta

	w.showGlyphsWithMargins(gg, left*1000/w.State.TextFontSize, right*1000/w.State.TextFontSize)
}

func (w *Writer) showGlyphsWithMargins(gg glyph.Seq, left, right float64) float64 {
	if len(gg) == 0 || w.Err != nil {
		return 0
	}

	xBefore := w.State.TextMatrix[4]
	pdfLeft, pdfGG := convertGlyphs(gg, w.State.TextFont.(font.Layouter).FontMatrix(), w.State.TextFontSize)

	w.TextShowGlyphs(pdfLeft+left/1000, pdfGG, right/1000)

	return w.State.TextMatrix[4] - xBefore
}

func convertGlyphs(gg glyph.Seq, fontMatrix []float64, fontSize float64) (float64, []font.Glyph) {
	var xOffset float64
	res := make([]font.Glyph, len(gg))
	for i, g := range gg {
		fontDx := float64(g.XOffset)
		fontDy := float64(g.YOffset)
		pdfDx := (fontMatrix[0]*fontDx + fontMatrix[2]*fontDy + fontMatrix[4]) * fontSize
		pdfDy := (fontMatrix[1]*fontDx + fontMatrix[3]*fontDy + fontMatrix[5]) * fontSize

		fontAdvanceX := float64(g.Advance)
		pdfAdvanceX := fontMatrix[0] * fontAdvanceX * fontSize // TODO(voss): is this correct?

		if i > 0 {
			res[i-1].Advance += pdfDx
		} else {
			xOffset = pdfDx
		}
		res[i].GID = g.GID
		res[i].Advance = pdfAdvanceX
		res[i].Rise = pdfDy
		res[i].Text = g.Text
	}
	return xOffset, res
}
