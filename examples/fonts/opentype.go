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
	"math"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"
	"seehuhn.de/go/sfnt/type1"
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

func (o *openTypeSimple) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	return o, nil
}

func (o *openTypeSimple) GetGeometry() *font.Geometry {
	return o.geometry
}

func (o *openTypeSimple) ResourceName() pdf.Name {
	return o.resName
}

func (o *openTypeSimple) Reference() pdf.Reference {
	return o.ref
}

func (o *openTypeSimple) Layout(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)
	return o.info.Layout(rr, o.gsubLookups, o.gposLookups)
}

func (o *openTypeSimple) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	o.text[gid] = rr
	return append(s, o.enc.Encode(gid, rr))
}

func (o *openTypeSimple) Close() error {
	if o.closed {
		return nil
	}
	o.closed = true

	if o.enc.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q (%s)",
			o.resName, o.info.PostscriptName())
	}
	o.enc = cmap.NewFrozenSimpleEncoder(o.enc)

	w := o.w

	// subset the font
	var ss []subset.Glyph
	ss = append(ss, subset.Glyph{OrigGID: 0, CID: 0})
	encoding := o.enc.Encoding()
	for cid, gid := range encoding {
		if gid != 0 {
			ss = append(ss, subset.Glyph{OrigGID: gid, CID: type1.CID(cid)})
		}
	}
	subsetTag := subset.Tag(ss, o.info.NumGlyphs())
	subsetInfo, err := subset.Simple(o.info, ss)
	if err != nil {
		return fmt.Errorf("font subset: %w", err)
	}

	fontName := pdf.Name(subsetTag + "+" + subsetInfo.PostscriptName())

	q := 1000 / float64(subsetInfo.UnitsPerEm)

	var Widths pdf.Array
	var firstChar type1.CID
	for int(firstChar) < len(encoding) && encoding[firstChar] == 0 {
		firstChar++
	}
	lastChar := type1.CID(len(encoding) - 1)
	for lastChar > firstChar && encoding[lastChar] == 0 {
		lastChar--
	}
	for i := firstChar; i <= lastChar; i++ {
		var width pdf.Integer
		gid := encoding[i]
		if gid != 0 {
			width = pdf.Integer(math.Round(o.info.GlyphWidth(gid).AsFloat(q)))
		}
		Widths = append(Widths, width)
	}

	FontDictRef := o.ref
	FontDescriptorRef := w.Alloc()
	WidthsRef := w.Alloc()
	FontFileRef := w.Alloc()
	ToUnicodeRef := w.Alloc()

	FontDict := pdf.Dict{ // See section 9.6.2.1 of PDF 32000-1:2008.
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("Type1"),
		"BaseFont":       fontName,
		"FirstChar":      pdf.Integer(firstChar),
		"LastChar":       pdf.Integer(lastChar),
		"Widths":         WidthsRef,
		"FontDescriptor": FontDescriptorRef,
		"ToUnicode":      ToUnicodeRef,
	}

	FontDescriptor := pdf.Dict{ // See section 9.8.1 of PDF 32000-1:2008.
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    fontName,
		"FontFile3":   FontFileRef,
		"Flags":       pdf.Integer(font.MakeFlags(subsetInfo, true)),
		"FontBBox":    &pdf.Rectangle{}, // empty rectangle is always allowed
		"ItalicAngle": pdf.Number(subsetInfo.ItalicAngle),
		"Ascent":      pdf.Integer(math.Round(subsetInfo.Ascent.AsFloat(q))),
		"Descent":     pdf.Integer(math.Round(subsetInfo.Descent.AsFloat(q))),
		"CapHeight":   pdf.Integer(math.Round(subsetInfo.CapHeight.AsFloat(q))),
		"StemV":       pdf.Integer(70), // information not available in sfnt files
	}

	// TODO(voss): use PrivateDict.StdVW from StemV in CFF fonts?

	compressedRefs := []pdf.Reference{FontDictRef, FontDescriptorRef, WidthsRef}
	compressedObjects := []pdf.Object{FontDict, FontDescriptor, Widths}

	// Write the "font program".
	// See section 9.9 of PDF 32000-1:2008 for details.
	fontFileDict := pdf.Dict{
		"Subtype": pdf.Name("OpenType"),
	}
	fontFileStream, err := w.OpenStream(FontFileRef, fontFileDict, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	_, err = subsetInfo.WriteCFFOpenTypePDF(fontFileStream)
	if err != nil {
		return err
	}
	if err != nil {
		return fmt.Errorf("embedding OpenType font %q: %w", fontName, err)
	}
	err = fontFileStream.Close()
	if err != nil {
		return err
	}

	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return err
	}

	var cc2text []font.SimpleMapping
	for code, gid := range encoding {
		if gid == 0 || len(o.text[gid]) == 0 {
			continue
		}
		rr := o.text[gid]
		cc2text = append(cc2text, font.SimpleMapping{Code: byte(code), Text: rr})
	}
	err = font.WriteToUnicodeSimple(w, ToUnicodeRef, subsetTag, cc2text)
	if err != nil {
		return err
	}

	return nil
}
