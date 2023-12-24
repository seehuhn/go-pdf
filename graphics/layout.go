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
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/sfnt/glyph"
)

// TextLayout returns the glyph sequence for a string.
// The function panics if no font is set.
func (w *Writer) TextLayout(s string) glyph.Seq {
	st := w.State
	return st.TextFont.(font.Embedded).Layout(s, st.TextFontSize)
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

// TextShowGlyphs draws a sequence of glyphs.
func (w *Writer) TextShowGlyphs(gg glyph.Seq) float64 {
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
	geom := w.State.TextFont.(font.Embedded).GetGeometry()
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
	if len(gg) == 0 {
		return 0
	}

	// TODO(voss): Update p.Tm

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

		if w.Err != nil {
			return
		}
		if len(out) == 1 {
			if s, ok := out[0].(pdf.String); ok {
				w.Err = s.PDF(w.Content)
				if w.Err != nil {
					return
				}
				_, w.Err = fmt.Fprintln(w.Content, " Tj")
				out = nil
				return
			}
		}

		w.Err = out.PDF(w.Content)
		if w.Err != nil {
			return
		}
		_, w.Err = fmt.Fprintln(w.Content, " TJ")
		out = nil
	}

	F := w.State.TextFont
	geom := F.(font.Embedded).GetGeometry()
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

		if newYPos := float64(glyph.YOffset) * q; w.State.Set&StateTextRise == 0 || newYPos != w.State.TextRise {
			flush()
			w.State.TextRise = newYPos
			if w.Err != nil {
				return 0
			}
			w.Err = pdf.Number(w.State.TextRise).PDF(w.Content) // TODO(voss): rounding?
			if w.Err != nil {
				return 0
			}
			_, w.Err = fmt.Fprintln(w.Content, " Ts")
		}

		run = F.(font.Embedded).AppendEncoded(run, glyph.Gid, glyph.Text)

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
	return xActual * w.State.TextFontSize / 1000
}
