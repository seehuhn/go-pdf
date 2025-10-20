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
	"fmt"
	"io"
	"time"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/measure"
	"seehuhn.de/go/pdf/metadata"
	"seehuhn.de/go/pdf/oc"
	"seehuhn.de/go/pdf/pieceinfo"
	"seehuhn.de/go/pdf/structure"
)

// PDF 2.0 sections: 8.10

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
	StructParent structure.Key

	// TODO(voss): StructParents

	// TODO(voss): AF
}

// Subtype returns "Form".
// This implements the [graphics.XObject] interface.
func (f *Form) Subtype() pdf.Name {
	return "Form"
}

// Embed implements the pdf.Embedder interface for form XObjects.
func (f *Form) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {

	err := f.validate()
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	contents := graphics.NewWriter(buf, rm.GetRM())
	contents.State.Set = 0 // make sure the XObject is independent of the current graphics state
	err = f.Draw(contents)
	if err != nil {
		return nil, err
	}
	if contents.Err != nil {
		return nil, contents.Err
	}

	ref := rm.Alloc()

	dict := pdf.Dict{
		"Subtype": pdf.Name("Form"),
		"BBox":    &f.BBox,
	}
	if rm.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("XObject")
	}
	if f.Matrix != matrix.Identity && f.Matrix != matrix.Zero {
		dict["Matrix"] = toPDF(f.Matrix[:])
	}
	if contents.Resources != nil {
		dict["Resources"] = pdf.AsDict(contents.Resources)
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

	var filters []pdf.Filter
	if !rm.Out().GetOptions().HasAny(pdf.OptPretty) {
		filters = append(filters, &pdf.FilterCompress{})
	}
	stm, err := rm.Out().OpenStream(ref, dict, filters...)
	if err != nil {
		return nil, err
	}
	_, err = stm.Write(buf.Bytes())
	if err != nil {
		return nil, err
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
	return nil
}

// Extract extracts a form XObject from a PDF file.
func Extract(x *pdf.Extractor, obj pdf.Object) (*Form, error) {
	stream, err := x.GetStream(obj)
	if err != nil {
		return nil, err
	} else if stream == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing form XObject"),
		}
	}
	dict := stream.Dict

	subtypeName, _ := x.GetName(dict["Subtype"])
	if subtypeName != "Form" {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("invalid Subtype for form XObject"),
		}
	}

	// Read required BBox
	bbox, err := pdf.GetRectangle(x.R, dict["BBox"])
	if err != nil {
		return nil, fmt.Errorf("failed to read BBox: %w", err)
	} else if bbox == nil || bbox.IsZero() {
		return nil, pdf.Error("missing BBox")
	}

	form := &Form{
		BBox: *bbox,
	}

	form.Matrix, err = pdf.GetMatrix(x.R, dict["Matrix"])
	if err != nil {
		form.Matrix = matrix.Identity
	}

	form.Metadata, _ = metadata.Extract(x.R, dict["Metadata"])

	// Read optional PieceInfo
	if pieceInfoObj, ok := dict["PieceInfo"]; ok {
		var err error
		form.PieceInfo, err = pieceinfo.Extract(x.R, pieceInfoObj)
		if err != nil {
			return nil, fmt.Errorf("failed to extract PieceInfo: %w", err)
		}
	}

	// LastModified (optional)
	if lastModDate, err := pdf.Optional(pdf.GetDate(x.R, dict["LastModified"])); err != nil {
		return nil, err
	} else {
		form.LastModified = time.Time(lastModDate)
	}

	// OC (optional)
	if oc, err := pdf.ExtractorGetOptional(x, dict["OC"], oc.ExtractConditional); err != nil {
		return nil, err
	} else {
		form.OptionalContent = oc
	}

	// Measure (optional)
	if m, err := pdf.Optional(measure.Extract(x.R, dict["Measure"])); err != nil {
		return nil, err
	} else {
		form.Measure = m
	}

	// PtData (optional)
	if ptData, err := pdf.Optional(measure.ExtractPtData(x.R, dict["PtData"])); err != nil {
		return nil, err
	} else {
		form.PtData = ptData
	}

	// Extract StructParent
	if keyObj := dict["StructParent"]; keyObj != nil {
		if key, err := pdf.Optional(x.GetInteger(dict["StructParent"])); err != nil {
			return nil, err
		} else {
			form.StructParent.Set(key)
		}
	}

	// Create Draw function as closure
	form.Draw = func(w *graphics.Writer) error {
		copier := pdf.NewCopier(w.RM.Out, x.R)

		// Handle resources
		origResources, err := x.GetDict(dict["Resources"])
		if err != nil {
			return err
		}
		if origResources != nil {
			resourceObj, err := copier.Copy(origResources)
			if err != nil {
				return err
			}
			w.Resources, err = pdf.ExtractResources(nil, resourceObj)
			if err != nil {
				return err
			}
		}

		// Handle stream content
		stm, err := pdf.DecodeStream(x.R, stream, 0)
		if err != nil {
			return err
		}
		_, err = io.Copy(w.Content, stm)
		if err != nil {
			return err
		}

		return nil
	}

	return form, nil
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
