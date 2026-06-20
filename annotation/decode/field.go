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
)

// inherited accumulates the inheritable field attributes of a field's ancestors
// (12.7.4.1, 12.7.4.3), rooted at the interactive form dictionary's document
// defaults (/DA, /Q). The decoder threads it down the field tree so that every
// terminal field can be flattened to its effective values.
type inherited struct {
	ft     pdf.Name
	ff     acroform.FieldFlags
	v      pdf.Object
	dv     pdf.Object
	da     string
	q      pdf.TextAlign
	maxLen int
	opt    pdf.Object // button fields only; the choice /Opt is not inheritable
}

// applyOwnContext returns ctx overridden by dict's own inheritable entries. It
// is used both to extend the context as the tree is walked down and, for a
// terminal field, to compute the field's own effective values.
func applyOwnContext(ctx inherited, c pdf.Cursor, dict pdf.Dict) inherited {
	if name, _ := pdf.Optional(c.Name(dict["FT"])); isValidFieldType(name) {
		ctx.ft = name
	}
	if _, ok := dict["Ff"]; ok {
		ff, _ := pdf.Optional(c.Integer(dict["Ff"]))
		ctx.ff = acroform.FieldFlags(uint32(ff))
	}
	if v, ok := dict["V"]; ok {
		ctx.v = v
	}
	if dv, ok := dict["DV"]; ok {
		ctx.dv = dv
	}
	if _, ok := dict["DA"]; ok {
		da, _ := pdf.Optional(c.String(dict["DA"]))
		ctx.da = string(da)
	}
	if _, ok := dict["Q"]; ok {
		if q, _ := pdf.Optional(c.Integer(dict["Q"])); q >= 0 && q <= 2 {
			ctx.q = pdf.TextAlign(q)
		}
	}
	if _, ok := dict["MaxLen"]; ok {
		if ml, _ := pdf.Optional(c.Integer(dict["MaxLen"])); ml > 0 {
			ctx.maxLen = int(ml)
		}
	}
	if opt, ok := dict["Opt"]; ok {
		ctx.opt = opt
	}
	return ctx
}

// fieldTreeDecoder decodes one interactive form's field tree. It deduplicates
// nodes across the whole tree (a field reachable from two parents is kept at
// its first position only, so the decoded tree can be written back) and records
// each terminal field by reference so the form's /CO can be resolved against the
// same field values.
type fieldTreeDecoder struct {
	seen  map[pdf.Reference]bool
	byRef map[pdf.Reference]acroform.Field
}

func newFieldTreeDecoder() *fieldTreeDecoder {
	return &fieldTreeDecoder{
		seen:  map[pdf.Reference]bool{},
		byRef: map[pdf.Reference]acroform.Field{},
	}
}

// treeResult wraps a decoded tree node. The wrapper lets the node itself be nil
// (a dropped field) while keeping the value passed through [pdf.Decode] a
// non-nil concrete pointer, which its cache requires.
type treeResult struct {
	node acroform.TreeNode
}

// nodeFunc returns an extractor function that decodes a tree node with the given
// inherited context. The context is captured per call, so a node reached from
// two contexts keeps the first (the matching duplicate is dropped by seen).
func (d *fieldTreeDecoder) nodeFunc(ctx inherited) func(pdf.Cursor, pdf.Object, bool) (*treeResult, error) {
	return func(c pdf.Cursor, obj pdf.Object, _ bool) (*treeResult, error) {
		node, err := d.decodeNode(c, obj, ctx)
		if err != nil {
			return nil, err
		}
		return &treeResult{node: node}, nil
	}
}

// decodeRoots decodes the /Fields (or another root array) of a form into tree
// nodes, deduplicating and skipping entries that are not references.
func (d *fieldTreeDecoder) decodeRoots(c pdf.Cursor, obj pdf.Object, ctx inherited) ([]acroform.TreeNode, error) {
	arr, err := pdf.Optional(c.Array(obj))
	if err != nil {
		return nil, err
	}
	var roots []acroform.TreeNode
	for _, el := range arr {
		ref, ok := el.(pdf.Reference)
		if !ok || d.seen[ref] {
			continue
		}
		d.seen[ref] = true
		res, err := pdf.DecodeOptional(c, ref, d.nodeFunc(ctx))
		if err != nil {
			return nil, err
		}
		if res != nil && res.node != nil {
			roots = append(roots, res.node)
		}
	}
	return roots, nil
}

// decodeNode decodes one field-tree node: a non-terminal field as a
// [acroform.Group], or a terminal field as a concrete [acroform.Field]. A node
// whose effective field type is unknown is dropped (nil is returned).
//
// Always invoke this through [fieldTreeDecoder.nodeFunc] and [pdf.Decode]
// so that indirect references are resolved, the depth is bounded, and cycle
// detection covers the field hierarchy.
func (d *fieldTreeDecoder) decodeNode(c pdf.Cursor, obj pdf.Object, ctx inherited) (acroform.TreeNode, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, nil
	}

	// a field merged with its single widget is one object that is both a field
	// and a Widget annotation; decode it as a linked field+widget pair so the
	// page's /Annots entry and the field tree share one widget object
	p := c.Path()
	if p != nil && isMergedFieldDict(dict) {
		f, _, err := decodeMergedField(c, p.Ref, dict, ctx)
		if err != nil {
			return nil, err
		}
		if f == nil {
			return nil, nil
		}
		d.byRef[p.Ref] = f
		return f, nil
	}

	// partition the children into sub-fields and widget annotations
	kids, err := pdf.Optional(c.Array(dict["Kids"]))
	if err != nil {
		return nil, err
	}
	local := map[pdf.Reference]bool{}
	if p != nil {
		local[p.Ref] = true
	}
	var fieldKids, widgetKids []pdf.Reference
	for _, el := range kids {
		ref, ok := el.(pdf.Reference)
		if !ok || local[ref] {
			continue
		}
		local[ref] = true
		kidDict, err := pdf.Optional(c.Dict(ref))
		if err != nil {
			return nil, err
		}
		if kidDict == nil {
			continue
		}
		if isWidgetKid(kidDict) {
			widgetKids = append(widgetKids, ref)
		} else {
			fieldKids = append(fieldKids, ref)
		}
	}

	if len(fieldKids) > 0 {
		// a non-terminal field: a group of sub-fields. Any widget kids are
		// dropped from the tree; they survive through the page's /Annots.
		return d.decodeGroup(c, dict, ctx, fieldKids)
	}
	return d.decodeTerminal(c, dict, ctx, widgetKids)
}

// decodeGroup decodes a non-terminal field into a [acroform.Group]. The group's
// own inheritable entries extend the context for its descendants but are
// otherwise dropped (its TU/TM/AA and value entries are not represented). A
// group whose children all drop out is itself dropped.
func (d *fieldTreeDecoder) decodeGroup(c pdf.Cursor, dict pdf.Dict, ctx inherited, fieldKids []pdf.Reference) (acroform.TreeNode, error) {
	childCtx := applyOwnContext(ctx, c, dict)
	g := &acroform.Group{Name: partialName(c, dict)}
	for _, ref := range fieldKids {
		if d.seen[ref] {
			continue
		}
		d.seen[ref] = true
		res, err := pdf.DecodeOptional(c, ref, d.nodeFunc(childCtx))
		if err != nil {
			return nil, err
		}
		if res != nil && res.node != nil {
			g.Kids = append(g.Kids, res.node)
		}
	}
	if len(g.Kids) == 0 {
		return nil, nil
	}
	return g, nil
}

// decodeTerminal decodes a terminal field and its widget annotations. It returns
// nil if the field's effective type is unknown (the field is dropped; its widget
// kids survive through the page's /Annots).
func (d *fieldTreeDecoder) decodeTerminal(c pdf.Cursor, dict pdf.Dict, ctx inherited, widgetKids []pdf.Reference) (acroform.TreeNode, error) {
	f, err := buildTerminal(c, dict, ctx)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, nil
	}
	for _, ref := range widgetKids {
		if d.seen[ref] {
			continue
		}
		d.seen[ref] = true
		a, err := pdf.Optional(pdf.Decode(c, ref, Annotation))
		if err != nil {
			return nil, err
		}
		if w, ok := a.(*annotation.Widget); ok && w != nil {
			w.Parent = f
			f.AddWidget(w)
		}
	}
	if p := c.Path(); p != nil {
		d.byRef[p.Ref] = f
	}
	return f, nil
}

// buildTerminal constructs a terminal field from its dictionary, flattening the
// inheritable attributes against ctx. It returns nil if the effective field type
// is not one of the four defined types.
func buildTerminal(c pdf.Cursor, dict pdf.Dict, ctx inherited) (acroform.Field, error) {
	eff := applyOwnContext(ctx, c, dict)
	if !isValidFieldType(eff.ft) {
		return nil, nil
	}

	name := partialName(c, dict)
	tu, _ := pdf.Optional(c.TextString(dict["TU"]))
	tm, _ := pdf.Optional(c.TextString(dict["TM"]))
	aa, err := decodeFieldAA(c, dict)
	if err != nil {
		return nil, err
	}

	switch eff.ft {
	case "Tx":
		f := acroform.NewTextField(name)
		f.TU, f.TM, f.Ff, f.AA = string(tu), string(tm), eff.ff, aa
		fillVariableText(c, dict, eff, &f.VariableText)
		f.V = eff.v
		f.DV = eff.dv
		f.MaxLen = eff.maxLen
		// the Comb flag is valid only with a MaxLen and with Multiline, Password
		// and FileSelect all clear; drop an invalid one so the field stays
		// writable
		if f.Ff&acroform.FieldComb != 0 {
			conflict := f.Ff & (acroform.FieldMultiline | acroform.FieldPassword | acroform.FieldFileSelect)
			if f.MaxLen == 0 || conflict != 0 {
				f.Ff &^= acroform.FieldComb
			}
		}
		return f, nil

	case "Btn":
		f := acroform.NewButtonField(name)
		f.TU, f.TM, f.Ff, f.AA = string(tu), string(tm), eff.ff, aa
		fillVariableText(c, dict, eff, &f.VariableText)
		if v, err := pdf.Optional(c.Name(eff.v)); err != nil {
			return nil, err
		} else {
			f.V = v
		}
		if dv, err := pdf.Optional(c.Name(eff.dv)); err != nil {
			return nil, err
		} else {
			f.DV = dv
		}
		if err := decodeExportValues(c, eff.opt, &f.Opt); err != nil {
			return nil, err
		}
		return f, nil

	case "Ch":
		f := acroform.NewChoiceField(name)
		f.TU, f.TM, f.Ff, f.AA = string(tu), string(tm), eff.ff, aa
		fillVariableText(c, dict, eff, &f.VariableText)
		f.V = eff.v
		f.DV = eff.dv
		// the choice /Opt is not inheritable; read it from the field itself
		if arr, err := pdf.Optional(c.Array(dict["Opt"])); err != nil {
			return nil, err
		} else {
			for _, el := range arr {
				if opt, ok := decodeChoiceOption(c, el); ok {
					f.Opt = append(f.Opt, opt)
				}
			}
		}
		if ti, err := pdf.Optional(c.Integer(dict["TI"])); err != nil {
			return nil, err
		} else if ti > 0 {
			f.TopIndex = int(ti)
		}
		if arr, err := pdf.Optional(c.Array(dict["I"])); err != nil {
			return nil, err
		} else {
			for _, el := range arr {
				if idx, err := pdf.Optional(c.Integer(el)); err != nil {
					return nil, err
				} else if idx >= 0 {
					f.Selected = append(f.Selected, int(idx))
				}
			}
		}
		return f, nil

	case "Sig":
		f := acroform.NewSignatureField(name)
		f.TU, f.TM, f.Ff, f.AA = string(tu), string(tm), eff.ff, aa
		f.V = eff.v
		f.DV = eff.dv
		if lock, err := pdf.DecodeOptional(c, dict["Lock"], sigFieldLock); err != nil {
			return nil, err
		} else {
			f.Lock = lock
		}
		if sv, err := pdf.DecodeOptional(c, dict["SV"], sigSeedValue); err != nil {
			return nil, err
		} else {
			f.SV = sv
		}
		return f, nil

	default:
		return nil, nil
	}
}

// decodeFieldAA reads the field half (K/F/V/C) of a field's additional-actions
// dictionary. In a merged field/widget the entry is shared with the widget half
// (E/X/Fo/Bl/…); the empty result of the split is dropped.
func decodeFieldAA(c pdf.Cursor, dict pdf.Dict) (*triggers.Form, error) {
	aa, err := pdf.DecodeOptional(c, dict["AA"], triggers.DecodeForm)
	if err != nil {
		return nil, err
	}
	if aa != nil && aa.IsEmpty() {
		aa = nil
	}
	return aa, nil
}

// fillVariableText fills the variable-text attributes of a field. The default
// appearance and quadding come from the effective context; the rich-text
// entries (DS, RV) are not inheritable and are read from the field's own dict.
func fillVariableText(c pdf.Cursor, dict pdf.Dict, eff inherited, v *acroform.VariableText) {
	v.DefaultAppearance = eff.da
	v.Align = eff.q
	if ds, err := pdf.Optional(c.TextString(dict["DS"])); err == nil {
		v.DefaultStyle = string(ds)
	}
	v.RichValue = dict["RV"]
}

// partialName reads a field's partial name (/T), stripping any period so the
// name can be written back (a partial name must not contain the separator used
// in fully qualified names).
func partialName(c pdf.Cursor, dict pdf.Dict) string {
	t, _ := pdf.Optional(c.TextString(dict["T"]))
	return strings.ReplaceAll(string(t), ".", "")
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

// decodeExportValues reads a button field's Opt array of export values from the
// given object into out.
func decodeExportValues(c pdf.Cursor, obj pdf.Object, out *[]string) error {
	arr, err := pdf.Optional(c.Array(obj))
	if err != nil {
		return err
	}
	if len(arr) == 0 {
		return nil
	}
	opt := make([]string, 0, len(arr))
	for _, el := range arr {
		s, err := pdf.Optional(c.TextString(el))
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
func decodeChoiceOption(c pdf.Cursor, el pdf.Object) (acroform.ChoiceOption, bool) {
	if arr, err := pdf.Optional(c.Array(el)); err == nil && len(arr) == 2 {
		export, ok1 := choiceOptionString(c, arr[0])
		display, ok2 := choiceOptionString(c, arr[1])
		if ok1 && ok2 {
			return acroform.ChoiceOption{Export: export, Display: display}, true
		}
		return acroform.ChoiceOption{}, false
	}
	if s, ok := choiceOptionString(c, el); ok {
		return acroform.ChoiceOption{Export: s, Display: s}, true
	}
	return acroform.ChoiceOption{}, false
}

// choiceOptionString reads obj as a text string. It returns false if obj is
// absent or not a string, so that a non-string /Opt entry is skipped rather
// than silently turned into an empty option.
func choiceOptionString(c pdf.Cursor, obj pdf.Object) (string, bool) {
	resolved, err := c.Resolve(obj)
	if err != nil {
		return "", false
	}
	s, ok := resolved.(pdf.String)
	if !ok {
		return "", false
	}
	return string(s.AsTextString()), true
}

// acroFormDefaults returns the interactive form dictionary's document-wide /DA
// and /Q defaults, the root of field-attribute inheritance. It is used on the
// page side, where the field tree's top-down context is not available.
func acroFormDefaults(c pdf.Cursor) (da string, q pdf.TextAlign) {
	meta := c.Getter().GetMeta()
	if meta == nil || meta.Catalog == nil {
		return "", pdf.TextAlignLeft
	}
	form, err := pdf.Optional(c.Dict(meta.Catalog.AcroForm))
	if err != nil || form == nil {
		return "", pdf.TextAlignLeft
	}
	if s, err := pdf.Optional(c.String(form["DA"])); err == nil {
		da = string(s)
	}
	if v, err := pdf.Optional(c.Integer(form["Q"])); err == nil && v >= 0 && v <= 2 {
		q = pdf.TextAlign(v)
	}
	return da, q
}

// inheritedFromChain reconstructs a field's inherited context by walking its
// /Parent chain up to the root and seeding it with the interactive form
// dictionary's /DA and /Q defaults. It is used on the page side to flatten a
// merged field/widget reached from a page's /Annots, where the tree's top-down
// context is unavailable. It computes the same values as the top-down walk, so
// both directions produce identical flattened fields.
func inheritedFromChain(c pdf.Cursor, dict pdf.Dict) inherited {
	var chain []pdf.Dict
	visited := map[pdf.Reference]bool{}
	cur := dict
	for {
		ref, ok := cur["Parent"].(pdf.Reference)
		if !ok || visited[ref] {
			break
		}
		visited[ref] = true
		parent, err := c.Dict(ref)
		if err != nil || parent == nil {
			break
		}
		chain = append(chain, parent)
		cur = parent
	}

	da, q := acroFormDefaults(c)
	ctx := inherited{da: da, q: q}
	// apply ancestors from the root down, so a nearer ancestor wins
	for i := len(chain) - 1; i >= 0; i-- {
		ctx = applyOwnContext(ctx, c, chain[i])
	}
	return ctx
}

// decodeMergedField decodes one dictionary that is both a form field and its
// single widget annotation (12.5.6.19) into a linked field+widget pair, and
// publishes both typed views under ref so that the page's /Annots entry and the
// field tree share one widget object. It builds both halves directly from the
// dictionary — never resolving ref recursively — so there is no self-cycle. The
// field's inheritable attributes are flattened against ctx; it returns a nil
// field (but a decoded widget) when the effective field type is unknown.
func decodeMergedField(c pdf.Cursor, ref pdf.Reference, dict pdf.Dict, ctx inherited) (acroform.Field, *annotation.Widget, error) {
	w, err := decodeWidgetBody(c, dict)
	if err != nil {
		return nil, nil, err
	}

	f, err := buildTerminal(c, dict, ctx)
	if err != nil {
		return nil, nil, err
	}
	if f == nil {
		// not a recognisable field: decode as a plain widget only
		return nil, w, nil
	}

	// link the pair before publishing: StoreOrLoadPair publishes both halves
	// atomically, so the winner's already-linked f/w become the shared pair and
	// a losing concurrent decoder adopts them without mutating shared state.
	f.AddWidget(w)
	w.Parent = f
	fc, ac := pdf.StoreOrLoadPair[acroform.Field, annotation.Annotation](c.Extractor(), ref, f, w)
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
