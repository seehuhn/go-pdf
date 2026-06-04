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
	"fmt"
	"maps"
	"slices"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/optional"
)

// PDF 2.0 sections: 12.7.4.1 12.7.4.2

// Node is a child of a field in the field hierarchy. A node is either a
// sub-field (*Field) or a widget annotation (*annotation.Widget).
type Node interface {
	// IsFieldNode is a marker method that has no effect.
	IsFieldNode()
}

// Field represents a single field in a PDF interactive form.
//
// Fields form a tree: a non-terminal field has sub-fields as its children,
// while a terminal field has widget annotations as its children. When a
// terminal field has a single widget annotation, the two may be combined into
// one dictionary, indicated by the Merged flag.
//
// Several attributes (FT, Ff, V, DV) are inheritable: a field that does not
// specify them takes the value from its nearest ancestor that does. The stored
// values are not flattened; use the Resolved* methods to obtain the effective
// value.
type Field struct {
	// FT is the field type, one of "Btn" (button), "Tx" (text), "Ch"
	// (choice), or "Sig" (signature). An empty value indicates that the type
	// is inherited from an ancestor, or that this is a non-terminal field
	// without a type of its own.
	FT pdf.Name

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
	// stored value is the FieldFlags bit set, converted to uint.
	Ff optional.UInt

	// V (optional) is the field's value. Its type depends on the field type.
	V pdf.Object

	// DV (optional) is the field's default value, used when the form is reset.
	// Its type depends on the field type.
	DV pdf.Object

	// AA (optional) is the field's additional-actions dictionary.
	AA *triggers.Form

	// Kids holds the field's children, either sub-fields or widget
	// annotations. It is empty for a merged terminal field.
	Kids []Node

	// Merged indicates that this field's single widget annotation is combined
	// with the field into one dictionary. When Merged is true, Widget holds
	// the widget annotation and Kids is empty.
	Merged bool

	// Widget holds the combined widget annotation of a merged terminal field.
	// It is set if and only if Merged is true.
	Widget *annotation.Widget

	// Data (optional) holds dictionary entries that are not represented by the
	// other fields, such as type-specific entries. These entries are preserved
	// when the field is written back to a PDF file.
	//
	// TODO(voss): remove this field once the individual field types (Tx, Btn,
	// Ch, Sig) are implemented and model their type-specific entries directly.
	Data pdf.Dict

	// parent points to the field's parent in the hierarchy, for inheritance
	// and name resolution. It is set when the field is decoded as a child and
	// is not written to the PDF file.
	parent *Field
}

// IsFieldNode implements the [Node] interface.
func (*Field) IsFieldNode() {}

var (
	_ Node        = (*Field)(nil)
	_ Node        = (*annotation.Widget)(nil)
	_ pdf.Encoder = (*Field)(nil)
)

// fieldModeledKeys are the entries represented by the typed Field fields.
// They are removed from Data on decode and supplied on encode.
var fieldModeledKeys = []pdf.Name{
	"Type", "FT", "Parent", "Kids", "T", "TU", "TM", "Ff", "V", "DV", "AA",
}

// widgetModeledKeys are the annotation entries consumed by the widget half of
// a merged field. They are removed from Data in the merged case.
var widgetModeledKeys = []pdf.Name{
	"Subtype", "Rect", "Contents", "P", "NM", "M", "F", "AP", "AS", "Border",
	"C", "StructParent", "OC", "AF", "ca", "CA", "BM", "Lang", "H", "MK", "A",
	"BS",
}

func isFieldModeledKey(key pdf.Name) bool {
	return slices.Contains(fieldModeledKeys, key)
}

// isValidFieldType reports whether name is one of the defined field types.
func isValidFieldType(name pdf.Name) bool {
	switch name {
	case "Btn", "Tx", "Ch", "Sig":
		return true
	default:
		return false
	}
}

// DecodeField reads a field dictionary from a PDF file.
//
// Always invoke this via [pdf.ExtractorGet] so that indirect references are
// resolved and cycle detection covers the field hierarchy.
func DecodeField(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, _ bool) (*Field, error) {
	dict, err := x.GetDict(path, obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, nil
	}

	f := &Field{}

	// FT (inheritable); an unrecognised type is dropped, leaving the field
	// typeless so it stays writable
	if ft, err := pdf.Optional(x.GetName(path, dict["FT"])); err != nil {
		return nil, err
	} else if isValidFieldType(ft) {
		f.FT = ft
	}

	// T, TU, TM
	if t, err := pdf.Optional(pdf.GetTextString(x.R, dict["T"])); err != nil {
		return nil, err
	} else {
		// a partial name must not contain a period (the separator used in
		// fully qualified names); strip any so the field can be written back
		f.T = strings.ReplaceAll(string(t), ".", "")
	}
	if tu, err := pdf.Optional(pdf.GetTextString(x.R, dict["TU"])); err != nil {
		return nil, err
	} else {
		f.TU = string(tu)
	}
	if tm, err := pdf.Optional(pdf.GetTextString(x.R, dict["TM"])); err != nil {
		return nil, err
	} else {
		f.TM = string(tm)
	}

	// Ff (inheritable)
	if _, ok := dict["Ff"]; ok {
		if ff, err := pdf.Optional(x.GetInteger(path, dict["Ff"])); err != nil {
			return nil, err
		} else {
			f.Ff = optional.NewUInt(uint(uint32(ff)))
		}
	}

	// V, DV (inheritable, type-specific, kept opaque)
	f.V = dict["V"]
	f.DV = dict["DV"]

	// AA
	if aa, err := pdf.ExtractorGetOptional(x, path, dict["AA"], triggers.DecodeForm); err != nil {
		return nil, err
	} else {
		f.AA = aa
	}

	// detect a merged terminal field (single widget folded into this dict)
	subtype, _ := pdf.Optional(x.GetName(path, dict["Subtype"]))
	_, hasKids := dict["Kids"]
	if subtype == "Widget" && !hasKids {
		if w, err := pdf.Optional(decodeWidgetHalf(x, path, obj)); err != nil {
			return nil, err
		} else if w != nil {
			f.Merged = true
			f.Widget = w
			// the field tree owns the parent linkage
			w.Parent = 0
			// the shared /AA splits between the field (K/F/V/C) and the widget
			// (E/X/Fo/Bl/…); both decoders read the same dictionary, so drop
			// whichever half it left without any triggers
			if f.AA != nil && f.AA.IsEmpty() {
				f.AA = nil
			}
			if w.AA != nil && w.AA.IsEmpty() {
				w.AA = nil
			}
		}
	} else {
		if err := f.decodeKids(x, path, dict); err != nil {
			return nil, err
		}
	}

	// Data passthrough
	f.Data = maps.Clone(dict)
	for _, k := range fieldModeledKeys {
		delete(f.Data, k)
	}
	if f.Merged {
		for _, k := range widgetModeledKeys {
			delete(f.Data, k)
		}
	}
	if len(f.Data) == 0 {
		f.Data = nil
	}

	return f, nil
}

// decodeWidgetHalf decodes the widget annotation contained in a merged field
// dictionary.
func decodeWidgetHalf(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) (*annotation.Widget, error) {
	a, err := annotation.Decode(x, path, obj, false)
	if err != nil {
		return nil, err
	}
	w, _ := a.(*annotation.Widget)
	return w, nil
}

func (f *Field) decodeKids(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) error {
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
			a, err := pdf.Optional(pdf.ExtractorGet(x, path, ref, annotation.Decode))
			if err != nil {
				return err
			}
			if w, ok := a.(*annotation.Widget); ok && w != nil {
				// the field tree owns the parent linkage
				w.Parent = 0
				f.Kids = append(f.Kids, w)
				continue
			}
		}

		child, err := pdf.Optional(pdf.ExtractorGet(x, path, ref, DecodeField))
		if err != nil {
			return err
		}
		if child != nil {
			child.parent = f
			f.Kids = append(f.Kids, child)
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

// Encode writes the field, and all of its descendants, to the PDF file and
// returns a reference to the field dictionary.
//
// This implements the [pdf.Encoder] interface.
func (f *Field) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	ref := rm.Out.Alloc()
	if err := f.encodeAt(rm, ref, 0); err != nil {
		return nil, err
	}
	return ref, nil
}

// encodeAt writes the field to the reference self, recording parent as its
// /Parent entry (omitted when parent is zero).
func (f *Field) encodeAt(rm *pdf.ResourceManager, self, parent pdf.Reference) error {
	if err := pdf.CheckVersion(rm.Out, "interactive form field", pdf.V1_2); err != nil {
		return err
	}

	dict := pdf.Dict{}

	// passthrough entries first
	for k, v := range f.Data {
		if isFieldModeledKey(k) {
			return fmt.Errorf("field Data contains modeled key %q", k)
		}
		dict[k] = v
	}

	if parent != 0 {
		dict["Parent"] = parent
	}

	if f.FT != "" {
		if !isValidFieldType(f.FT) {
			return fmt.Errorf("invalid field type %q", f.FT)
		}
		dict["FT"] = f.FT
	}

	if f.T != "" {
		if strings.Contains(f.T, ".") {
			return errors.New("field partial name must not contain a period")
		}
		dict["T"] = pdf.TextString(f.T)
	}
	if f.TU != "" {
		if err := pdf.CheckVersion(rm.Out, "field TU entry", pdf.V1_3); err != nil {
			return err
		}
		dict["TU"] = pdf.TextString(f.TU)
	}
	if f.TM != "" {
		if err := pdf.CheckVersion(rm.Out, "field TM entry", pdf.V1_3); err != nil {
			return err
		}
		dict["TM"] = pdf.TextString(f.TM)
	}
	if ff, ok := f.Ff.Get(); ok {
		dict["Ff"] = pdf.Integer(uint32(ff))
	}
	if f.V != nil {
		dict["V"] = f.V
	}
	if f.DV != nil {
		dict["DV"] = f.DV
	}
	if f.AA != nil {
		if err := pdf.CheckVersion(rm.Out, "field AA entry", pdf.V1_3); err != nil {
			return err
		}
		aa, err := f.AA.Encode(rm)
		if err != nil {
			return err
		}
		dict["AA"] = aa
	}

	switch {
	case f.Merged:
		if f.Widget == nil {
			return errors.New("merged field requires a widget")
		}
		if len(f.Kids) > 0 {
			return errors.New("merged field must not have kids")
		}
		wd, err := f.Widget.Encode(rm)
		if err != nil {
			return err
		}
		wDict, ok := wd.(pdf.Dict)
		if !ok {
			return errors.New("widget did not encode to a dictionary")
		}
		// field entries take precedence over the widget's entries, except for
		// the shared /AA, whose field half (K/F/V/C) and widget half
		// (E/X/Fo/Bl/…) are combined into one dictionary
		for k, v := range wDict {
			if existing, exists := dict[k]; exists {
				if k == "AA" {
					merged, err := mergeAADicts(existing, v)
					if err != nil {
						return err
					}
					dict[k] = merged
				}
				continue
			}
			dict[k] = v
		}

	default:
		var kidRefs pdf.Array
		for _, kid := range f.Kids {
			switch k := kid.(type) {
			case *Field:
				kidRef := rm.Out.Alloc()
				if err := k.encodeAt(rm, kidRef, self); err != nil {
					return err
				}
				kidRefs = append(kidRefs, kidRef)
			case *annotation.Widget:
				wd, err := k.Encode(rm)
				if err != nil {
					return err
				}
				wDict, ok := wd.(pdf.Dict)
				if !ok {
					return errors.New("widget did not encode to a dictionary")
				}
				wDict["Parent"] = self
				kidRef := rm.Out.Alloc()
				if err := rm.Out.Put(kidRef, wDict); err != nil {
					return err
				}
				kidRefs = append(kidRefs, kidRef)
			default:
				return fmt.Errorf("unsupported field child type %T", kid)
			}
		}
		if len(kidRefs) > 0 {
			dict["Kids"] = kidRefs
		}
	}

	return rm.Out.Put(self, dict)
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
func (f *Field) ResolvedFT() pdf.Name {
	for n := f; n != nil; n = n.parent {
		if n.FT != "" {
			return n.FT
		}
	}
	return ""
}

// ResolvedFf returns the field's effective flags, inherited from an ancestor
// if the field itself does not specify them.
func (f *Field) ResolvedFf() FieldFlags {
	for n := f; n != nil; n = n.parent {
		if ff, ok := n.Ff.Get(); ok {
			return FieldFlags(ff)
		}
	}
	return 0
}

// ResolvedV returns the field's effective value, inherited from an ancestor if
// the field itself does not specify one.
func (f *Field) ResolvedV() pdf.Object {
	for n := f; n != nil; n = n.parent {
		if n.V != nil {
			return n.V
		}
	}
	return nil
}

// ResolvedDV returns the field's effective default value, inherited from an
// ancestor if the field itself does not specify one.
func (f *Field) ResolvedDV() pdf.Object {
	for n := f; n != nil; n = n.parent {
		if n.DV != nil {
			return n.DV
		}
	}
	return nil
}

// FullyQualifiedName returns the field's fully qualified name, formed by
// joining the partial names of the field and its ancestors with a period.
// Ancestors without a partial name are skipped.
func (f *Field) FullyQualifiedName() string {
	var parts []string
	for n := f; n != nil; n = n.parent {
		if n.T != "" {
			parts = append(parts, n.T)
		}
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, ".")
}

// RootFields decodes the form's root fields. The parent back-pointers of the
// returned fields are nil; those of their descendants are set.
func (form *InteractiveForm) RootFields(x *pdf.Extractor) ([]*Field, error) {
	var fields []*Field
	seen := map[pdf.Reference]bool{}
	for _, ref := range form.Fields {
		if seen[ref] {
			continue
		}
		seen[ref] = true
		fld, err := pdf.Optional(pdf.ExtractorGet(x, nil, ref, DecodeField))
		if err != nil {
			return nil, err
		}
		if fld != nil {
			fields = append(fields, fld)
		}
	}
	return fields, nil
}
