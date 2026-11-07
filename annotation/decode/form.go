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
// Always invoke this via [pdf.Cursor.Decode] so that the form dictionary's
// reference is resolved and cached.  [DocumentForm] reads the document's own
// form once per extractor, also where its dictionary is not an indirect
// object.
func Form(c pdf.Cursor, obj pdf.Object, isDirect bool) (*acroform.InteractiveForm, error) {
	// an indirect form shares its values with the cached [FormIndex]; the
	// path is rewound by one step, since p.Ref is the object being decoded
	if p := c.Path(); !isDirect && p != nil {
		idx, err := pdf.CursorAt(c.Extractor(), p.Parent).DecodeExclusive(p.Ref, FormWithRefs)
		if err != nil || idx == nil {
			return nil, err
		}
		return idx.Form, nil
	}

	idx, err := FormWithRefs(c, obj, isDirect)
	if err != nil || idx == nil {
		return nil, err
	}
	return idx.Form, nil
}

// FormIndex is an interactive form together with the references of the
// dictionaries its terminal fields and their widgets were read from.
type FormIndex struct {
	// Form is the decoded interactive form.
	Form *acroform.InteractiveForm

	// FieldRefs maps each terminal field of Form to its dictionary.
	FieldRefs map[acroform.Field]pdf.Reference

	// WidgetRefs maps each widget of these fields to its annotation
	// dictionary, including widgets which are on no page.
	WidgetRefs map[acroform.Widget]pdf.Reference
}

// documentForm is the document's form index, as cached by [DocumentForm].
type documentForm struct {
	idx *FormIndex
}

// DocumentForm reads the interactive form named by the AcroForm entry of the
// document catalog, as [FormWithRefs] does.  It returns nil if the document
// has no form.
//
// The form is read once per extractor, whether its dictionary is an indirect
// object or stored directly in the catalog.  [Annotation] links widgets to
// the fields of this form, and for an indirect form dictionary [Form] returns
// the same form value.  The AcroForm entry must not change while x is in use.
func DocumentForm(x *pdf.Extractor) (*FormIndex, error) {
	m := x.R.GetMeta()
	if m == nil || m.Catalog == nil || m.Catalog.AcroForm == nil {
		return nil, nil
	}
	if ref, ok := m.Catalog.AcroForm.(pdf.Reference); ok {
		return pdf.CursorAt(x, nil).DecodeExclusive(ref, FormWithRefs)
	}

	// a direct dictionary has no reference to cache the form under
	res, err := x.Once(func(c pdf.Cursor) (documentForm, error) {
		idx, err := FormWithRefs(c, m.Catalog.AcroForm, true)
		return documentForm{idx: idx}, err
	})
	return res.idx, err
}

// FormWithRefs reads an interactive form like [Form] and also records the
// reference of the dictionary each terminal field and each of their widgets
// was read from.  For a field with widget kids the field's reference is that
// of the field dictionary, for a merged field/widget dictionary it is that
// dictionary.  It returns nil if obj is nil.
//
// Always invoke this via [pdf.Cursor.Decode] or [pdf.Cursor.DecodeExclusive]:
// the Form of the result is then the very value [Form] returns for the same
// reference.  To read the document's own form, use [DocumentForm].
func FormWithRefs(c pdf.Cursor, obj pdf.Object, _ bool) (*FormIndex, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, nil
	}

	form := &acroform.InteractiveForm{}

	// the document-wide /DA and /Q defaults seed field-attribute inheritance,
	// which the decoder flattens away; the values are not kept on the form
	da, err := pdf.Optional(c.String(dict["DA"]))
	if err != nil {
		return nil, err
	}
	var q pdf.TextAlign
	if v, err := pdf.Optional(c.Integer(dict["Q"])); err == nil && v >= 0 && v <= 2 {
		q = pdf.TextAlign(v)
	} else if pdf.IsReadError(err) {
		return nil, err
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
	na, _, err := c.OptionalBoolean(dict["NeedAppearances"])
	if err != nil {
		return nil, err
	}
	form.NeedAppearances = na

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
		if dr, err := pdf.Optional(c.Decode(drObj, extract.Resources)); err != nil {
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

	refs := make(map[acroform.Field]pdf.Reference, len(d.byRef))
	for ref, f := range d.byRef {
		refs[f] = ref
	}
	return &FormIndex{Form: form, FieldRefs: refs, WidgetRefs: d.widgetRefs}, nil
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

// shareValue gives dst the value and default value of src, the first field
// decoded under the same fully qualified name, which must agree with it
// (12.7.4.2).  A rich text value travels with the value it formats.  It
// returns false, leaving dst unchanged, if dst cannot hold the values: if the
// fields differ in type, if dst is a password field and src has a value, or
// if dst is a single-selection choice field and src holds an explicit empty
// or a multiple selection.  A check box or radio button value is checked
// against the field's widgets once they are linked, by [settleButtonValue].
func shareValue(dst, src acroform.Field) bool {
	switch dst := dst.(type) {
	case *acroform.TextField:
		src, ok := src.(*acroform.TextField)
		if !ok || dst.Flags&acroform.FieldPassword != 0 && (src.V != nil || src.RichValue != nil) {
			return false
		}
		dst.V, dst.DV = cloneText(src.V), cloneText(src.DV)
		dst.RichValue = cloneText(src.RichValue)
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
		if dst.Flags&acroform.FieldMultiSelect == 0 && (!isSingleSelection(src.V) || !isSingleSelection(src.DV)) {
			return false
		}
		dst.V, dst.DV = slices.Clone(src.V), slices.Clone(src.DV)
		dst.RichValue = cloneText(src.RichValue)
		// /I indexes dst's own options; keep it only where it restates /V
		if !dst.SelectionAgrees() {
			dst.Selected = nil
		}
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

// isSingleSelection reports whether a choice value can be held by a field
// without the MultiSelect flag: it is absent or names a single option.
func isSingleSelection(v []pdf.TextString) bool {
	return v == nil || len(v) == 1
}

// cloneText returns a copy of a text value.
func cloneText(v *pdf.StringOrStream) *pdf.StringOrStream {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}
