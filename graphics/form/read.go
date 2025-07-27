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
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/metadata"
	"seehuhn.de/go/pdf/pdfcopy"
)

// Extract extracts a form XObject from a PDF file.
func Extract(r pdf.Getter, obj pdf.Object) (*Form, error) {
	stream, err := pdf.GetStream(r, obj)
	if err != nil {
		return nil, err
	} else if stream == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing form XObject"),
		}
	}
	dict := stream.Dict

	subtypeName, _ := pdf.GetName(r, dict["Subtype"])
	if subtypeName != "Form" {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("invalid Subtype for form XObject"),
		}
	}

	// Read required BBox
	bbox, err := pdf.GetRectangle(r, dict["BBox"])
	if err != nil {
		return nil, fmt.Errorf("failed to read BBox: %w", err)
	} else if bbox == nil || bbox.IsZero() {
		return nil, pdf.Error("missing BBox")
	}

	form := &Form{
		BBox: *bbox,
	}

	form.Matrix, err = pdf.GetMatrix(r, dict["Matrix"])
	if err != nil {
		form.Matrix = matrix.Identity
	}

	form.Metadata, _ = metadata.Extract(r, dict["Metadata"])

	// Read optional PieceInfo
	if pieceInfoObj, ok := dict["PieceInfo"]; ok {
		form.PieceInfo = pieceInfoObj
	}

	// Read optional LastModified
	lastModDate, _ := pdf.GetDate(r, dict["LastModified"])
	if !lastModDate.IsZero() {
		form.LastModified = time.Time(lastModDate)
	}

	// Create Draw function as closure
	form.Draw = func(w *graphics.Writer) error {
		copier := pdfcopy.NewCopier(w.RM.Out, r)

		// Handle resources
		origResources, err := pdf.GetDict(r, dict["Resources"])
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
		stm, err := pdf.DecodeStream(r, stream, 0)
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
