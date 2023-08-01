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

package font

import (
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"
)

type Type1Info struct {
	// Font is the (subsetted as needed) font to embed.
	Font *type1.Font

	// ResName is the resource name for the font.
	// This is only used for PDF version 1.0.
	ResName pdf.Name

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if Font is the full font.
	SubsetTag string

	// Encoding (a slice of length 256) is the encoding vector used by the client.
	// This is used to determine the `Encoding` entry of the PDF font dictionary.
	Encoding []string

	// ToUnicode (optional) is a map from character codes to unicode strings
	ToUnicode map[byte][]rune
}

func (info *Type1Info) Embed(w pdf.Putter, ref pdf.Reference) error {
	var fontName string
	if info.SubsetTag == "" {
		fontName = info.Font.Info.FontName
	} else {
		fontName = info.SubsetTag + "+" + info.Font.Info.FontName
	}

	if len(info.Encoding) != 256 || len(info.Font.Encoding) != 256 {
		panic("unreachable") // TODO(voss): remove
	}

	ww := make([]funit.Int16, 256)
	for i := range ww {
		ww[i] = info.Font.GlyphInfo[info.Font.Encoding[i]].WidthX
	}
	widthsInfo := compressWidths(ww, info.Font.UnitsPerEm)

	FontDictRef := ref
	FontDescriptorRef := w.Alloc()
	WidthsRef := w.Alloc()
	var FontFileRef pdf.Reference
	if info.Font.Outlines != nil {
		FontFileRef = w.Alloc()
	}
	var toUnicodeRef pdf.Reference
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
	}

	q := 1000 / float64(info.Font.UnitsPerEm)
	bbox := info.Font.BBox()
	fontBBox := &pdf.Rectangle{
		LLx: bbox.LLx.AsFloat(q),
		LLy: bbox.LLy.AsFloat(q),
		URx: bbox.URx.AsFloat(q),
		URy: bbox.URy.AsFloat(q),
	}

	// See section 9.6.2.1 of PDF 32000-1:2008.
	FontDict := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("Type1"),
		"BaseFont":       pdf.Name(fontName),
		"FirstChar":      widthsInfo.FirstChar,
		"LastChar":       widthsInfo.LastChar,
		"Widths":         WidthsRef,
		"FontDescriptor": FontDescriptorRef,
		"Encoding":       DescribeEncoding(info.Encoding, info.Font.Encoding),
	}
	if w.GetMeta().Version == pdf.V1_0 {
		FontDict["Name"] = info.ResName
	}
	if toUnicodeRef != 0 {
		FontDict["ToUnicode"] = toUnicodeRef
	}

	// See section 9.8.1 of PDF 32000-1:2008.
	FontDescriptor := pdf.Dict{
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    pdf.Name(fontName),
		"Flags":       pdf.Integer(type1MakeFlags(info.Font, true)),
		"FontBBox":    fontBBox,
		"ItalicAngle": pdf.Number(info.Font.Info.ItalicAngle),
		"Ascent":      pdf.Integer(math.Round(info.Font.Ascent.AsFloat(q))),
		"Descent":     pdf.Integer(math.Round(info.Font.Descent.AsFloat(q))),
		"StemV":       pdf.Integer(math.Round(info.Font.Private.StdVW * q)),
	}
	if info.Font.CapHeight != 0 {
		FontDescriptor["CapHeight"] = pdf.Integer(math.Round(info.Font.CapHeight.AsFloat(q)))
	}
	if widthsInfo.MissingWidth != 0 {
		FontDescriptor["MissingWidth"] = widthsInfo.MissingWidth
	}
	if FontFileRef != 0 {
		FontDescriptor["FontFile"] = FontFileRef
	}

	compressedRefs := []pdf.Reference{FontDictRef, FontDescriptorRef, WidthsRef}
	compressedObjects := []pdf.Object{FontDict, FontDescriptor, widthsInfo.Widths}
	err := w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return err
	}

	if FontFileRef != 0 {
		// See section 9.9 of PDF 32000-1:2008.
		length1 := pdf.NewPlaceholder(w, 10)
		length2 := pdf.NewPlaceholder(w, 10)
		length3 := pdf.NewPlaceholder(w, 10)
		fontFileDict := pdf.Dict{
			"Length1": length1,
			"Length2": length2,
			"Length3": length3,
		}
		fontFileStream, err := w.OpenStream(FontFileRef, fontFileDict, pdf.FilterCompress{})
		if err != nil {
			return err
		}
		l1, l2, l3, err := info.Font.WritePDF(fontFileStream)
		if err != nil {
			return err
		}
		length1.Set(pdf.Integer(l1))
		length2.Set(pdf.Integer(l2))
		length3.Set(pdf.Integer(l3))
		err = fontFileStream.Close()
		if err != nil {
			return err
		}
	}

	if toUnicodeRef != 0 {
		// a CMap file that maps character codes to Unicode values
	}

	return nil
}

// MakeFlags returns the PDF font flags for the font.
// See section 9.8.2 of PDF 32000-1:2008.
func type1MakeFlags(info *type1.Font, symbolic bool) Flags {
	var flags Flags

	if info.Info.IsFixedPitch {
		flags |= FlagFixedPitch
	}
	// TODO(voss): flags |= font.FlagSerif

	if symbolic {
		flags |= FlagSymbolic
	} else {
		flags |= FlagNonsymbolic
	}

	// flags |= FlagScript
	if info.Info.ItalicAngle != 0 {
		flags |= FlagItalic
	}

	if info.Private.ForceBold {
		flags |= FlagForceBold
	}

	return flags
}
