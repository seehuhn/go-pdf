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

package acroform

import (
	"errors"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/optional"
)

// PDF 2.0 sections: 12.7.4.1 12.7.4.2

// Node is a child of a field in the field hierarchy. A node is either a [Field]
// (a sub-field) or a widget annotation ("seehuhn.de/go/pdf/annotation".Widget).
type Node interface {
	// A node encodes to its own dictionary. Fields are written through the
	// form (see [InteractiveForm.Encode]); a single-widget terminal field has
	// no object of its own (it is folded into its widget), so its Encode is
	// never called — its reference is the widget's.
	pdf.Encoder

	// FieldParent returns the form field this node is a child of, or nil if
	// the node is not linked to a parent field.
	FieldParent() Field
}

// Field is a single field in a PDF interactive form.
//
// Fields form a tree: a non-terminal field has sub-fields as its children,
// while a terminal field has widget annotations as its children. A terminal
// field with exactly one widget is written as a single combined field/widget
// dictionary; this merging is applied automatically and is transparent to the
// Go representation, where the widget is always a child in [FieldCommon.Kids].
//
// Fields are not written individually; they are written, with the whole
// subtree rooted at each, by [InteractiveForm.Encode] when the form is stored.
//
// Each terminal field has a concrete type matching its field type: [FieldTx]
// (text), [FieldBtn] (button), [FieldChoice] (choice), and [FieldSig]
// (signature). A field with no field type of its own — a non-terminal field,
// or one whose type is inherited — is represented by a [*FieldCommon].
//
// Several attributes (the field type, Ff, V, DV) are inheritable: a field that
// does not specify them takes the value from its nearest ancestor that does.
// The stored values are not flattened; use the Resolved* functions to obtain
// the effective value.
type Field interface {
	Node

	// FieldType returns the field type, one of "Btn", "Tx", "Ch", or "Sig".
	// An empty value indicates a field with no type of its own.
	FieldType() pdf.Name

	// GetFieldCommon returns the attributes common to all field types.
	GetFieldCommon() *FieldCommon

	fillTypeDict(rm *pdf.ResourceManager, dict pdf.Dict) error
	ownValue() pdf.Object
	ownDefaultValue() pdf.Object
}

// FieldCommon holds the attributes shared by all field types. Concrete field
// types embed it; on its own, a *FieldCommon represents a non-terminal field or
// a field whose type is inherited from an ancestor.
type FieldCommon struct {
	// T (optional) is the partial field name. An empty value indicates that
	// the field has no name of its own and does not contribute to fully
	// qualified field names.
	T string

	// TU (optional) is an alternative field name used in the user interface
	// and for accessibility.
	TU string

	// TM (optional) is the mapping name used when exporting field data.
	TM string

	// Ff holds the field flags common to all field types. Because the flags
	// are inheritable, a present value of zero is distinct from an absent
	// entry: a present zero blocks inheritance, while an absent value lets the
	// field inherit its flags from the nearest ancestor that sets them.
	Ff optional.Value[FieldFlags]

	// AA (optional) is the field's additional-actions dictionary.
	AA *triggers.Form

	// Kids holds the field's children, either sub-fields or widget annotations.
	// A terminal field with a single widget child is encoded as one combined
	// field/widget dictionary; the merge is applied automatically on write.
	Kids []Node

	// Parent points to the field's parent in the hierarchy, for inheritance
	// and name resolution. It is nil for a root field. The builder functions
	// and decoding set it; when assembling a field's Kids by hand, set it on
	// each sub-field child.
	//
	// This corresponds to the /Parent entry.
	Parent Field
}

// FieldParent implements the [Node] interface; it returns
// [FieldCommon.Parent].
func (c *FieldCommon) FieldParent() Field { return c.Parent }

// GetFieldCommon implements the [Field] interface.
func (c *FieldCommon) GetFieldCommon() *FieldCommon { return c }

// FieldType implements the [Field] interface. For a bare [FieldCommon] it
// returns the empty string.
func (c *FieldCommon) FieldType() pdf.Name { return "" }

func (c *FieldCommon) fillTypeDict(*pdf.ResourceManager, pdf.Dict) error { return nil }

// a typeless field has no value of its own; it serves only as a container for
// inheritable attributes
func (c *FieldCommon) ownValue() pdf.Object        { return nil }
func (c *FieldCommon) ownDefaultValue() pdf.Object { return nil }

// Encode implements [pdf.Encoder]; see [encodeField].
func (c *FieldCommon) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	return encodeField(rm, c)
}

var (
	_ Field = (*FieldCommon)(nil)
	_ Node  = (*FieldCommon)(nil)
)

// newField returns an empty concrete field for the given field type, one of
// "Tx", "Ch", "Btn", or "Sig". A field with no recognised type is represented by
// a bare [FieldCommon]. A button field's variant (check box / radio / push) is
// not fixed here; it is derived on demand from the effective flags (see
// [FieldBtn.Variant]).
//
// This is the factory used when decoding, reached through the formhooks
// package; field trees are otherwise built with the builder functions and the
// methods on [InteractiveForm].
func newField(ft pdf.Name) Field {
	switch ft {
	case "Tx":
		return &FieldTx{}
	case "Ch":
		return &FieldChoice{}
	case "Sig":
		return &FieldSig{}
	case "Btn":
		return &FieldBtn{}
	default:
		return &FieldCommon{}
	}
}

// encodeField builds the dictionary for a field that has its own object — every
// field except a single-widget terminal field, which is folded into its widget
// (see [fieldRef]). It writes the field's own entries, its /Parent, and its
// /Kids (each kid named by [fieldRef] for sub-fields or by the widget's
// reference for widget kids). It implements [Field.Encode]; the form, not the
// caller, drives this through [InteractiveForm.Encode].
//
// Widget annotations are not written here: each is written later, when the
// page listing it is written, and then folds in its field's entries using its
// parent link. The ordering contract (store the form before closing pages)
// ensures the link is still in place by then.
//
// Encoding never modifies the tree: it fails with an error if a child's
// parent link does not point back to f.
func encodeField(rm *pdf.ResourceManager, f Field) (pdf.Native, error) {
	dict, err := fieldEntries(rm, f)
	if err != nil {
		return nil, err
	}
	c := f.GetFieldCommon()
	if c.Parent != nil {
		dict["Parent"] = rm.GetReference(c.Parent)
	}

	var kidRefs pdf.Array
	for _, kid := range c.Kids {
		if kid.FieldParent() != f {
			return nil, errors.New("field kid with missing or wrong Parent link")
		}
		if k, ok := kid.(Field); ok {
			kidRef, err := fieldRef(rm, k)
			if err != nil {
				return nil, err
			}
			kidRefs = append(kidRefs, kidRef)
		} else {
			// a widget annotation kid, named by its own reference
			kidRefs = append(kidRefs, rm.GetReference(kid))
		}
	}
	if len(kidRefs) > 0 {
		dict["Kids"] = kidRefs
	}
	return dict, nil
}

// fieldRef returns the reference that names f in the file. A single-widget
// terminal field has no object of its own: its reference is its widget's, which
// writes the merged field/widget dictionary (the widget folds in the field's
// entries via foldFieldIntoWidget). Every other field is written via rm.Store
// and named by its own reference.
func fieldRef(rm *pdf.ResourceManager, f Field) (pdf.Reference, error) {
	widgets, hasSubfield := classifyKids(f.GetFieldCommon().Kids)
	if !hasSubfield && len(widgets) == 1 {
		w := widgets[0]
		if w.FieldParent() != f {
			return 0, errors.New("widget with missing or wrong Parent link")
		}
		return rm.GetReference(w), nil
	}
	return rm.Store(f)
}

// classifyKids splits a field's children into widget annotations (any child that
// is not itself a [Field]) and a flag for whether any child is a sub-field.
func classifyKids(kids []Node) (widgets []Node, hasSubfield bool) {
	for _, kid := range kids {
		if _, ok := kid.(Field); ok {
			hasSubfield = true
		} else {
			widgets = append(widgets, kid)
		}
	}
	return widgets, hasSubfield
}

// fieldEntries builds the field-level dictionary entries (FT, T, the flags, AA,
// and the type-specific entries), excluding /Parent and /Kids. The annotation
// package reaches it through the formhooks package, to fold a terminal field's
// entries into the field's single widget annotation.
func fieldEntries(rm *pdf.ResourceManager, f Field) (pdf.Dict, error) {
	if err := pdf.CheckVersion(rm.Out, "interactive form field", pdf.V1_2); err != nil {
		return nil, err
	}

	c := f.GetFieldCommon()
	dict := pdf.Dict{}

	if ft := f.FieldType(); ft != "" {
		dict["FT"] = ft
	}
	if c.T != "" {
		if strings.Contains(c.T, ".") {
			return nil, errors.New("field partial name must not contain a period")
		}
		dict["T"] = pdf.TextString(c.T)
	}
	if c.TU != "" {
		if err := pdf.CheckVersion(rm.Out, "field TU entry", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["TU"] = pdf.TextString(c.TU)
	}
	if c.TM != "" {
		if err := pdf.CheckVersion(rm.Out, "field TM entry", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["TM"] = pdf.TextString(c.TM)
	}
	if ff, ok := c.Ff.Get(); ok {
		if err := checkFlagVersions(rm.Out, ff); err != nil {
			return nil, err
		}
		dict["Ff"] = pdf.Integer(uint32(ff))
	}
	if c.AA != nil {
		if err := pdf.CheckVersion(rm.Out, "field AA entry", pdf.V1_3); err != nil {
			return nil, err
		}
		aa, err := c.AA.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["AA"] = aa
	}

	// type-specific entries (V, DV, DA, Q, MaxLen, Opt, …)
	if err := f.fillTypeDict(rm, dict); err != nil {
		return nil, err
	}

	return dict, nil
}

// ResolvedFT returns the field's effective type, inherited from an ancestor if
// the field itself does not specify one.
//
// The resolution walks the field's ancestors, so the parent links must be in
// place: the builder functions and decoding set them; set [FieldCommon.Parent]
// yourself when assembling a tree by hand. The same applies to [ResolvedFf],
// [ResolvedV], and [ResolvedDV].
func ResolvedFT(f Field) pdf.Name {
	for n := f; n != nil; n = n.FieldParent() {
		if ft := n.FieldType(); ft != "" {
			return ft
		}
	}
	return ""
}

// ResolvedFf returns the field's effective flags, inherited from an ancestor
// if the field itself does not specify them.
func ResolvedFf(f Field) FieldFlags {
	for n := f; n != nil; n = n.FieldParent() {
		if ff, ok := n.GetFieldCommon().Ff.Get(); ok {
			return ff
		}
	}
	return 0
}

// ResolvedV returns the field's effective value, inherited from an ancestor if
// the field itself does not specify one.
func ResolvedV(f Field) pdf.Object {
	for n := f; n != nil; n = n.FieldParent() {
		if v := n.ownValue(); v != nil {
			return v
		}
	}
	return nil
}

// ResolvedDV returns the field's effective default value, inherited from an
// ancestor if the field itself does not specify one.
func ResolvedDV(f Field) pdf.Object {
	for n := f; n != nil; n = n.FieldParent() {
		if dv := n.ownDefaultValue(); dv != nil {
			return dv
		}
	}
	return nil
}

// ResolvedMaxLen returns a text field's effective maximum text length,
// inherited from an ancestor if the field itself does not set one. A value of
// zero indicates that no maximum is set.
func ResolvedMaxLen(f Field) int {
	for n := f; n != nil; n = n.FieldParent() {
		if x, ok := n.(*FieldTx); ok && x.MaxLen > 0 {
			return x.MaxLen
		}
	}
	return 0
}

// FullyQualifiedName returns the field's fully qualified name, formed by
// joining the partial names of the field and its ancestors with a period.
// Ancestors without a partial name are skipped.
//
// The name walks the field's ancestors, so the parent links must be in place:
// the builder functions and decoding set them; set [FieldCommon.Parent]
// yourself when assembling a tree by hand.
func (c *FieldCommon) FullyQualifiedName() string {
	var parts []string
	for n := c; n != nil; {
		if n.T != "" {
			parts = append(parts, n.T)
		}
		if n.Parent == nil {
			break
		}
		n = n.Parent.GetFieldCommon()
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, ".")
}
