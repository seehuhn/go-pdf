package simple

import (
	"errors"
	"fmt"
	"sort"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/sfntcff"
	"seehuhn.de/go/pdf/font/type1"
)

// Embed embeds a TrueType or OpenType font into a PDF document.
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

	switch outlines := s.info.Outlines.(type) {
	case *cff.Outlines:
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

		FontDescriptorRef := w.Alloc()
		WidthsRef := w.Alloc()
		FontFileRef := w.Alloc()
		ToUnicodeRef := w.Alloc()

		fontName := pdf.Name(subsetTag) + "+" + fontInfo.FontName

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

		_, err := w.WriteCompressed(
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

		var cc2text []font.SimpleMapping
		for gid, text := range s.text {
			charCode := s.enc[gid]
			cc2text = append(cc2text, font.SimpleMapping{CharCode: charCode, Text: text})
		}
		err = font.WriteToUnicodeSimple(w, subsetTag, cc2text, ToUnicodeRef)
		if err != nil {
			return err
		}
	}
	return nil
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
