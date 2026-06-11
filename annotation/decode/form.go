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

// PDF 2.0 sections: 12.7.3

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/graphics/extract"
)

// Form reads an interactive form dictionary from a PDF file. The obj argument
// should be the value of the AcroForm entry in the document catalog. It returns
// nil if obj is nil.
//
// Always invoke this via [pdf.ExtractorGet] so that the form dictionary's
// reference is resolved and cached.
func Form(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, _ bool) (*acroform.InteractiveForm, error) {
	dict, err := x.GetDict(path, obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, nil
	}

	form := &acroform.InteractiveForm{}

	// the document-wide /DA and /Q defaults seed field-attribute inheritance,
	// which the decoder flattens away; the values are not kept on the form
	da, _ := pdf.Optional(x.GetString(path, dict["DA"]))
	var q pdf.TextAlign
	if v, err := pdf.Optional(x.GetInteger(path, dict["Q"])); err == nil && v >= 0 && v <= 2 {
		q = pdf.TextAlign(v)
	}
	rootCtx := inherited{da: string(da), q: q}

	// Fields (required)
	d := newFieldTreeDecoder()
	if fields, err := d.decodeRoots(x, path, dict["Fields"], rootCtx); err != nil {
		return nil, err
	} else {
		form.Fields = fields
	}

	// NeedAppearances (optional)
	if na, err := pdf.Optional(x.GetBoolean(path, dict["NeedAppearances"])); err != nil {
		return nil, err
	} else {
		form.NeedAppearances = bool(na)
	}

	// SigFlags (optional)
	if sf, err := pdf.Optional(x.GetInteger(path, dict["SigFlags"])); err != nil {
		return nil, err
	} else {
		form.SigFlags = acroform.SignatureFlags(sf)
	}

	// CO (optional); each entry resolves to a field already in the tree, so the
	// same field value is shared with the Fields tree
	if co, err := decodeCalculationOrder(x, path, dict["CO"], d); err != nil {
		return nil, err
	} else {
		form.CalculationOrder = co
	}

	// DR (optional)
	if drObj := dict["DR"]; drObj != nil {
		if dr, err := pdf.Optional(pdf.ExtractorGet(x, path, drObj, extract.Resources)); err != nil {
			return nil, err
		} else {
			form.DefaultResources = dr
		}
	}

	// XFA (optional)
	form.XFA = dict["XFA"]

	return form, nil
}

// decodeCalculationOrder decodes the /CO array into the fields it names,
// resolving each reference against the fields already decoded from the tree and
// dropping any that names a field not in the tree.
func decodeCalculationOrder(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, d *fieldTreeDecoder) ([]acroform.Field, error) {
	arr, err := pdf.Optional(x.GetArray(path, obj))
	if err != nil {
		return nil, err
	}
	var co []acroform.Field
	for _, el := range arr {
		ref, ok := el.(pdf.Reference)
		if !ok {
			continue
		}
		if fld := d.byRef[ref]; fld != nil {
			co = append(co, fld)
		}
	}
	return co, nil
}
