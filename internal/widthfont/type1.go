// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package widthfont

import (
	"errors"
	"math"

	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/funit"
	pst1 "seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/type1"
)

func Type1(out pdf.Putter, unitsPerEm funit.Int16) (font.Embedded, error) {
	if unitsPerEm <= 0 || unitsPerEm%10 != 0 {
		return nil, errors.New("unitPerEm must be a multiple of 10")
	}

	encoding := make([]string, 256)
	for i := range encoding {
		encoding[i] = ".notdef"
	}

	q := 1 / float64(unitsPerEm)
	psfont := &pst1.Font{
		FontInfo: &pst1.FontInfo{
			FontName:   "widthtest",
			Version:    "001.000",
			Copyright:  "Copyright (c) 2024 Jochen Voss <voss@seehuhn.de>",
			FontMatrix: [6]float64{q, 0, 0, q, 0, 0},
		},
		Glyphs:   map[string]*pst1.Glyph{},
		Private:  &pst1.PrivateDict{BlueValues: []funit.Int16{0, 0}},
		Encoding: encoding,
	}
	metrics := &afm.Metrics{
		Glyphs:    map[string]*afm.GlyphInfo{},
		Encoding:  psfont.Encoding,
		FontName:  psfont.FontName,
		CapHeight: float64(unitsPerEm) / 2,
		XHeight:   float64(unitsPerEm) / 2,
		Ascent:    float64(unitsPerEm) / 2,
		Descent:   0,
	}

	g := &pst1.Glyph{
		WidthX: float64(unitsPerEm),
	}
	wf := float64(unitsPerEm)
	hf := float64(unitsPerEm / 2)
	df := float64(unitsPerEm / 20)
	g.MoveTo(0, 0)
	g.LineTo(wf, 0)
	g.LineTo(wf, hf)
	g.LineTo(0, hf)
	g.ClosePath()
	g.MoveTo(df, df)
	g.LineTo(df, hf-df)
	g.LineTo(wf-df, hf-df)
	g.LineTo(wf-df, df)
	g.ClosePath()
	psfont.Glyphs[".notdef"] = g
	metrics.Glyphs[".notdef"] = &afm.GlyphInfo{
		WidthX: float64(unitsPerEm),
		BBox: funit.Rect16{
			LLx: 0,
			LLy: 0,
			URx: unitsPerEm,
			URy: unitsPerEm / 2,
		},
	}

	g = &pst1.Glyph{
		WidthX: float64(unitsPerEm) / 2,
	}
	psfont.Glyphs["space"] = g
	metrics.Glyphs["space"] = &afm.GlyphInfo{
		WidthX: float64(unitsPerEm) / 2,
	}
	encoding[' '] = "space"

	g = &pst1.Glyph{
		WidthX: 0,
	}
	d := unitsPerEm / 10
	g.MoveTo(0, 0)
	g.LineTo(float64(d), -float64(d))
	g.LineTo(-float64(d), -float64(d))
	g.ClosePath()
	psfont.Glyphs["zero"] = g
	metrics.Glyphs["zero"] = &afm.GlyphInfo{
		WidthX: 0,
		BBox: funit.Rect16{
			LLx: -d,
			LLy: -d,
			URx: d,
			URy: 0,
		},
	}
	encoding['0'] = "zero"

	for c := '1'; c <= '9'; c++ {
		name := names.FromUnicode(c)
		h := float64(unitsPerEm) / 5
		w := float64(unitsPerEm) / 10 * float64(c-'0')
		w0 := max(h/10, w-h/2)

		g = &pst1.Glyph{
			WidthX: w,
		}
		g.MoveTo(0, 0)
		g.LineTo(float64(w0), 0)
		g.LineTo(float64(w), float64(h)/2)
		g.LineTo(float64(w0), float64(h))
		g.LineTo(0, float64(h))
		g.ClosePath()
		psfont.Glyphs[name] = g
		metrics.Glyphs[name] = &afm.GlyphInfo{
			WidthX: w,
			BBox: funit.Rect16{
				URx: funit.Int16(math.Round(w)),
				URy: funit.Int16(math.Round(h)),
			},
		}
		encoding[c] = name
	}

	F, err := type1.NewFont(psfont, metrics)
	if err != nil {
		return nil, err
	}

	return F.Embed(out, nil)
}
