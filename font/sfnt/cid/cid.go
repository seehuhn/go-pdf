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

package cid

import (
	"errors"
	"os"
	"sort"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/funit"
	"seehuhn.de/go/pdf/font/glyph"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/glyf"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
	"seehuhn.de/go/pdf/font/type1"
)

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
		FontRef:  w.Alloc(),
		instName: instName,

		info:        info,
		widths:      widths,
		GsubLookups: info.Gsub.FindLookups(loc, gtab.GsubDefaultFeatures),
		GposLookups: info.Gpos.FindLookups(loc, gtab.GposDefaultFeatures),

		text: map[glyph.ID][]rune{},
		used: map[uint16]bool{},
	}

	w.OnClose(s.WriteFont)

	res := &font.Font{
		InstName:           instName,
		Ref:                s.FontRef,
		Layout:             s.Layout,
		Enc:                s.Enc,
		UnitsPerEm:         info.UnitsPerEm,
		Ascent:             info.Ascent,
		Descent:            info.Descent,
		BaseLineSkip:       info.Ascent - info.Descent + info.LineGap,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
		GlyphExtents:       info.Extents(),
		Widths:             widths,
	}
	return res, nil
}

type fontHandler struct {
	FontRef  *pdf.Reference
	instName pdf.Name

	info        *sfnt.Info
	widths      []funit.Int16
	GsubLookups []gtab.LookupIndex
	GposLookups []gtab.LookupIndex

	text map[glyph.ID][]rune
	used map[uint16]bool
}

func (s *fontHandler) Layout(rr []rune) []glyph.Info {
	info := s.info

	seq := make([]glyph.Info, len(rr))
	for i, r := range rr {
		gid := info.CMap.Lookup(r)
		seq[i].Gid = gid
		seq[i].Text = []rune{r}
	}

	for _, lookupIndex := range s.GsubLookups {
		seq = info.Gsub.LookupList.ApplyLookup(seq, lookupIndex, info.Gdef)
	}

	for i := range seq {
		gid := seq[i].Gid
		if info.Gdef.GlyphClass[gid] != gdef.GlyphClassMark {
			seq[i].Advance = s.widths[gid]
		}
	}
	for _, lookupIndex := range s.GposLookups {
		seq = info.Gpos.LookupList.ApplyLookup(seq, lookupIndex, info.Gdef)
	}

	for _, g := range seq {
		if _, seen := s.text[g.Gid]; !seen && len(g.Text) > 0 {
			// copy the slice, in case the caller modifies it later
			s.text[g.Gid] = append([]rune{}, g.Text...)
		}
	}

	return seq
}

func (s *fontHandler) Enc(gid glyph.ID) pdf.String {
	var c uint16
	if gid <= 0xFFFF {
		c = uint16(gid)
	}
	s.used[c] = true
	return pdf.String{byte(c >> 8), byte(c)}
}

func (s *fontHandler) WriteFont(w *pdf.Writer) error {
	// Determine the subset of glyphs to include.
	s.used[0] = true // always include .notdef
	includeGlyphs := make([]glyph.ID, 0, len(s.used))
	for c := range s.used {
		includeGlyphs = append(includeGlyphs, glyph.ID(c))
	}
	sort.Slice(includeGlyphs, func(i, j int) bool { return includeGlyphs[i] < includeGlyphs[j] })
	subsetTag := font.GetSubsetTag(includeGlyphs, s.info.NumGlyphs())

	// TODO(voss): make sure there is only one copy of this per PDF file.
	CIDSystemInfo := &type1.ROS{
		Registry:   "Adobe",
		Ordering:   "Identity",
		Supplement: 0,
	}

	// subset the font
	subsetInfo := &sfnt.Info{}
	*subsetInfo = *s.info
	switch outlines := s.info.Outlines.(type) {
	case *cff.Outlines:
		o2 := &cff.Outlines{}
		pIdxMap := make(map[int]int)
		for _, gid := range includeGlyphs {
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
		o2.Gid2cid = make([]int32, len(includeGlyphs))
		for i, gid := range includeGlyphs {
			o2.Gid2cid[i] = int32(gid)
		}
		subsetInfo.Outlines = o2

	case *glyf.Outlines:
		newGid := make(map[glyph.ID]glyph.ID)
		todo := make(map[glyph.ID]bool)
		nextGid := glyph.ID(0)
		for _, gid := range includeGlyphs {
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
			includeGlyphs = append(includeGlyphs, gid)
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
		for _, gid := range includeGlyphs {
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

	FontDescriptor := pdf.Dict{ // See section 9.8.1 of PDF 32000-1:2008.
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    fontName,
		"Flags":       pdf.Integer(subsetInfo.Flags(true)), // TODO(voss)
		"FontBBox":    subsetInfo.BBox().AsPDF(q),
		"ItalicAngle": pdf.Number(subsetInfo.ItalicAngle),
		"Ascent":      subsetInfo.Ascent.AsInteger(q),
		"Descent":     subsetInfo.Descent.AsInteger(q),
		"CapHeight":   subsetInfo.CapHeight.AsInteger(q),
		"StemV":       pdf.Integer(70), // information not available in sfnt files
	}

	compressedRefs := []*pdf.Reference{s.FontRef, CIDFontRef, CIDSystemInfoRef, FontDescriptorRef}
	compressedObjects := []pdf.Object{Font, CIDFont, CIDSystemInfo, FontDescriptor}
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
		for gid, cid := range includeGlyphs {
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
		n, err := subsetInfo.Embed(fontFileStream)
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
	err := font.WriteToUnicodeCID(w, cc2text, ToUnicodeRef)
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
