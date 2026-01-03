// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package extract

import (
	"errors"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding"
)

// extractFontType3 reads a Type 3 font dictionary from a PDF file.
func extractFontType3(x *pdf.Extractor, obj pdf.Object) (*dict.Type3, error) {
	fontDict, err := x.GetDictTyped(obj, "Font")
	if err != nil {
		return nil, err
	} else if fontDict == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing font dictionary"),
		}
	}
	subtype, err := x.GetName(fontDict["Subtype"])
	if err != nil {
		return nil, err
	}
	if subtype != "" && subtype != "Type3" {
		return nil, pdf.Errorf("expected font subtype Type3, got %q", subtype)
	}

	d := &dict.Type3{}

	d.Name, _ = x.GetName(fontDict["Name"])

	fdDict, err := x.GetDictTyped(fontDict["FontDescriptor"], "FontDescriptor")
	if pdf.IsReadError(err) {
		return nil, err
	}
	fd, _ := font.ExtractDescriptor(x.R, fdDict)
	d.Descriptor = fd

	enc, err := encoding.ExtractType3(x.R, fontDict["Encoding"])
	if err != nil {
		return nil, err
	}
	d.Encoding = enc

	var defaultWidth float64
	if fd != nil {
		defaultWidth = fd.MissingWidth
	}
	getSimpleWidths(d.Width[:], x.R, fontDict, defaultWidth)

	d.ToUnicode, _ = cmap.ExtractToUnicode(x.R, fontDict["ToUnicode"])

	charProcs, err := x.GetDict(fontDict["CharProcs"])
	if err != nil {
		return nil, pdf.Wrap(err, "CharProcs")
	}
	glyphs := make(map[pdf.Name]pdf.Reference, len(charProcs))
	for name, ref := range charProcs {
		if ref, ok := ref.(pdf.Reference); ok {
			glyphs[name] = ref
		}
	}
	d.CharProcs = glyphs

	fontBBox, _ := pdf.GetRectangle(x.R, fontDict["FontBBox"])
	if fontBBox != nil && !fontBBox.IsZero() {
		d.FontBBox = fontBBox
	}

	d.FontMatrix, _ = pdf.GetMatrix(x.R, fontDict["FontMatrix"])

	d.Resources, err = pdf.ExtractResources(x.R, fontDict["Resources"])
	if pdf.IsReadError(err) {
		return nil, err
	}

	repairType3(d, x.R)

	return d, nil
}

// repairType3 fixes invalid data in a Type3 font dictionary after extraction.
func repairType3(d *dict.Type3, r pdf.Getter) {
	if v := pdf.GetVersion(r); v == pdf.V1_0 {
		if d.Name == "" {
			d.Name = "Font"
		}
	}

	if d.FontMatrix.IsZero() {
		d.FontMatrix = matrix.Matrix{0.001, 0, 0, 0.001, 0, 0}
	}
}
