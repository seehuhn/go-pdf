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

package truetype

import (
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/tounicode"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1/names"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"
)

type PDFInfo struct {
	// Font is the font to embed (already subsetted, if needed).
	Font *sfnt.Font

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

func (info *PDFInfo) WritePDF(w pdf.Putter, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "embedding of TrueType fonts", pdf.V1_1)
	if err != nil {
		return err
	}

	trueTypeFont := info.Font

	var fontName pdf.Name
	postScriptName := trueTypeFont.PostscriptName()
	if info.SubsetTag == "" {
		fontName = pdf.Name(postScriptName)
	} else {
		fontName = pdf.Name(info.SubsetTag + "+" + postScriptName)
	}

	unitsPerEm := trueTypeFont.UnitsPerEm

	ww := make([]funit.Int16, 256)
	for i := range ww {
		ww[i] = trueTypeFont.GlyphWidth(info.Encoding[i])
	}
	widthsInfo := font.CompressWidths(ww, unitsPerEm)

	var isSymbolic bool
	var encoding pdf.Object
	var cmapTable cmap.Table
	toUnicode := info.ToUnicode
	if glyphNames := info.makeNameEncoding(); glyphNames != nil && w.GetMeta().Version >= pdf.V1_3 {
		// Mark the font as "nonsymbolic", set "/Encoding", and use a (3, 1)
		// "cmap" subtable to map unicode values to glyphs.
		isSymbolic = false

		encoding = font.DescribeEncodingTrueType(glyphNames)

		subtable := cmap.Format4{}
		for i, name := range glyphNames {
			if name == ".notdef" {
				continue
			}
			r := names.ToUnicode(name, false)[0]
			subtable[uint16(r)] = info.Encoding[i]
		}
		cmapTable = cmap.Table{
			{PlatformID: 3, EncodingID: 1}: subtable.Encode(0),
		}
		// TODO(voss): check whether the toUnicode map still needs to be embedded.
	} else {
		// Mark the font as "symbolic", and use a (1, 0) "cmap" subtable to map
		// character codes to glyphs.
		isSymbolic = true

		subtable := cmap.Format4{}
		for i, gid := range info.Encoding {
			if gid == 0 || i >= 256 {
				continue
			}
			subtable[uint16(i)] = gid
		}
		cmapTable = cmap.Table{
			{PlatformID: 1, EncodingID: 0}: subtable.Encode(0),
		}
	}
	cmapData := cmapTable.Encode()

	q := 1000 / float64(unitsPerEm)
	bbox := trueTypeFont.BBox()
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
		"Subtype":        pdf.Name("TrueType"),
		"BaseFont":       fontName,
		"FirstChar":      widthsInfo.FirstChar,
		"LastChar":       widthsInfo.LastChar,
		"Widths":         widthsRef,
		"FontDescriptor": fontDescriptorRef,
	}
	if encoding != nil {
		fontDict["Encoding"] = encoding
	}
	var toUnicodeRef pdf.Reference
	if toUnicode != nil {
		toUnicodeRef = w.Alloc()
		fontDict["ToUnicode"] = toUnicodeRef
	}

	// See section 9.8.1 of PDF 32000-1:2008.
	fontDescriptor := pdf.Dict{
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    fontName,
		"Flags":       pdf.Integer(font.MakeFlags(trueTypeFont, isSymbolic)),
		"FontBBox":    fontBBox,
		"ItalicAngle": pdf.Number(trueTypeFont.ItalicAngle),
		"Ascent":      pdf.Integer(math.Round(trueTypeFont.Ascent.AsFloat(q))),
		"Descent":     pdf.Integer(math.Round(trueTypeFont.Descent.AsFloat(q))),
		"StemV":       pdf.Integer(0), // TODO(voss): can we do better?
		"FontFile2":   fontFileRef,
	}
	if trueTypeFont.CapHeight != 0 {
		fontDescriptor["CapHeight"] = pdf.Integer(math.Round(trueTypeFont.CapHeight.AsFloat(q)))
	}
	if widthsInfo.MissingWidth != 0 {
		fontDescriptor["MissingWidth"] = widthsInfo.MissingWidth
	}

	compressedRefs := []pdf.Reference{fontDictRef, fontDescriptorRef, widthsRef}
	compressedObjects := []pdf.Object{fontDict, fontDescriptor, widthsInfo.Widths}
	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return err
	}

	// See section 9.9 of PDF 32000-1:2008 for details.
	length1 := pdf.NewPlaceholder(w, 10)
	fontFileDict := pdf.Dict{
		"Length1": length1,
	}
	fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	n, err := trueTypeFont.WriteTrueTypePDF(fontFileStream, cmapData)
	if err != nil {
		return fmt.Errorf("embedding TrueType font %q: %w", fontName, err)
	}
	err = length1.Set(pdf.Integer(n))
	if err != nil {
		return err
	}
	err = fontFileStream.Close()
	if err != nil {
		return err
	}

	if toUnicodeRef != 0 {
		err = tounicode.Embed(w, toUnicodeRef, charcode.Simple, toUnicode)
		if err != nil {
			return err
		}
	}

	return nil
}

func (info *PDFInfo) makeNameEncoding() []string {
	encoding := make([]string, 256)
	seen := make(map[string]bool)

	// step 1: mark the .notdef entries
	for code, gid := range info.Encoding {
		if gid == 0 {
			encoding[code] = ".notdef"
			continue
		}
	}

	// step 2: try to determine glyph names from the ToUnicode map
	if info.ToUnicode != nil {
		for code, name := range encoding {
			if name != "" {
				continue
			}

			rr := info.ToUnicode[charcode.CharCode(code)]
			if len(rr) != 1 {
				continue
			}
			r := rr[0]

			name, ok := pdfenc.ToStandardLatin[r]
			if ok && !seen[name] {
				encoding[code] = name
				seen[name] = true
			}
		}
	}

	// step 3: use info.Font.CMap to determine glyph names
	// TODO(voss): implement this

	// step 4: check whether the font has usable glyph names
	namesInFont := info.Font.Outlines.(*glyf.Outlines).Names
	if len(namesInFont) > 0 {
		for code, gid := range info.Encoding {
			if encoding[code] != "" || int(gid) >= len(namesInFont) {
				continue
			}
			name := namesInFont[gid]

			// If possible, use the name as is, ...
			if pdfenc.IsStandardLatin[name] && !seen[name] {
				encoding[code] = name
				seen[name] = true
				continue
			}

			// ... otherwise try to normalize the name.
			rr := names.ToUnicode(name, false)
			if len(rr) != 1 {
				continue
			}
			name = names.FromUnicode(rr[0])
			if pdfenc.IsStandardLatin[name] && !seen[name] {
				encoding[code] = name
				seen[name] = true
				continue
			}
		}
	}

	for _, name := range encoding {
		if name == "" {
			return nil
		}
	}
	return encoding
}
