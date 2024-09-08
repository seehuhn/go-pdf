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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/matrix"
)

type Form struct {
	*Properties
	Resources *pdf.Resources
	Contents  []byte
	RM        *pdf.ResourceManager
}

// IsDirect returns true if the Form object does not contain any
// references to indirect PDF objects.
func (f *Form) IsDirect() bool {
	return f.Resources.IsDirect() && f.Properties.IsDirect()
}

func (f *Form) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	if !f.IsDirect() && f.RM != rm {
		return nil, zero, errors.New("Form: resource manager mismatch")
	}

	dict := pdf.Dict{
		// "Type":     pdf.Name("XObject"),
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
	if f.Metadata != 0 {
		dict["Metadata"] = f.Metadata
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
	if pdf.GetVersion(rm.Out) == pdf.V1_0 {
		if f.DefaultName == "" {
			return nil, zero, errors.New("Form.DefaultName must be set in PDF 1.0")
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

	ref := rm.Out.Alloc()
	stm, err := rm.Out.OpenStream(ref, dict, &pdf.FilterCompress{})
	if err != nil {
		return nil, zero, err
	}
	_, err = stm.Write(f.Contents)
	if err != nil {
		return nil, zero, err
	}
	err = stm.Close()
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

// Subtype returns /Form.
// This implements the [graphics.XObject] interface.
func (f *Form) Subtype() pdf.Name {
	return "Form"
}
