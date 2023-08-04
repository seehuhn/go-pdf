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
	"errors"
	"fmt"

	"golang.org/x/text/language"

	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/font/subset"
)

type openTypeSimple struct {
	info        *sfnt.Font
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex
	geometry    *font.Geometry

	w       pdf.Putter
	ref     pdf.Reference
	resName pdf.Name

	enc    cmap.SimpleEncoder
	text   map[glyph.ID][]rune
	closed bool
}

func embedOpenTypeSimple(w pdf.Putter, info *sfnt.Font, resName pdf.Name, loc language.Tag) (font.Embedded, error) {
	if !info.IsCFF() {
		return nil, errors.New("wrong font type")
	}
	err := pdf.CheckVersion(w, "use of OpenType fonts", pdf.V1_6)
	if err != nil {
		return nil, err
	}

	geometry := &font.Geometry{
		UnitsPerEm:   info.UnitsPerEm,
		GlyphExtents: info.Extents(),
		Widths:       info.Widths(),

		Ascent:             info.Ascent,
		Descent:            info.Descent,
		BaseLineSkip:       info.Ascent - info.Descent + info.LineGap,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
	}

	res := &openTypeSimple{
		info:        info,
		gsubLookups: info.Gsub.FindLookups(loc, gtab.GsubDefaultFeatures),
		gposLookups: info.Gpos.FindLookups(loc, gtab.GposDefaultFeatures),
		geometry:    geometry,

		w:       w,
		ref:     w.Alloc(),
		resName: resName,

		enc:  cmap.NewSimpleEncoder(),
		text: make(map[glyph.ID][]rune),
	}
	return res, nil
}

func (f *openTypeSimple) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	return f, nil
}

func (f *openTypeSimple) GetGeometry() *font.Geometry {
	return f.geometry
}

func (f *openTypeSimple) ResourceName() pdf.Name {
	return f.resName
}

func (f *openTypeSimple) Reference() pdf.Reference {
	return f.ref
}

func (f *openTypeSimple) Layout(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)
	return f.info.Layout(rr, f.gsubLookups, f.gposLookups)
}

func (f *openTypeSimple) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	f.text[gid] = rr
	return append(s, f.enc.Encode(gid, rr))
}

func (f *openTypeSimple) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.enc.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q (%s)",
			f.resName, f.info.PostscriptName())
	}
	f.enc = cmap.NewFrozenSimpleEncoder(f.enc)

	// subset the font
	var ss []subset.Glyph
	ss = append(ss, subset.Glyph{OrigGID: 0, CID: 0})
	encoding := f.enc.Encoding()
	for cid, gid := range encoding {
		if gid != 0 {
			ss = append(ss, subset.Glyph{OrigGID: gid, CID: type1.CID(cid)})
		}
	}
	subsetTag := subset.Tag(ss, f.info.NumGlyphs())
	subsetInfo, err := subset.Simple(f.info, ss)
	if err != nil {
		return fmt.Errorf("font subset: %w", err)
	}

	m := make(map[charcode.CharCode][]rune)
	for code, gid := range encoding {
		if gid == 0 || len(f.text[gid]) == 0 {
			continue
		}
		m[charcode.CharCode(code)] = f.text[gid]
	}
	info := opentype.PDFFont{
		Font:      subsetInfo,
		SubsetTag: subsetTag,
		Encoding:  subsetInfo.Outlines.(*cff.Outlines).Encoding,
		ToUnicode: m,
	}
	return info.Embed(f.w, f.ref)
}
