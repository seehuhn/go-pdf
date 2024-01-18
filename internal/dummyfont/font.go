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

package dummyfont

import (
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	pdfcff "seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/postscript/type1"
)

// Embed installs a dummy font into the given PDF file.
// The only glyphs available are the space and the capital letter A.
// The font is embedded as a simple CFF font.
//
// If a write error on w occurs, the function panics.
func Embed(w pdf.Putter, defaultName pdf.Name) font.NewFont {
	encoding := make([]glyph.ID, 256)
	encoding[' '] = 1
	encoding['A'] = 2

	in := &cff.Font{
		FontInfo: &type1.FontInfo{
			FontName:   "Dummy",
			FontMatrix: [6]float64{0.001, 0, 0, 0.001, 0, 0},
		},
		Outlines: &cff.Outlines{
			Private:  []*type1.PrivateDict{{}},
			FDSelect: func(gi glyph.ID) int { return 0 },
			Encoding: encoding,
		},
	}

	g := cff.NewGlyph(".notdef", 500)
	in.Glyphs = append(in.Glyphs, g)

	g = cff.NewGlyph("space", 1000)
	in.Glyphs = append(in.Glyphs, g)

	g = cff.NewGlyph("A", 900)
	g.MoveTo(50, 50)
	g.LineTo(850, 50)
	g.LineTo(850, 850)
	g.LineTo(50, 850)
	in.Glyphs = append(in.Glyphs, g)

	toUni := map[charcode.CharCode][]rune{
		' ': {' '},
		'A': {'A'},
	}

	ref := w.Alloc()
	info := &pdfcff.EmbedInfoCFFSimple{
		Font:      in,
		Encoding:  encoding,
		Ascent:    850,
		Descent:   0,
		CapHeight: 850,
		ToUnicode: cmap.NewToUnicode(charcode.Simple, toUni),
	}
	err := info.Embed(w, ref)
	if err != nil {
		panic(err)
	}

	tmp := pdf.NewData(pdf.GetVersion(w))

	// We need to make sure that ref is allocated in tmp, so that
	// the allocations in info.Embed don't clash with ref.
	//
	// TODO(voss): find a better way to do this.
	for tmp.Alloc() != ref {
	}

	err = info.Embed(tmp, ref)
	if err != nil {
		panic(err)
	}
	F, err := font.Read(tmp, ref, defaultName)
	if err != nil {
		panic(err)
	}
	return F
}
