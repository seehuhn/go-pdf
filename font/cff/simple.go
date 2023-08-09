// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package cff

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/tounicode"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
)

type PDFInfoSimple struct {
	// Font is the font to embed (already subsetted, if needed).
	Font *cff.Font

	UnitsPerEm uint16 // TODO(voss): get this from the font matrix instead

	IsSerif    bool
	IsScript   bool
	IsAllCap   bool
	IsSmallCap bool

	Ascent    funit.Int16
	Descent   funit.Int16
	CapHeight funit.Int16

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	// Encoding is the encoding vector used by the client (a slice of length 256).
	// Together with the font's built-in encoding, this is used to determine
	// the `Encoding` entry of the PDF font dictionary.
	Encoding []glyph.ID

	// ToUnicode (optional) is a map from character codes to unicode strings.
	// Character codes must be in the range 0, ..., 255.
	// TODO(voss): or else?
	ToUnicode map[charcode.CharCode][]rune
}

func (info *PDFInfoSimple) WritePDF(w pdf.Putter, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "CFF fonts", pdf.V1_2)
	if err != nil {
		return err
	}

	cffFont := info.Font
	if len(cffFont.Encoding) != 256 ||
		len(cffFont.Private) != 1 ||
		len(cffFont.Glyphs) == 0 ||
		len(cffFont.Glyphs[0].Name) == 0 {
		return errors.New("font is not a simple CFF font")
	}

	var fontName string
	if info.SubsetTag == "" {
		fontName = cffFont.FontInfo.FontName
	} else {
		fontName = info.SubsetTag + "+" + cffFont.FontInfo.FontName
	}

	unitsPerEm := info.UnitsPerEm

	ww := make([]funit.Int16, 256)
	for i := range ww {
		ww[i] = cffFont.Glyphs[info.Encoding[i]].Width
	}
	widthsInfo := font.CompressWidths(ww, unitsPerEm)

	encoding := make([]string, 256)
	builtin := make([]string, 256)
	for i := 0; i < 256; i++ {
		encoding[i] = cffFont.Glyphs[info.Encoding[i]].Name
		builtin[i] = cffFont.Glyphs[cffFont.Encoding[i]].Name
	}

	q := 1000 / float64(unitsPerEm)
	bbox := cffFont.BBox()
	fontBBox := &pdf.Rectangle{
		LLx: bbox.LLx.AsFloat(q),
		LLy: bbox.LLy.AsFloat(q),
		URx: bbox.URx.AsFloat(q),
		URy: bbox.URy.AsFloat(q),
	}

	widthsRef := w.Alloc()
	fontDescriptorRef := w.Alloc()
	fontFileRef := w.Alloc()

	// See section 9.6.2.1 of PDF 32000-1:2008.
	fontDict := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("Type1"),
		"BaseFont":       pdf.Name(fontName),
		"FirstChar":      widthsInfo.FirstChar,
		"LastChar":       widthsInfo.LastChar,
		"Widths":         widthsRef,
		"FontDescriptor": fontDescriptorRef,
	}
	if enc := font.DescribeEncodingType1(encoding, builtin); enc != nil {
		fontDict["Encoding"] = enc
	}
	var toUnicodeRef pdf.Reference
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
		fontDict["ToUnicode"] = toUnicodeRef
	}

	// See section 9.8.1 of PDF 32000-1:2008.
	fd := &font.Descriptor{
		FontName:     fontName,
		IsFixedPitch: info.Font.IsFixedPitch,
		IsSerif:      info.IsSerif,
		IsScript:     info.IsScript,
		IsItalic:     info.Font.ItalicAngle != 0,
		IsAllCap:     info.IsAllCap,
		IsSmallCap:   info.IsSmallCap,
		ForceBold:    info.Font.Private[0].ForceBold,
		FontBBox:     fontBBox,
		ItalicAngle:  info.Font.ItalicAngle,
		Ascent:       info.Ascent.AsFloat(q),
		Descent:      info.Descent.AsFloat(q),
		CapHeight:    info.CapHeight.AsFloat(q),
		StemV:        cffFont.Private[0].StdVW * q,
		MissingWidth: float64(widthsInfo.MissingWidth),
	}
	fontDescriptor := fd.AsDict(true)
	fontDescriptor["FontFile3"] = fontFileRef

	compressedRefs := []pdf.Reference{fontDictRef, fontDescriptorRef, widthsRef}
	compressedObjects := []pdf.Object{fontDict, fontDescriptor, widthsInfo.Widths}
	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return err
	}

	// See section 9.9 of PDF 32000-1:2008 for details.
	fontFileDict := pdf.Dict{
		"Subtype": pdf.Name("Type1C"),
	}
	fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	err = cffFont.Encode(fontFileStream)
	if err != nil {
		return fmt.Errorf("embedding CFF font %q: %w", fontName, err)
	}
	err = fontFileStream.Close()
	if err != nil {
		return err
	}

	if toUnicodeRef != 0 {
		err = tounicode.Embed(w, toUnicodeRef, charcode.Simple, info.ToUnicode)
		if err != nil {
			return err
		}
	}

	return nil
}
