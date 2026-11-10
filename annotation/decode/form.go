// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// PDF 2.0 sections: 12.7.3

package decode

import (
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/opaque"
)

// FormIndex is an interactive form together with the references of the
// dictionaries its terminal fields and their widgets were read from.
type FormIndex struct {
	// Form is the decoded interactive form.
	Form *acroform.InteractiveForm

	// FieldRefs maps each terminal field of Form to its dictionary.  For a
	// field with widget kids this is the field dictionary, for a field merged
	// with its widget it is the merged dictionary.
	FieldRefs map[acroform.Field]pdf.Reference

	// WidgetRefs maps each widget of these fields to its annotation
	// dictionary, including widgets which are on no page.
	WidgetRefs map[acroform.Widget]pdf.Reference
}

// documentForm is the result of reading the document's form, as kept by
// [DocumentForm].
type documentForm struct {
	idx *FormIndex
	err error
}

// DocumentForm reads the interactive form named by the AcroForm entry of the
// document catalog.  It returns nil if the document has no form.
//
// A document has one interactive form, and this is the only way to read it.
// The form is read once per extractor, and later calls return the same
// result.  This includes a failure: once reading the form has failed, every
// call through x returns the same error.  [Annotation] links widgets to the
// fields of this form.  The AcroForm entry must not change while x is in use.
func DocumentForm(x *pdf.Extractor) (*FormIndex, error) {
	res, _ := x.Once(func(c pdf.Cursor) (documentForm, error) {
		m := x.R.GetMeta()
		if m == nil || m.Catalog == nil || m.Catalog.AcroForm == nil {
			return documentForm{}, nil
		}
		idx, err := c.Decode(m.Catalog.AcroForm, readForm)
		return documentForm{idx: idx, err: err}, nil
	})
	return res.idx, res.err
}

// readForm reads an interactive form dictionary together with the references
// of its terminal fields and their widgets.  It returns nil if obj is nil.
//
// The widgets of the form are published in the extractor's cache only once
// they are linked to their fields, so that [Annotation] never returns a
// widget which is still being linked.  Only [DocumentForm] may call this
// outside of tests, and at most once per extractor: a second reading takes
// the field tree from the cache and so finds no field or widget references,
// which leaves the index empty and /CO unresolved; see [fieldTreeDecoder].
func readForm(c pdf.Cursor, obj pdf.Object, _ bool) (*FormIndex, error) {
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

	d.publishWidgets(c.Extractor())

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
// fields differ in type, if dst is a password field and src has a value or a
// default value, if dst is a single-selection choice field and src holds an
// explicit empty or a multiple selection, or if a choice value of src names
// an option which dst lacks.  A check box or radio button sets the field's
// widgets from the value once they are linked, by [settleButtonValue].
func shareValue(dst, src acroform.Field) bool {
	switch dst := dst.(type) {
	case *acroform.TextField:
		src, ok := src.(*acroform.TextField)
		if !ok || dst.Flags&acroform.FieldPassword != 0 && (src.Text != nil || src.RichText != nil || src.DefaultText != nil) {
			return false
		}
		dst.Text, dst.DefaultText = cloneText(src.Text), cloneText(src.DefaultText)
		dst.RichText = cloneText(src.RichText)
	case *acroform.ButtonField:
		src, ok := src.(*acroform.ButtonField)
		if !ok {
			return false
		}
		dst.State, dst.DefaultState = src.State, src.DefaultState
	case *acroform.PushButtonField:
		// a push button has no value to share
		_, ok := src.(*acroform.PushButtonField)
		return ok
	case *acroform.ChoiceField:
		src, ok := src.(*acroform.ChoiceField)
		if !ok {
			return false
		}
		if dst.Flags&acroform.FieldMultiSelect == 0 && (!isSingleSelection(src.Value) || !isSingleSelection(src.DefaultValue)) {
			return false
		}
		// dst must have the options the values name
		if !dst.IsEditable() && (!allOptions(dst, src.Value) || !allOptions(dst, src.DefaultValue)) {
			return false
		}
		dst.Value, dst.DefaultValue = slices.Clone(src.Value), slices.Clone(src.DefaultValue)
		dst.RichText = cloneText(src.RichText)
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

// allOptions reports whether every entry of v names an option of f.
func allOptions(f *acroform.ChoiceField, v []pdf.TextString) bool {
	for _, s := range v {
		if !namesOption(f, s) {
			return false
		}
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
