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

// PDF 2.0 sections: 12.7.4.1 12.7.4.2 12.5.6.19

package decode

import (
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/internal/formhooks"
	"seehuhn.de/go/pdf/optional"
)

// newField returns an empty concrete field for the given field type, using
// the factory that package acroform registers in the formhooks package.
func newField(fieldType pdf.Name) acroform.Field {
	return formhooks.NewField(fieldType).(acroform.Field)
}

// field reads a field dictionary from a PDF file and returns the matching
// concrete [acroform.Field] type.
//
// Always invoke this via [pdf.ExtractorGet] so that indirect references are
// resolved and cycle detection covers the field hierarchy.
func field(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, _ bool) (acroform.Field, error) {
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
	if err := decodeFieldTypeEntries(x, path, dict, f); err != nil {
		return nil, err
	}
	return f, nil
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

// fieldTypeOf reads a field dictionary's effective field type. The type is
// inheritable: a field without its own /FT takes the type of the nearest
// ancestor (via /Parent) that sets one, so that the field's type-specific
// entries are preserved. An unrecognised or absent type yields the empty name,
// leaving the field typeless (a bare [acroform.FieldCommon]) so that it stays
// writable.
func fieldTypeOf(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) pdf.Name {
	if name, _ := pdf.Optional(x.GetName(path, fieldInherited(x.R, dict, "FT"))); isValidFieldType(name) {
		return name
	}
	return ""
}

// fieldInherited returns the value of an inheritable field attribute: the
// dictionary's own value for key, or the value of the nearest ancestor (via
// /Parent links) that sets one. The result may be an indirect reference.
func fieldInherited(r pdf.Getter, dict pdf.Dict, key pdf.Name) pdf.Object {
	visited := map[pdf.Reference]bool{}
	for {
		if obj, ok := dict[key]; ok {
			return obj
		}
		ref, ok := dict["Parent"].(pdf.Reference)
		if !ok || visited[ref] {
			return nil
		}
		visited[ref] = true
		parent, err := pdf.GetDict(r, ref)
		if err != nil || parent == nil {
			return nil
		}
		dict = parent
	}
}

// decodeFieldCommonEntries reads the entries common to all field types (the name
// entries, flags, and the field half of /AA) into the field's
// [acroform.FieldCommon]. It does not read /Kids or the merged widget; those are
// handled by the caller.
func decodeFieldCommonEntries(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict, f acroform.Field) error {
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
			c.Ff = optional.New(acroform.FieldFlags(uint32(ff)))
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

// decodeFieldTypeEntries reads the entries specific to a field's concrete type
// (the variable-text entries and the type's own V/DV and other keys) into f. A
// typeless field ([acroform.FieldCommon]) has no type-specific entries.
func decodeFieldTypeEntries(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict, f acroform.Field) error {
	switch f := f.(type) {
	case *acroform.FieldTx:
		if err := decodeVariableText(x, path, dict, &f.VariableText); err != nil {
			return err
		}
		f.V = dict["V"]
		f.DV = dict["DV"]
		if ml, err := pdf.Optional(x.GetInteger(path, dict["MaxLen"])); err != nil {
			return err
		} else if ml > 0 {
			f.MaxLen = int(ml)
		}
		// the Comb flag may be set only with a MaxLen (both inheritable) and
		// with the Multiline, Password and FileSelect flags clear; clear an
		// invalid Comb flag, and materialise an inherited MaxLen, so that the
		// decoded field can always be written back
		ff, ok := f.Ff.Get()
		if !ok {
			i, _ := pdf.Optional(pdf.GetInteger(x.R, fieldInherited(x.R, dict, "Ff")))
			ff = acroform.FieldFlags(uint32(i))
		}
		if ff&acroform.FieldComb != 0 {
			if f.MaxLen == 0 {
				if i, _ := pdf.Optional(pdf.GetInteger(x.R, fieldInherited(x.R, dict, "MaxLen"))); i > 0 {
					f.MaxLen = int(i)
				}
			}
			conflict := ff & (acroform.FieldMultiline | acroform.FieldPassword | acroform.FieldFileSelect)
			if f.MaxLen == 0 || conflict != 0 {
				f.Ff = optional.New(ff &^ acroform.FieldComb)
			}
		}
	case *acroform.FieldChoice:
		if err := decodeVariableText(x, path, dict, &f.VariableText); err != nil {
			return err
		}
		if arr, err := pdf.Optional(x.GetArray(path, dict["Opt"])); err != nil {
			return err
		} else {
			for _, el := range arr {
				if opt, ok := decodeChoiceOption(x, path, el); ok {
					f.Opt = append(f.Opt, opt)
				}
			}
		}
		if ti, err := pdf.Optional(x.GetInteger(path, dict["TI"])); err != nil {
			return err
		} else if ti > 0 {
			f.TopIndex = int(ti)
		}
		if arr, err := pdf.Optional(x.GetArray(path, dict["I"])); err != nil {
			return err
		} else {
			for _, el := range arr {
				if idx, err := pdf.Optional(x.GetInteger(path, el)); err != nil {
					return err
				} else if idx >= 0 {
					f.Selected = append(f.Selected, int(idx))
				}
			}
		}
		f.V = dict["V"]
		f.DV = dict["DV"]
	case *acroform.FieldBtn:
		if err := decodeVariableText(x, path, dict, &f.VariableText); err != nil {
			return err
		}
		if err := decodeExportValues(x, path, dict, &f.Opt); err != nil {
			return err
		}
		if v, err := pdf.Optional(x.GetName(path, dict["V"])); err != nil {
			return err
		} else {
			f.V = v
		}
		if dv, err := pdf.Optional(x.GetName(path, dict["DV"])); err != nil {
			return err
		} else {
			f.DV = dv
		}
	case *acroform.FieldSig:
		f.V = dict["V"]
		f.DV = dict["DV"]
		if lock, err := pdf.ExtractorGetOptional(x, path, dict["Lock"], sigFieldLock); err != nil {
			return err
		} else {
			f.Lock = lock
		}
		if sv, err := pdf.ExtractorGetOptional(x, path, dict["SV"], sigSeedValue); err != nil {
			return err
		} else {
			f.SV = sv
		}
	}
	return nil
}

// decodeVariableText reads the variable-text entries (the default appearance,
// justification, and rich-text attributes) from a PDF dictionary.
func decodeVariableText(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict, v *acroform.VariableText) error {
	if da, err := pdf.Optional(x.GetString(path, dict["DA"])); err != nil {
		return err
	} else {
		v.DefaultAppearance = string(da)
	}

	if q, err := pdf.Optional(x.GetInteger(path, dict["Q"])); err != nil {
		return err
	} else if q >= 0 && q <= 2 {
		v.Align = pdf.TextAlign(q)
	}

	if ds, err := pdf.Optional(pdf.GetTextString(x.R, dict["DS"])); err != nil {
		return err
	} else {
		v.DefaultStyle = string(ds)
	}

	v.RichValue = dict["RV"]

	return nil
}

// decodeExportValues reads a button field's Opt array of export values into out.
func decodeExportValues(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict, out *[]string) error {
	arr, err := pdf.Optional(x.GetArray(path, dict["Opt"]))
	if err != nil {
		return err
	}
	if len(arr) == 0 {
		return nil
	}
	opt := make([]string, 0, len(arr))
	for _, el := range arr {
		s, err := pdf.Optional(pdf.GetTextString(x.R, el))
		if err != nil {
			return err
		}
		opt = append(opt, string(s))
	}
	*out = opt
	return nil
}

// decodeChoiceOption reads a single /Opt entry, which is either a string (used
// for both export and display) or a two-element [export, display] array. An
// entry that is neither is skipped (ok is false).
func decodeChoiceOption(x *pdf.Extractor, path *pdf.CycleCheck, el pdf.Object) (acroform.ChoiceOption, bool) {
	if arr, err := pdf.Optional(x.GetArray(path, el)); err == nil && len(arr) == 2 {
		export, ok1 := choiceOptionString(x, arr[0])
		display, ok2 := choiceOptionString(x, arr[1])
		if ok1 && ok2 {
			return acroform.ChoiceOption{Export: export, Display: display}, true
		}
		return acroform.ChoiceOption{}, false
	}
	if s, ok := choiceOptionString(x, el); ok {
		return acroform.ChoiceOption{Export: s, Display: s}, true
	}
	return acroform.ChoiceOption{}, false
}

// choiceOptionString reads obj as a text string. It returns false if obj is
// absent or not a string, so that a non-string /Opt entry is skipped rather
// than silently turned into an empty option.
func choiceOptionString(x *pdf.Extractor, obj pdf.Object) (string, bool) {
	resolved, err := pdf.Resolve(x.R, obj)
	if err != nil {
		return "", false
	}
	s, ok := resolved.(pdf.String)
	if !ok {
		return "", false
	}
	return string(s.AsTextString()), true
}

// decodeMergedField decodes one dictionary that is both a form field and its
// single widget annotation (12.5.6.19) into a linked field+widget pair, and
// publishes both typed views under ref so that the page's /Annots entry and the
// field tree share one widget object. It builds both halves directly from the
// dictionary — never resolving ref recursively — so there is no self-cycle.
func decodeMergedField(x *pdf.Extractor, path *pdf.CycleCheck, ref pdf.Reference, dict pdf.Dict) (acroform.Field, *annotation.Widget, error) {
	f := newField(fieldTypeOf(x, path, dict))
	if err := decodeFieldCommonEntries(x, path, dict, f); err != nil {
		return nil, nil, err
	}
	if err := decodeFieldTypeEntries(x, path, dict, f); err != nil {
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
	f.GetFieldCommon().Kids = []acroform.Node{w}
	w.Parent = f
	fc, ac := pdf.StoreOrLoadPair[acroform.Field, annotation.Annotation](x, ref, f, w)
	return fc, ac.(*annotation.Widget), nil
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

// decodeKids reads the /Kids of a field, decoding each child as a widget
// annotation or a sub-field, appending it to the parent's
// [acroform.FieldCommon.Kids] and setting the child's parent link.
func decodeKids(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict, parent acroform.Field) error {
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
			a, err := pdf.Optional(pdf.ExtractorGet(x, path, ref, Annotation))
			if err != nil {
				return err
			}
			if w, ok := a.(*annotation.Widget); ok && w != nil {
				w.Parent = parent
				c.Kids = append(c.Kids, w)
				continue
			}
		}

		child, err := pdf.Optional(pdf.ExtractorGet(x, path, ref, field))
		if err != nil {
			return err
		}
		if child != nil {
			child.GetFieldCommon().Parent = parent
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
