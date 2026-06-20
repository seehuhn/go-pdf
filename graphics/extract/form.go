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
	"fmt"
	"io"
	"math"
	"time"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/group"
	"seehuhn.de/go/pdf/graphics/opi"
	"seehuhn.de/go/pdf/graphics/reference"
	"seehuhn.de/go/pdf/measure"

	"seehuhn.de/go/pdf/oc"
	"seehuhn.de/go/pdf/pieceinfo"
)

// Form extracts a form XObject from a PDF file.
func Form(c pdf.Cursor, obj pdf.Object, _ bool) (*form.Form, error) {
	stream, err := c.Stream(obj)
	if err != nil {
		return nil, err
	} else if stream == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing form XObject"),
		}
	}
	dict := stream.Dict

	subtypeName, _ := c.Name(dict["Subtype"])
	if subtypeName != "Form" {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("invalid Subtype for form XObject"),
		}
	}

	// read required BBox
	bbox, err := c.Rectangle(dict["BBox"])
	if err != nil {
		return nil, fmt.Errorf("failed to read BBox: %w", err)
	} else if bbox == nil || bbox.IsZero() {
		return nil, pdf.Error("missing BBox")
	}

	f := &form.Form{
		BBox: *bbox,
	}

	f.Name, _ = c.Name(dict["Name"])

	f.Matrix, err = c.Matrix(dict["Matrix"])
	if err != nil {
		f.Matrix = matrix.Identity
	}

	// Group (optional)
	if g, err := pdf.DecodeOptional(c, dict["Group"], group.ExtractTransparencyAttributes); err != nil {
		return nil, err
	} else {
		f.Group = g
	}

	// Ref (optional)
	if r, err := pdf.DecodeOptional(c, dict["Ref"], reference.ExtractDict); err != nil {
		return nil, err
	} else {
		f.Ref = r
	}

	// OPI (optional)
	if o, err := pdf.DecodeOptional(c, dict["OPI"], opi.Extract); err != nil {
		return nil, err
	} else {
		f.OPI = o
	}

	// Metadata (optional)
	if meta, err := pdf.DecodeOptional(c, dict["Metadata"], pdf.ExtractMetadataStream); err != nil {
		return nil, err
	} else {
		f.Metadata = meta
	}

	// PieceInfo (optional)
	if piece, err := pdf.DecodeOptional(c, dict["PieceInfo"], pieceinfo.Extract); err != nil {
		return nil, err
	} else {
		f.PieceInfo = piece
	}

	// LastModified (optional)
	if lastModDate, err := pdf.Optional(c.Date(dict["LastModified"])); err != nil {
		return nil, err
	} else {
		f.LastModified = time.Time(lastModDate)
	}

	// OC (optional)
	if ocObj, err := pdf.DecodeOptional(c, dict["OC"], oc.ExtractConditional); err != nil {
		return nil, err
	} else {
		f.OptionalContent = ocObj
	}

	// Measure (optional)
	if m, err := pdf.DecodeOptional(c, dict["Measure"], measure.Extract); err != nil {
		return nil, err
	} else {
		f.Measure = m
	}

	// PtData (optional)
	if ptData, err := pdf.DecodeOptional(c, dict["PtData"], measure.ExtractPtData); err != nil {
		return nil, err
	} else {
		f.PtData = ptData
	}

	// extract StructParent
	if keyObj := dict["StructParent"]; keyObj != nil {
		if key, err := pdf.Optional(c.Integer(dict["StructParent"])); err != nil {
			return nil, err
		} else if key >= 0 && uint64(key) <= math.MaxUint {
			f.StructParent.Set(uint(key))
		}
	}

	// extract AssociatedFiles (AF)
	if afArray, err := pdf.Optional(c.Array(dict["AF"])); err != nil {
		return nil, err
	} else if afArray != nil {
		f.AssociatedFiles = make([]*file.Specification, 0, len(afArray))
		for _, afObj := range afArray {
			if spec, err := pdf.DecodeOptional(c, afObj, file.ExtractSpecification); err != nil {
				return nil, err
			} else if spec != nil {
				f.AssociatedFiles = append(f.AssociatedFiles, spec)
			}
		}
	}

	// extract resources
	version := c.Version()
	if resObj := dict["Resources"]; resObj != nil {
		res, err := pdf.Decode(c, resObj, Resources)
		if err != nil {
			return nil, err
		}
		f.Res = res
	} else if version >= pdf.V2_0 {
		// PDF 2.0 requires a Resources entry; normalise the malformed
		// input to an empty Resources dict.
		f.Res = &content.Resources{}
	}
	// f.Res remains nil for pre-2.0 forms without a Resources entry; the
	// renderer falls back to the surrounding page's resources.

	// store a reader factory closure so each iteration re-opens the PDF stream
	stm := stream // capture for closure
	f.Content = content.NewScanner(
		func() (io.ReadCloser, error) {
			return c.StreamReader(stm)
		},
	)

	repairForm(f, c.Getter())

	return f, nil
}

// repairForm fixes invalid data in a form XObject after extraction.
func repairForm(f *form.Form, r pdf.Getter) {
	if v := pdf.GetVersion(r); v == pdf.V1_0 {
		if f.Name == "" {
			f.Name = "Form"
		}
	} else if v >= pdf.V2_0 {
		f.Name = ""
	}
}
