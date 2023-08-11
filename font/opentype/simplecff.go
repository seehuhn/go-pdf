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

package opentype

import (
	"errors"
	"fmt"
	"math"

	"golang.org/x/text/language"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/tounicode"
)

type SimpleFontCFF struct {
	info        *sfnt.Font
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex
	*font.Geometry
}

func NewSimpleCFF(info *sfnt.Font, loc language.Tag) (*SimpleFontCFF, error) {
	if !info.IsCFF() {
		return nil, errors.New("wrong font type")
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

	res := &SimpleFontCFF{
		info:        info,
		gsubLookups: info.Gsub.FindLookups(loc, gtab.GsubDefaultFeatures),
		gposLookups: info.Gpos.FindLookups(loc, gtab.GposDefaultFeatures),
		Geometry:    geometry,
	}
	return res, nil
}

func (f *SimpleFontCFF) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	err := pdf.CheckVersion(w, "use of OpenType fonts", pdf.V1_6)
	if err != nil {
		return nil, err
	}
	res := &embeddedSimpleCFF{
		SimpleFontCFF: f,
		w:             w,
		Resource:      pdf.Resource{Ref: w.Alloc(), Name: resName},
		SimpleEncoder: cmap.NewSimpleEncoder(),
		text:          map[glyph.ID][]rune{},
	}
	return res, nil
}

func (f *SimpleFontCFF) Layout(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)
	return f.info.Layout(rr, f.gsubLookups, f.gposLookups)
}

type embeddedSimpleCFF struct {
	*SimpleFontCFF
	w pdf.Putter
	pdf.Resource

	cmap.SimpleEncoder
	text   map[glyph.ID][]rune
	closed bool
}

func (f *embeddedSimpleCFF) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	f.text[gid] = rr
	return f.SimpleEncoder.AppendEncoded(s, gid, rr)
}

func (f *embeddedSimpleCFF) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.SimpleEncoder.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q (%s)",
			f.Name, f.info.PostscriptName())
	}
	f.SimpleEncoder = cmap.NewFrozenSimpleEncoder(f.SimpleEncoder)

	// subset the font
	var ss []subset.Glyph
	ss = append(ss, subset.Glyph{OrigGID: 0, CID: 0})
	encoding := f.SimpleEncoder.Encoding()
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
	info := PDFInfoCFF{
		Font:      subsetInfo,
		SubsetTag: subsetTag,
		Encoding:  subsetInfo.Outlines.(*cff.Outlines).Encoding,
		ToUnicode: m,
	}
	return info.WritePDF(f.w, f.Ref)
}

type PDFInfoCFF struct {
	// Font is the font to embed (already subsetted, if needed).
	Font *sfnt.Font

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	// Encoding is the encoding vector used by the client (a slice of length 256).
	// Together with the font's built-in encoding, this is used to determine
	// the `Encoding` entry of the PDF font dictionary.
	Encoding []glyph.ID

	// ToUnicode (optional) is a map from character codes to unicode strings.
	// Character codes must be in the range 0, ..., 255.
	// TODO(voss): or else?
	ToUnicode map[charcode.CharCode][]rune
}

func (info *PDFInfoCFF) WritePDF(w pdf.Putter, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "embedding of OpenType fonts", pdf.V1_6)
	if err != nil {
		return err
	}

	openTypeFont := info.Font
	cffOutlines, ok := openTypeFont.Outlines.(*cff.Outlines)
	if !ok ||
		len(cffOutlines.Encoding) != 256 ||
		len(cffOutlines.Private) != 1 ||
		len(cffOutlines.Glyphs) == 0 ||
		len(cffOutlines.Glyphs[0].Name) == 0 {
		return errors.New("not a CFF-based simple OpenType font")
	}

	var fontName pdf.Name
	postScriptName := openTypeFont.PostscriptName()
	if info.SubsetTag == "" {
		fontName = pdf.Name(postScriptName)
	} else {
		fontName = pdf.Name(info.SubsetTag + "+" + postScriptName)
	}

	unitsPerEm := openTypeFont.UnitsPerEm

	ww := make([]funit.Int16, 256)
	for i := range ww {
		ww[i] = openTypeFont.GlyphWidth(info.Encoding[i])
	}
	widthsInfo := font.CompressWidths(ww, unitsPerEm)

	encoding := make([]string, 256)
	builtin := make([]string, 256)
	for i := 0; i < 256; i++ {
		encoding[i] = openTypeFont.GlyphName(info.Encoding[i])
		builtin[i] = openTypeFont.GlyphName(cffOutlines.Encoding[i])
	}

	q := 1000 / float64(unitsPerEm)
	bbox := openTypeFont.BBox()
	fontBBox := &pdf.Rectangle{
		LLx: bbox.LLx.AsFloat(q),
		LLy: bbox.LLy.AsFloat(q),
		URx: bbox.URx.AsFloat(q),
		URy: bbox.URy.AsFloat(q),
	}

	widthsRef := w.Alloc()
	fontDescriptorRef := w.Alloc()
	fontFileRef := w.Alloc()

	// See section 9.6.2.1 of PDF 32000-1:2008.
	fontDict := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("Type1"),
		"BaseFont":       fontName,
		"FirstChar":      widthsInfo.FirstChar,
		"LastChar":       widthsInfo.LastChar,
		"Widths":         widthsRef,
		"FontDescriptor": fontDescriptorRef,
	}
	if enc := font.DescribeEncodingType1(encoding, builtin); enc != nil {
		fontDict["Encoding"] = enc
	}
	var toUnicodeRef pdf.Reference
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
		fontDict["ToUnicode"] = toUnicodeRef
	}

	// See section 9.8.1 of PDF 32000-1:2008.
	fontDescriptor := pdf.Dict{
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    fontName,
		"Flags":       pdf.Integer(font.MakeFlags(openTypeFont, true)),
		"FontBBox":    fontBBox,
		"ItalicAngle": pdf.Number(openTypeFont.ItalicAngle),
		"Ascent":      pdf.Integer(math.Round(openTypeFont.Ascent.AsFloat(q))),
		"Descent":     pdf.Integer(math.Round(openTypeFont.Descent.AsFloat(q))),
		"StemV":       pdf.Integer(math.Round(cffOutlines.Private[0].StdVW * q)),
		"FontFile3":   fontFileRef,
	}
	if openTypeFont.CapHeight != 0 {
		fontDescriptor["CapHeight"] = pdf.Integer(math.Round(openTypeFont.CapHeight.AsFloat(q)))
	}
	if widthsInfo.MissingWidth != 0 {
		fontDescriptor["MissingWidth"] = widthsInfo.MissingWidth
	}

	compressedRefs := []pdf.Reference{fontDictRef, fontDescriptorRef, widthsRef}
	compressedObjects := []pdf.Object{fontDict, fontDescriptor, widthsInfo.Widths}
	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return err
	}

	// See section 9.9 of PDF 32000-1:2008 for details.
	fontFileDict := pdf.Dict{
		"Subtype": pdf.Name("OpenType"),
	}
	fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	err = openTypeFont.WriteCFFOpenTypePDF(fontFileStream)
	if err != nil {
		return fmt.Errorf("embedding OpenType font %q: %w", fontName, err)
	}
	err = fontFileStream.Close()
	if err != nil {
		return err
	}

	if toUnicodeRef != 0 {
		err = tounicode.Embed(w, toUnicodeRef, charcode.Simple, info.ToUnicode)
		if err != nil {
			return err
		}
	}

	return nil
}
