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
	"sort"

	"golang.org/x/text/language"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/funit"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"
	"seehuhn.de/go/sfnt/type1"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
)

// LoadFont loads a font from a file as a PDF CIDFont.
// At the moment, only TrueType and OpenType fonts are supported.
//
// CIDFonts lead to larger PDF files than simple fonts, but there is no limit
// on the number of distinct glyphs which can be accessed.
func LoadFont(fname string, resourceName pdf.Name, loc language.Tag) (*font.NewFont, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	info, err := sfnt.Read(fd)
	if err != nil {
		return nil, err
	}
	return Font(info, resourceName, loc)
}

// Font creates a PDF CIDFont.
//
// CIDFonts lead to larger PDF files than simple fonts, but there is no limit
// on the number of distinct glyphs which can be accessed.
func Font(info *sfnt.Info, resourceName pdf.Name, loc language.Tag) (*font.NewFont, error) {
	gsubLookups := info.Gsub.FindLookups(loc, gtab.GsubDefaultFeatures)
	gposLookups := info.Gpos.FindLookups(loc, gtab.GposDefaultFeatures)

	defaultText := make(map[glyph.ID][]rune)
	low, high := info.CMap.CodeRange()
	for r := low; r <= high; r++ {
		gid := info.CMap.Lookup(r)
		if gid != 0 {
			defaultText[gid] = []rune{r}
		}
	}

	widths := info.Widths()

	sfi := &sharedFontInfo{
		info:        info,
		widths:      widths,
		gsubLookups: gsubLookups,
		gposLookups: gposLookups,
		defaultText: defaultText,
	}

	res := &font.NewFont{
		Geometry: font.Geometry{
			UnitsPerEm:         info.UnitsPerEm,
			Ascent:             info.Ascent,
			Descent:            info.Descent,
			BaseLineSkip:       info.Ascent - info.Descent + info.LineGap,
			UnderlinePosition:  info.UnderlinePosition,
			UnderlineThickness: info.UnderlineThickness,
			GlyphExtents:       info.Extents(),
			Widths:             widths,
		},
		Layout:       sfi.Typeset,
		ResourceName: resourceName,
		GetDict: func(w *pdf.Writer, resName pdf.Name) (font.Dict, error) {
			return getDict(w, resName, sfi)
		},
	}
	return res, nil
}

type sharedFontInfo struct {
	info        *sfnt.Info
	widths      []funit.Int16
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex
	defaultText map[glyph.ID][]rune
}

func (sfi *sharedFontInfo) Typeset(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)
	return sfi.info.Layout(rr, sfi.gsubLookups, sfi.gposLookups)
}

type fontDict struct {
	w           *pdf.Writer
	fontDictRef *pdf.Reference
	resName     pdf.Name
	*sharedFontInfo
	enc cmap.CIDEncoder
}

func getDict(w *pdf.Writer, resName pdf.Name, sfi *sharedFontInfo) (font.Dict, error) {
	if sfi.info.IsGlyf() {
		err := w.CheckVersion("use of TrueType glyph outlines", pdf.V1_1)
		if err != nil {
			return nil, err
		}
	} else if sfi.info.IsCFF() {
		err := w.CheckVersion("use of CFF glyph outlines", pdf.V1_3)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("unsupported font format")
	}

	return &fontDict{
		w:              w,
		fontDictRef:    w.Alloc(),
		resName:        resName,
		sharedFontInfo: sfi,
		enc:            cmap.NewCIDEncoder(),
	}, nil
}

func (fd *fontDict) Reference() *pdf.Reference {
	return fd.fontDictRef
}

func (fd *fontDict) ResourceName() pdf.Name {
	return fd.resName
}

func (fd *fontDict) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	if rr == nil {
		rr = fd.defaultText[gid]
	}
	return append(s, fd.enc.Encode(gid, rr)...)
}

func (fd *fontDict) GetUnitsPerEm() uint16 {
	return fd.info.UnitsPerEm
}

func (fd *fontDict) GetWidths() []funit.Int16 {
	return fd.widths
}

func (fd *fontDict) Close() error {
	w := fd.w

	// Determine the subset of glyphs to include.
	encoding := fd.enc.Encoding()
	var subsetGlyphs []glyph.ID
	subsetGlyphs = append(subsetGlyphs, 0) // always include the .notdef glyph
	for _, p := range encoding {
		subsetGlyphs = append(subsetGlyphs, p.GID)
	}
	subsetTag := font.GetSubsetTag(subsetGlyphs, fd.info.NumGlyphs())

	// TODO(voss): make sure there is only one copy of this per PDF file.
	CIDSystemInfo := fd.enc.CIDSystemInfo()
	ROS := pdf.Dict{
		"Registry":   pdf.String(CIDSystemInfo.Registry),
		"Ordering":   pdf.String(CIDSystemInfo.Ordering),
		"Supplement": pdf.Integer(CIDSystemInfo.Supplement),
	}

	// There are three cases for mapping CID to GID values:
	// - For CFF-based CIDFonts, [cff.Outlines.Gid2cid] is used.
	// - For CFF-based simple fonts we have CID == GID.
	//   (We can avoid this case writing any CFF font as a CIDFont.)
	// - For TrueType fonts, /CidToGidMap in the CIDFont dictionary is used.

	// subset the font
	subsetInfo := &sfnt.Info{}
	*subsetInfo = *fd.info
	switch outlines := fd.info.Outlines.(type) {
	case *cff.Outlines:
		o2 := &cff.Outlines{}
		pIdxMap := make(map[int]int)
		for _, gid := range subsetGlyphs {
			o2.Glyphs = append(o2.Glyphs, outlines.Glyphs[gid])
			oldPIdx := outlines.FdSelect(gid)
			_, ok := pIdxMap[oldPIdx]
			if !ok {
				newPIdx := len(o2.Private)
				pIdxMap[oldPIdx] = newPIdx
				o2.Private = append(o2.Private, outlines.Private[oldPIdx])
			}
		}
		o2.FdSelect = func(gid glyph.ID) int {
			return pIdxMap[outlines.FdSelect(gid)]
		}
		o2.ROS = CIDSystemInfo
		o2.Gid2cid = make([]int32, len(subsetGlyphs))
		for subsetGid, origGid := range subsetGlyphs {
			// TODO(voss): we need to properly translate GID -> CID here.
			o2.Gid2cid[subsetGid] = int32(origGid)
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

	DW, W := encodeWidths(fd.info.Widths(), q)

	FontDictRef := fd.fontDictRef
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

	rect := subsetInfo.BBox()
	fontBBox := &pdf.Rectangle{
		LLx: rect.LLx.AsFloat(q),
		LLy: rect.LLy.AsFloat(q),
		URx: rect.URx.AsFloat(q),
		URy: rect.URy.AsFloat(q),
	}

	FontDescriptor := pdf.Dict{ // See section 9.8.1 of PDF 32000-1:2008.
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    fontName,
		"Flags":       pdf.Integer(font.MakeFlags(subsetInfo, true)), // TODO(voss)
		"FontBBox":    fontBBox,
		"ItalicAngle": pdf.Number(subsetInfo.ItalicAngle),
		"Ascent":      pdf.Integer(math.Round(subsetInfo.Ascent.AsFloat(q))),
		"Descent":     pdf.Integer(math.Round(subsetInfo.Descent.AsFloat(q))),
		"CapHeight":   pdf.Integer(math.Round(subsetInfo.CapHeight.AsFloat(q))),
		"StemV":       pdf.Integer(70), // information not available in sfnt files
	}

	compressedRefs := []*pdf.Reference{FontDictRef, CIDFontRef, CIDSystemInfoRef, FontDescriptorRef}
	compressedObjects := []pdf.Object{FontDict, CIDFont, ROS, FontDescriptor}

	if W != nil {
		WidthsRef := w.Alloc()
		CIDFont["W"] = WidthsRef
		if DW != 1000 {
			CIDFont["DW"] = DW
		}

		compressedRefs = append(compressedRefs, WidthsRef)
		compressedObjects = append(compressedObjects, W)
	}

	compress := &pdf.FilterInfo{Name: pdf.Name("LZWDecode")}
	if w.Version >= pdf.V1_2 {
		compress.Name = "FlateDecode"
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
		fontFileStream, _, err := w.OpenStream(fontFileDict, FontFileRef,
			compress)
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

		cid2gidStream, _, err := w.OpenStream(nil, CID2GIDMapRef,
			&pdf.FilterInfo{
				Name: compress.Name,
				Parms: pdf.Dict{
					"Predictor": pdf.Integer(12),
					"Columns":   pdf.Integer(2),
				},
			})
		if err != nil {
			return err
		}
		cid2gid := make([]byte, 2*fd.info.NumGlyphs())
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
		size := w.NewPlaceholder(10)
		fontFileDict := pdf.Dict{
			"Length1": size,
		}
		fontFileStream, _, err := w.OpenStream(fontFileDict, FontFileRef, compress)
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

	_, err := w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return err
	}

	var cc2text []font.CIDMapping
	for _, e := range encoding {
		cc2text = append(cc2text, font.CIDMapping{
			CharCode: uint16(e.CID),
			Text:     e.Text,
		})
	}
	_, err = font.WriteToUnicodeCID(w, cc2text, ToUnicodeRef)
	if err != nil {
		return err
	}

	return nil
}

// EmbedFile embeds the named font file into the PDF document.
func EmbedFile(w *pdf.Writer, fname string, instName pdf.Name, loc language.Tag) (*font.Font, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	fontInfo, err := sfnt.Read(fd)
	if err != nil {
		return nil, err
	}
	return Embed(w, fontInfo, instName, language.AmericanEnglish)
}

// Embed embeds a TrueType or OpenType font into a PDF document as a CID font.
//
// In comparison, fonts embedded via simple.Embed() lead to smaller PDF files,
// but only up to 256 glyphs of the font can be accessed via the returned font
// object.
//
// This requires PDF version 1.1 or higher, and
// use of CFF-based OpenType fonts requires PDF version 1.3 or higher.
func Embed(w *pdf.Writer, info *sfnt.Info, instName pdf.Name, loc language.Tag) (*font.Font, error) {
	if info.IsGlyf() {
		err := w.CheckVersion("use of TrueType glyph outlines", pdf.V1_1)
		if err != nil {
			return nil, err
		}
	} else if info.IsCFF() {
		err := w.CheckVersion("use of CFF glyph outlines", pdf.V1_3)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("unsupported glyph format")
	}

	widths := info.Widths()
	if widths == nil {
		return nil, errors.New("no glyph widths found")
	}

	s := &fontHandler{
		FontRef: w.Alloc(),

		info:        info,
		widths:      widths,
		GsubLookups: info.Gsub.FindLookups(loc, gtab.GsubDefaultFeatures),
		GposLookups: info.Gpos.FindLookups(loc, gtab.GposDefaultFeatures),

		text: map[glyph.ID][]rune{},
		used: map[uint16]bool{},
	}

	w.OnClose(s.WriteFont)

	res := &font.Font{
		InstName: instName,
		Ref:      s.FontRef,
		Layout:   s.Layout,
		Enc:      s.Enc,
		Geometry: font.Geometry{
			UnitsPerEm:         info.UnitsPerEm,
			Ascent:             info.Ascent,
			Descent:            info.Descent,
			BaseLineSkip:       info.Ascent - info.Descent + info.LineGap,
			UnderlinePosition:  info.UnderlinePosition,
			UnderlineThickness: info.UnderlineThickness,
			GlyphExtents:       info.Extents(),
			Widths:             widths,
		},
	}
	return res, nil
}

type fontHandler struct {
	FontRef *pdf.Reference

	info        *sfnt.Info
	widths      []funit.Int16
	GsubLookups []gtab.LookupIndex
	GposLookups []gtab.LookupIndex

	text map[glyph.ID][]rune
	used map[uint16]bool
}

func (s *fontHandler) Layout(rr []rune) glyph.Seq {
	seq := s.info.Layout(rr, s.GsubLookups, s.GposLookups)

	for _, g := range seq {
		if _, seen := s.text[g.Gid]; !seen && len(g.Text) > 0 {
			// copy the slice, in case the caller modifies it later
			s.text[g.Gid] = append([]rune{}, g.Text...)
		}
	}

	return seq
}

func (s *fontHandler) Enc(enc pdf.String, gid glyph.ID) pdf.String {
	var c uint16
	if gid <= 0xFFFF {
		c = uint16(gid)
	}
	s.used[c] = true
	return append(enc, byte(c>>8), byte(c))
}

func (s *fontHandler) WriteFont(w *pdf.Writer) error {
	// Determine the subset of glyphs to include.
	s.used[0] = true // always include .notdef
	subsetGlyphs := make([]glyph.ID, 0, len(s.used))
	for c := range s.used {
		subsetGlyphs = append(subsetGlyphs, glyph.ID(c))
	}

	if len(subsetGlyphs) == 1 {
		// only the .notdef glyph is used, so we don't need to write the font
		return nil
	}

	sort.Slice(subsetGlyphs, func(i, j int) bool { return subsetGlyphs[i] < subsetGlyphs[j] })
	subsetTag := font.GetSubsetTag(subsetGlyphs, s.info.NumGlyphs())

	// TODO(voss): make sure there is only one copy of this per PDF file.
	CIDSystemInfo := &type1.CIDSystemInfo{
		Registry:   "Adobe",
		Ordering:   "Identity",
		Supplement: 0,
	}
	ROS := pdf.Dict{
		"Registry":   pdf.String(CIDSystemInfo.Registry),
		"Ordering":   pdf.String(CIDSystemInfo.Ordering),
		"Supplement": pdf.Integer(CIDSystemInfo.Supplement),
	}

	// subset the font
	subsetInfo := &sfnt.Info{}
	*subsetInfo = *s.info
	switch outlines := s.info.Outlines.(type) {
	case *cff.Outlines:
		o2 := &cff.Outlines{}
		pIdxMap := make(map[int]int)
		for _, gid := range subsetGlyphs {
			o2.Glyphs = append(o2.Glyphs, outlines.Glyphs[gid])
			oldPIdx := outlines.FdSelect(gid)
			_, ok := pIdxMap[oldPIdx]
			if !ok {
				newPIdx := len(o2.Private)
				pIdxMap[oldPIdx] = newPIdx
				o2.Private = append(o2.Private, outlines.Private[oldPIdx])
			}
		}
		o2.FdSelect = func(gid glyph.ID) int {
			return pIdxMap[outlines.FdSelect(gid)]
		}
		o2.ROS = CIDSystemInfo
		o2.Gid2cid = make([]int32, len(subsetGlyphs))
		for i, gid := range subsetGlyphs {
			o2.Gid2cid[i] = int32(gid)
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
			// o2.Names = append(o2.Names, outlines.Names[gid])
		}
		subsetInfo.Outlines = o2
		subsetInfo.CMap = nil
	}

	fontName := pdf.Name(subsetTag + "+" + subsetInfo.PostscriptName())

	q := 1000 / float64(subsetInfo.UnitsPerEm)

	DW, W := encodeWidths(s.info.Widths(), q)

	CIDFontRef := w.Alloc()
	CIDSystemInfoRef := w.Alloc()
	FontDescriptorRef := w.Alloc()
	var WidthsRef *pdf.Reference
	if W != nil {
		WidthsRef = w.Alloc()
	}
	FontFileRef := w.Alloc()
	ToUnicodeRef := w.Alloc()

	Font := pdf.Dict{ // See section 9.7.6.1 of PDF 32000-1:2008.
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"Encoding":        pdf.Name("Identity-H"),
		"DescendantFonts": pdf.Array{CIDFontRef},
		"ToUnicode":       ToUnicodeRef,
	}

	CIDFont := pdf.Dict{ // See section 9.7.4.1 of PDF 32000-1:2008.
		"Type":           pdf.Name("Font"),
		"BaseFont":       fontName,
		"CIDSystemInfo":  CIDSystemInfoRef,
		"FontDescriptor": FontDescriptorRef,
	}
	if W != nil {
		CIDFont["W"] = WidthsRef
	}
	if DW != 1000 {
		CIDFont["DW"] = DW
	}

	rect := subsetInfo.BBox()
	fontBBox := &pdf.Rectangle{
		LLx: rect.LLx.AsFloat(q),
		LLy: rect.LLy.AsFloat(q),
		URx: rect.URx.AsFloat(q),
		URy: rect.URy.AsFloat(q),
	}

	FontDescriptor := pdf.Dict{ // See section 9.8.1 of PDF 32000-1:2008.
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    fontName,
		"Flags":       pdf.Integer(font.MakeFlags(subsetInfo, true)), // TODO(voss)
		"FontBBox":    fontBBox,
		"ItalicAngle": pdf.Number(subsetInfo.ItalicAngle),
		"Ascent":      pdf.Integer(math.Round(subsetInfo.Ascent.AsFloat(q))),
		"Descent":     pdf.Integer(math.Round(subsetInfo.Descent.AsFloat(q))),
		"CapHeight":   pdf.Integer(math.Round(subsetInfo.CapHeight.AsFloat(q))),
		"StemV":       pdf.Integer(70), // information not available in sfnt files
	}

	compressedRefs := []*pdf.Reference{s.FontRef, CIDFontRef, CIDSystemInfoRef, FontDescriptorRef}
	compressedObjects := []pdf.Object{Font, CIDFont, ROS, FontDescriptor}
	if W != nil {
		compressedRefs = append(compressedRefs, WidthsRef)
		compressedObjects = append(compressedObjects, W)
	}

	switch outlines := subsetInfo.Outlines.(type) {
	case *cff.Outlines:
		Font["BaseFont"] = fontName + "-" + "Identity-H"
		CIDFont["Subtype"] = pdf.Name("CIDFontType0")
		FontDescriptor["FontFile3"] = FontFileRef
		_, err := w.WriteCompressed(compressedRefs, compressedObjects...)
		if err != nil {
			return err
		}

		// Write the font file itself.
		// See section 9.9 of PDF 32000-1:2008 for details.
		fontFileDict := pdf.Dict{
			"Subtype": pdf.Name("CIDFontType0C"),
		}
		fontFileStream, _, err := w.OpenStream(fontFileDict, FontFileRef,
			&pdf.FilterInfo{Name: "FlateDecode"})
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

		Font["BaseFont"] = fontName
		CIDFont["Subtype"] = pdf.Name("CIDFontType2")
		CIDFont["CIDToGIDMap"] = CID2GIDMapRef
		FontDescriptor["FontFile2"] = FontFileRef

		_, err := w.WriteCompressed(compressedRefs, compressedObjects...)
		if err != nil {
			return err
		}

		cid2gidStream, _, err := w.OpenStream(nil, CID2GIDMapRef,
			&pdf.FilterInfo{
				Name: "FlateDecode",
				Parms: pdf.Dict{
					"Predictor": pdf.Integer(12),
					"Columns":   pdf.Integer(2),
				},
			})
		if err != nil {
			return err
		}
		cid2gid := make([]byte, 2*s.info.NumGlyphs())
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

		// Write the font file itself.
		// See section 9.9 of PDF 32000-1:2008 for details.
		size := w.NewPlaceholder(10)
		fontFileDict := pdf.Dict{
			"Length1": size,
		}
		compress := &pdf.FilterInfo{Name: pdf.Name("LZWDecode")}
		if w.Version >= pdf.V1_2 {
			compress = &pdf.FilterInfo{Name: pdf.Name("FlateDecode")}
		}
		fontFileStream, _, err := w.OpenStream(fontFileDict, FontFileRef, compress)
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
	for gid, text := range s.text {
		cc2text = append(cc2text, font.CIDMapping{
			CharCode: uint16(gid),
			Text:     text,
		})
	}
	_, err := font.WriteToUnicodeCID(w, cc2text, ToUnicodeRef)
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
