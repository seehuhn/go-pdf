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

package type3

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/funit"
)

type Font struct {
	BBox       funit.Rect16
	FontMatrix [6]float64
	CharProcs  map[pdf.Name]*GlyphData
	Encoding   []string
	Resources  *pdf.Resources
	*font.Descriptor
}

type GlyphData struct {
	Width   funit.Int16
	Content []byte
}

type PDFFont struct {
	// PSFont is the (subsetted as needed) font to embed.
	PSFont *Font

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	// Encoding (a slice of length 256) is the encoding vector used by the client.
	// This is used to determine the `Encoding` entry of the PDF font dictionary.
	Encoding []string

	// ToUnicode (optional) is a map from character codes to unicode strings.
	// Character codes must be in the range 0, ..., 255.
	ToUnicode map[charcode.CharCode][]rune

	// ResName is the resource name for the font.
	// This is only used for PDF version 1.0.
	ResName pdf.Name
}

func (info *PDFFont) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	if len(info.Encoding) != 256 || len(info.PSFont.Encoding) != 256 {
		panic("unreachable") // TODO(voss): remove
	}

	psFont := info.PSFont

	var fontName pdf.Name
	if info.SubsetTag == "" {
		fontName = psFont.Descriptor.FontName
	} else {
		fontName = pdf.Name(info.SubsetTag+"+") + psFont.Descriptor.FontName
	}

	q := 1000 / float64(psFont.UnitsPerEm)
	bbox := psFont.BBox()
	fontBBox := &pdf.Rectangle{
		LLx: bbox.LLx.AsFloat(q),
		LLy: bbox.LLy.AsFloat(q),
		URx: bbox.URx.AsFloat(q),
		URy: bbox.URy.AsFloat(q),
	}

	// See section 9.6.4 of PDF 32000-1:2008.
	FontDict := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type3"),
		"BaseFont": fontName,
		"FontBBox": fontBBox,
	}
	if w.GetMeta().Version == pdf.V1_0 {
		FontDict["Name"] = info.ResName
	}

}
