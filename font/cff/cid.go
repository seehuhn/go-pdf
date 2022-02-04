package cff

import (
	"sort"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/names"
)

func EmbedFontCID(w *pdf.Writer, cff *Font, instName pdf.Name) (*font.Font, error) {
	cmap := make(map[rune]font.GlyphID)
	for gid, glyph := range cff.Glyphs {
		rr := names.ToUnicode(string(glyph.Name), false)
		if len(rr) == 1 {
			cmap[rr[0]] = font.GlyphID(gid)
		}
	}

	glyphExtents := cff.GlyphExtents()
	bbox := &pdf.Rectangle{}
	for i, ext := range glyphExtents {
		if x := float64(ext.LLx); i == 0 || x < bbox.LLx {
			bbox.LLx = x
		}
		if x := float64(ext.URx); i == 0 || x > bbox.URx {
			bbox.URx = x
		}
		if y := float64(ext.LLy); i == 0 || y < bbox.LLy {
			bbox.LLy = y
		}
		if y := float64(ext.URy); i == 0 || y > bbox.URy {
			bbox.URy = y
		}
	}
	// TODO(voss): adjust, if glyphunits != 1000

	t := &cidFont{
		Cff:      cff,
		FontRef:  w.Alloc(),
		FontBBox: bbox,

		used: map[font.GlyphID]bool{},
		text: map[font.GlyphID][]rune{},
	}

	w.OnClose(t.WriteFont)

	font := &font.Font{
		InstName: instName,
		Ref:      t.FontRef,
		Layout: func(rr []rune) []font.Glyph {
			gg := make([]font.Glyph, len(rr))
			for i, r := range rr {
				gid := cmap[r]
				gg[i].Gid = gid
				gg[i].Chars = []rune{r}
				gg[i].Advance = cff.Glyphs[gid].Width

				if _, seen := t.text[gid]; !seen {
					t.text[gid] = []rune{r}
				}
			}
			return gg
		},
		Enc: func(gid font.GlyphID) pdf.String {
			t.used[gid] = true
			return pdf.String{byte(gid >> 8), byte(gid)}
		},
		GlyphUnits:  1000, // TODO(voss): get from CFF
		Ascent:      0,    // TODO(voss): ???
		Descent:     0,    // TODO(voss): ???
		GlyphExtent: glyphExtents,
		Width:       cff.Widths(),
	}
	return font, nil
}

type cidFont struct {
	Cff      *Font
	FontRef  *pdf.Reference
	FontBBox *pdf.Rectangle

	used map[font.GlyphID]bool
	text map[font.GlyphID][]rune // GID -> text
}

func (t *cidFont) WriteFont(w *pdf.Writer) error {
	DW, W := font.EncodeCIDWidths(t.Cff.Widths())

	t.used[0] = true // always include .notdef
	includeGlyphs := make([]font.GlyphID, 0, len(t.used))
	for gid, ok := range t.used {
		if ok {
			includeGlyphs = append(includeGlyphs, gid)
		}
	}
	sort.Slice(includeGlyphs, func(i, j int) bool {
		return includeGlyphs[i] < includeGlyphs[j]
	})

	cff, err := t.Cff.Subset(includeGlyphs)
	if err != nil {
		return err
	}

	flags := font.FlagSymbolic
	if cff.Info.IsFixedPitch {
		flags |= font.FlagFixedPitch
	}
	if cff.Info.ItalicAngle != 0 {
		flags |= font.FlagItalic
	}
	if cff.Info.Private[0].ForceBold {
		flags |= font.FlagForceBold
	}
	// TODO(voss): add more flags

	CIDFontRef := w.Alloc()
	CIDSystemInfoRef := w.Alloc()
	FontDescriptorRef := w.Alloc()
	WRef := w.Alloc()
	FontFileRef := w.Alloc()
	ToUnicodeRef := w.Alloc()

	Font := pdf.Dict{ // See section 9.7.6.1 of PDF 32000-1:2008.
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        cff.Info.FontName + "-" + "Identity-H",
		"Encoding":        pdf.Name("Identity-H"),
		"DescendantFonts": pdf.Array{CIDFontRef},
		"ToUnicode":       ToUnicodeRef,
	}

	CIDFont := pdf.Dict{ // See section 9.7.4.1 of PDF 32000-1:2008.
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("CIDFontType0"),
		"BaseFont":       cff.Info.FontName,
		"CIDSystemInfo":  CIDSystemInfoRef,
		"FontDescriptor": FontDescriptorRef,
	}
	if W != nil {
		CIDFont["W"] = WRef
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

	FontDescriptor := pdf.Dict{ // See sections 9.8.1 of PDF 32000-1:2008.
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    cff.Info.FontName,
		"Flags":       pdf.Integer(flags),
		"FontBBox":    t.FontBBox,
		"ItalicAngle": pdf.Number(cff.Info.ItalicAngle),
		"Ascent":      pdf.Integer(0),  // TODO(voss): ???
		"Descent":     pdf.Integer(0),  // TODO(voss): ???
		"CapHeight":   pdf.Integer(0),  // TODO(voss): ???
		"StemV":       pdf.Integer(70), // TODO(voss): ???
		"FontFile3":   FontFileRef,
	}

	_, err = w.WriteCompressed(
		[]*pdf.Reference{
			t.FontRef, CIDFontRef, CIDSystemInfoRef, FontDescriptorRef, WRef,
		},
		Font, CIDFont, CIDSystemInfo, FontDescriptor, W)
	if err != nil {
		return err
	}

	// write all the streams

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
	err = cff.EncodeCID(fontFileStream, "Adobe", "Identity", 0)
	if err != nil {
		return err
	}
	err = fontFileStream.Close()
	if err != nil {
		return err
	}

	// Write the ToUnicode CMap.
	var cid2text []font.CIDMapping
	for cid, text := range t.text {
		cid2text = append(cid2text, font.CIDMapping{CharCode: uint16(cid), Text: text})
	}
	err = font.WriteToUnicodeCID(w, cid2text, ToUnicodeRef)
	if err != nil {
		return err
	}

	return nil
}
