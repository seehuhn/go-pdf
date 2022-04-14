// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package opentype

import (
	"errors"
	"math"
	"sort"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/locale"
)

// EmbedSimple embeds an OpenType font into a pdf file as a simple font.
// Up to 256 arbitrary glyphs from the font file can be accessed via the
// returned font object.
//
// In comparison, fonts embedded via EmbedCID lead to larger PDF files, but
// there is no limit on the number of glyphs which can be accessed.
//
// Use of simple OpenType fonts in PDF requires PDF version 1.2 or higher.
func EmbedSimple(w *pdf.Writer, fileName string, instName pdf.Name, loc *locale.Locale) (*font.Font, error) {
	tt, err := sfnt.Open(fileName, loc)
	if err != nil {
		return nil, err
	}

	return EmbedFontSimple(w, tt, instName)
}

// EmbedFontSimple embeds an OpenType font into a pdf file as a simple font.
// Up to 256 arbitrary glyphs from the font file can be accessed via the
// returned font object.
//
// This function takes ownership of tt and will close the font tt once it is no
// longer needed.
//
// In comparison, fonts embedded via EmbedFontCID lead to larger PDF files, but
// there is no limit on the number of glyphs which can be accessed.
//
// Use of simple OpenType fonts requires PDF version 1.2 or higher.
func EmbedFontSimple(w *pdf.Writer, tt *sfnt.Font, instName pdf.Name) (*font.Font, error) {
	if !tt.IsOpenType() {
		return nil, errors.New("not an OpenType font")
	}
	if tt.IsTrueType() {
		return truetype.EmbedFontSimple(w, tt, instName)
	}
	err := w.CheckVersion("use of simple OpenType fonts", pdf.V1_2)
	if err != nil {
		return nil, err
	}

	r, err := tt.GetTableReader("CFF ", nil)
	if err != nil {
		return nil, err
	}
	cff, err := cff.Read(r)
	if err != nil {
		return nil, err
	}

	q := 1000 / float64(tt.GlyphUnits)

	ee := cff.GlyphExtents()
	glyphExtents := make([]font.Rect, len(ee))
	for i, ext := range ee {
		glyphExtents[i] = font.Rect{
			LLx: int16(math.Round(float64(ext.LLx) * q)),
			LLy: int16(math.Round(float64(ext.LLy) * q)),
			URx: int16(math.Round(float64(ext.URx) * q)),
			URy: int16(math.Round(float64(ext.URy) * q)),
		}
	}

	widths := make([]uint16, len(tt.HmtxInfo.Widths))
	for i, w := range tt.HmtxInfo.Widths {
		widths[i] = uint16(math.Round(float64(w) * q))
	}

	s := &simple{
		Sfnt:   tt,
		Cff:    cff,
		widths: widths,

		FontRef: w.Alloc(),

		text: make(map[font.GlyphID][]rune),
		enc:  make(map[font.GlyphID]byte),
	}

	w.OnClose(s.WriteFont)

	res := &font.Font{
		InstName: instName,
		Ref:      s.FontRef,

		Ascent:       int(tt.HmtxInfo.Ascent),
		Descent:      int(tt.HmtxInfo.Descent),
		GlyphExtents: glyphExtents,
		Widths:       widths,

		Layout: s.Layout,
		Enc:    s.Enc,
	}
	return res, nil
}

type simple struct {
	Sfnt   *sfnt.Font
	Cff    *cff.Font
	widths []uint16

	FontRef *pdf.Reference

	text map[font.GlyphID][]rune // GID -> text
	enc  map[font.GlyphID]byte   // GID -> CharCode

	nextCharCode int // next available CharCode
	allowNotdef  bool
}

func (s *simple) Layout(rr []rune) []font.Glyph {
	gg := make([]font.Glyph, len(rr))
	for i, r := range rr {
		gid := s.Sfnt.CMap[r]
		gg[i].Gid = gid
		gg[i].Text = []rune{r}
	}

	gg = s.Sfnt.GSUB.ApplyAll(gg)
	for i := range gg {
		gg[i].Advance = int32(s.Sfnt.HmtxInfo.Widths[gg[i].Gid])
	}
	gg = s.Sfnt.GPOS.ApplyAll(gg)

	for _, g := range gg {
		if _, seen := s.text[g.Gid]; !seen && len(g.Text) > 0 {
			// copy the slice, in case the caller modifies it later
			s.text[g.Gid] = append([]rune{}, g.Text...)
		}
	}

	return gg
}

func (s *simple) Enc(gid font.GlyphID) pdf.String {
	c, found := s.enc[gid]
	if found {
		return pdf.String{c}
	}

	if gid == 0 {
		c = 255
		s.allowNotdef = true
	} else {
		c = byte(s.nextCharCode)
		s.nextCharCode++
	}

	s.enc[gid] = c
	return pdf.String{c}
}

func (s *simple) WriteFont(w *pdf.Writer) error {
	if s.allowNotdef {
		s.nextCharCode++
	}
	if s.nextCharCode > 256 {
		return errors.New("too many different glyphs for simple font " + s.Sfnt.FontName)
	}

	// Determine the subset of glyphs to include.
	var mapping []font.CMapEntry
	for origGid, charCode := range s.enc {
		mapping = append(mapping, font.CMapEntry{
			CharCode: uint16(charCode),
			GID:      origGid,
		})
	}
	if len(mapping) == 0 {
		// It is not clear how a font with no glyphs should be included
		// in a PDF file.  In order to avoid problems, add a dummy glyph.
		mapping = append(mapping, font.CMapEntry{
			CharCode: 0,
			GID:      0,
		})
	}
	sort.Slice(mapping, func(i, j int) bool { return mapping[i].CharCode < mapping[j].CharCode })
	firstCharCode := mapping[0].CharCode
	lastCharCode := mapping[len(mapping)-1].CharCode
	_, includeGlyphs := font.MakeSubset(mapping)
	subsetTag := font.GetSubsetTag(includeGlyphs, s.Sfnt.NumGlyphs())
	cff, err := s.Cff.Subset(includeGlyphs)
	if err != nil {
		return err
	}
	fontName := pdf.Name(s.Cff.FontInfo.FontName) // includes the subset tag

	q := 1000 / float64(s.Sfnt.GlyphUnits)
	FontBBox := &pdf.Rectangle{
		LLx: math.Round(float64(s.Sfnt.FontBBox.LLx) * q),
		LLy: math.Round(float64(s.Sfnt.FontBBox.LLy) * q),
		URx: math.Round(float64(s.Sfnt.FontBBox.URx) * q),
		URy: math.Round(float64(s.Sfnt.FontBBox.URy) * q),
	}

	FontDescriptorRef := w.Alloc()
	WidthsRef := w.Alloc()
	FontFileRef := w.Alloc()
	ToUnicodeRef := w.Alloc()

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
		"Flags":       pdf.Integer(s.Sfnt.Flags),
		"FontBBox":    FontBBox,
		"ItalicAngle": pdf.Number(s.Sfnt.ItalicAngle),
		"Ascent":      pdf.Integer(q*float64(s.Sfnt.HmtxInfo.Ascent) + 0.5),
		"Descent":     pdf.Integer(q*float64(s.Sfnt.HmtxInfo.Descent) + 0.5),
		"CapHeight":   pdf.Integer(q*float64(s.Sfnt.CapHeight) + 0.5),
		"StemV":       pdf.Integer(70), // information not available in sfnt files
		"FontFile3":   FontFileRef,
	}

	var Widths pdf.Array
	pos := 0
	for i := firstCharCode; i <= lastCharCode; i++ {
		width := 0
		if i == mapping[pos].CharCode {
			gid := mapping[pos].GID
			width = int(float64(s.Sfnt.HmtxInfo.Widths[gid])*q + 0.5)
			pos++
		}
		Widths = append(Widths, pdf.Integer(width))
	}

	_, err = w.WriteCompressed(
		[]*pdf.Reference{s.FontRef, FontDescriptorRef, WidthsRef},
		Font, FontDescriptor, Widths)
	if err != nil {
		return err
	}

	// write all the streams

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
	{ // TODO(voss): fix the encoding in a more systematic way
		nCodes := len(cff.Glyphs) - 1
		if nCodes > 256 {
			nCodes = 256
		}
		cff.Encoding = make([]font.GlyphID, nCodes)
		for i := 0; i < nCodes; i++ {
			cff.Encoding[i] = font.GlyphID(i + 1)
		}
		cff.ROS = nil
	}
	err = cff.Encode(fontFileStream)
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

	err = s.Sfnt.Close()
	if err != nil {
		return err
	}

	return nil
}
