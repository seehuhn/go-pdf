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

package truetype

import (
	"errors"
	"fmt"

	"golang.org/x/text/language"

	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	pdfcmap "seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/tounicode"
)

type SimpleFont struct {
	ttf         *sfnt.Font
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex
	*font.Geometry
}

func NewSimple(info *sfnt.Font, loc language.Tag) (*SimpleFont, error) {
	if !info.IsGlyf() {
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

	res := &SimpleFont{
		ttf:         info,
		gsubLookups: info.Gsub.FindLookups(loc, gtab.GsubDefaultFeatures),
		gposLookups: info.Gpos.FindLookups(loc, gtab.GposDefaultFeatures),
		Geometry:    geometry,
	}
	return res, nil
}

func (f *SimpleFont) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	err := pdf.CheckVersion(w, "simple TrueType fonts", pdf.V1_1)
	if err != nil {
		return nil, err
	}
	res := &embeddedSimple{
		SimpleFont:    f,
		w:             w,
		Resource:      pdf.Resource{Ref: w.Alloc(), Name: resName},
		SimpleEncoder: pdfcmap.NewSimpleEncoder(),
		text:          map[glyph.ID][]rune{},
	}
	w.AutoClose(res)
	return res, nil
}

func (f *SimpleFont) Layout(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)
	return f.ttf.Layout(rr, f.gsubLookups, f.gposLookups)
}

type embeddedSimple struct {
	*SimpleFont
	w pdf.Putter
	pdf.Resource

	pdfcmap.SimpleEncoder
	text   map[glyph.ID][]rune
	closed bool
}

func (f *embeddedSimple) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	f.text[gid] = rr
	return f.SimpleEncoder.AppendEncoded(s, gid, rr)
}

func (f *embeddedSimple) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.SimpleEncoder.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q (%s)",
			f.Name, f.ttf.PostscriptName())
	}
	f.SimpleEncoder = pdfcmap.NewFrozenSimpleEncoder(f.SimpleEncoder)

	// subset the font
	var ss []subset.Glyph
	ss = append(ss, subset.Glyph{OrigGID: 0, CID: 0})
	encoding := f.SimpleEncoder.Encoding()
	for cid, gid := range encoding {
		if gid != 0 {
			ss = append(ss, subset.Glyph{OrigGID: gid, CID: type1.CID(cid)})
		}
	}
	subsetTag := subset.Tag(ss, f.ttf.NumGlyphs())
	ttfSubset, err := subset.Simple(f.ttf, ss)
	if err != nil {
		return fmt.Errorf("font subset: %w", err)
	}

	subsetEncoding := make([]glyph.ID, 256)
	for subsetGid, g := range ss {
		if subsetGid == 0 {
			continue
		}
		subsetEncoding[g.CID] = glyph.ID(subsetGid)
	}

	toUnicode := make(map[charcode.CharCode][]rune)
	for code, gid := range encoding {
		if gid == 0 || len(f.text[gid]) == 0 {
			continue
		}
		toUnicode[charcode.CharCode(code)] = f.text[gid]
	}

	info := EmbedInfo{
		Font:      ttfSubset,
		SubsetTag: subsetTag,
		Encoding:  subsetEncoding,
		ToUnicode: toUnicode,
	}
	return info.Embed(f.w, f.Ref)
}

// TODO(voss): should this be merged with opentype.PDFInfoGlyf?
type EmbedInfo struct {
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

	IsAllCap   bool
	IsSmallCap bool
	ForceBold  bool
}

func (info *EmbedInfo) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "simple TrueType fonts", pdf.V1_1)
	if err != nil {
		return err
	}

	ttf := info.Font
	if !ttf.IsGlyf() {
		return fmt.Errorf("not a TrueType font")
	}

	fontName := ttf.PostscriptName()
	if info.SubsetTag != "" {
		fontName = info.SubsetTag + "+" + fontName
	}

	unitsPerEm := ttf.UnitsPerEm

	ww := make([]funit.Int16, 256)
	for i := range ww {
		ww[i] = ttf.GlyphWidth(info.Encoding[i])
	}
	widthsInfo := font.CompressWidths(ww, unitsPerEm)

	var encoding pdf.Object
	var cmapTable cmap.Table

	// Mark the font as "symbolic", and use a (1, 0) "cmap" subtable to map
	// character codes to glyphs.
	isSymbolic := true
	subtable := cmap.Format4{}
	for i, gid := range info.Encoding {
		if gid == 0 || i >= 256 {
			continue
		}
		subtable[uint16(i)] = gid
	}
	cmapTable = cmap.Table{
		{PlatformID: 1, EncodingID: 0}: subtable.Encode(0),
	}
	cmapData := cmapTable.Encode()

	q := 1000 / float64(unitsPerEm)
	bbox := ttf.BBox()
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
		"Subtype":        pdf.Name("TrueType"),
		"BaseFont":       pdf.Name(fontName),
		"FirstChar":      widthsInfo.FirstChar,
		"LastChar":       widthsInfo.LastChar,
		"Widths":         widthsRef,
		"FontDescriptor": fontDescriptorRef,
	}
	if encoding != nil {
		fontDict["Encoding"] = encoding
	}
	var toUnicodeRef pdf.Reference
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
		fontDict["ToUnicode"] = toUnicodeRef
	}
	fd := &font.Descriptor{
		FontName:     fontName,
		IsFixedPitch: ttf.IsFixedPitch(),
		IsSerif:      ttf.IsSerif,
		IsScript:     ttf.IsScript,
		IsItalic:     ttf.ItalicAngle != 0,
		IsAllCap:     info.IsAllCap,
		IsSmallCap:   info.IsSmallCap,
		ForceBold:    info.ForceBold,
		FontBBox:     fontBBox,
		ItalicAngle:  ttf.ItalicAngle,
		Ascent:       ttf.Ascent.AsFloat(q),
		Descent:      ttf.Descent.AsFloat(q),
		CapHeight:    ttf.CapHeight.AsFloat(q),
		MissingWidth: widthsInfo.MissingWidth,
	}
	fontDescriptor := fd.AsDict(isSymbolic)
	fontDescriptor["FontFile2"] = fontFileRef

	compressedRefs := []pdf.Reference{fontDictRef, fontDescriptorRef, widthsRef}
	compressedObjects := []pdf.Object{fontDict, fontDescriptor, widthsInfo.Widths}
	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "simple TrueType font dicts")
	}

	// See section 9.9 of PDF 32000-1:2008 for details.
	length1 := pdf.NewPlaceholder(w, 10)
	fontFileDict := pdf.Dict{
		"Length1": length1,
	}
	fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	n, err := ttf.WriteTrueTypePDF(fontFileStream, cmapData)
	if err != nil {
		return fmt.Errorf("embedding TrueType font %q: %w", fontName, err)
	}
	err = length1.Set(pdf.Integer(n))
	if err != nil {
		return err
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
