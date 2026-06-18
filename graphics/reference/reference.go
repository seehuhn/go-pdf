// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

// Package reference implements the reference dictionary of a reference XObject.
//
// A reference XObject is a form XObject whose form dictionary contains a Ref
// entry. It serves as a proxy for a single page imported from another PDF
// file.
package reference

import (
	"reflect"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
)

// PDF 2.0 sections: 8.10.4

// Dict is the reference dictionary of a reference XObject. It identifies a
// single page in a target PDF file that the form XObject acts as a proxy for.
type Dict struct {
	// F is the target PDF file containing the page to import.
	F *file.Specification

	// PageIndex selects the target page by its zero-based index.
	// It is used when PageLabel is empty.
	PageIndex int

	// PageLabel (optional) selects the target page by its page label.
	// A non-empty value takes precedence over PageIndex.
	PageLabel string

	// ID (optional) is the file identifier of the target file: a slice of
	// two byte strings, as in the target file's trailer.
	ID []pdf.String

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// Equal reports whether two reference dictionaries are equal.
func (d *Dict) Equal(other *Dict) bool {
	if d == nil || other == nil {
		return d == other
	}
	if !reflect.DeepEqual(d.F, other.F) {
		return false
	}
	if d.PageIndex != other.PageIndex ||
		d.PageLabel != other.PageLabel ||
		d.SingleUse != other.SingleUse {
		return false
	}
	return slices.EqualFunc(d.ID, other.ID, func(a, b pdf.String) bool {
		return slices.Equal(a, b)
	})
}

// Embed adds the reference dictionary to a PDF file.
//
// This implements the [pdf.Embedder] interface.
func (d *Dict) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "reference XObject", pdf.V1_4); err != nil {
		return nil, err
	}
	if d.F == nil {
		return nil, pdf.Error("reference dictionary requires F entry")
	}
	if len(d.ID) != 0 && len(d.ID) != 2 {
		return nil, pdf.Error("reference dictionary ID must have two elements")
	}

	fObj, err := rm.Embed(d.F)
	if err != nil {
		return nil, err
	}
	dict := pdf.Dict{"F": fObj}

	if d.PageLabel != "" {
		dict["Page"] = pdf.TextString(d.PageLabel)
	} else {
		dict["Page"] = pdf.Integer(d.PageIndex)
	}

	if len(d.ID) == 2 {
		dict["ID"] = pdf.Array{d.ID[0], d.ID[1]}
	}

	if d.SingleUse {
		return dict, nil
	}
	ref := rm.Alloc()
	if err := rm.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

// ExtractDict reads a reference dictionary from a PDF file.
func ExtractDict(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, isDirect bool) (*Dict, error) {
	dict, err := x.GetDict(path, obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing reference dictionary")
	}

	d := &Dict{SingleUse: isDirect}

	fs, err := pdf.ExtractorGet(x, path, dict["F"], file.ExtractSpecification)
	if err != nil {
		return nil, err
	} else if fs == nil {
		return nil, pdf.Error("reference dictionary requires F entry")
	}
	d.F = fs

	// Page is an integer page index or a text-string page label.
	pageObj, err := pdf.Resolve(x.R, dict["Page"])
	if err != nil {
		return nil, err
	}
	switch p := pageObj.(type) {
	case pdf.Integer:
		if p >= 0 && int64(p) <= maxInt {
			d.PageIndex = int(p)
		}
	case pdf.String:
		d.PageLabel = string(p.AsTextString())
	}

	if idArray, err := pdf.Optional(x.GetArray(path, dict["ID"])); err != nil {
		return nil, err
	} else if len(idArray) == 2 {
		id0, err := pdf.Optional(x.GetString(path, idArray[0]))
		if err != nil {
			return nil, err
		}
		id1, err := pdf.Optional(x.GetString(path, idArray[1]))
		if err != nil {
			return nil, err
		}
		d.ID = []pdf.String{id0, id1}
	}

	return d, nil
}

const maxInt = int64(^uint(0) >> 1)
