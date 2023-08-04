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
	"sync"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"
	"seehuhn.de/go/sfnt/glyph"
)

type Font string

// The 14 built-in PDF fonts.
const (
	Courier              Font = "Courier"
	CourierBold          Font = "Courier-Bold"
	CourierBoldOblique   Font = "Courier-BoldOblique"
	CourierOblique       Font = "Courier-Oblique"
	Helvetica            Font = "Helvetica"
	HelveticaBold        Font = "Helvetica-Bold"
	HelveticaBoldOblique Font = "Helvetica-BoldOblique"
	HelveticaOblique     Font = "Helvetica-Oblique"
	TimesRoman           Font = "Times-Roman"
	TimesBold            Font = "Times-Bold"
	TimesBoldItalic      Font = "Times-BoldItalic"
	TimesItalic          Font = "Times-Italic"
	Symbol               Font = "Symbol"
	ZapfDingbats         Font = "ZapfDingbats"
)

// All contains the 14 built-in PDF fonts.
var All = []Font{
	Courier,
	CourierBold,
	CourierBoldOblique,
	CourierOblique,
	Helvetica,
	HelveticaBold,
	HelveticaBoldOblique,
	HelveticaOblique,
	TimesRoman,
	TimesBold,
	TimesBoldItalic,
	TimesItalic,
	Symbol,
	ZapfDingbats,
}

func (f Font) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	info, err := getFontInfo(f)
	if err != nil {
		return nil, err
	}

	res := &embedded{
		fontInfo: info,
		w:        w,
		ref:      w.Alloc(),
		resName:  resName,
		enc:      cmap.NewSimpleEncoder(),
	}

	w.AutoClose(res)

	return res, nil
}

func (f Font) GetGeometry() *font.Geometry {
	info, _ := getFontInfo(f)
	return info.GetGeometry()
}

func (f Font) Layout(s string, ptSize float64) glyph.Seq {
	info, _ := getFontInfo(f)
	return info.Layout(s, ptSize)
}

type fontInfo struct {
	afm      *type1.Font
	names    []string
	geom     *font.Geometry
	encoding []glyph.ID
	cmap     map[rune]glyph.ID
	lig      map[glyph.Pair]glyph.ID
	kern     map[glyph.Pair]funit.Int16
}

func getFontInfo(f Font) (*fontInfo, error) {
	fontCacheLock.Lock()
	defer fontCacheLock.Unlock()

	if res, ok := fontCache[f]; ok {
		return res, nil
	}

	afm, err := f.Afm()
	if err != nil {
		return nil, err
	}

	glyphNames := afm.GlyphList()
	nameGid := make(map[string]glyph.ID, len(glyphNames))
	for i, name := range glyphNames {
		nameGid[name] = glyph.ID(i)
	}

	widths := make([]funit.Int16, len(glyphNames))
	extents := make([]funit.Rect16, len(glyphNames))
	for i, name := range glyphNames {
		gi := afm.GlyphInfo[name]
		widths[i] = gi.WidthX
		extents[i] = gi.Extent
	}

	geom := &font.Geometry{
		UnitsPerEm:   afm.UnitsPerEm,
		Widths:       widths,
		GlyphExtents: extents,

		Ascent:             afm.Ascent,
		Descent:            afm.Descent,
		BaseLineSkip:       1200, // TODO(voss): is this ok?
		UnderlinePosition:  afm.FontInfo.UnderlinePosition,
		UnderlineThickness: afm.FontInfo.UnderlineThickness,
	}

	encoding := make([]glyph.ID, 256)
	for i, name := range afm.Encoding {
		encoding[i] = nameGid[name]
	}

	cmap := make(map[rune]glyph.ID)
	isDingbats := afm.FontInfo.FontName == "ZapfDingbats"
	for gid, name := range glyphNames {
		rr := names.ToUnicode(name, isDingbats)
		if len(rr) != 1 {
			continue
		}
		r := rr[0]

		if _, exists := cmap[r]; exists {
			continue
		}
		cmap[r] = glyph.ID(gid)
	}

	lig := make(map[glyph.Pair]glyph.ID)
	for left, name := range glyphNames {
		gi := afm.GlyphInfo[name]
		for right, repl := range gi.Ligatures {
			lig[glyph.Pair{Left: glyph.ID(left), Right: nameGid[right]}] = nameGid[repl]
		}
	}

	kern := make(map[glyph.Pair]funit.Int16)
	for _, k := range afm.Kern {
		left, right := nameGid[k.Left], nameGid[k.Right]
		kern[glyph.Pair{Left: left, Right: right}] = k.Adjust
	}

	res := &fontInfo{
		names:    glyphNames,
		afm:      afm,
		geom:     geom,
		encoding: encoding,
		cmap:     cmap,
		lig:      lig,
		kern:     kern,
	}
	fontCache[f] = res
	return res, nil
}

func (info *fontInfo) GetGeometry() *font.Geometry {
	return info.geom
}

func (info *fontInfo) Layout(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)

	gg := make(glyph.Seq, 0, len(rr))
	var prev glyph.ID
	for i, r := range rr {
		gid := info.cmap[r]
		if i > 0 {
			if repl, ok := info.lig[glyph.Pair{Left: prev, Right: gid}]; ok {
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
			if adj, ok := info.kern[glyph.Pair{Left: prev, Right: g.Gid}]; ok {
				gg[i-1].Advance += adj
			}
		}
		gg[i].Advance = info.geom.Widths[g.Gid]
		prev = g.Gid
	}

	return gg
}

var (
	fontCache     = make(map[Font]*fontInfo)
	fontCacheLock sync.Mutex
)
