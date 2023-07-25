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

package main

import (
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
)

func embedType1(w pdf.Putter, info *type1.Font, resName pdf.Name) (font.Embedded, error) {
	geometry := &font.Geometry{
		UnitsPerEm: info.UnitsPerEm,
		// GlyphExtents: info.Extents(),
		// Widths:       info.Widths(),
		// Ascent:             info.Ascent,
		// Descent:            info.Descent,
		// BaseLineSkip:       info.Ascent - info.Descent + info.LineGap,
		// UnderlinePosition:  info.UnderlinePosition,
		// UnderlineThickness: info.UnderlineThickness,
	}

	res := &type1Simple{
		info:     info,
		geometry: geometry,

		w:       w,
		ref:     w.Alloc(),
		resName: resName,

		enc:  cmap.NewSimpleEncoder(),
		text: make(map[glyph.ID][]rune),
	}
	return res, nil
}

type type1Simple struct {
	info     *type1.Font
	geometry *font.Geometry

	w       pdf.Putter
	ref     pdf.Reference
	resName pdf.Name

	enc    cmap.SimpleEncoder
	text   map[glyph.ID][]rune
	closed bool
}

func (f *type1Simple) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	return f, nil
}

func (f *type1Simple) GetGeometry() *font.Geometry {
	return f.geometry
}

func (f *type1Simple) ResourceName() pdf.Name {
	return f.resName
}

func (f *type1Simple) Reference() pdf.Reference {
	return f.ref
}

func (f *type1Simple) Layout(s string, ptSize float64) glyph.Seq {
	panic("not implemented")
}

func (f *type1Simple) AppendEncoded(pdf.String, glyph.ID, []rune) pdf.String {
	panic("not implemented")
}

func (f *type1Simple) Close() error {
	panic("not implemented")
}
