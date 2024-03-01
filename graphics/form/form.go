// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package form

import (
	"errors"
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/matrix"
)

// Form represents a PDF Form XObject.
//
// See section 8.10 of ISO 32000-2:2020 for details.
type Form struct {
	BBox         *pdf.Rectangle
	Matrix       matrix.Matrix
	Resources    *pdf.Resources
	MetaData     pdf.Reference
	PieceInfo    pdf.Object
	LastModified time.Time
	// TODO(voss): StructParent, StructParents
	OC          pdf.Object
	DefaultName pdf.Name
	AF          pdf.Object
	Measure     pdf.Object
	PtData      pdf.Object
}

// Embedded represents a Form XObject embedded in a PDF file.
type Embedded struct {
	pdf.Res
	BBox *pdf.Rectangle
}

// Embed creates a new Form XObject and embeds it in the PDF file.
//
// TODO(voss): can/should we have a streaming interface for the body?
func (f *Form) Embed(w pdf.Putter, body []byte) (*Embedded, error) {
	dict := pdf.Dict{
		"Subtype":  pdf.Name("Form"),
		"FormType": pdf.Integer(1),
		"BBox":     f.BBox,
	}
	if f.Matrix != matrix.Identity && f.Matrix != matrix.Zero {
		dict["Matrix"] = toPDF(f.Matrix[:])
	}
	if f.Resources != nil {
		dict["Resources"] = pdf.AsDict(f.Resources)
	}
	if f.MetaData != 0 {
		dict["Metadata"] = f.MetaData
	}
	if f.PieceInfo != nil {
		dict["PieceInfo"] = f.PieceInfo
	}
	if !f.LastModified.IsZero() {
		dict["LastModified"] = pdf.Date(f.LastModified)
	}
	if f.OC != nil {
		dict["OC"] = f.OC
	}
	if pdf.GetVersion(w) == pdf.V1_0 {
		if f.DefaultName == "" {
			return nil, errors.New("Form.DefaultName must be set in PDF 1.0")
		}
		dict["Name"] = f.DefaultName
	}
	if f.AF != nil {
		dict["AF"] = f.AF
	}
	if f.Measure != nil {
		dict["Measure"] = f.Measure
	}
	if f.PtData != nil {
		dict["PtData"] = f.PtData
	}

	ref := w.Alloc()
	stm, err := w.OpenStream(ref, dict, &pdf.FilterCompress{})
	if err != nil {
		return nil, err
	}
	_, err = stm.Write(body)
	if err != nil {
		return nil, err
	}
	err = stm.Close()
	if err != nil {
		return nil, err
	}

	res := &Embedded{
		Res: pdf.Res{
			DefName: f.DefaultName,
			Data:    ref,
		},
		BBox: f.BBox,
	}
	return res, nil
}

func toPDF(x []float64) pdf.Array {
	res := make(pdf.Array, len(x))
	for i, xi := range x {
		res[i] = pdf.Number(xi)
	}
	return res
}
