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
	"seehuhn.de/go/pdf/graphics/printermark"
	"seehuhn.de/go/pdf/graphics/reference"
	"seehuhn.de/go/pdf/graphics/trapnet"
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
	if g, err := c.DecodeOptional(dict["Group"], group.ExtractTransparencyAttributes); err != nil {
		return nil, err
	} else {
		f.Group = g
	}

	// Ref (optional)
	if r, err := c.DecodeOptional(dict["Ref"], reference.ExtractDict); err != nil {
		return nil, err
	} else {
		f.Ref = r
	}

	// PrinterMark entries (optional)
	if pm, err := printermark.ExtractAttributes(c, dict); err != nil {
		return nil, err
	} else {
		f.PrinterMark = pm
	}

	// TrapNet entries (optional)
	if tn, err := trapnet.ExtractAttributes(c, dict); err != nil {
		return nil, err
	} else {
		f.TrapNet = tn
	}

	// OPI (optional)
	if o, err := c.DecodeOptional(dict["OPI"], opi.Extract); err != nil {
		return nil, err
	} else {
		f.OPI = o
	}

	// Metadata (optional)
	if meta, err := c.DecodeOptional(dict["Metadata"], pdf.ExtractMetadataStream); err != nil {
		return nil, err
	} else {
		f.Metadata = meta
	}

	// PieceInfo (optional)
	if piece, err := c.DecodeOptional(dict["PieceInfo"], pieceinfo.Extract); err != nil {
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
	if ocObj, err := c.DecodeOptional(dict["OC"], oc.ExtractConditional); err != nil {
		return nil, err
	} else {
		f.OptionalContent = ocObj
	}

	// Measure (optional)
	if m, err := c.DecodeOptional(dict["Measure"], measure.Extract); err != nil {
		return nil, err
	} else {
		f.Measure = m
	}

	// PtData (optional)
	if ptData, err := c.DecodeOptional(dict["PtData"], measure.ExtractPtData); err != nil {
		return nil, err
	} else {
		f.PtData = ptData
	}

	// extract StructParent
	if key, err := c.Integer(dict["StructParent"]); err == nil {
		if key >= 0 && uint64(key) <= math.MaxUint {
			f.StructParent.Set(uint(key))
		}
	} else if pdf.IsReadError(err) {
		return nil, err
	}

	// extract AssociatedFiles (AF)
	f.AssociatedFiles, err = file.ExtractAssociatedFiles(c, dict["AF"])
	if err != nil {
		return nil, err
	}

	// extract resources
	version := c.Version()
	if resObj := dict["Resources"]; resObj != nil {
		res, err := c.Decode(resObj, Resources)
		if err != nil {
			return nil, err
		}
		f.Res = res
	}
	if f.Res == nil && version >= pdf.V2_0 {
		// PDF 2.0 requires a Resources entry; normalise a missing or null
		// one to an empty Resources dict.
		f.Res = &content.Resources{FontFallback: StandardFontFallback}
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
	v := pdf.GetVersion(r)

	if v == pdf.V1_0 {
		if f.Name == "" {
			f.Name = "Form"
		}
	} else if v >= pdf.V2_0 {
		f.Name = ""
	}

	// Drop entries which cannot be written at this PDF version, so that
	// everything we read can be written back.  Each case mirrors a version
	// check on the write side, either in [form.Form.Embed] or in the Embed
	// method of the entry's own type.
	if v < pdf.V1_2 {
		f.OPI = nil
	}
	if v < pdf.V1_3 {
		f.StructParent.Clear()
		f.TrapNet = nil
	}
	if v < pdf.V1_4 {
		f.Group = nil
		f.Ref = nil
		f.PrinterMark = nil
	}
	if v < pdf.V1_5 {
		f.OptionalContent = nil
	}
	if v < pdf.V2_0 {
		f.Measure = nil
		f.PtData = nil
		f.AssociatedFiles = nil
	}
}
