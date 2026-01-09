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
	"time"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/group"
	"seehuhn.de/go/pdf/measure"
	"seehuhn.de/go/pdf/metadata"
	"seehuhn.de/go/pdf/oc"
	"seehuhn.de/go/pdf/pieceinfo"
)

// Form extracts a form XObject from a PDF file.
func Form(x *pdf.Extractor, obj pdf.Object) (*form.Form, error) {
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

	// read required BBox
	bbox, err := pdf.GetRectangle(x.R, dict["BBox"])
	if err != nil {
		return nil, fmt.Errorf("failed to read BBox: %w", err)
	} else if bbox == nil || bbox.IsZero() {
		return nil, pdf.Error("missing BBox")
	}

	f := &form.Form{
		BBox: *bbox,
	}

	f.Matrix, err = pdf.GetMatrix(x.R, dict["Matrix"])
	if err != nil {
		f.Matrix = matrix.Identity
	}

	// Group (optional)
	if g, err := pdf.ExtractorGetOptional(x, dict["Group"], group.ExtractTransparencyAttributes); err != nil {
		return nil, err
	} else {
		f.Group = g
	}

	// Metadata (optional)
	if meta, err := pdf.Optional(metadata.Extract(x.R, dict["Metadata"])); err != nil {
		return nil, err
	} else {
		f.Metadata = meta
	}

	// PieceInfo (optional)
	if piece, err := pdf.Optional(pieceinfo.Extract(x, dict["PieceInfo"])); err != nil {
		return nil, err
	} else {
		f.PieceInfo = piece
	}

	// LastModified (optional)
	if lastModDate, err := pdf.Optional(pdf.GetDate(x.R, dict["LastModified"])); err != nil {
		return nil, err
	} else {
		f.LastModified = time.Time(lastModDate)
	}

	// OC (optional)
	if ocObj, err := pdf.ExtractorGetOptional(x, dict["OC"], oc.ExtractConditional); err != nil {
		return nil, err
	} else {
		f.OptionalContent = ocObj
	}

	// Measure (optional)
	if m, err := pdf.Optional(measure.Extract(x, dict["Measure"])); err != nil {
		return nil, err
	} else {
		f.Measure = m
	}

	// PtData (optional)
	if ptData, err := pdf.Optional(measure.ExtractPtData(x, dict["PtData"])); err != nil {
		return nil, err
	} else {
		f.PtData = ptData
	}

	// extract StructParent
	if keyObj := dict["StructParent"]; keyObj != nil {
		if key, err := pdf.Optional(x.GetInteger(dict["StructParent"])); err != nil {
			return nil, err
		} else {
			f.StructParent.Set(key)
		}
	}

	// extract AssociatedFiles (AF)
	if afArray, err := pdf.Optional(x.GetArray(dict["AF"])); err != nil {
		return nil, err
	} else if afArray != nil {
		f.AssociatedFiles = make([]*file.Specification, 0, len(afArray))
		for _, afObj := range afArray {
			if spec, err := pdf.ExtractorGetOptional(x, afObj, file.ExtractSpecification); err != nil {
				return nil, err
			} else if spec != nil {
				f.AssociatedFiles = append(f.AssociatedFiles, spec)
			}
		}
	}

	// extract resources
	f.Res = &content.Resources{}
	if resObj := dict["Resources"]; resObj != nil {
		res, err := Resources(x, resObj)
		if err != nil {
			return nil, err
		}
		if res != nil {
			f.Res = res
		}
	}

	// read content stream
	stm, err := pdf.DecodeStream(x.R, stream, 0)
	if err != nil {
		return nil, err
	}

	f.Content, err = content.ReadStream(stm, pdf.GetVersion(x.R), content.Form)
	closeErr := stm.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}

	return f, nil
}
