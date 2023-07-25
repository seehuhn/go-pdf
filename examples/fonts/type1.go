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
	"math"

	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
)

type type1Simple struct {
	names []string
	info  *type1.Font
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

func embedType1(w pdf.Putter, f *type1.Font, resName pdf.Name) (font.Embedded, error) {
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
		extents[i] = gi.Extent
	}

	geometry := &font.Geometry{
		UnitsPerEm:   f.UnitsPerEm,
		Widths:       widths,
		GlyphExtents: extents,

		Ascent:             f.Ascent,
		Descent:            f.Descent,
		BaseLineSkip:       (f.Ascent - f.Descent) * 6 / 5, // TODO(voss)
		UnderlinePosition:  f.Info.UnderlinePosition,
		UnderlineThickness: f.Info.UnderlineThickness,
	}

	cMap := make(map[rune]glyph.ID)
	isDingbats := f.Info.FontName == "ZapfDingbats"
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
			f.resName, f.info.Info.FontName)
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
			ss = append(ss, subset.Glyph{OrigGID: gid, CID: type1.CID(code)})
		}
	}
	subsetTag := subset.Tag(ss, f.info.NumGlyphs())

	fontName := pdf.Name(subsetTag + "+" + f.info.Info.FontName)

	subset := &type1.Font{}
	*subset = *f.info
	subset.Outlines = make(map[string]*type1.Glyph, len(includeGlyph))
	subset.GlyphInfo = make(map[string]*type1.GlyphInfo, len(includeGlyph))
	for name := range includeGlyph {
		subset.Outlines[name] = f.info.Outlines[name]
		subset.GlyphInfo[name] = f.info.GlyphInfo[name]
	}
	subset.Encoding = make([]string, 256)
	for i, gid := range encodingGid {
		subset.Encoding[i] = f.names[gid]
	}

	q := 1000 / float64(subset.UnitsPerEm)

	var Widths pdf.Array
	var firstChar type1.CID
	for int(firstChar) < len(encodingGid) && encodingGid[firstChar] == 0 {
		firstChar++
	}
	lastChar := type1.CID(len(encodingGid) - 1)
	for lastChar > firstChar && encodingGid[lastChar] == 0 {
		lastChar--
	}
	for i := firstChar; i <= lastChar; i++ {
		var width pdf.Integer
		gid := encodingGid[i]
		if gid != 0 {
			w := f.info.GlyphInfo[f.names[gid]].WidthX
			width = pdf.Integer(math.Round(w.AsFloat(q)))
		}
		Widths = append(Widths, width)
	}
	// TODO(voss): use "MissingWidth"

	FontDictRef := f.ref
	FontDescriptorRef := f.w.Alloc()
	WidthsRef := f.w.Alloc()
	FontFileRef := f.w.Alloc()

	// See section 9.6.2.1 of PDF 32000-1:2008.
	FontDict := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("Type1"),
		"BaseFont":       fontName,
		"FirstChar":      pdf.Integer(firstChar),
		"LastChar":       pdf.Integer(lastChar),
		"Widths":         WidthsRef,
		"FontDescriptor": FontDescriptorRef,
		// "ToUnicode":      ToUnicodeRef,
	}
	if f.w.GetMeta().Version == pdf.V1_0 {
		// TODO(voss): check this
		FontDict["Name"] = f.resName
	}

	FontDescriptor := pdf.Dict{ // See section 9.8.1 of PDF 32000-1:2008.
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    fontName,
		"Flags":       pdf.Integer(makeFlags(subset, true)),
		"FontBBox":    &pdf.Rectangle{}, // empty rectangle is always allowed
		"ItalicAngle": pdf.Number(subset.Info.ItalicAngle),
		"Ascent":      pdf.Integer(math.Round(subset.Ascent.AsFloat(q))),
		"Descent":     pdf.Integer(math.Round(subset.Descent.AsFloat(q))),
		"CapHeight":   pdf.Integer(math.Round(subset.CapHeight.AsFloat(q))),
		"StemV":       pdf.Integer(subset.Private.StdVW),
		"FontFile":    FontFileRef,
	}

	compressedRefs := []pdf.Reference{FontDictRef, FontDescriptorRef, WidthsRef}
	compressedObjects := []pdf.Object{FontDict, FontDescriptor, Widths}
	err := f.w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return err
	}

	// See section 9.9 of PDF 32000-1:2008.
	length1 := pdf.NewPlaceholder(f.w, 10)
	length2 := pdf.NewPlaceholder(f.w, 10)
	length3 := pdf.NewPlaceholder(f.w, 10)
	fontFileDict := pdf.Dict{
		"Length1": length1,
		"Length2": length2,
		"Length3": length3,
	}
	fontFileStream, err := f.w.OpenStream(FontFileRef, fontFileDict, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	l1, l2, l3, err := subset.WritePDF(fontFileStream)
	if err != nil {
		return err
	}
	length1.Set(pdf.Integer(l1))
	length2.Set(pdf.Integer(l2))
	length3.Set(pdf.Integer(l3))
	err = fontFileStream.Close()
	if err != nil {
		return err
	}

	return nil
}

// MakeFlags returns the PDF font flags for the font.
// See section 9.8.2 of PDF 32000-1:2008.
func makeFlags(info *type1.Font, symbolic bool) font.Flags {
	var flags font.Flags

	if info.Info.IsFixedPitch {
		flags |= font.FlagFixedPitch
	}
	// TODO(voss): flags |= font.FlagSerif

	if symbolic {
		flags |= font.FlagSymbolic
	} else {
		flags |= font.FlagNonsymbolic
	}

	// flags |= FlagScript
	if info.Info.ItalicAngle != 0 {
		flags |= font.FlagItalic
	}

	if info.Private.ForceBold {
		flags |= font.FlagForceBold
	}

	return flags
}
