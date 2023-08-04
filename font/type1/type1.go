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

package type1

import (
	"math"

	"seehuhn.de/go/pdf"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/tounicode"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"
)

type PDFFont struct {
	// PSFont is the (subsetted as needed) font to embed.
	PSFont *type1.Font

	// ResName is the resource name for the font.
	// This is only used for PDF version 1.0.
	ResName pdf.Name

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	// Encoding (a slice of length 256) is the encoding vector used by the client.
	// This is used to determine the `Encoding` entry of the PDF font dictionary.
	Encoding []string

	// ToUnicode (optional) is a map from character codes to unicode strings.
	// Character codes must be in the range 0, ..., 255.
	ToUnicode map[charcode.CharCode][]rune
}

func (info *PDFFont) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	useBuiltin := w.GetMeta().Version < pdf.V2_0 && IsBuiltin(info.PSFont)

	if len(info.Encoding) != 256 || len(info.PSFont.Encoding) != 256 {
		panic("unreachable") // TODO(voss): remove
	}

	var fontName pdf.Name
	if info.SubsetTag == "" {
		fontName = pdf.Name(info.PSFont.FontInfo.FontName)
	} else {
		fontName = pdf.Name(info.SubsetTag + "+" + info.PSFont.FontInfo.FontName)
	}

	var toUnicodeRef pdf.Reference
	var fontFileRef pdf.Reference

	// See section 9.6.2.1 of PDF 32000-1:2008.
	FontDict := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": fontName,
	}
	if w.GetMeta().Version == pdf.V1_0 {
		FontDict["Name"] = info.ResName
	}
	if enc := font.DescribeEncoding(info.Encoding, info.PSFont.Encoding); enc != nil {
		FontDict["Encoding"] = enc
	}
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
		FontDict["ToUnicode"] = toUnicodeRef
	}
	compressedRefs := []pdf.Reference{fontDictRef}
	compressedObjects := []pdf.Object{FontDict}

	if !useBuiltin {
		psFont := info.PSFont

		widthsRef := w.Alloc()
		ww := make([]funit.Int16, 256)
		for i := range ww {
			ww[i] = psFont.GlyphInfo[info.Encoding[i]].WidthX
		}
		widthsInfo := font.CompressWidths(ww, psFont.UnitsPerEm)
		FontDict["FirstChar"] = widthsInfo.FirstChar
		FontDict["LastChar"] = widthsInfo.LastChar
		FontDict["Widths"] = widthsRef
		compressedRefs = append(compressedRefs, widthsRef)
		compressedObjects = append(compressedObjects, widthsInfo.Widths)

		FontDescriptorRef := w.Alloc()
		FontDict["FontDescriptor"] = FontDescriptorRef

		q := 1000 / float64(psFont.UnitsPerEm)
		bbox := psFont.BBox()
		fontBBox := &pdf.Rectangle{
			LLx: bbox.LLx.AsFloat(q),
			LLy: bbox.LLy.AsFloat(q),
			URx: bbox.URx.AsFloat(q),
			URy: bbox.URy.AsFloat(q),
		}

		// See section 9.8.1 of PDF 32000-1:2008.
		FontDescriptor := pdf.Dict{
			"Type":        pdf.Name("FontDescriptor"),
			"FontName":    fontName,
			"Flags":       pdf.Integer(type1MakeFlags(psFont, true)),
			"FontBBox":    fontBBox,
			"ItalicAngle": pdf.Number(psFont.FontInfo.ItalicAngle),
			"Ascent":      pdf.Integer(math.Round(psFont.Ascent.AsFloat(q))),
			"Descent":     pdf.Integer(math.Round(psFont.Descent.AsFloat(q))),
			"StemV":       pdf.Integer(math.Round(psFont.Private.StdVW * q)),
		}
		if psFont.CapHeight != 0 {
			FontDescriptor["CapHeight"] = pdf.Integer(math.Round(psFont.CapHeight.AsFloat(q)))
		}
		if widthsInfo.MissingWidth != 0 {
			FontDescriptor["MissingWidth"] = widthsInfo.MissingWidth
		}
		if psFont.Outlines != nil {
			fontFileRef = w.Alloc()
			FontDescriptor["FontFile"] = fontFileRef
		}
		compressedRefs = append(compressedRefs, FontDescriptorRef)
		compressedObjects = append(compressedObjects, FontDescriptor)
	}

	err := w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return err
	}

	if fontFileRef != 0 {
		// See section 9.9 of PDF 32000-1:2008.
		length1 := pdf.NewPlaceholder(w, 10)
		length2 := pdf.NewPlaceholder(w, 10)
		fontFileDict := pdf.Dict{
			"Length1": length1,
			"Length2": length2,
			"Length3": pdf.Integer(0),
		}
		fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
		if err != nil {
			return err
		}
		l1, l2, err := info.PSFont.WritePDF(fontFileStream)
		if err != nil {
			return err
		}
		length1.Set(pdf.Integer(l1))
		length2.Set(pdf.Integer(l2))
		err = fontFileStream.Close()
		if err != nil {
			return err
		}
	}

	if toUnicodeRef != 0 {
		err = tounicode.Embed(w, toUnicodeRef, charcode.Simple, info.ToUnicode)
		if err != nil {
			return err
		}
	}

	return nil
}

// type1MakeFlags returns the PDF font flags for the font.
// See section 9.8.2 of PDF 32000-1:2008.
//
// TODO(voss): try to unify with cffMakeFlags.
func type1MakeFlags(info *type1.Font, symbolic bool) font.Flags {
	var flags font.Flags

	if info.FontInfo.IsFixedPitch {
		flags |= font.FlagFixedPitch
	}
	// TODO(voss): flags |= font.FlagSerif

	if symbolic {
		flags |= font.FlagSymbolic
	} else {
		flags |= font.FlagNonsymbolic
	}

	// flags |= FlagScript
	if info.FontInfo.ItalicAngle != 0 {
		flags |= font.FlagItalic
	}

	if info.Private.ForceBold {
		flags |= font.FlagForceBold
	}

	return flags
}
