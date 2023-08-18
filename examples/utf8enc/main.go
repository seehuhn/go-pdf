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
	"golang.org/x/text/language"

	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/charcode"
)

func main() {
	err := doit()
	if err != nil {
		panic(err)
	}
}

func doit() error {
	page, err := document.CreateSinglePage("test.pdf", document.A4, nil)
	if err != nil {
		return err
	}

	loc := language.German

	info, err := sfnt.ReadFile("../../../otf/SourceSerif4-Regular.otf")
	if err != nil {
		return err
	}
	geometry := &font.Geometry{
		UnitsPerEm:   info.UnitsPerEm,
		GlyphExtents: info.GlyphBBoxes(),
		Widths:       info.Widths(),

		Ascent:             info.Ascent,
		Descent:            info.Descent,
		BaseLineSkip:       info.Ascent - info.Descent + info.LineGap,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
	}

	cmap := info.CMap
	a, b := cmap.CodeRange()
	enc := make(map[glyph.ID]rune)
	for r := a; r <= b; r++ {
		gid := cmap.Lookup(r)
		_, seen := enc[gid]
		if gid != 0 && !seen {
			enc[gid] = r
		}
	}
	special := rune(0xE000) // private use area
	for gid := glyph.ID(1); int(gid) < info.NumGlyphs(); gid++ {
		_, seen := enc[gid]
		if !seen {
			enc[gid] = special
			special++
		}
	}

	F := &funkel{
		w: page.Out,

		info:        info,
		gsubLookups: info.Gsub.FindLookups(loc, gtab.GsubDefaultFeatures),
		gposLookups: info.Gpos.FindLookups(loc, gtab.GposDefaultFeatures),
		Geometry:    geometry,

		enc:  enc,
		used: make(map[glyph.ID]bool),

		Resource: pdf.Resource{
			Ref:  page.Out.Alloc(),
			Name: "F",
		},
	}
	page.Out.AutoClose(F)

	page.TextSetFont(F, 36)
	page.TextStart()
	page.TextFirstLine(100, 700)
	page.TextShow("Größenwahn")
	page.TextEnd()

	return page.Close()
}

type funkel struct {
	w pdf.Putter

	info        *sfnt.Font
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex
	*font.Geometry

	enc  map[glyph.ID]rune
	used map[glyph.ID]bool

	pdf.Resource
}

func (f *funkel) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	return f, nil
}

func (f *funkel) Layout(s string, ptSize float64) glyph.Seq {
	return f.info.Layout(s, f.gsubLookups, f.gposLookups)
}

func (f *funkel) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	f.used[gid] = true
	r := string([]rune{f.enc[gid]})
	return append(s, []byte(r)...)
}

func (f *funkel) Close() error {
	ROS := &type1.CIDSystemInfo{
		Registry:   "Seehuhn",
		Ordering:   "Sonderbar",
		Supplement: 0,
	}

	cffFont := f.info.AsCFF()

	// convert to CID-keyed font
	cffFont.Encoding = nil
	cffFont.ROS = ROS
	gid2cid := make([]type1.CID, len(cffFont.Glyphs))
	for gid, r := range f.enc {
		gid2cid[gid] = type1.CID(r)
	}
	cffFont.Gid2Cid = gid2cid

	cffFont = cffFont.Subset(func(gid glyph.ID) bool {
		return f.used[gid]
	})

	cmap := make(map[charcode.CharCode]type1.CID)
	tounicode := make(map[charcode.CharCode][]rune)
	for gid := range f.used {
		r := f.enc[gid]
		code := charcode.CharCode(r)
		cmap[code] = type1.CID(r)
		tounicode[code] = []rune{r}
	}

	info := &cff.EmbedInfoComposite{
		Font:       cffFont,
		SubsetTag:  "ABCDEF",
		CS:         charcode.UTF8,
		ROS:        ROS,
		CMap:       cmap,
		ToUnicode:  tounicode,
		UnitsPerEm: f.info.UnitsPerEm,
		Ascent:     f.info.Ascent,
		Descent:    f.info.Descent,
		CapHeight:  f.info.CapHeight,
		IsSerif:    f.info.IsSerif,
		IsScript:   f.info.IsScript,
	}
	return info.Embed(f.w, f.Ref)
}
