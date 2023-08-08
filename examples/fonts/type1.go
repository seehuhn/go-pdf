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
	"fmt"

	"seehuhn.de/go/postscript/funit"
	pst1 "seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/type1"
)

type type1Simple struct {
	names []string
	info  *pst1.Font
	geom  *font.Geometry
	cmap  map[rune]glyph.ID
	lig   map[glyph.Pair]glyph.ID
	kern  map[glyph.Pair]funit.Int16

	w       pdf.Putter
	ref     pdf.Reference
	resName pdf.Name

	enc    cmap.SimpleEncoder
	text   map[glyph.ID][]rune
	closed bool
}

func embedType1(w pdf.Putter, f *pst1.Font, resName pdf.Name) (font.Embedded, error) {
	glyphNames := f.GlyphList()
	nameGid := make(map[string]glyph.ID, len(glyphNames))
	for i, name := range glyphNames {
		nameGid[name] = glyph.ID(i)
	}

	widths := make([]funit.Int16, len(glyphNames))
	extents := make([]funit.Rect16, len(glyphNames))
	for i, name := range glyphNames {
		gi := f.GlyphInfo[name]
		widths[i] = gi.WidthX
		extents[i] = gi.BBox
	}

	geometry := &font.Geometry{
		UnitsPerEm:   f.UnitsPerEm,
		Widths:       widths,
		GlyphExtents: extents,

		Ascent:             f.Ascent,
		Descent:            f.Descent,
		BaseLineSkip:       (f.Ascent - f.Descent) * 6 / 5, // TODO(voss)
		UnderlinePosition:  f.FontInfo.UnderlinePosition,
		UnderlineThickness: f.FontInfo.UnderlineThickness,
	}

	cMap := make(map[rune]glyph.ID)
	isDingbats := f.FontInfo.FontName == "ZapfDingbats"
	for gid, name := range glyphNames {
		rr := names.ToUnicode(name, isDingbats)
		if len(rr) != 1 {
			continue
		}
		r := rr[0]

		if _, exists := cMap[r]; exists {
			continue
		}
		cMap[r] = glyph.ID(gid)
	}

	lig := make(map[glyph.Pair]glyph.ID)
	for left, name := range glyphNames {
		gi := f.GlyphInfo[name]
		for right, repl := range gi.Ligatures {
			lig[glyph.Pair{Left: glyph.ID(left), Right: nameGid[right]}] = nameGid[repl]
		}
	}

	kern := make(map[glyph.Pair]funit.Int16)
	for _, k := range f.Kern {
		left, right := nameGid[k.Left], nameGid[k.Right]
		kern[glyph.Pair{Left: left, Right: right}] = k.Adjust
	}

	res := &type1Simple{
		names: glyphNames,
		info:  f,
		geom:  geometry,
		cmap:  cMap,
		lig:   lig,
		kern:  kern,

		w:       w,
		ref:     w.Alloc(),
		resName: resName,

		enc:  cmap.NewSimpleEncoder(),
		text: make(map[glyph.ID][]rune),
	}
	return res, nil
}

func (f *type1Simple) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	res := &type1Simple{
		info:    f.info,
		geom:    f.geom,
		w:       w,
		ref:     w.Alloc(),
		resName: resName,
		enc:     cmap.NewSimpleEncoder(),
		text:    map[glyph.ID][]rune{},
		closed:  false,
	}
	return res, nil
}

func (f *type1Simple) GetGeometry() *font.Geometry {
	return f.geom
}

func (f *type1Simple) ResourceName() pdf.Name {
	return f.resName
}

func (f *type1Simple) Reference() pdf.Reference {
	return f.ref
}

func (f *type1Simple) Layout(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)

	gg := make(glyph.Seq, 0, len(rr))
	var prev glyph.ID
	for i, r := range rr {
		gid := f.cmap[r]
		if i > 0 {
			if repl, ok := f.lig[glyph.Pair{Left: prev, Right: gid}]; ok {
				gg[len(gg)-1].Gid = repl
				gg[len(gg)-1].Text = append(gg[len(gg)-1].Text, r)
				prev = repl
				continue
			}
		}
		gg = append(gg, glyph.Info{
			Gid:  gid,
			Text: []rune{r},
		})
		prev = gid
	}

	for i, g := range gg {
		if i > 0 {
			if adj, ok := f.kern[glyph.Pair{Left: prev, Right: g.Gid}]; ok {
				gg[i-1].Advance += adj
			}
		}
		gg[i].Advance = f.geom.Widths[g.Gid]
		prev = g.Gid
	}

	return gg
}

func (f *type1Simple) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	return append(s, f.enc.Encode(gid, rr))
}

func (f *type1Simple) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.enc.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q (%s)",
			f.resName, f.info.FontInfo.FontName)
	}
	f.enc = cmap.NewFrozenSimpleEncoder(f.enc)

	encodingGid := f.enc.Encoding()
	includeGlyph := make(map[string]bool)
	includeGlyph[".notdef"] = true
	for _, gid := range encodingGid {
		includeGlyph[f.names[gid]] = true
	}

	var ss []subset.Glyph
	ss = append(ss, subset.Glyph{OrigGID: 0, CID: 0}) // .notdef
	for code, gid := range encodingGid {
		if gid != 0 {
			ss = append(ss, subset.Glyph{OrigGID: gid, CID: pst1.CID(code)})
		}
	}
	subsetTag := subset.Tag(ss, f.info.NumGlyphs())

	subset := &pst1.Font{}
	*subset = *f.info
	subset.Outlines = make(map[string]*pst1.Glyph, len(includeGlyph))
	subset.GlyphInfo = make(map[string]*pst1.GlyphInfo, len(includeGlyph))
	for name := range includeGlyph {
		subset.Outlines[name] = f.info.Outlines[name]
		subset.GlyphInfo[name] = f.info.GlyphInfo[name]
	}
	subset.Encoding = make([]string, 256)
	for i, gid := range encodingGid {
		subset.Encoding[i] = f.names[gid]
	}

	t1info := type1.Font{
		PSFont:    subset,
		ResName:   f.resName,
		SubsetTag: subsetTag,
		Encoding:  subset.Encoding,
		// ToUnicode: map[charcode.CharCode][]rune{},
	}

	err := t1info.Embed(f.w, f.ref)
	if err != nil {
		return err
	}

	return nil
}
