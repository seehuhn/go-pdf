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

package form

import (
	"bytes"
	"time"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/metadata"
)

// Form represents a PDF form XObject that can contain reusable graphics content.
type Form struct {
	// Draw is the function that renders the form's content.
	Draw func(*graphics.Writer) error

	// BBox is the form's bounding box in form coordinate space.
	BBox pdf.Rectangle

	// Matrix transforms form coordinates to user space coordinates.
	Matrix matrix.Matrix

	// Metadata is an optional reference to metadata for this form.
	Metadata *metadata.Stream

	// PieceInfo contains private application data.
	PieceInfo pdf.Object

	// LastModified is the date the form was last modified.
	LastModified time.Time

	// TODO(voss): StructParent, StructParents
	// TODO(voss): OC
	// TODO(voss): AF
	// TODO(voss): Measure
	// TODO(voss): PtData
}

// Subtype returns the XObject subtype for forms.
func (f *Form) Subtype() pdf.Name {
	return "Form"
}

func (f *Form) validate() error {
	if f.BBox.IsZero() {
		return pdf.Error("missing BBox")
	}
	return nil
}

// Embed implements the pdf.Embedder interface for form XObjects.
func (f *Form) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	err := f.validate()
	if err != nil {
		return nil, zero, err
	}

	buf := &bytes.Buffer{}
	contents := graphics.NewWriter(buf, rm)
	err = f.Draw(contents)
	if err != nil {
		return nil, zero, err
	}

	ref := rm.Out.Alloc()

	dict := pdf.Dict{
		"Subtype": pdf.Name("Form"),
		"BBox":    &f.BBox,
	}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("XObject")
	}
	if f.Matrix != matrix.Identity && f.Matrix != matrix.Zero {
		dict["Matrix"] = toPDF(f.Matrix[:])
	}
	if contents.Resources != nil {
		dict["Resources"] = pdf.AsDict(contents.Resources)
	}
	if f.Metadata != nil {
		rmEmbedded, _, err := pdf.ResourceManagerEmbed(rm, f.Metadata)
		if err != nil {
			return nil, zero, err
		}
		dict["Metadata"] = rmEmbedded
	}
	if f.PieceInfo != nil {
		dict["PieceInfo"] = f.PieceInfo
	}
	if !f.LastModified.IsZero() {
		dict["LastModified"] = pdf.Date(f.LastModified)
	}

	stm, err := rm.Out.OpenStream(ref, dict, &pdf.FilterCompress{})
	if err != nil {
		return nil, zero, err
	}
	_, err = stm.Write(buf.Bytes())
	if err != nil {
		return nil, zero, err
	}
	err = stm.Close()
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

func toPDF(x []float64) pdf.Array {
	res := make(pdf.Array, len(x))
	for i, xi := range x {
		res[i] = pdf.Number(xi)
	}
	return res
}
