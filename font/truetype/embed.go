package truetype

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/parser"
	"seehuhn.de/go/pdf/font/sfnt/table"
	"seehuhn.de/go/pdf/locale"
)

// TrueType fonts with <=255 glyphs (PDF 1.1)
//   Type=Font, Subtype=TrueType
//   --FontDescriptor-> Type=FontDescriptor
//   --FontFile2-> Length1=...

// TrueType fonts with >255 glyphs (PDF 1.3)
//   Type=Font, Subtype=Type0
//   --DescendantFonts-> Type=Font, Subtype=CIDFontType2
//   --FontDescriptor-> Type=FontDescriptor
//   --FontFile2-> Length1=...

// Embed embeds a TrueType font into a pdf file as a CIDFont.
// This requires PDF version 1.3 or higher.
func Embed(w *pdf.Writer, refName string, fname string, loc *locale.Locale) (*font.Font, error) {
	tt, err := sfnt.Open(fname)
	if err != nil {
		return nil, err
	}
	defer tt.Close() // TODO(voss): is this a good idea?

	return EmbedFont(w, refName, tt, loc)
}

// EmbedFont embeds a TrueType font into a pdf file as a CIDFont.
// This requires PDF version 1.3 or higher.
func EmbedFont(w *pdf.Writer, refName string, tt *sfnt.Font, loc *locale.Locale) (*font.Font, error) {
	err := w.CheckVersion("use of TrueType-based CIDfonts", pdf.V1_3)
	if err != nil {
		return nil, err
	}

	t, err := newTruetype(w, tt, loc)
	if err != nil {
		return nil, err
	}
	w.OnClose(t.WriteFontDict)

	res := &font.Font{
		Name:        pdf.Name(refName),
		Ref:         t.Ref,
		CMap:        tt.CMap,
		Layout:      t.Layout,
		Enc:         t.Enc,
		GlyphUnits:  t.GlyphUnits,
		Ascent:      t.Ascent,
		Descent:     t.Descent,
		GlyphExtent: t.GlyphExtent,
		Width:       t.Width,
	}
	return res, nil
}

type truetype struct {
	Ref         *pdf.Reference
	GlyphUnits  int
	Ascent      float64 // Ascent in glyph coordinate units
	Descent     float64 // Descent in glyph coordinate units, as a negative number
	GlyphExtent []font.Rect
	Width       []int

	Lookups  parser.Lookups
	KernInfo map[font.GlyphPair]int
}

func newTruetype(w *pdf.Writer, tt *sfnt.Font, loc *locale.Locale) (*truetype, error) {
	if !tt.IsTrueType() {
		return nil, errors.New("not a TrueType font")
	}

	hheaInfo, err := tt.GetHHeaInfo()
	if err != nil {
		return nil, err
	}

	hmtx, err := tt.GetHMtxInfo(hheaInfo.NumOfLongHorMetrics)
	if err != nil {
		return nil, err
	}

	os2Info, err := tt.GetOS2Info()
	if err != nil && !table.IsMissing(err) {
		// The "OS/2" table is optional for TrueType fonts, but required for
		// OpenType fonts.
		return nil, err
	}

	glyf, err := tt.GetGlyfInfo()
	if err != nil {
		return nil, err
	}

	Ascent := float64(hheaInfo.Ascent)
	Descent := float64(hheaInfo.Descent)
	if os2Info != nil && os2Info.V0MSValid {
		if os2Info.V0.Selection&(1<<7) != 0 {
			Ascent = float64(os2Info.V0MS.TypoAscender)
			Descent = float64(os2Info.V0MS.TypoDescender)
		} else {
			Ascent = float64(os2Info.V0MS.WinAscent)
			Descent = -float64(os2Info.V0MS.WinDescent)
		}
	}

	GlyphExtent := make([]font.Rect, tt.NumGlyphs)
	for i := 0; i < tt.NumGlyphs; i++ {
		GlyphExtent[i].LLx = int(glyf.Data[i].XMin)
		GlyphExtent[i].LLy = int(glyf.Data[i].YMin)
		GlyphExtent[i].URx = int(glyf.Data[i].XMax)
		GlyphExtent[i].URy = int(glyf.Data[i].YMax)
	}

	Width := make([]int, tt.NumGlyphs)
	for i := 0; i < tt.NumGlyphs; i++ {
		j := i % len(hmtx.HMetrics)
		Width[i] = int(hmtx.HMetrics[j].AdvanceWidth)
	}

	pars := parser.New(tt)
	gsub, err := pars.ReadGsubTable(loc)
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}
	gpos, err := pars.ReadGposTable(loc)
	var kernInfo map[font.GlyphPair]int
	if table.IsMissing(err) {
		kernInfo, err = tt.ReadKernInfo()
	}
	if err != nil {
		return nil, err
	}

	res := &truetype{
		Ref:         w.Alloc(),
		GlyphUnits:  int(tt.Head.UnitsPerEm),
		Ascent:      Ascent,
		Descent:     Descent,
		GlyphExtent: GlyphExtent,
		Width:       Width,

		Lookups:  append(gsub, gpos...),
		KernInfo: kernInfo,
	}
	return res, nil
}

func (t *truetype) Layout(gg []font.Glyph) []font.Glyph {
	gg = t.Lookups.ApplyAll(gg)

	for i, g := range gg {
		gg[i].Advance = t.Width[g.Gid]
	}

	if t.KernInfo != nil {
		for i := 0; i+1 < len(gg); i++ {
			pair := font.GlyphPair{gg[i].Gid, gg[i+1].Gid}
			if dx, ok := t.KernInfo[pair]; ok {
				gg[i].Advance += dx
			}
		}
	}

	return gg
}

func (t *truetype) Enc(gid font.GlyphID) pdf.String {
	// TODO(voss): be more clever here
	return pdf.String{byte(gid >> 8), byte(gid)}
}

func (t *truetype) WriteFontDict(w *pdf.Writer) error {
	panic("not implemented")
}
