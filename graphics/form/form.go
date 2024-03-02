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
	"bytes"
	"errors"
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/matrix"
)

// FormProperties has data for a form XObject.
//
// See section 8.10 of ISO 32000-2:2020 for details.
type FormProperties struct {
	BBox         *pdf.Rectangle
	Matrix       matrix.Matrix
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

type FormBuilder struct {
	Out pdf.Putter
	*graphics.Writer
	*FormProperties
}

func Raw(w pdf.Putter, f *FormProperties, contents []byte, resources *pdf.Resources) (*graphics.XObject, error) {
	dict := pdf.Dict{
		"Subtype":  pdf.Name("Form"),
		"FormType": pdf.Integer(1),
		"BBox":     f.BBox,
	}
	if f.Matrix != matrix.Identity && f.Matrix != matrix.Zero {
		dict["Matrix"] = toPDF(f.Matrix[:])
	}
	if resources != nil {
		dict["Resources"] = pdf.AsDict(resources)
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
	_, err = stm.Write(contents)
	if err != nil {
		return nil, err
	}
	err = stm.Close()
	if err != nil {
		return nil, err
	}

	res := &graphics.XObject{
		Res: pdf.Res{
			DefName: f.DefaultName,
			Data:    ref,
		},
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

func New(w pdf.Putter, prop *FormProperties) *FormBuilder {
	contents := graphics.NewWriter(&bytes.Buffer{}, pdf.GetVersion(w))
	return &FormBuilder{
		Out:            w,
		Writer:         contents,
		FormProperties: prop,
	}
}

func (f *FormBuilder) Finish() (*graphics.XObject, error) {
	contents := f.Writer.Content.(*bytes.Buffer).Bytes()
	resources := f.Writer.Resources

	// disable the writer to prevent further writes
	f.Writer = nil

	return Raw(f.Out, f.FormProperties, contents, resources)
}
