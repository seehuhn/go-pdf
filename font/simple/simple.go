// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

// Package simple provides support for embedding simple fonts into PDF documents.
package simple

import (
	"errors"
	"fmt"
	"math"
	"os"

	"golang.org/x/text/language"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"
	"seehuhn.de/go/sfnt/type1"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
)

// EmbedFile loads a font from a file and embeds it into a PDF file.
// At the moment, only TrueType and OpenType fonts are supported.
//
// Up to 256 distinct glyphs from the font file can be accessed via the
// returned font object.  In comparison, fonts embedded via cid.EmbedFile() lead
// to larger PDF files but there is no limit on the number of distinct glyphs
// which can be accessed.
func EmbedFile(w pdf.Putter, fname string, resName pdf.Name, loc language.Tag) (font.Embedded, error) {
	font, err := LoadFont(fname, loc)
	if err != nil {
		return nil, err
	}
	return font.Embed(w, resName)
}

// LoadFont loads a font from a file as a simple PDF font.
// At the moment, only TrueType and OpenType fonts are supported.
//
// Up to 256 distinct glyphs from the font file can be accessed via the
// returned font object.  In comparison, fonts embedded via cid.LoadFont() lead
// to larger PDF files but there is no limit on the number of distinct glyphs
// which can be accessed.
func LoadFont(fname string, loc language.Tag) (font.Font, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	info, err := sfnt.Read(fd)
	if err != nil {
		return nil, err
	}
	return Font(info, loc)
}

// Font creates a simple PDF font.
//
// Up to 256 distinct glyphs from the font file can be accessed via the
// returned font object.  In comparison, fonts embedded via cid.Font() lead
// to larger PDF files but there is no limit on the number of distinct glyphs
// which can be accessed.
func Font(info *sfnt.Info, loc language.Tag) (font.Font, error) {
	gsubLookups := info.Gsub.FindLookups(loc, gtab.GsubDefaultFeatures)
	gposLookups := info.Gpos.FindLookups(loc, gtab.GposDefaultFeatures)

	g := &font.Geometry{
		UnitsPerEm:   info.UnitsPerEm,
		GlyphExtents: info.Extents(),
		Widths:       info.Widths(),

		Ascent:             info.Ascent,
		Descent:            info.Descent,
		BaseLineSkip:       info.Ascent - info.Descent + info.LineGap,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
	}

	res := &simple{
		info:        info,
		gsubLookups: gsubLookups,
		gposLookups: gposLookups,
		g:           g,
	}

	return res, nil
}

type simple struct {
	info        *sfnt.Info
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex
	g           *font.Geometry
}

func (f *simple) GetGeometry() *font.Geometry {
	return f.g
}

func (f *simple) Layout(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)
	return f.info.Layout(rr, f.gsubLookups, f.gposLookups)
}

func (f *simple) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	if f.info.IsGlyf() {
		err := pdf.CheckVersion(w, "use of TrueType glyph outlines", pdf.V1_1)
		if err != nil {
			return nil, err
		}
	} else if f.info.IsCFF() {
		err := pdf.CheckVersion(w, "use of CFF glyph outlines", pdf.V1_2)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("unsupported font format")
	}

	res := &embedded{
		simple:  f,
		w:       w,
		ref:     w.Alloc(),
		resName: resName,
		enc:     cmap.NewSimpleEncoder(),
		text:    make(map[glyph.ID][]rune),
	}

	w.AutoClose(res)

	return res, nil
}

type embedded struct {
	*simple
	w       pdf.Putter
	ref     pdf.Reference
	resName pdf.Name
	enc     cmap.SimpleEncoder
	text    map[glyph.ID][]rune
	closed  bool
}

func (e *embedded) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	e.text[gid] = rr
	return append(s, e.enc.Encode(gid, rr))
}

func (e *embedded) Reference() pdf.Reference {
	return e.ref
}

func (e *embedded) ResourceName() pdf.Name {
	return e.resName
}

func (e *embedded) Close() error {
	if e.closed {
		return nil
	}
	e.closed = true

	if e.enc.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q (%s)",
			e.resName, e.info.PostscriptName())
	}
	e.enc = cmap.NewFrozenSimpleEncoder(e.enc)

	w := e.w

	// subset the font
	var ss []subset.Glyph
	ss = append(ss, subset.Glyph{OrigGID: 0, CID: 0})
	encoding := e.enc.Encoding()
	for cid, gid := range encoding {
		if gid != 0 {
			ss = append(ss, subset.Glyph{OrigGID: gid, CID: type1.CID(cid)})
		}
	}
	subsetTag := subset.Tag(ss, e.info.NumGlyphs())
	subsetInfo, err := subset.Simple(e.info, ss)
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
			width = pdf.Integer(math.Round(e.info.GlyphWidth(gid).AsFloat(q)))
		}
		Widths = append(Widths, width)
	}

	FontDictRef := e.ref
	FontDescriptorRef := w.Alloc()
	WidthsRef := w.Alloc()
	FontFileRef := w.Alloc()
	ToUnicodeRef := w.Alloc()

	FontDict := pdf.Dict{ // See section 9.6.2.1 of PDF 32000-1:2008.
		"Type":           pdf.Name("Font"),
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

	switch outlines := subsetInfo.Outlines.(type) {
	case *cff.Outlines:
		FontDict["Subtype"] = pdf.Name("Type1")
		FontDescriptor["FontFile3"] = FontFileRef

		// Write the "font program".
		// See section 9.9 of PDF 32000-1:2008 for details.
		fontFileDict := pdf.Dict{
			"Subtype": pdf.Name("Type1C"),
		}
		fontFileStream, err := w.OpenStream(FontFileRef, fontFileDict, pdf.FilterCompress{})
		if err != nil {
			return err
		}
		fontFile := cff.Font{
			FontInfo: subsetInfo.GetFontInfo(),
			Outlines: outlines,
		}
		err = fontFile.Encode(fontFileStream)
		if err != nil {
			return fmt.Errorf("embedding CFF font %q: %w", fontName, err)
		}
		err = fontFileStream.Close()
		if err != nil {
			return err
		}

	case *glyf.Outlines:
		FontDict["Subtype"] = pdf.Name("TrueType")
		FontDescriptor["FontFile2"] = FontFileRef

		// Write the "font program".
		// See section 9.9 of PDF 32000-1:2008 for details.
		size := pdf.NewPlaceholder(w, 10)
		fontFileDict := pdf.Dict{
			"Length1": size,
		}
		fontFileStream, err := w.OpenStream(FontFileRef, fontFileDict, pdf.FilterCompress{})
		if err != nil {
			return err
		}
		n, err := subsetInfo.WriteTrueTypePDF(fontFileStream)
		if err != nil {
			return err
		}
		err = size.Set(pdf.Integer(n))
		if err != nil {
			return err
		}
		err = fontFileStream.Close()
		if err != nil {
			return err
		}
	}

	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return err
	}

	var cc2text []font.SimpleMapping
	for code, gid := range encoding {
		if gid == 0 || len(e.text[gid]) == 0 {
			continue
		}
		rr := e.text[gid]
		cc2text = append(cc2text, font.SimpleMapping{Code: byte(code), Text: rr})
	}
	err = font.WriteToUnicodeSimple(w, ToUnicodeRef, subsetTag, cc2text)
	if err != nil {
		return err
	}

	return nil
}
