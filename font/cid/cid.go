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
)

// Inside the PDF documents on my laptop, the following encodings are used
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
func Embed(w pdf.Putter, info *sfnt.Info, resName pdf.Name, loc language.Tag) (font.Embedded, error) {
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

	res := &cidfont{
		info:        info,
		gsubLookups: gsubLookups,
		gposLookups: gposLookups,
		g:           g,
	}

	return res, nil
}

type cidfont struct {
	info        *sfnt.Info
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
		cidfont: f,
		w:       w,
		ref:     w.Alloc(),
		resName: resName,
		enc:     cmap.NewCIDEncoder(),
		text:    make(map[glyph.ID][]rune),
	}

	w.AutoClose(res)

	return res, nil
}

type embedded struct {
	*cidfont
	w       pdf.Putter
	ref     pdf.Reference
	resName pdf.Name
	enc     cmap.CIDEncoder
	text    map[glyph.ID][]rune
	closed  bool
}

func (e *embedded) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	e.text[gid] = rr
	return append(s, e.enc.Encode(gid, rr)...)
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
	encoding := e.enc.Encoding()
	var subsetGlyphs []glyph.ID
	subsetGlyphs = append(subsetGlyphs, 0) // always include the .notdef glyph
	for _, p := range encoding {
		subsetGlyphs = append(subsetGlyphs, p.GID)
	}
	subsetTag := font.GetSubsetTag(subsetGlyphs, e.info.NumGlyphs())

	// TODO(voss): make sure there is only one copy of this per PDF file.
	CIDSystemInfo := e.enc.CIDSystemInfo()
	ROS := pdf.Dict{
		"Registry":   pdf.String(CIDSystemInfo.Registry),
		"Ordering":   pdf.String(CIDSystemInfo.Ordering),
		"Supplement": pdf.Integer(CIDSystemInfo.Supplement),
	}

	// There are three cases for mapping CID to GID values:
	// - For TrueType-based CIDFonts (Type 2 CIDFonts), /CidToGidMap in the CIDFont dictionary is used.
	// - For CFF-based CIDFonts (Type 0 CIDFonts), [cff.Outlines.Gid2cid] is used.
	// - When CFF-based simple fonts are used as CIDFonts we have CID == GID.
	//   (We can avoid this case by converting the CFF font to a CIDFont.)

	// subset the font
	subsetInfo := &sfnt.Info{}
	*subsetInfo = *e.info
	switch outlines := e.info.Outlines.(type) {
	case *cff.Outlines:
		o2 := &cff.Outlines{}
		pIdxMap := make(map[int]int)
		for _, gid := range subsetGlyphs {
			o2.Glyphs = append(o2.Glyphs, outlines.Glyphs[gid])
			oldPIdx := outlines.FdSelect(gid)
			_, ok := pIdxMap[oldPIdx]
			if !ok {
				newPIdx := len(o2.Private)
				o2.Private = append(o2.Private, outlines.Private[oldPIdx])
				pIdxMap[oldPIdx] = newPIdx
			}
		}
		o2.FdSelect = func(gid glyph.ID) int {
			origGid := glyph.ID(o2.Gid2cid[gid])
			return pIdxMap[outlines.FdSelect(origGid)]
		}
		o2.ROS = CIDSystemInfo
		o2.Gid2cid = make([]type1.CID, len(subsetGlyphs))
		if len(outlines.Gid2cid) > 0 {
			for subsetGid, origGid := range subsetGlyphs {
				o2.Gid2cid[subsetGid] = outlines.Gid2cid[origGid]
			}
		} else {
			// TODO(voss): what to do here?
			for subsetGid, origGid := range subsetGlyphs {
				o2.Gid2cid[subsetGid] = type1.CID(origGid)
			}
		}
		subsetInfo.Outlines = o2

	case *glyf.Outlines:
		newGid := make(map[glyph.ID]glyph.ID)
		todo := make(map[glyph.ID]bool)
		nextGid := glyph.ID(0)
		for _, gid := range subsetGlyphs {
			newGid[gid] = nextGid
			nextGid++

			for _, gid2 := range outlines.Glyphs[gid].Components() {
				if _, ok := newGid[gid2]; !ok {
					todo[gid2] = true
				}
			}
		}
		for len(todo) > 0 {
			gid := pop(todo)
			subsetGlyphs = append(subsetGlyphs, gid)
			newGid[gid] = nextGid
			nextGid++

			for _, gid2 := range outlines.Glyphs[gid].Components() {
				if _, ok := newGid[gid2]; !ok {
					todo[gid2] = true
				}
			}
		}

		o2 := &glyf.Outlines{
			Tables: outlines.Tables,
			Maxp:   outlines.Maxp,
		}
		for _, gid := range subsetGlyphs {
			g := outlines.Glyphs[gid]
			o2.Glyphs = append(o2.Glyphs, g.FixComponents(newGid))
			o2.Widths = append(o2.Widths, outlines.Widths[gid])
		}
		subsetInfo.Outlines = o2

		// Use /CidToGidMap in the CIDFont dictionary (below) to specify the
		// mapping from CID to GID values.
		subsetInfo.CMap = nil
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

	// TODO(voss): For CFF fonts we can get StemV from the PrivateDict structure.
	FontDescriptor := pdf.Dict{ // See section 9.8.1 of PDF 32000-1:2008.
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    fontName,
		"Flags":       pdf.Integer(font.MakeFlags(subsetInfo, true)),
		"FontBBox":    &pdf.Rectangle{}, // empty rectangle is allowed here
		"ItalicAngle": pdf.Number(subsetInfo.ItalicAngle),
		"Ascent":      pdf.Integer(math.Round(subsetInfo.Ascent.AsFloat(q))),
		"Descent":     pdf.Integer(math.Round(subsetInfo.Descent.AsFloat(q))),
		"CapHeight":   pdf.Integer(math.Round(subsetInfo.CapHeight.AsFloat(q))),
		"StemV":       pdf.Integer(70), // information not available in sfnt files
	}

	compressedRefs := []pdf.Reference{FontDictRef, CIDFontRef, CIDSystemInfoRef, FontDescriptorRef}
	compressedObjects := []pdf.Object{FontDict, CIDFont, ROS, FontDescriptor}

	DW, W := encodeWidths(e.info.Widths(), q)
	if W != nil {
		WidthsRef := w.Alloc()
		CIDFont["W"] = WidthsRef
		if DW != 1000 {
			CIDFont["DW"] = DW
		}

		compressedRefs = append(compressedRefs, WidthsRef)
		compressedObjects = append(compressedObjects, W)
	}

	err := w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return err
	}

	switch outlines := subsetInfo.Outlines.(type) {
	case *cff.Outlines:
		CIDFont["Subtype"] = pdf.Name("CIDFontType0")
		FontDescriptor["FontFile3"] = FontFileRef
		FontDict["BaseFont"] = fontName + "-" + "Identity-H"

		// Write the "font program".
		// See section 9.9 of PDF 32000-1:2008 for details.
		fontFileDict := pdf.Dict{
			"Subtype": pdf.Name("CIDFontType0C"),
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
			return err
		}
		err = fontFileStream.Close()
		if err != nil {
			return err
		}

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
		n, err := subsetInfo.PDFEmbedTrueType(fontFileStream)
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

func pop(todo map[glyph.ID]bool) glyph.ID {
	for key := range todo {
		delete(todo, key)
		return key
	}
	panic("empty map")
}
