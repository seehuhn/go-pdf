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
	"io"
	"time"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/group"
	"seehuhn.de/go/pdf/graphics/opi"
	"seehuhn.de/go/pdf/graphics/printermark"
	"seehuhn.de/go/pdf/graphics/reference"
	"seehuhn.de/go/pdf/graphics/trapnet"
	"seehuhn.de/go/pdf/measure"

	"seehuhn.de/go/pdf/oc"
	"seehuhn.de/go/pdf/optional"
	"seehuhn.de/go/pdf/pieceinfo"
)

// PDF 2.0 sections: 8.10

// Form represents a PDF form XObject that can contain reusable graphics content.
//
// To extract a form XObject from a PDF file, use
// [seehuhn.de/go/pdf/graphics/extract.Form].
type Form struct {
	// Content is the content stream that draws the form.
	Content content.Stream

	// Res contains the resources used by the content stream.
	// Required in PDF 2.0; optional in PDF 1.7 and earlier, where a nil
	// value means the form inherits resources from the surrounding page.
	Res *content.Resources

	// BBox is the form's bounding box in form coordinate space.
	BBox pdf.Rectangle

	// Matrix transforms form coordinates to user space coordinates.
	//
	// When writing forms to a PDF file, the zero matrix can be used as an
	// alternative to the identity matrix for convenience.
	Matrix matrix.Matrix

	// Group (optional) specifies transparency group attributes.
	// If non-nil, this form XObject is a transparency group XObject.
	Group *group.TransparencyAttributes

	// Ref (optional) makes this form a reference XObject: a proxy for
	// a single page imported from another PDF file.
	Ref *reference.Dict

	// PrinterMark (optional) holds the entries specific to a printer's
	// mark.  They take effect where the form is used as the normal appearance
	// of a printer's mark annotation, and are ignored elsewhere.
	PrinterMark *printermark.Attributes

	// TrapNet (optional) holds the entries specific to a trap network.
	// They take effect where the form is used as the normal appearance of a
	// trap network annotation, and are ignored elsewhere.
	TrapNet *trapnet.Attributes

	// OPI (optional) is an Open Prepress Interface dictionary
	// describing a low-resolution proxy for a high-resolution image.
	// OPI is deprecated in PDF 2.0.
	OPI opi.Dict

	// Metadata is an optional reference to metadata for this form.
	Metadata *pdf.MetadataStream

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

	// Name is the PDF resource-dictionary key under which this form
	// XObject is referenced in content streams.  If non-empty, the builder
	// uses this value as the /XObject subdictionary key; the spec requires
	// the two to match (PDF 2.0 Table 93).  Required in PDF 1.0; optional
	// in PDF 1.1–1.7; deprecated (forbidden by this library's writer) in
	// PDF 2.0.
	Name pdf.Name
}

// Subtype returns "Form".
// This implements the [graphics.XObject] interface.
func (f *Form) Subtype() pdf.Name {
	return "Form"
}

// ResourceName returns the preferred resource-dictionary key for this form.
// See [graphics.XObject.ResourceName].
func (f *Form) ResourceName() pdf.Name {
	return f.Name
}

// Embed implements the [pdf.Embedder] interface for form XObjects.
func (f *Form) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := f.validate(); err != nil {
		return nil, err
	}

	v := pdf.GetVersion(e.Out())
	if f.Res == nil && v >= pdf.V2_0 {
		return nil, errors.New("missing resources")
	}

	ref := e.Alloc()

	dict := pdf.Dict{
		"Subtype": pdf.Name("Form"),
		"BBox":    &f.BBox,
	}
	if f.Res != nil {
		resObj, err := e.Embed(f.Res)
		if err != nil {
			return nil, err
		}
		dict["Resources"] = resObj
	}
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("XObject")
	}
	if v == pdf.V1_0 {
		if f.Name == "" {
			return nil, errors.New("missing form XObject name")
		}
	} else if v >= pdf.V2_0 {
		if f.Name != "" {
			return nil, errors.New("unexpected form XObject name")
		}
	}
	if f.Name != "" {
		dict["Name"] = f.Name
	}
	if f.Matrix != matrix.Identity && f.Matrix != matrix.Zero {
		dict["Matrix"] = toPDF(f.Matrix[:])
	}
	if f.Group != nil {
		if err := pdf.CheckVersion(e.Out(), "transparency group XObject", pdf.V1_4); err != nil {
			return nil, err
		}
		groupObj, err := e.Embed(f.Group)
		if err != nil {
			return nil, err
		}
		dict["Group"] = groupObj
	}
	if f.Ref != nil {
		refObj, err := e.Embed(f.Ref)
		if err != nil {
			return nil, err
		}
		dict["Ref"] = refObj
	}
	if f.PrinterMark != nil {
		if err := f.PrinterMark.FillDict(e, dict); err != nil {
			return nil, err
		}
	}
	if f.TrapNet != nil {
		if err := f.TrapNet.FillDict(e, dict); err != nil {
			return nil, err
		}
	}
	if f.OPI != nil {
		opiObj, err := e.Embed(f.OPI)
		if err != nil {
			return nil, err
		}
		dict["OPI"] = opiObj
	}
	if f.Metadata != nil {
		embedded, err := e.Embed(f.Metadata)
		if err != nil {
			return nil, err
		}
		dict["Metadata"] = embedded
	}
	if f.PieceInfo != nil {
		if f.LastModified.IsZero() {
			return nil, errors.New("missing LastModified")
		}
		pieceInfoObj, err := e.Embed(f.PieceInfo)
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
		if err := pdf.CheckVersion(e.Out(), "form XObject OC entry", pdf.V1_5); err != nil {
			return nil, err
		}
		embedded, err := e.Embed(f.OptionalContent)
		if err != nil {
			return nil, err
		}
		dict["OC"] = embedded
	}

	if f.Measure != nil {
		if err := pdf.CheckVersion(e.Out(), "form XObject Measure entry", pdf.V2_0); err != nil {
			return nil, err
		}
		embedded, err := e.Embed(f.Measure)
		if err != nil {
			return nil, err
		}
		dict["Measure"] = embedded
	}

	// PtData (optional; PDF 2.0)
	if f.PtData != nil {
		if err := pdf.CheckVersion(e.Out(), "form XObject PtData entry", pdf.V2_0); err != nil {
			return nil, err
		}
		embedded, err := e.Embed(f.PtData)
		if err != nil {
			return nil, err
		}
		dict["PtData"] = embedded
	}

	if key, ok := f.StructParent.Get(); ok {
		if err := pdf.CheckVersion(e.Out(), "form XObject StructParent entry", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["StructParent"] = pdf.Integer(key)
	}

	if f.AssociatedFiles != nil {
		if err := pdf.CheckVersion(e.Out(), "form XObject AF entry", pdf.V2_0); err != nil {
			return nil, err
		}

		// Validate each file specification can be used as associated file
		version := pdf.GetVersion(e.Out())
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
				embedded, err := e.Embed(spec)
				if err != nil {
					return nil, err
				}
				afArray = append(afArray, embedded)
			}
		}
		dict["AF"] = afArray
	}

	stm, err := e.Out().OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return nil, err
	}

	if f.Content != nil {
		rc, err := f.Content.RawBytes()
		if err != nil {
			stm.Close()
			return nil, err
		}
		_, err = io.Copy(stm, rc)
		rc.Close()
		if err != nil {
			stm.Close()
			return nil, err
		}
	}

	if err := stm.Close(); err != nil {
		return nil, err
	}

	return ref, nil
}

func (f *Form) validate() error {
	if f.BBox.IsZero() {
		return errors.New("missing BBox")
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
	if !f.Ref.Equal(other.Ref) {
		return false
	}
	if !f.PrinterMark.Equal(other.PrinterMark) {
		return false
	}
	if !f.TrapNet.Equal(other.TrapNet) {
		return false
	}
	if (f.OPI == nil) != (other.OPI == nil) {
		return false
	}
	if f.OPI != nil && !f.OPI.Equal(other.OPI) {
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
