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
	"errors"
	"time"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/metadata"
	"seehuhn.de/go/pdf/pieceinfo"
)

// Form represents a PDF form XObject that can contain reusable graphics content.
type Form struct {
	// Draw is the function that renders the form's content.
	Draw func(*graphics.Writer) error

	// BBox is the form's bounding box in form coordinate space.
	BBox pdf.Rectangle

	// Matrix transforms form coordinates to user space coordinates.
	//
	// When writing forms to a PDF file, the zero matrix can be used as an
	// alternative to the identity matrix for convenience.
	Matrix matrix.Matrix

	// Metadata is an optional reference to metadata for this form.
	Metadata *metadata.Stream

	// PieceInfo contains private application data.
	PieceInfo *pieceinfo.PieceInfo

	// LastModified (Required if PieceInfo is present; optional otherwise; PDF
	// 1.3) is the date the form was last modified.
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
		return errors.New("missing BBox")
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
	if contents.Err != nil {
		return nil, zero, contents.Err
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
		if f.LastModified.IsZero() {
			return nil, zero, errors.New("missing LastModified")
		}
		pieceInfoObj, _, err := f.PieceInfo.Embed(rm)
		if err != nil {
			return nil, zero, err
		}
		if pieceInfoObj != nil {
			dict["PieceInfo"] = pieceInfoObj
		}
	}
	if !f.LastModified.IsZero() {
		dict["LastModified"] = pdf.Date(f.LastModified)
	}

	var filters []pdf.Filter
	if !rm.Out.GetOptions().HasAny(pdf.OptPretty) {
		filters = append(filters, &pdf.FilterCompress{})
	}
	stm, err := rm.Out.OpenStream(ref, dict, filters...)
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

// Equal compares two forms by comparing their content streams.
// It ignores resource dictionaries and other metadata.
func (f *Form) Equal(other *Form) bool {
	if f == nil || other == nil || f == other {
		return f == other
	}

	buf1 := &bytes.Buffer{}
	w1, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	c1 := graphics.NewWriter(buf1, pdf.NewResourceManager(w1))
	err1 := f.Draw(c1)

	buf2 := &bytes.Buffer{}
	w2, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	c2 := graphics.NewWriter(buf2, pdf.NewResourceManager(w2))
	err2 := other.Draw(c2)

	if err1 != nil || err2 != nil {
		return false
	}

	return buf1.String() == buf2.String()
}

func toPDF(x []float64) pdf.Array {
	res := make(pdf.Array, len(x))
	for i, xi := range x {
		res[i] = pdf.Number(xi)
	}
	return res
}
