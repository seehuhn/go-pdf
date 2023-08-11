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

// Package cid provides support for embedding CID fonts into PDF documents.
package cid

import (
	"errors"
	"fmt"
	"math"
	"os"

	"golang.org/x/text/language"

	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	ttfcmap "seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	pdfcff "seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
)

// Inside the PDF documents on my laptop, the following encoding CMaps are used
// for CIDFonts.  The numbers are the number of occurences of the encoding:
//
//   3110 /Identity-H
//      6 /UniCNS-UTF16-H
//      5 /Identity-V
//      5 /UniGB-UCS2-H
//      4 /UniKS-UCS2-H
//      3 /UniJIS-UCS2-H
//      2 /90msp-RKSJ-H
//      2 /KSCms-UHC-H
//      2 /UniGB-UTF16-H
//      2 (indirect reference to embedded CMap)
//      1 /ETenms-B5-H
//      1 /GBK-EUC-H

// EmbedFile loads a font from a file and embeds it into a PDF file.
// At the moment, only TrueType and OpenType fonts are supported.
func EmbedFile(w pdf.Putter, fname string, resName pdf.Name, loc language.Tag) (font.Embedded, error) {
	font, err := LoadFont(fname, loc)
	if err != nil {
		return nil, err
	}
	return font.Embed(w, resName)
}

// Embed creates a PDF CIDFont and embeds it into a PDF file.
// At the moment, only TrueType and OpenType fonts are supported.
func Embed(w pdf.Putter, info *sfnt.Font, resName pdf.Name, loc language.Tag) (font.Embedded, error) {
	f, err := Font(info, loc)
	if err != nil {
		return nil, err
	}
	return f.Embed(w, resName)
}

// LoadFont loads a font from a file as a PDF CIDFont.
// At the moment, only TrueType and OpenType fonts are supported.
//
// CIDFonts lead to larger PDF files than simple fonts, but there is no limit
// on the number of distinct glyphs which can be accessed.
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

// Font creates a PDF CIDFont.
//
// CIDFonts lead to larger PDF files than simple fonts, but there is no limit
// on the number of distinct glyphs which can be accessed.
func Font(info *sfnt.Font, loc language.Tag) (font.Font, error) {
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

	res := &cidfont{
		info:        info,
		gsubLookups: gsubLookups,
		gposLookups: gposLookups,
		g:           g,
	}

	return res, nil
}

type cidfont struct {
	info        *sfnt.Font
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex
	g           *font.Geometry
}

func (f *cidfont) GetGeometry() *font.Geometry {
	return f.g
}

func (f *cidfont) Layout(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)
	return f.info.Layout(rr, f.gsubLookups, f.gposLookups)
}

func (f *cidfont) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	if f.info.IsGlyf() {
		err := pdf.CheckVersion(w, "use of TrueType glyph outlines", pdf.V1_1)
		if err != nil {
			return nil, err
		}
	} else if f.info.IsCFF() {
		err := pdf.CheckVersion(w, "use of CFF glyph outlines", pdf.V1_3)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("unsupported font format")
	}

	res := &embedded{
		cidfont:    f,
		w:          w,
		ref:        w.Alloc(),
		resName:    resName,
		CIDEncoder: cmap.NewCIDEncoder(),
	}

	w.AutoClose(res)

	return res, nil
}

type embedded struct {
	*cidfont
	w       pdf.Putter
	ref     pdf.Reference
	resName pdf.Name
	cmap.CIDEncoder
	closed bool
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

	w := e.w

	// Determine the subset of glyphs to include.
	encoding := e.CIDEncoder.Encoding()
	var subsetGlyphs []glyph.ID
	subsetGlyphs = append(subsetGlyphs, 0) // always include the .notdef glyph
	for _, p := range encoding {
		subsetGlyphs = append(subsetGlyphs, p.GID)
	}

	// TODO(voss): make sure there is only one copy of this per PDF file.
	CIDSystemInfo := e.CIDEncoder.CIDSystemInfo()
	ROS := pdf.Dict{
		"Registry":   pdf.String(CIDSystemInfo.Registry),
		"Ordering":   pdf.String(CIDSystemInfo.Ordering),
		"Supplement": pdf.Integer(CIDSystemInfo.Supplement),
	}

	// subset the font
	var ss []subset.Glyph
	ss = append(ss, subset.Glyph{OrigGID: 0, CID: 0})
	for _, p := range encoding {
		ss = append(ss, subset.Glyph{OrigGID: p.GID, CID: p.CID})
	}
	subsetInfo, err := subset.CID(e.info, ss, CIDSystemInfo)
	if err != nil {
		return fmt.Errorf("font subset: %w", err)
	}
	subsetTag := subset.Tag(ss, e.info.NumGlyphs())

	if _, ok := subsetInfo.Outlines.(*cff.Outlines); ok {
		cmap := make(map[charcode.CharCode]type1.CID)
		for _, s := range ss {
			cmap[charcode.CharCode(s.CID)] = s.CID
		}
		toUnicode := make(map[charcode.CharCode][]rune)
		for _, e := range encoding {
			toUnicode[charcode.CharCode(e.CID)] = e.Text
		}
		info := &pdfcff.PDFInfoCID{
			Font:       subsetInfo.AsCFF(),
			SubsetTag:  subsetTag,
			CS:         charcode.UCS2,
			ROS:        CIDSystemInfo,
			CMap:       cmap,
			ToUnicode:  toUnicode,
			UnitsPerEm: subsetInfo.UnitsPerEm,
			Ascent:     subsetInfo.Ascent,
			Descent:    subsetInfo.Descent,
			CapHeight:  subsetInfo.CapHeight,
			IsSerif:    subsetInfo.IsSerif,
			IsScript:   subsetInfo.IsScript,
		}
		return info.Embed(w, e.ref)
	}

	fontName := pdf.Name(subsetTag + "+" + subsetInfo.PostscriptName())

	q := 1000 / float64(subsetInfo.UnitsPerEm)

	FontDictRef := e.ref
	CIDFontRef := w.Alloc()
	CIDSystemInfoRef := w.Alloc()
	FontDescriptorRef := w.Alloc()
	FontFileRef := w.Alloc()
	ToUnicodeRef := w.Alloc()

	FontDict := pdf.Dict{ // See section 9.7.6.1 of PDF 32000-1:2008.
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"Encoding":        pdf.Name("Identity-H"), // TODO(voss)
		"DescendantFonts": pdf.Array{CIDFontRef},
		"ToUnicode":       ToUnicodeRef,
	}

	CIDFont := pdf.Dict{ // See section 9.7.4.1 of PDF 32000-1:2008.
		"Type":           pdf.Name("Font"),
		"BaseFont":       fontName,
		"CIDSystemInfo":  CIDSystemInfoRef,
		"FontDescriptor": FontDescriptorRef,
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

	// TODO(voss): use PrivateDict.StdVW for StemV in CFF fonts?

	compressedRefs := []pdf.Reference{FontDictRef, CIDFontRef, CIDSystemInfoRef, FontDescriptorRef}
	compressedObjects := []pdf.Object{FontDict, CIDFont, ROS, FontDescriptor}

	var ww []font.CIDWidth
	widths := subsetInfo.Widths()
	for subsetGid, g := range ss {
		ww = append(ww, font.CIDWidth{CID: g.CID, GlyphWidth: widths[subsetGid]})
	}
	DW, W := font.EncodeCIDWidths(ww, subsetInfo.UnitsPerEm)
	if W != nil {
		WidthsRef := w.Alloc()
		CIDFont["W"] = WidthsRef
		if DW != 1000 {
			CIDFont["DW"] = DW
		}

		compressedRefs = append(compressedRefs, WidthsRef)
		compressedObjects = append(compressedObjects, W)
	}

	switch subsetInfo.Outlines.(type) {
	case *glyf.Outlines:
		CID2GIDMapRef := w.Alloc()

		CIDFont["Subtype"] = pdf.Name("CIDFontType2")
		CIDFont["CIDToGIDMap"] = CID2GIDMapRef
		FontDescriptor["FontFile2"] = FontFileRef
		FontDict["BaseFont"] = fontName

		cid2gidStream, err := w.OpenStream(CID2GIDMapRef, nil,
			pdf.FilterCompress{
				"Predictor": pdf.Integer(12),
				"Columns":   pdf.Integer(2),
			})
		if err != nil {
			return err
		}
		cid2gid := make([]byte, 2*e.info.NumGlyphs())
		for gid, cid := range subsetGlyphs {
			cid2gid[2*cid] = byte(gid >> 8)
			cid2gid[2*cid+1] = byte(gid)
		}
		_, err = cid2gidStream.Write(cid2gid)
		if err != nil {
			return err
		}
		err = cid2gidStream.Close()
		if err != nil {
			return err
		}

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
		var cmapData []byte
		if subsetInfo.CMap != nil {
			ss := ttfcmap.Table{
				{PlatformID: 1, EncodingID: 0}: subsetInfo.CMap.Encode(0),
			}
			cmapData = ss.Encode()
		}
		n, err := subsetInfo.WriteTrueTypePDF(fontFileStream, cmapData)
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

	var cc2text []font.CIDMapping
	for _, e := range encoding {
		if len(e.Text) == 0 {
			continue
		}
		cc2text = append(cc2text, font.CIDMapping{
			CharCode: uint16(e.CID),
			Text:     e.Text,
		})
	}
	err = font.WriteToUnicodeCID(ToUnicodeRef, w, cc2text)
	if err != nil {
		return err
	}

	return nil
}
