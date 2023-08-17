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

package type1

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
)

type Font struct {
	names    []string
	outlines *type1.Font
	*font.Geometry

	CMap map[rune]glyph.ID
	lig  map[glyph.Pair]glyph.ID
	kern map[glyph.Pair]funit.Int16
}

func New(psFont *type1.Font) (*Font, error) {
	glyphNames := psFont.GlyphList()
	nameGid := make(map[string]glyph.ID, len(glyphNames))
	for i, name := range glyphNames {
		nameGid[name] = glyph.ID(i)
	}

	widths := make([]funit.Int16, len(glyphNames))
	extents := make([]funit.Rect16, len(glyphNames))
	for i, name := range glyphNames {
		gi := psFont.GlyphInfo[name]
		widths[i] = gi.WidthX
		extents[i] = gi.BBox
	}

	geometry := &font.Geometry{
		UnitsPerEm:   psFont.UnitsPerEm,
		Widths:       widths,
		GlyphExtents: extents,

		Ascent:             psFont.Ascent,
		Descent:            psFont.Descent,
		BaseLineSkip:       (psFont.Ascent - psFont.Descent) * 6 / 5, // TODO(voss)
		UnderlinePosition:  psFont.FontInfo.UnderlinePosition,
		UnderlineThickness: psFont.FontInfo.UnderlineThickness,
	}

	cMap := make(map[rune]glyph.ID)
	isDingbats := psFont.FontInfo.FontName == "ZapfDingbats"
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
		gi := psFont.GlyphInfo[name]
		for right, repl := range gi.Ligatures {
			lig[glyph.Pair{Left: glyph.ID(left), Right: nameGid[right]}] = nameGid[repl]
		}
	}

	kern := make(map[glyph.Pair]funit.Int16)
	for _, k := range psFont.Kern {
		left, right := nameGid[k.Left], nameGid[k.Right]
		kern[glyph.Pair{Left: left, Right: right}] = k.Adjust
	}

	res := &Font{
		names:    glyphNames,
		outlines: psFont,
		Geometry: geometry,
		CMap:     cMap,
		lig:      lig,
		kern:     kern,
	}
	return res, nil
}

func (f *Font) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	e := &embedded{
		Font: f,
		w:    w,
		Resource: pdf.Resource{
			Ref:  w.Alloc(),
			Name: resName,
		},
		SimpleEncoder: cmap.NewSimpleEncoder(),
	}
	w.AutoClose(e)
	return e, nil
}

func (f *Font) Layout(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)

	gg := make(glyph.Seq, 0, len(rr))
	var prev glyph.ID
	for i, r := range rr {
		gid := f.CMap[r]
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
		gg[i].Advance = f.Widths[g.Gid]
		prev = g.Gid
	}

	return gg
}
