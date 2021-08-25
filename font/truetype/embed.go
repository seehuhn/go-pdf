package truetype

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/table"
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

// Embed embeds a TrueType font into a pdf file.
func Embed(w *pdf.Writer, refName string, fname string) (*font.Font, error) {
	tt, err := sfnt.Open(fname)
	if err != nil {
		return nil, err
	}
	defer tt.Close() // TODO(voss): is this a good idea?

	return EmbedFont(w, refName, tt)
}

// EmbedFont embeds a TrueType font into a pdf file.
func EmbedFont(w *pdf.Writer, refName string, tt *sfnt.Font) (*font.Font, error) {
	err := w.CheckVersion("use of TrueType-based CIDfonts", pdf.V1_3)
	if err != nil {
		return nil, err
	}

	t, err := newTruetype(w, tt)
	if err != nil {
		return nil, err
	}
	w.OnClose(t.WriteFontDict)

	res := &font.Font{
		Name:        pdf.Name(refName),
		Ref:         t.fontRef,
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
	fontRef     *pdf.Reference
	GlyphUnits  int
	Ascent      float64     // Ascent in glyph coordinate units
	Descent     float64     // Descent in glyph coordinate units, as a negative number
	GlyphExtent []font.Rect // TODO(voss): needed?
	Width       []int       // TODO(voss): needed?
}

func newTruetype(w *pdf.Writer, tt *sfnt.Font) (*truetype, error) {
	if !tt.IsTrueType() {
		return nil, errors.New("not a TrueType font")
	}

	hheaInfo, err := tt.GetHHeaInfo()
	if err != nil {
		return nil, err
	}

	// The "OS/2" table is optional for TrueType fonts, but required for
	// OpenType fonts.
	os2Info, err := tt.GetOS2Info()
	if err != nil && !table.IsMissing(err) {
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

	res := &truetype{
		fontRef:     w.Alloc(),
		GlyphUnits:  int(tt.Head.UnitsPerEm),
		Ascent:      Ascent,
		Descent:     Descent,
		GlyphExtent: []font.Rect{}, // TODO(voss)
		Width:       []int{},       // TODO(voss)
	}
	return res, nil
}

func (t *truetype) Layout([]font.Glyph) []font.Glyph {
	panic("not implemented")
}

func (t *truetype) Enc(font.GlyphID) pdf.String {
	panic("not implemented")
}

func (t *truetype) WriteFontDict(w *pdf.Writer) error {
	panic("not implemented")
}
