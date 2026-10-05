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
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/opaque"
)

// Form reads an interactive form dictionary from a PDF file. The obj argument
// should be the value of the AcroForm entry in the document catalog. It returns
// nil if obj is nil.
//
// Always invoke this via [pdf.Decode] so that the form dictionary's
// reference is resolved and cached.
func Form(c pdf.Cursor, obj pdf.Object, _ bool) (*acroform.InteractiveForm, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, nil
	}

	form := &acroform.InteractiveForm{}

	// the document-wide /DA and /Q defaults seed field-attribute inheritance,
	// which the decoder flattens away; the values are not kept on the form
	da, _ := pdf.Optional(c.String(dict["DA"]))
	var q pdf.TextAlign
	if v, err := pdf.Optional(c.Integer(dict["Q"])); err == nil && v >= 0 && v <= 2 {
		q = pdf.TextAlign(v)
	}
	rootCtx := inherited{da: string(da), q: q}

	// Fields (required)
	d := newFieldTreeDecoder()
	if fields, err := d.decodeRoots(c, dict["Fields"], rootCtx); err != nil {
		return nil, err
	} else {
		form.Fields = fields
	}

	// NeedAppearances (optional)
	if na, err := pdf.Optional(c.Boolean(dict["NeedAppearances"])); err != nil {
		return nil, err
	} else {
		form.NeedAppearances = bool(na)
	}

	// SigFlags (optional)
	if sf, err := pdf.Optional(c.Integer(dict["SigFlags"])); err != nil {
		return nil, err
	} else {
		// bits not defined by the spec are cleared
		form.SigFlags = acroform.SignatureFlags(sf) & (acroform.SignaturesExist | acroform.AppendOnly)
	}

	// CO (optional); each entry resolves to a field already in the tree, so the
	// same field value is shared with the Fields tree
	if co, err := decodeCalculationOrder(c, dict["CO"], d); err != nil {
		return nil, err
	} else {
		form.CalculationOrder = co
	}

	// DR (optional)
	if drObj := dict["DR"]; drObj != nil {
		if dr, err := pdf.Optional(pdf.Decode(c, drObj, extract.Resources)); err != nil {
			return nil, err
		} else {
			form.DefaultResources = dr
		}
	}

	// XFA (optional); either a stream or an array of packets
	if xfa, err := pdf.Optional(c.Resolve(dict["XFA"])); err != nil {
		return nil, err
	} else {
		switch xfa.(type) {
		case *pdf.Stream:
			// streams are indirect: keep the reference for the copier
			form.XFA = opaque.Extract(c.Extractor(), dict["XFA"])
		case pdf.Array:
			// keep the array itself, so that the writer sees its type
			form.XFA = opaque.Extract(c.Extractor(), xfa)
		}
	}

	return form, nil
}

// decodeCalculationOrder decodes the /CO array into the fields it names,
// resolving each reference against the fields already decoded from the tree and
// dropping any that names a field not in the tree.
func decodeCalculationOrder(c pdf.Cursor, obj pdf.Object, d *fieldTreeDecoder) ([]acroform.Field, error) {
	arr, err := pdf.Optional(c.Array(obj))
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

// copyValue sets the value and default value of dst to those of src.  It
// returns false, leaving dst unchanged, if the fields differ in type, or if
// dst is a password field and src has a value: a password field never stores
// a value, so the two cannot be made to agree.
func copyValue(dst, src acroform.Field) bool {
	switch dst := dst.(type) {
	case *acroform.TextField:
		src, ok := src.(*acroform.TextField)
		if !ok || dst.Flags&acroform.FieldPassword != 0 && src.V != nil {
			return false
		}
		dst.V, dst.DV = cloneText(src.V), cloneText(src.DV)
	case *acroform.ButtonField:
		src, ok := src.(*acroform.ButtonField)
		if !ok {
			return false
		}
		dst.V, dst.DV = src.V, src.DV
	case *acroform.ChoiceField:
		src, ok := src.(*acroform.ChoiceField)
		if !ok {
			return false
		}
		dst.V, dst.DV = slices.Clone(src.V), slices.Clone(src.DV)
	case *acroform.SignatureField:
		src, ok := src.(*acroform.SignatureField)
		if !ok {
			return false
		}
		dst.V, dst.DV = src.V, src.DV
	default:
		return false
	}
	return true
}

// cloneText returns a copy of a text value.
func cloneText(v *pdf.StringOrStream) *pdf.StringOrStream {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}
