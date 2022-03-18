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

package simple

import (
	"errors"
	"fmt"
	"sort"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/sfnt/cmap"
	"seehuhn.de/go/pdf/font/sfntcff"
	"seehuhn.de/go/pdf/font/type1"
)

// Embed embeds a TrueType or OpenType font into a PDF document.
//
// This requires PDF version 1.1 or higher, and
// use of CFF-based OpenType fonts requires PDF version 1.2 or higher.
func Embed(w *pdf.Writer, info *sfntcff.Info, instName pdf.Name) (*font.Font, error) {
	isTrueType := info.IsTrueType()
	isOpenType := info.IsOpenType()
	if !(isTrueType || isOpenType) {
		return nil, errors.New("no glyph outlines found")
	}
	if info.CMap == nil {
		return nil, errors.New("no useable CMap found")
	}

	widths := info.Widths()
	if widths == nil {
		return nil, errors.New("no glyph widths found")
	}

	s := &simple{
		FontRef: w.Alloc(),
		info:    info,
		widths:  widths,
		text:    map[font.GlyphID][]rune{},
		enc:     map[font.GlyphID]byte{},
	}

	w.OnClose(s.WriteFont)

	res := &font.Font{
		InstName:    instName,
		Ref:         s.FontRef,
		Layout:      s.Layout,
		Enc:         s.Enc,
		GlyphUnits:  int(info.UnitsPerEm),
		Ascent:      int(info.Ascent),
		Descent:     int(info.Descent),
		GlyphExtent: info.Extents(),
		Widths:      widths,
	}
	return res, nil
}

type simple struct {
	FontRef      *pdf.Reference
	info         *sfntcff.Info
	widths       []uint16
	text         map[font.GlyphID][]rune
	enc          map[font.GlyphID]byte
	nextCharCode int
}

func (s *simple) Layout(rr []rune) []font.Glyph {
	gg := make([]font.Glyph, len(rr))
	for i, r := range rr {
		gid := s.info.CMap.Lookup(r)
		gg[i].Gid = gid
		gg[i].Chars = []rune{r}
		gg[i].Advance = int32(s.widths[gid])

		if _, seen := s.text[gid]; !seen {
			s.text[gid] = []rune{r}
		}
	}
	return gg
}

func (s *simple) Enc(gid font.GlyphID) pdf.String {
	c, ok := s.enc[gid]
	if ok {
		return pdf.String{c}
	}

	c = byte(s.nextCharCode)
	s.nextCharCode++
	s.enc[gid] = c
	return pdf.String{c}
}

func (s *simple) WriteFont(w *pdf.Writer) error {
	if s.nextCharCode > 256 {
		return fmt.Errorf("too many different glyphs for simple font %q",
			s.info.FullName())
	}

	// Determine the subset of glyphs to include.
	mapping := make([]font.CMapEntry, 0, len(s.enc))
	for gid, c := range s.enc {
		if gid == 0 {
			continue
		}
		mapping = append(mapping, font.CMapEntry{
			CharCode: uint16(c),
			GID:      gid,
		})
	}
	sort.Slice(mapping, func(i, j int) bool { return mapping[i].CharCode < mapping[j].CharCode })

	firstCharCode := mapping[0].CharCode
	lastCharCode := mapping[len(mapping)-1].CharCode

	includeGlyphs := make([]font.GlyphID, 0, len(mapping)+1)
	includeGlyphs = append(includeGlyphs, 0) // always include the .notdef glyph
	for _, m := range mapping {
		if m.GID == 0 {
			continue
		}
		includeGlyphs = append(includeGlyphs, m.GID)
	}
	subsetTag := font.GetSubsetTag(includeGlyphs, s.info.NumGlyphs())

	fontName := pdf.Name(subsetTag) + "+" + s.info.PostscriptName()

	FontDescriptorRef := w.Alloc()
	WidthsRef := w.Alloc()
	FontFileRef := w.Alloc()
	ToUnicodeRef := w.Alloc()

	switch outlines := s.info.Outlines.(type) {
	case *cff.Outlines:
		err := w.CheckVersion("use of CFF glyph outlines", pdf.V1_2)
		if err != nil {
			return err
		}

		o2 := &cff.Outlines{}
		pIdxMap := make(map[int]int)
		for _, gid := range includeGlyphs {
			o2.Glyphs = append(o2.Glyphs, outlines.Glyphs[gid])
			oldPIdx := outlines.FdSelect(gid)
			newPIdx, ok := pIdxMap[oldPIdx]
			if !ok {
				newPIdx = len(o2.Private)
				pIdxMap[oldPIdx] = newPIdx
				o2.Private = append(o2.Private, outlines.Private[oldPIdx])
			}
		}
		o2.FdSelect = func(gid font.GlyphID) int {
			return pIdxMap[outlines.FdSelect(gid)]
		}
		if len(o2.Private) > 1 {
			// Embed as a CID-keyed CFF font.
			o2.ROS = &type1.ROS{
				Registry:   "Adobe",
				Ordering:   "Identity",
				Supplement: 0,
			}
			o2.Gid2cid = make([]int32, len(includeGlyphs))
			for i, gid := range includeGlyphs {
				o2.Gid2cid[i] = int32(s.enc[gid])
			}
		} else {
			// Embed as a simple CFF font.
			o2.Encoding = make([]font.GlyphID, 256)
			for i, gid := range includeGlyphs {
				o2.Encoding[s.enc[gid]] = font.GlyphID(i)
			}
		}

		fontInfo := s.info.GetFontInfo()
		fontFile := cff.Font{
			FontInfo: fontInfo,
			Outlines: o2,
		}

		Font := pdf.Dict{ // See section 9.6.2.1 of PDF 32000-1:2008.
			"Type":           pdf.Name("Font"),
			"Subtype":        pdf.Name("Type1"),
			"BaseFont":       fontName,
			"FirstChar":      pdf.Integer(firstCharCode),
			"LastChar":       pdf.Integer(lastCharCode),
			"FontDescriptor": FontDescriptorRef,
			"Widths":         WidthsRef,
			"ToUnicode":      ToUnicodeRef,
		}

		FontDescriptor := pdf.Dict{ // See section 9.8.1 of PDF 32000-1:2008.
			"Type":        pdf.Name("FontDescriptor"),
			"FontName":    fontName,
			"Flags":       pdf.Integer(flags(s.info, true)), // TODO(voss)
			"FontBBox":    s.info.BBox(),
			"ItalicAngle": pdf.Number(fontInfo.ItalicAngle),
			"Ascent":      pdf.Integer(s.info.Ascent),
			"Descent":     pdf.Integer(s.info.Descent),
			"CapHeight":   pdf.Integer(s.info.CapHeight),
			"StemV":       pdf.Integer(70), // information not available in sfnt files
			"FontFile3":   FontFileRef,
		}

		var Widths pdf.Array
		ww := s.widths
		for _, m := range mapping {
			Widths = append(Widths, pdf.Integer(ww[m.GID]))
		}

		_, err = w.WriteCompressed(
			[]*pdf.Reference{s.FontRef, FontDescriptorRef, WidthsRef},
			Font, FontDescriptor, Widths)
		if err != nil {
			return err
		}

		// Write the font file itself.
		// See section 9.9 of PDF 32000-1:2008 for details.
		fontFileDict := pdf.Dict{
			"Subtype": pdf.Name("Type1C"),
		}
		fontFileStream, _, err := w.OpenStream(fontFileDict, FontFileRef,
			&pdf.FilterInfo{Name: "FlateDecode"})
		if err != nil {
			return err
		}
		err = fontFile.Encode(fontFileStream)
		if err != nil {
			return err
		}
		err = fontFileStream.Close()
		if err != nil {
			return err
		}

	case *sfntcff.TTFOutlines:
		err := w.CheckVersion("use of TrueType glyph outlines", pdf.V1_1)
		if err != nil {
			return err
		}

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

		encoding := cmap.Format4{}
		for gid, c := range s.enc {
			gid = newGid[gid]
			if gid != 0 {
				encoding[uint16(c)] = gid
			}
		}

		o2 := &sfntcff.TTFOutlines{
			Tables: outlines.Tables,
			Maxp:   outlines.Maxp,
		}
		for _, gid := range includeGlyphs {
			g := outlines.Glyphs[gid]
			o2.Glyphs = append(o2.Glyphs, g.FixComponents(newGid))
			o2.Widths = append(o2.Widths, outlines.Widths[gid])
			// o2.Names = append(o2.Names, outlines.Names[gid])
		}
		i2 := &sfntcff.Info{}
		*i2 = *s.info
		i2.Outlines = o2

		// ---

		Font := pdf.Dict{ // See section 9.6.2.1 of PDF 32000-1:2008.
			"Type":           pdf.Name("Font"),
			"Subtype":        pdf.Name("TrueType"),
			"BaseFont":       fontName,
			"FirstChar":      pdf.Integer(firstCharCode),
			"LastChar":       pdf.Integer(lastCharCode),
			"FontDescriptor": FontDescriptorRef,
			"Widths":         WidthsRef,
			"ToUnicode":      ToUnicodeRef,
		}

		FontDescriptor := pdf.Dict{ // See section 9.8.1 of PDF 32000-1:2008.
			"Type":        pdf.Name("FontDescriptor"),
			"FontName":    fontName,
			"Flags":       pdf.Integer(flags(s.info, true)), // TODO(voss)
			"FontBBox":    i2.BBox(),
			"ItalicAngle": pdf.Number(i2.ItalicAngle),
			"Ascent":      pdf.Integer(i2.Ascent),
			"Descent":     pdf.Integer(i2.Descent),
			"CapHeight":   pdf.Integer(i2.CapHeight),
			"StemV":       pdf.Integer(70), // information not available in sfnt files
			"FontFile2":   FontFileRef,
		}

		var Widths pdf.Array
		for _, w := range o2.Widths {
			Widths = append(Widths, pdf.Integer(w))
		}

		_, err = w.WriteCompressed(
			[]*pdf.Reference{s.FontRef, FontDescriptorRef, WidthsRef},
			Font, FontDescriptor, Widths)
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
		n, err := i2.EmbedSimple(fontFileStream, encoding)
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

	default:
		panic("unsupported outlines type")
	}

	var cc2text []font.SimpleMapping
	for gid, text := range s.text {
		charCode := s.enc[gid]
		cc2text = append(cc2text, font.SimpleMapping{CharCode: charCode, Text: text})
	}
	err := font.WriteToUnicodeSimple(w, subsetTag, cc2text, ToUnicodeRef)
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
