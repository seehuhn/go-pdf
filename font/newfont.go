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

package font

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt/glyph"
)

// NewFont represents a font in a PDF document.
//
// TODO(voss): make sure that vertical writing can later be implemented
// without breaking the API.
type NewFont interface {
	DefaultName() pdf.Name // return "" to choose names automatically
	PDFObject() pdf.Object // value to use in the resource dictionary
	WritingMode() int      // 0 = horizontal, 1 = vertical

	AllWidther
}

// AllWidther is an interface for fonts which can return the width of all
// characters in PDF string.
type AllWidther interface {
	// AllWidths returns a function which iterates over all characters in the
	// given string.  The width values are returned in PDF text space units.
	AllWidths(s pdf.String) func(yield func(w float64, isSpace bool) bool) bool
}

type pdfGlyph struct {
	GID     glyph.ID
	Text    []rune
	Advance float64
}

type newFont2 interface {
	ToUnicode(pdf.String) []rune
}

type newFontSimple interface {
	newFont2
	CodeToGID(byte) glyph.ID
	GIDToCode(glyph.ID, []rune) byte
	Widths() []float64
}

type newFontComposite interface {
	newFont2
	CS() charcode.CodeSpaceRange
	CodeToCID(pdf.String) type1.CID
	AppendCode(pdf.String, type1.CID) pdf.String
	CIDToGID(type1.CID) glyph.ID
	GIDToCID(glyph.ID, []rune) type1.CID
	GlyphWidth(type1.CID) float64
}

type textParameters struct {
	TextCharacterSpacing float64 // character spacing (T_c)
	TextWordSpacing      float64 // word spacing (T_w)
	TextHorizonalScaling float64 // horizonal scaling (T_h, normal spacing = 1)
	TextLeading          float64 // leading (T_l)
	TextFont             newFont2
	TextFontSize         float64
}

// encodeStringFixed encodes a string into a PDF string, using the glyphs'
// natural widths.
func encodeStringFixed(gg []pdfGlyph, param *textParameters) pdf.String {
	var res pdf.String
	switch font := param.TextFont.(type) {
	case newFontSimple:
		res = make(pdf.String, len(gg))
		widths := font.Widths()
		for i, g := range gg {
			c := font.GIDToCode(g.GID, g.Text)
			res[i] = c

			width := widths[c]*param.TextFontSize + param.TextCharacterSpacing
			if c == ' ' {
				width += param.TextWordSpacing
			}
			gg[i].Advance = width * param.TextHorizonalScaling
		}
	case newFontComposite:
		for _, g := range gg {
			cid := font.GIDToCID(g.GID, g.Text)
			res = font.AppendCode(res, cid)

			width := font.GlyphWidth(cid)*param.TextFontSize + param.TextCharacterSpacing
			if len(g.Text) == 1 && g.Text[0] == ' ' {
				width += param.TextWordSpacing
			}
			g.Advance = width * param.TextHorizonalScaling
		}
	}
	return res
}

func decodeString(s pdf.String, param *textParameters) []pdfGlyph {
	var res []pdfGlyph
	switch font := param.TextFont.(type) {
	case newFontSimple:
		res = make([]pdfGlyph, len(s))
		widths := font.Widths()
		for i := 0; i < len(s); i++ {
			c := s[i]
			gid := font.CodeToGID(c)
			width := widths[c]*param.TextFontSize + param.TextCharacterSpacing
			if c == ' ' {
				width += param.TextWordSpacing
			}
			res[i] = pdfGlyph{
				GID:     gid,
				Advance: width * param.TextHorizonalScaling,
				Text:    font.ToUnicode(pdf.String{c}),
			}
		}
	case newFontComposite:
		cs := font.CS()
		cs.AllCodes(s)(func(code pdf.String, valid bool) bool {
			cid := font.CodeToCID(code)
			gid := font.CIDToGID(cid)
			width := font.GlyphWidth(cid)*param.TextFontSize + param.TextCharacterSpacing
			if len(code) == 1 && code[0] == ' ' {
				width += param.TextWordSpacing
			}
			g := pdfGlyph{
				GID:     gid,
				Advance: width * param.TextHorizonalScaling,
				Text:    font.ToUnicode(code),
			}
			res = append(res, g)

			return true
		})
	}
	return res
}
