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

package annotation

import (
	"errors"
	"fmt"
	"maps"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/optional"
)

// PDF 2.0 sections: 12.7.4.1 12.7.4.2

// Node is a child of a field in the field hierarchy. A node is either a
// sub-field (a [Field]) or a widget annotation (a [*Widget]).
type Node interface {
	// IsFieldNode is a marker method that has no effect.
	IsFieldNode()
}

// Field is a single field in a PDF interactive form.
//
// Fields form a tree: a non-terminal field has sub-fields as its children,
// while a terminal field has widget annotations as its children. A terminal
// field with exactly one widget is written as a single combined field/widget
// dictionary; this merging is applied automatically and is transparent to the
// Go representation, where the widget is always a child in [FieldCommon.Kids].
//
// Fields are not written individually; they are written, with the whole subtree
// rooted at each, by [InteractiveForm.Encode] when the form is stored.
//
// Each terminal field has a concrete type matching its field type: [FieldTx]
// (text), [FieldBtn] (button), [FieldChoice] (choice), and [FieldSig]
// (signature). A field with no field type of its own — a non-terminal field, or
// one whose type is inherited — is represented by a *[FieldCommon].
//
// Several attributes (the field type, Ff, V, DV) are inheritable: a field that
// does not specify them takes the value from its nearest ancestor that does.
// The stored values are not flattened; use the Resolved* functions to obtain
// the effective value.
type Field interface {
	Node

	// A field encodes to its own dictionary. Fields are written through the
	// form (see [InteractiveForm.Encode]); a single-widget terminal field has no
	// object of its own (it is folded into its widget), so its Encode is never
	// called — its reference is the widget's.
	pdf.Encoder

	// FieldType returns the field type, one of "Btn", "Tx", "Ch", or "Sig". An
	// empty value indicates a field with no type of its own.
	FieldType() pdf.Name

	// GetFieldCommon returns the attributes common to all field types.
	GetFieldCommon() *FieldCommon

	// unexported methods seal the interface to this package and let the shared
	// encode/decode/inheritance helpers reach each type's specific entries.
	fillTypeDict(rm *pdf.ResourceManager, dict pdf.Dict) error
	decodeType(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) error
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
	// field inherit its flags from the nearest ancestor that sets them. The
	// stored value is the [FieldFlags] bit set, converted to uint.
	Ff optional.UInt

	// AA (optional) is the field's additional-actions dictionary.
	AA *triggers.Form

	// Kids holds the field's children, either sub-fields or widget annotations.
	// A terminal field with a single widget child is encoded as one combined
	// field/widget dictionary; the merge is applied automatically on write.
	Kids []Node

	// parent points to the field's parent in the hierarchy, for inheritance
	// and name resolution. It is set by the builder methods, when the field is
	// decoded as a child, or at encode time, and is not written to the PDF file.
	parent Field

	// self is the concrete Field value that embeds this FieldCommon (for
	// example the outer *FieldBtn). A method promoted onto the embedded
	// *FieldCommon cannot otherwise reach its outer struct; the builder methods
	// use self to link a new child to the correct typed parent. It is set when
	// the field is created, by a builder or by decoding.
	self Field
}

// IsFieldNode implements the [Node] interface.
func (*FieldCommon) IsFieldNode() {}

// GetFieldCommon implements the [Field] interface.
func (c *FieldCommon) GetFieldCommon() *FieldCommon { return c }

// FieldType implements the [Field] interface. For a bare [FieldCommon] it
// returns the empty string.
func (c *FieldCommon) FieldType() pdf.Name { return "" }

func (c *FieldCommon) fillTypeDict(*pdf.ResourceManager, pdf.Dict) error { return nil }
func (c *FieldCommon) decodeType(*pdf.Extractor, *pdf.CycleCheck, pdf.Dict) error {
	return nil
}

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
	_ Node  = (*Widget)(nil)
)

// isValidFieldType reports whether name is one of the defined field types.
func isValidFieldType(name pdf.Name) bool {
	switch name {
	case "Btn", "Tx", "Ch", "Sig":
		return true
	default:
		return false
	}
}

// DecodeField reads a field dictionary from a PDF file and returns the matching
// concrete [Field] type.
//
// Always invoke this via [pdf.ExtractorGet] so that indirect references are
// resolved and cycle detection covers the field hierarchy.
func DecodeField(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, _ bool) (Field, error) {
	dict, err := x.GetDict(path, obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, nil
	}

	// a field merged with its single widget is one object that is both a field
	// and a Widget annotation; decode it as a linked field+widget pair so the
	// page's /Annots entry and the field tree share one widget object
	if path != nil && isMergedFieldDict(dict) {
		f, _, err := decodeMergedField(x, path, path.Ref, dict)
		return f, err
	}

	f := newField(fieldTypeOf(x, path, dict))
	if err := decodeFieldCommonEntries(x, path, dict, f); err != nil {
		return nil, err
	}
	if err := decodeKids(x, path, dict, f); err != nil {
		return nil, err
	}
	if err := f.decodeType(x, path, dict); err != nil {
		return nil, err
	}
	return f, nil
}

// fieldTypeOf reads a field dictionary's own field type. An unrecognised or
// absent type yields the empty name, leaving the field typeless (a bare
// [FieldCommon]) so that it stays writable. The type is inheritable; an
// inherited type is not resolved here (use [ResolvedFT] for the effective type).
func fieldTypeOf(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) pdf.Name {
	if name, _ := pdf.Optional(x.GetName(path, dict["FT"])); isValidFieldType(name) {
		return name
	}
	return ""
}

// newField returns an empty concrete field for the given field type. A field
// with no recognised type is represented by a bare [FieldCommon]. A button
// field's variant (check box / radio / push) is not fixed here; it is derived
// on demand from the effective flags (see [FieldBtn.Variant]).
func newField(ft pdf.Name) Field {
	var f Field
	switch ft {
	case "Tx":
		f = &FieldTx{}
	case "Ch":
		f = &FieldChoice{}
	case "Sig":
		f = &FieldSig{}
	case "Btn":
		f = &FieldBtn{}
	default:
		f = &FieldCommon{}
	}
	f.GetFieldCommon().self = f
	return f
}

// decodeFieldCommonEntries reads the entries common to all field types (the name
// entries, flags, and the field half of /AA) into the field's [FieldCommon]. It
// does not read /Kids or the merged widget; those are handled by the caller.
func decodeFieldCommonEntries(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict, f Field) error {
	c := f.GetFieldCommon()

	if t, err := pdf.Optional(pdf.GetTextString(x.R, dict["T"])); err != nil {
		return err
	} else {
		// a partial name must not contain a period (the separator used in
		// fully qualified names); strip any so the field can be written back
		c.T = strings.ReplaceAll(string(t), ".", "")
	}
	if tu, err := pdf.Optional(pdf.GetTextString(x.R, dict["TU"])); err != nil {
		return err
	} else {
		c.TU = string(tu)
	}
	if tm, err := pdf.Optional(pdf.GetTextString(x.R, dict["TM"])); err != nil {
		return err
	} else {
		c.TM = string(tm)
	}

	// Ff (inheritable)
	if _, ok := dict["Ff"]; ok {
		if ff, err := pdf.Optional(x.GetInteger(path, dict["Ff"])); err != nil {
			return err
		} else {
			c.Ff = optional.NewUInt(uint(uint32(ff)))
		}
	}

	// AA: in a merged dictionary /AA is shared, splitting into the field half
	// (K/F/V/C, read here) and the widget half (E/X/Fo/Bl/…, read by the widget);
	// drop this half if the split left it empty
	if aa, err := pdf.ExtractorGetOptional(x, path, dict["AA"], triggers.DecodeForm); err != nil {
		return err
	} else {
		c.AA = aa
	}
	if c.AA != nil && c.AA.IsEmpty() {
		c.AA = nil
	}

	return nil
}

// decodeMergedField decodes one dictionary that is both a form field and its
// single widget annotation (12.5.6.19) into a linked field+widget pair, and
// publishes both typed views under ref so that the page's /Annots entry and the
// field tree share one widget object. It builds both halves directly from the
// dictionary — never resolving ref recursively — so there is no self-cycle.
func decodeMergedField(x *pdf.Extractor, path *pdf.CycleCheck, ref pdf.Reference, dict pdf.Dict) (Field, *Widget, error) {
	f := newField(fieldTypeOf(x, path, dict))
	if err := decodeFieldCommonEntries(x, path, dict, f); err != nil {
		return nil, nil, err
	}
	if err := f.decodeType(x, path, dict); err != nil {
		return nil, nil, err
	}
	w, err := decodeWidgetBody(x, path, dict)
	if err != nil {
		return nil, nil, err
	}

	// link the pair before publishing: StoreOrLoadPair publishes both halves
	// atomically, so the winner's already-linked f/w become the shared pair and
	// a losing concurrent decoder adopts them without mutating shared state.
	// Linking after the publish would race two decoders writing the same field
	// and widget (reachable when a page's widget decode and the form's field
	// decode reach this merged object at once).
	f.GetFieldCommon().Kids = []Node{w}
	w.Parent = f
	fc, ac := pdf.StoreOrLoadPair[Field, Annotation](x, ref, f, w)
	return fc, ac.(*Widget), nil
}

// isMergedFieldDict reports whether a dictionary is a form field merged with its
// single widget annotation: a Widget annotation that also carries field entries
// and omits /Kids (12.5.6.19, 12.7.4.1).
func isMergedFieldDict(dict pdf.Dict) bool {
	if subtype, _ := dict["Subtype"].(pdf.Name); subtype != "Widget" {
		return false
	}
	if _, ok := dict["Kids"]; ok {
		return false
	}
	for _, key := range []pdf.Name{"FT", "T", "TU", "TM", "Ff", "V", "DV", "DA", "Q", "MaxLen", "Opt", "Lock", "SV"} {
		if _, ok := dict[key]; ok {
			return true
		}
	}
	return false
}

func decodeKids(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict, parent Field) error {
	c := parent.GetFieldCommon()

	// guard against a kid reference appearing more than once
	visited := map[pdf.Reference]bool{}
	if path != nil {
		visited[path.Ref] = true
	}

	kids, err := pdf.Optional(x.GetArray(path, dict["Kids"]))
	if err != nil {
		return err
	}
	for _, el := range kids {
		ref, ok := el.(pdf.Reference)
		if !ok {
			continue
		}
		if visited[ref] {
			continue
		}
		visited[ref] = true

		kidDict, err := pdf.Optional(x.GetDict(path, ref))
		if err != nil {
			return err
		}
		if kidDict == nil {
			continue
		}

		// a pure widget annotation kid; if it fails to decode as a widget,
		// fall through and decode it as a sub-field rather than dropping it
		if isWidgetKid(kidDict) {
			a, err := pdf.Optional(pdf.ExtractorGet(x, path, ref, Decode))
			if err != nil {
				return err
			}
			if w, ok := a.(*Widget); ok && w != nil {
				// link the widget back to its field (the back-edge of the cycle)
				w.Parent = parent
				c.Kids = append(c.Kids, w)
				continue
			}
		}

		child, err := pdf.Optional(pdf.ExtractorGet(x, path, ref, DecodeField))
		if err != nil {
			return err
		}
		if child != nil {
			child.GetFieldCommon().parent = parent
			c.Kids = append(c.Kids, child)
		}
	}
	return nil
}

// isWidgetKid reports whether a child dictionary is a pure widget annotation
// rather than a (possibly merged) sub-field. A widget has the Widget subtype
// and none of the field-distinguishing entries FT, T, or Kids; a child that
// carries any of those is treated as a sub-field.
func isWidgetKid(dict pdf.Dict) bool {
	if subtype, _ := dict["Subtype"].(pdf.Name); subtype != "Widget" {
		return false
	}
	if _, ok := dict["FT"]; ok {
		return false
	}
	if _, ok := dict["T"]; ok {
		return false
	}
	if _, ok := dict["Kids"]; ok {
		return false
	}
	return true
}

// encodeField builds the dictionary for a field that has its own object — every
// field except a single-widget terminal field, which is folded into its widget
// (see [fieldRef]). It writes the field's own entries, its /Parent, and its
// /Kids (each kid named by [fieldRef] for sub-fields or by the widget's
// reference for widget kids). It implements [Field.Encode]; the form, not the
// caller, drives this through [InteractiveForm.Encode].
//
// Widget annotations are not written here: each is linked to this field via
// [Widget.Parent] and written later, when the page listing it is written. The
// ordering contract (store the form before closing pages) ensures the link is in
// place by then.
func encodeField(rm *pdf.ResourceManager, f Field) (pdf.Native, error) {
	dict, err := fieldEntries(rm, f)
	if err != nil {
		return nil, err
	}
	c := f.GetFieldCommon()
	if c.parent != nil {
		dict["Parent"] = rm.GetReference(c.parent)
	}

	var kidRefs pdf.Array
	for _, kid := range c.Kids {
		switch k := kid.(type) {
		case Field:
			k.GetFieldCommon().parent = f // establish hierarchy for built trees
			kidRef, err := fieldRef(rm, k)
			if err != nil {
				return nil, err
			}
			kidRefs = append(kidRefs, kidRef)
		case *Widget:
			k.Parent = f
			kidRefs = append(kidRefs, rm.GetReference(k))
		default:
			return nil, fmt.Errorf("unsupported field child type %T", kid)
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
// entries via [foldFieldIntoWidget]). Every other field is written via rm.Store
// and named by its own reference.
func fieldRef(rm *pdf.ResourceManager, f Field) (pdf.Reference, error) {
	widgets, hasSubfield, err := classifyKids(f.GetFieldCommon().Kids)
	if err != nil {
		return 0, err
	}
	if !hasSubfield && len(widgets) == 1 {
		w := widgets[0]
		w.Parent = f
		return rm.GetReference(w), nil
	}
	return rm.Store(f)
}

// classifyKids splits a field's children into widget annotations and a flag for
// whether any child is a sub-field.
func classifyKids(kids []Node) ([]*Widget, bool, error) {
	var widgets []*Widget
	hasSubfield := false
	for _, kid := range kids {
		switch k := kid.(type) {
		case *Widget:
			widgets = append(widgets, k)
		case Field:
			hasSubfield = true
		default:
			return nil, false, fmt.Errorf("unsupported field child type %T", kid)
		}
	}
	return widgets, hasSubfield, nil
}

// fieldEntries builds the field-level dictionary entries (FT, T, the flags, AA,
// and the type-specific entries), excluding /Parent and /Kids.
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

// foldFieldIntoWidget ties an encoded widget dictionary to its form field
// (w.Parent). If the widget is the single widget of a terminal field, the
// field's own entries are folded into the widget dictionary, producing one
// merged field/widget object, and /Parent names the field's own parent (absent
// for a root field). Otherwise the widget gets a /Parent pointing at the field's
// own object. It is called from [Widget.Encode] and keeps the merge rules here.
//
// Field entries take precedence over the widget's, except for the shared /AA,
// whose field half (K/F/V/C) and widget half (E/X/Fo/Bl/…) are combined.
func foldFieldIntoWidget(rm *pdf.ResourceManager, w *Widget, dict pdf.Dict) (pdf.Native, error) {
	f := w.Parent
	widgets, hasSubfield, err := classifyKids(f.GetFieldCommon().Kids)
	if err != nil {
		return nil, err
	}
	if hasSubfield || len(widgets) != 1 || widgets[0] != w {
		// a non-merged widget kid of a multi-widget field
		dict["Parent"] = rm.GetReference(f)
		return dict, nil
	}

	// the single widget of a terminal field: fold the field's entries in
	entries, err := fieldEntries(rm, f)
	if err != nil {
		return nil, err
	}
	for k, v := range entries {
		if existing, exists := dict[k]; exists && k == "AA" {
			merged, err := mergeAADicts(v, existing)
			if err != nil {
				return nil, err
			}
			dict[k] = merged
			continue
		}
		dict[k] = v
	}
	if p := f.GetFieldCommon().parent; p != nil {
		dict["Parent"] = rm.GetReference(p)
	}
	return dict, nil
}

// mergeAADicts combines the field-level (K/F/V/C) and annotation-level
// (E/X/Fo/Bl/…) halves of a merged field's shared /AA dictionary. The two key
// sets are disjoint by construction, so an overlap signals a bug.
func mergeAADicts(field, widget pdf.Object) (pdf.Dict, error) {
	fd, ok := field.(pdf.Dict)
	if !ok {
		return nil, errors.New("field AA did not encode to a dictionary")
	}
	wd, ok := widget.(pdf.Dict)
	if !ok {
		return nil, errors.New("widget AA did not encode to a dictionary")
	}
	merged := maps.Clone(fd)
	for k, v := range wd {
		if _, exists := merged[k]; exists {
			return nil, fmt.Errorf("conflicting AA entry %q", k)
		}
		merged[k] = v
	}
	return merged, nil
}

// ResolvedFT returns the field's effective type, inherited from an ancestor if
// the field itself does not specify one.
//
// The resolution walks the field's ancestors, so the parent links must be in
// place: they are established by the builder methods, by decoding, or at encode
// time. The same applies to [ResolvedFf], [ResolvedV], and [ResolvedDV].
func ResolvedFT(f Field) pdf.Name {
	for n := f; n != nil; n = parentOf(n) {
		if ft := n.FieldType(); ft != "" {
			return ft
		}
	}
	return ""
}

// ResolvedFf returns the field's effective flags, inherited from an ancestor
// if the field itself does not specify them.
func ResolvedFf(f Field) FieldFlags {
	for n := f; n != nil; n = parentOf(n) {
		if ff, ok := n.GetFieldCommon().Ff.Get(); ok {
			return FieldFlags(ff)
		}
	}
	return 0
}

// ResolvedV returns the field's effective value, inherited from an ancestor if
// the field itself does not specify one.
func ResolvedV(f Field) pdf.Object {
	for n := f; n != nil; n = parentOf(n) {
		if v := n.ownValue(); v != nil {
			return v
		}
	}
	return nil
}

// ResolvedDV returns the field's effective default value, inherited from an
// ancestor if the field itself does not specify one.
func ResolvedDV(f Field) pdf.Object {
	for n := f; n != nil; n = parentOf(n) {
		if dv := n.ownDefaultValue(); dv != nil {
			return dv
		}
	}
	return nil
}

func parentOf(f Field) Field {
	p := f.GetFieldCommon().parent
	if p == nil {
		return nil
	}
	return p
}

// FullyQualifiedName returns the field's fully qualified name, formed by
// joining the partial names of the field and its ancestors with a period.
// Ancestors without a partial name are skipped.
//
// The name walks the field's ancestors, so the parent links must be in place:
// they are established by the builder methods, by decoding, or at encode time.
func (c *FieldCommon) FullyQualifiedName() string {
	var parts []string
	for n := c; n != nil; {
		if n.T != "" {
			parts = append(parts, n.T)
		}
		if n.parent == nil {
			break
		}
		n = n.parent.GetFieldCommon()
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, ".")
}
