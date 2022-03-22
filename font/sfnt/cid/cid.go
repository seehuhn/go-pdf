package cid

import (
	"errors"
	"math"
	"sort"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/sfnt/cmap"
	"seehuhn.de/go/pdf/font/sfntcff"
	"seehuhn.de/go/pdf/font/type1"
)

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
		enc:     map[font.GlyphID]byte{},
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
		gg[i].Chars = []rune{r}
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
	includeGlyphs := make([]font.GlyphID, 0, len(s.used)+1)
	includeGlyphs = append(includeGlyphs, 0) // always include .notdef
	for c := range s.used {
		includeGlyphs = append(includeGlyphs, font.GlyphID(c))
	}
	sort.Slice(includeGlyphs, func(i, j int) bool { return includeGlyphs[i] < includeGlyphs[j] })
	subsetTag := font.GetSubsetTag(includeGlyphs, s.info.NumGlyphs())

	// subset the font
	subsetInfo := &sfntcff.Info{}
	*subsetInfo = *s.info
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
		o2.ROS = &type1.ROS{
			Registry:   "Adobe",
			Ordering:   "Identity",
			Supplement: 0,
		}
		o2.Gid2cid = make([]int32, len(includeGlyphs))
		for i, gid := range includeGlyphs {
			o2.Gid2cid[i] = int32(gid)
		}
		subsetInfo.Outlines = o2

	case *sfntcff.GlyfOutlines:
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

		encoding := cmap.Format4{}
		for gid, c := range s.enc {
			encoding[uint16(c)] = newGid[gid]
		}
		subsetInfo.CMap = encoding

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

	// TODO(voss): make sure there is only one copy of this per PDF file.
	CIDSystemInfo := pdf.Dict{ // See sections 9.7.3 of PDF 32000-1:2008.
		"Registry":   pdf.String("Adobe"),
		"Ordering":   pdf.String("Identity"),
		"Supplement": pdf.Integer(0),
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
		n, err := subsetInfo.EmbedSimple(fontFileStream)
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
