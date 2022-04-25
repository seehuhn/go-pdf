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
	"math"
	"sort"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/sfntcff"
	"seehuhn.de/go/pdf/font/type1"
)

// TODO(voss): check PDF versions

// Embed embeds a TrueType or OpenType font into a PDF document.
func Embed(w *pdf.Writer, info *sfntcff.Info, instName pdf.Name) (*font.Font, error) {
	isTrueType := info.IsGlyf()
	isOpenType := info.IsCFF()
	if !(isTrueType || isOpenType) {
		return nil, errors.New("no glyph outlines found")
	}

	widths := info.Widths()
	if widths == nil {
		return nil, errors.New("no glyph widths found")
	}

	s := &fontHandler{
		FontRef: w.Alloc(),
		info:    info,
		widths:  widths,
		text:    map[font.GlyphID][]rune{},
		used:    map[uint16]bool{},
	}

	w.OnClose(s.WriteFont)

	res := &font.Font{
		InstName:     instName,
		Ref:          s.FontRef,
		Layout:       s.Layout,
		Enc:          s.Enc,
		Ascent:       int(info.Ascent),
		Descent:      int(info.Descent),
		GlyphExtents: info.Extents(),
		Widths:       widths,
	}
	return res, nil
}

type fontHandler struct {
	FontRef *pdf.Reference
	info    *sfntcff.Info
	widths  []uint16
	text    map[font.GlyphID][]rune
	used    map[uint16]bool
}

func (s *fontHandler) Layout(rr []rune) []font.Glyph {
	gg := make([]font.Glyph, len(rr))
	for i, r := range rr {
		gid := s.info.CMap.Lookup(r)
		gg[i].Gid = gid
		gg[i].Text = []rune{r}
		gg[i].Advance = int32(s.widths[gid])

		if _, seen := s.text[gid]; !seen {
			s.text[gid] = []rune{r}
		}
	}
	return gg
}

func (s *fontHandler) Enc(gid font.GlyphID) pdf.String {
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
	includeGlyphs := make([]font.GlyphID, 0, len(s.used))
	for c := range s.used {
		includeGlyphs = append(includeGlyphs, font.GlyphID(c))
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
	subsetInfo := &sfntcff.Info{}
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
		o2.FdSelect = func(gid font.GlyphID) int {
			return pIdxMap[outlines.FdSelect(gid)]
		}
		o2.ROS = CIDSystemInfo
		o2.Gid2cid = make([]int32, len(includeGlyphs))
		for i, gid := range includeGlyphs {
			o2.Gid2cid[i] = int32(gid)
		}
		subsetInfo.Outlines = o2

	case *sfntcff.GlyfOutlines:
		newGid := make(map[font.GlyphID]font.GlyphID)
		todo := make(map[font.GlyphID]bool)
		nextGid := font.GlyphID(0)
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

		o2 := &sfntcff.GlyfOutlines{
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

	default:
		panic("unsupported outlines type")
	}

	fontName := pdf.Name(subsetTag) + "+" + subsetInfo.PostscriptName()

	CIDFontRef := w.Alloc()
	CIDSystemInfoRef := w.Alloc()
	FontDescriptorRef := w.Alloc()
	WidthsRef := w.Alloc() // TODO(voss): don't allocte if W == nil.
	FontFileRef := w.Alloc()
	ToUnicodeRef := w.Alloc()

	q := 1000 / float64(subsetInfo.UnitsPerEm)

	Font := pdf.Dict{ // See section 9.7.6.1 of PDF 32000-1:2008.
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"Encoding":        pdf.Name("Identity-H"),
		"DescendantFonts": pdf.Array{CIDFontRef},
		"ToUnicode":       ToUnicodeRef,
	}

	DW, W := font.EncodeCIDWidths(s.widths)
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
		CIDFont["DW"] = pdf.Integer(DW)
	}

	FontDescriptor := pdf.Dict{ // See section 9.8.1 of PDF 32000-1:2008.
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    fontName,
		"Flags":       pdf.Integer(flags(subsetInfo, true)), // TODO(voss)
		"FontBBox":    subsetInfo.BBox(),
		"ItalicAngle": pdf.Number(subsetInfo.ItalicAngle),
		"Ascent":      pdf.Integer(math.Round(float64(subsetInfo.Ascent) * q)),
		"Descent":     pdf.Integer(math.Round(float64(subsetInfo.Descent) * q)),
		"CapHeight":   pdf.Integer(math.Round(float64(subsetInfo.CapHeight) * q)),
		"StemV":       pdf.Integer(70), // information not available in sfnt files
	}

	switch outlines := subsetInfo.Outlines.(type) {
	case *cff.Outlines:
		Font["BaseFont"] = fontName + "-" + "Identity-H"
		CIDFont["Subtype"] = pdf.Name("CIDFontType0")
		FontDescriptor["FontFile3"] = FontFileRef

		_, err := w.WriteCompressed(
			[]*pdf.Reference{s.FontRef, CIDFontRef, CIDSystemInfoRef, FontDescriptorRef, WidthsRef},
			Font, CIDFont, CIDSystemInfo, FontDescriptor, W)
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

	case *sfntcff.GlyfOutlines:
		CID2GIDMapRef := w.Alloc()

		Font["BaseFont"] = fontName
		CIDFont["Subtype"] = pdf.Name("CIDFontType2")
		CIDFont["CIDToGIDMap"] = CID2GIDMapRef
		FontDescriptor["FontFile2"] = FontFileRef

		_, err := w.WriteCompressed(
			[]*pdf.Reference{s.FontRef, CIDFontRef, CIDSystemInfoRef, FontDescriptorRef, WidthsRef},
			Font, CIDFont, CIDSystemInfo, FontDescriptor, W)
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
		err = fontFileStream.Close()
		if err != nil {
			return err
		}
		err = size.Set(pdf.Integer(n)) // TODO(voss): move this earlier once Placeholder is fixed
		if err != nil {
			return err
		}

	default:
		panic("unsupported outlines type")
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

func pop(todo map[font.GlyphID]bool) font.GlyphID {
	for key := range todo {
		delete(todo, key)
		return key
	}
	panic("empty map")
}

func flags(info *sfntcff.Info, symbolic bool) uint32 {
	var flags uint32
	if info.IsFixedPitch() {
		flags |= 1 << (1 - 1)
	}
	if info.IsSerif {
		flags |= 1 << (2 - 1)
	}
	if symbolic {
		flags |= 1 << (3 - 1)
	} else {
		flags |= 1 << (6 - 1)
	}
	if info.IsScript {
		flags |= 1 << (4 - 1)
	}
	if info.IsItalic {
		flags |= 1 << (7 - 1)
	}
	return flags
}
