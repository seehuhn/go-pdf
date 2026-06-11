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

// decodeFieldRefs decodes an array of field references (the /Fields or /CO entry)
// into the matching fields. The same extractor resolves both, so a reference
// shared between the two yields the same field value.
func decodeFieldRefs(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) ([]acroform.Field, error) {
	arr, err := pdf.Optional(x.GetArray(path, obj))
	if err != nil {
		return nil, err
	}
	var fields []acroform.Field
	for _, el := range arr {
		ref, ok := el.(pdf.Reference)
		if !ok {
			continue
		}
		fld, err := pdf.Optional(pdf.ExtractorGet(x, path, ref, Field))
		if err != nil {
			return nil, err
		}
		if fld != nil {
			fields = append(fields, fld)
		}
	}
	return fields, nil
}

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

	// Fields (required)
	if fields, err := decodeFieldRefs(x, path, dict["Fields"]); err != nil {
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

	// CO (optional); resolved through the same extractor, so each entry is the
	// same field value as in the Fields tree
	if co, err := decodeFieldRefs(x, path, dict["CO"]); err != nil {
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

	// DA (optional)
	if da, err := pdf.Optional(x.GetString(path, dict["DA"])); err != nil {
		return nil, err
	} else {
		form.DefaultAppearance = string(da)
	}

	// Q (optional)
	if q, err := pdf.Optional(x.GetInteger(path, dict["Q"])); err != nil {
		return nil, err
	} else if q >= 0 && q <= 2 {
		form.Align = pdf.TextAlign(q)
	}

	// XFA (optional)
	form.XFA = dict["XFA"]

	return form, nil
}
