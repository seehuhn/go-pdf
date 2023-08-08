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
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/tounicode"
	"seehuhn.de/go/postscript/funit"
)

type Glyph struct {
	WidthX  funit.Int16
	WidthY  funit.Int16
	BBox    funit.Rect16
	Content []byte
}

type PDFFont struct {
	FontMatrix [6]float64
	Glyphs     map[pdf.Name]*Glyph
	Resources  *pdf.Resources
	*font.Descriptor

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
	if len(info.Encoding) != 256 {
		panic("unreachable") // TODO(voss): remove
	}

	if len(info.FontMatrix) != 6 {
		return errors.New("invalid font matrix")
	}
	fontMatrix := make(pdf.Array, len(info.FontMatrix))
	for i, x := range info.FontMatrix {
		fontMatrix[i] = pdf.Number(x)
	}

	charProcs := make(pdf.Dict, len(info.Glyphs))
	for name := range info.Glyphs {
		// TODO(voss): should this only include encoded glyphs?
		charProcs[name] = w.Alloc()
	}

	differences := pdf.Array{}
	prev := 256
	for code, name := range info.Encoding {
		name := pdf.Name(name)
		if _, exists := charProcs[name]; !exists {
			continue
		}
		if code != prev+1 {
			differences = append(differences, pdf.Integer(code))
		}
		differences = append(differences, name)
		prev = code
	}
	encoding := pdf.Dict{
		"Differences": differences,
	}

	ww := make([]funit.Int16, 256)
	for i := range ww {
		name := pdf.Name(info.Encoding[i])
		g := info.Glyphs[name]
		if g != nil {
			ww[i] = g.WidthX
		}
	}
	var firstChar pdf.Integer
	lastChar := pdf.Integer(255)
	for lastChar > 0 && ww[lastChar] == 0 {
		lastChar--
	}
	for firstChar < lastChar && ww[firstChar] == 0 {
		firstChar++
	}
	widths := make(pdf.Array, lastChar-firstChar+1)
	for i := range widths {
		// TODO(voss): These widths shall be interpreted in glyph space as
		// specified by FontMatrix (unlike the widths of a Type 1 font, which
		// are in thousandths of a unit of text space).  If FontMatrix
		// specifies a rotation, only the horizontal component of the
		// transformed width shall be used.
		widths[i] = pdf.Integer(ww[int(firstChar)+i])
	}

	// See section 9.6.4 of PDF 32000-1:2008.
	fontDict := pdf.Dict{
		"Type":       pdf.Name("Font"),
		"Subtype":    pdf.Name("Type3"),
		"FontBBox":   &pdf.Rectangle{}, // [0 0 0 0] is always valid
		"FontMatrix": fontMatrix,
		"CharProcs":  charProcs,
		"Encoding":   encoding,
		"FirstChar":  firstChar,
		"LastChar":   lastChar,
		"Widths":     widths,
	}
	if w.GetMeta().Version == pdf.V1_0 {
		fontDict["Name"] = info.ResName
	}
	if !info.Resources.IsEmpty() {
		resources := pdf.AsDict(info.Resources)
		fontDict["Resources"] = resources
	}
	var toUnicodeRef pdf.Reference
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
		fontDict["ToUnicode"] = toUnicodeRef
	}

	compressedRefs := []pdf.Reference{fontDictRef}
	compressedObjects := []pdf.Object{fontDict}

	if info.Descriptor != nil {
		isSymbolic := true
		for name := range charProcs {
			// TODO(voss): should this only check encoded glyphs?
			if !pdfenc.IsStandardLatin[string(name)] {
				isSymbolic = false
				break
			}
		}

		fdRef := w.Alloc()
		fontDict["FontDescriptor"] = fdRef
		fontDescriptor := info.Descriptor.AsDict(isSymbolic)
		compressedObjects = append(compressedObjects, fontDescriptor)
		compressedRefs = append(compressedRefs, fdRef)
	}

	err := w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return err
	}

	for name, g := range info.Glyphs {
		gRef := charProcs[name].(pdf.Reference)
		stm, err := w.OpenStream(gRef, nil, pdf.FilterCompress{})
		if err != nil {
			return nil
		}
		_, err = stm.Write(g.Content)
		if err != nil {
			return nil
		}
		err = stm.Close()
		if err != nil {
			return nil
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
