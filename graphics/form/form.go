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
	"errors"
	"fmt"
	"time"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/group"
	"seehuhn.de/go/pdf/measure"
	"seehuhn.de/go/pdf/metadata"
	"seehuhn.de/go/pdf/oc"
	"seehuhn.de/go/pdf/optional"
	"seehuhn.de/go/pdf/pieceinfo"
)

// PDF 2.0 sections: 8.10

// Form represents a PDF form XObject that can contain reusable graphics content.
type Form struct {
	// Content is the content stream that draws the form.
	Content content.Stream

	// Res contains the resources used by the content stream (required).
	Res *content.Resources

	// BBox is the form's bounding box in form coordinate space.
	BBox pdf.Rectangle

	// Matrix transforms form coordinates to user space coordinates.
	//
	// When writing forms to a PDF file, the zero matrix can be used as an
	// alternative to the identity matrix for convenience.
	Matrix matrix.Matrix

	// Group specifies transparency group attributes (PDF 1.4).
	// If non-nil, this form XObject is a transparency group XObject.
	Group *group.TransparencyAttributes

	// Metadata is an optional reference to metadata for this form.
	Metadata *metadata.Stream

	// PieceInfo contains private application data.
	PieceInfo *pieceinfo.PieceInfo

	// LastModified (Required if PieceInfo is present; optional otherwise) is
	// the date the form was last modified.
	LastModified time.Time

	// OptionalContent (optional) allows to control the visibility of the form.
	OptionalContent oc.Conditional

	// Measure (optional) is a measure dictionary that specifies the scale and
	// units which shall apply to the form.
	Measure measure.Measure

	// PtData (optional; PDF 2.0) contains extended geospatial point data.
	PtData *measure.PtData

	// StructParent (required if the form is a structural content item)
	// is the integer key of the form's entry in the structural parent tree.
	StructParent optional.UInt

	// TODO(voss): StructParents

	// AssociatedFiles (optional; PDF 2.0) is an array of files associated with
	// the form XObject. The relationship that the associated files have to the
	// XObject is supplied by the Specification.AFRelationship field.
	//
	// This corresponds to the AF entry in the form XObject dictionary.
	AssociatedFiles []*file.Specification
}

// Subtype returns "Form".
// This implements the [graphics.XObject] interface.
func (f *Form) Subtype() pdf.Name {
	return "Form"
}

// Embed implements the [pdf.Embedder] interface for form XObjects.
func (f *Form) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := f.validate(); err != nil {
		return nil, err
	}

	// embed resources
	resObj, err := f.Res.Embed(rm)
	if err != nil {
		return nil, err
	}

	ref := rm.Alloc()

	dict := pdf.Dict{
		"Subtype":   pdf.Name("Form"),
		"BBox":      &f.BBox,
		"Resources": resObj,
	}
	if rm.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("XObject")
	}
	if f.Matrix != matrix.Identity && f.Matrix != matrix.Zero {
		dict["Matrix"] = toPDF(f.Matrix[:])
	}
	if f.Group != nil {
		if err := pdf.CheckVersion(rm.Out(), "transparency group XObject", pdf.V1_4); err != nil {
			return nil, err
		}
		groupObj, err := rm.Embed(f.Group)
		if err != nil {
			return nil, err
		}
		dict["Group"] = groupObj
	}
	if f.Metadata != nil {
		rmEmbedded, err := rm.Embed(f.Metadata)
		if err != nil {
			return nil, err
		}
		dict["Metadata"] = rmEmbedded
	}
	if f.PieceInfo != nil {
		if f.LastModified.IsZero() {
			return nil, errors.New("missing LastModified")
		}
		pieceInfoObj, err := rm.Embed(f.PieceInfo)
		if err != nil {
			return nil, err
		}
		if pieceInfoObj != nil {
			dict["PieceInfo"] = pieceInfoObj
		}
	}
	if !f.LastModified.IsZero() {
		dict["LastModified"] = pdf.Date(f.LastModified)
	}

	if f.OptionalContent != nil {
		if err := pdf.CheckVersion(rm.Out(), "form XObject OC entry", pdf.V1_5); err != nil {
			return nil, err
		}
		embedded, err := rm.Embed(f.OptionalContent)
		if err != nil {
			return nil, err
		}
		dict["OC"] = embedded
	}

	if f.Measure != nil {
		if err := pdf.CheckVersion(rm.Out(), "form XObject Measure entry", pdf.V2_0); err != nil {
			return nil, err
		}
		embedded, err := rm.Embed(f.Measure)
		if err != nil {
			return nil, err
		}
		dict["Measure"] = embedded
	}

	// PtData (optional; PDF 2.0)
	if f.PtData != nil {
		if err := pdf.CheckVersion(rm.Out(), "form XObject PtData entry", pdf.V2_0); err != nil {
			return nil, err
		}
		embedded, err := rm.Embed(f.PtData)
		if err != nil {
			return nil, err
		}
		dict["PtData"] = embedded
	}

	if key, ok := f.StructParent.Get(); ok {
		if err := pdf.CheckVersion(rm.Out(), "form XObject StructParent entry", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["StructParent"] = pdf.Integer(key)
	}

	if f.AssociatedFiles != nil {
		if err := pdf.CheckVersion(rm.Out(), "form XObject AF entry", pdf.V2_0); err != nil {
			return nil, err
		}

		// Validate each file specification can be used as associated file
		version := pdf.GetVersion(rm.Out())
		for i, spec := range f.AssociatedFiles {
			if spec == nil {
				continue
			}
			if err := spec.CanBeAF(version); err != nil {
				return nil, fmt.Errorf("AssociatedFiles[%d]: %w", i, err)
			}
		}

		// Embed the file specifications
		var afArray pdf.Array
		for _, spec := range f.AssociatedFiles {
			if spec != nil {
				embedded, err := rm.Embed(spec)
				if err != nil {
					return nil, err
				}
				afArray = append(afArray, embedded)
			}
		}
		dict["AF"] = afArray
	}

	stm, err := rm.Out().OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return nil, err
	}

	ct := content.Form
	if f.Group != nil {
		ct = content.TransparencyGroup
	}
	if f.Content != nil {
		err = content.Write(stm, f.Content, pdf.GetVersion(rm.Out()), ct, f.Res)
		if err != nil {
			return nil, err
		}
	}

	err = stm.Close()
	if err != nil {
		return nil, err
	}

	return ref, nil
}

func (f *Form) validate() error {
	if f.BBox.IsZero() {
		return errors.New("missing BBox")
	}
	if f.Res == nil {
		return errors.New("missing resources")
	}
	return nil
}

// Equal compares two forms for value equality.
//
// TODO(voss): at the moment, Metadata, PieceInfo, OptionalContent, Measure,
// PtData, and AssociatedFiles are ignored in the comparison.  Implement and
// use proper equality checks for these types.
func (f *Form) Equal(other *Form) bool {
	if f == nil || other == nil || f == other {
		return f == other
	}
	if !content.StreamsEqual(f.Content, other.Content) {
		return false
	}
	if !f.Res.Equal(other.Res) {
		return false
	}
	if f.BBox != other.BBox {
		return false
	}
	if f.Matrix != other.Matrix {
		return false
	}
	if !f.Group.Equal(other.Group) {
		return false
	}

	if !f.LastModified.Equal(other.LastModified) {
		return false
	}

	if !f.StructParent.Equal(other.StructParent) {
		return false
	}
	return true
}

func toPDF(x []float64) pdf.Array {
	res := make(pdf.Array, len(x))
	for i, xi := range x {
		res[i] = pdf.Number(xi)
	}
	return res
}
