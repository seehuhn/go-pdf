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
	"slices"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/opaque"
)

// inherited accumulates the inheritable field attributes of a field's ancestors
// (12.7.4.1, 12.7.4.3), rooted at the interactive form dictionary's document
// defaults (/DA, /Q). The decoder threads it down the field tree so that every
// terminal field can be flattened to its effective values.
type inherited struct {
	name   string // fully qualified name of the parent field
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
func applyOwnContext(ctx inherited, c pdf.Cursor, dict pdf.Dict) (inherited, error) {
	if name, err := pdf.Optional(c.Name(dict["FT"])); err != nil {
		return ctx, err
	} else if isValidFieldType(name) {
		ctx.ft = name
	}
	// an entry that is null or fails to decode is treated as absent, so that
	// the inherited value stays in effect
	if ff, err := c.Integer(dict["Ff"]); err == nil {
		ctx.ff = acroform.FieldFlags(uint32(ff))
	} else if pdf.IsReadError(err) {
		return ctx, err
	}
	if v, err := nonNull(c, dict["V"]); err != nil {
		return ctx, err
	} else if v != nil {
		ctx.v = v
	}
	if dv, err := nonNull(c, dict["DV"]); err != nil {
		return ctx, err
	} else if dv != nil {
		ctx.dv = dv
	}
	if da, err := c.String(dict["DA"]); err == nil && da != nil {
		ctx.da = string(da)
	} else if pdf.IsReadError(err) {
		return ctx, err
	}
	if q, err := c.Integer(dict["Q"]); err == nil && q >= 0 && q <= 2 {
		ctx.q = pdf.TextAlign(q)
	} else if pdf.IsReadError(err) {
		return ctx, err
	}
	if ml, err := c.Integer(dict["MaxLen"]); err == nil && ml > 0 {
		ctx.maxLen = int(ml)
	} else if pdf.IsReadError(err) {
		return ctx, err
	}
	if opt, err := nonNull(c, dict["Opt"]); err != nil {
		return ctx, err
	} else if opt != nil {
		ctx.opt = opt
	}
	return ctx, nil
}

// fieldTreeDecoder decodes one interactive form's field tree. It deduplicates
// nodes across the whole tree (a field reachable from two parents is kept at
// its first position only, so the decoded tree can be written back) and records
// each terminal field by reference so the form's /CO can be resolved against the
// same field values.
//
// The nodes are cached by the extractor, so a second walk of the same tree
// through the same extractor finds them decoded already.  Each node's result
// therefore records what decoding it contributed to the walk's state, and a
// walk replays the results it takes from the cache; see
// [fieldTreeDecoder.replay].
type fieldTreeDecoder struct {
	seen  map[pdf.Reference]bool
	byRef map[pdf.Reference]acroform.Field

	// widgetRefs holds the reference of each widget linked to a field
	widgetRefs map[acroform.Widget]pdf.Reference

	// byName holds the first terminal field decoded under each fully
	// qualified name
	byName map[string]acroform.Field
}

func newFieldTreeDecoder() *fieldTreeDecoder {
	return &fieldTreeDecoder{
		seen:       map[pdf.Reference]bool{},
		byRef:      map[pdf.Reference]acroform.Field{},
		widgetRefs: map[acroform.Widget]pdf.Reference{},
		byName:     map[string]acroform.Field{},
	}
}

// treeResult wraps a decoded tree node. The wrapper lets the node itself be nil
// (a dropped field) while keeping the value passed through [pdf.Cursor.Decode] a
// non-nil concrete pointer, which its cache requires.
//
// The remaining fields record what decoding the node added to the state of the
// walk, so that a later walk which takes the node from the cache can add the
// same.
type treeResult struct {
	node acroform.Node

	// walk is the decoder which produced the result
	walk *fieldTreeDecoder

	// claimed lists the references the node marked as seen: the field kids
	// of a group, or the widget kids of a terminal field
	claimed []pdf.Reference

	// kids holds the results of a group's field kids
	kids []*treeResult

	// field, ref and name are a terminal field, the reference of its
	// dictionary and its fully qualified name; field is nil for any other
	// node
	field acroform.Field
	ref   pdf.Reference
	name  string

	// widgets lists the widgets linked to a terminal field, and widgetRefs
	// their references
	widgets    []acroform.Widget
	widgetRefs []pdf.Reference
}

// claim marks a reference as seen by the node res.
func (d *fieldTreeDecoder) claim(res *treeResult, ref pdf.Reference) {
	d.seen[ref] = true
	res.claimed = append(res.claimed, ref)
}

// addField records the terminal field of the node res.
func (d *fieldTreeDecoder) addField(res *treeResult, f acroform.Field, ref pdf.Reference, name string) {
	res.field, res.ref, res.name = f, ref, name
	d.byRef[ref] = f
}

// addWidget records a widget linked to the terminal field of the node res.
func (d *fieldTreeDecoder) addWidget(res *treeResult, w acroform.Widget, ref pdf.Reference) {
	res.widgets = append(res.widgets, w)
	res.widgetRefs = append(res.widgetRefs, ref)
	d.widgetRefs[w] = ref
}

// replay adds to the walk what decoding the subtree of res added to the walk
// which produced it.  It is called for results taken from the cache.
func (d *fieldTreeDecoder) replay(res *treeResult) {
	for _, ref := range res.claimed {
		d.seen[ref] = true
	}
	if res.field != nil {
		d.byRef[res.ref] = res.field
		if _, ok := d.byName[res.name]; !ok {
			d.byName[res.name] = res.field
		}
	}
	for i, w := range res.widgets {
		d.widgetRefs[w] = res.widgetRefs[i]
	}
	for _, kid := range res.kids {
		d.replay(kid)
	}
}

// decodeOne decodes the node at ref, replaying the result if it comes from
// the cache.
func (d *fieldTreeDecoder) decodeOne(c pdf.Cursor, ref pdf.Reference, ctx inherited) (*treeResult, error) {
	res, err := c.DecodeOptional(ref, d.nodeFunc(ctx))
	if err != nil {
		return nil, err
	}
	if res != nil && res.walk != d {
		d.replay(res)
	}
	return res, nil
}

// nodeFunc returns an extractor function that decodes a tree node with the given
// inherited context. The context is captured per call, so a node reached from
// two contexts keeps the first (the matching duplicate is dropped by seen).
func (d *fieldTreeDecoder) nodeFunc(ctx inherited) func(pdf.Cursor, pdf.Object, bool) (*treeResult, error) {
	return func(c pdf.Cursor, obj pdf.Object, _ bool) (*treeResult, error) {
		res := &treeResult{walk: d}
		node, err := d.decodeNode(c, obj, ctx, res)
		if err != nil {
			return nil, err
		}
		res.node = node
		return res, nil
	}
}

// decodeRoots decodes the /Fields (or another root array) of a form into tree
// nodes, deduplicating and skipping entries that are not references.
func (d *fieldTreeDecoder) decodeRoots(c pdf.Cursor, obj pdf.Object, ctx inherited) ([]acroform.Node, error) {
	arr, err := pdf.Optional(c.Array(obj))
	if err != nil {
		return nil, err
	}
	var roots []acroform.Node
	for _, el := range arr {
		ref, ok := el.(pdf.Reference)
		if !ok || d.seen[ref] {
			continue
		}
		d.seen[ref] = true
		res, err := d.decodeOne(c, ref, ctx)
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
// Always invoke this through [fieldTreeDecoder.nodeFunc] and [pdf.Cursor.Decode]
// so that indirect references are resolved, the depth is bounded, and cycle
// detection covers the field hierarchy.  What the node adds to the walk is
// recorded in res.
func (d *fieldTreeDecoder) decodeNode(c pdf.Cursor, obj pdf.Object, ctx inherited, res *treeResult) (acroform.Node, error) {
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
	merged := false
	if p != nil {
		if merged, err = isMergedFieldDict(c, dict); err != nil {
			return nil, err
		}
	}
	if merged {
		f, w, err := decodeMergedField(c, p.Ref, dict, ctx, d.byName)
		if err != nil {
			return nil, err
		}
		if f == nil {
			return nil, nil
		}
		d.addField(res, f, p.Ref, joinName(ctx.name, f.PartialName()))
		d.addWidget(res, w, p.Ref)
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
		isWidget, err := isWidgetKid(c, kidDict)
		if err != nil {
			return nil, err
		}
		if isWidget {
			widgetKids = append(widgetKids, ref)
		} else {
			fieldKids = append(fieldKids, ref)
		}
	}

	if len(fieldKids) > 0 {
		// a non-terminal field: a group of sub-fields. Any widget kids are
		// dropped from the tree; they survive through the page's /Annots.
		return d.decodeGroup(c, dict, ctx, fieldKids, res)
	}
	return d.decodeTerminal(c, dict, ctx, widgetKids, res)
}

// decodeGroup decodes a non-terminal field into a [acroform.Group]. The group's
// own inheritable entries extend the context for its descendants but are
// otherwise dropped (its TU/TM/AA and value entries are not represented). A
// group whose children all drop out is itself dropped.
func (d *fieldTreeDecoder) decodeGroup(c pdf.Cursor, dict pdf.Dict, ctx inherited, fieldKids []pdf.Reference, res *treeResult) (acroform.Node, error) {
	childCtx, err := applyOwnContext(ctx, c, dict)
	if err != nil {
		return nil, err
	}
	name, err := partialName(c, dict)
	if err != nil {
		return nil, err
	}
	g := &acroform.Group{Name: name}
	childCtx.name = joinName(ctx.name, g.Name)
	for _, ref := range fieldKids {
		if d.seen[ref] {
			continue
		}
		d.claim(res, ref)
		kid, err := d.decodeOne(c, ref, childCtx)
		if err != nil {
			return nil, err
		}
		if kid == nil {
			continue
		}
		res.kids = append(res.kids, kid)
		if kid.node != nil {
			g.Children = append(g.Children, kid.node)
		}
	}
	if len(g.Children) == 0 {
		return nil, nil
	}
	return g, nil
}

// decodeTerminal decodes a terminal field and its widget annotations. It returns
// nil if the field's effective type is unknown, or if it cannot hold the value
// of an earlier field of the same name (the field is dropped; its widget kids
// survive through the page's /Annots).
func (d *fieldTreeDecoder) decodeTerminal(c pdf.Cursor, dict pdf.Dict, ctx inherited, widgetKids []pdf.Reference, res *treeResult) (acroform.Node, error) {
	f, shared, err := buildTerminal(c, dict, ctx, d.byName)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, nil
	}
	fc := f.GetCommon()
	var widgets []*annotation.Widget
	var widgetRefs []pdf.Reference
	for _, ref := range widgetKids {
		if d.seen[ref] {
			continue
		}
		d.claim(res, ref)
		// annotationBody rather than Annotation: the latter reads the form,
		// which is what is being decoded here
		a, err := pdf.Optional(c.Decode(ref, annotationBody))
		if err != nil {
			return nil, err
		}
		if w, ok := a.(*annotation.Widget); ok && w != nil {
			fc.Widgets = append(fc.Widgets, w)
			widgets = append(widgets, w)
			widgetRefs = append(widgetRefs, ref)
		}
	}
	// The widgets point to the field only once it is kept: they are shared
	// through the extractor's cache, and those of a dropped field survive
	// through the page's /Annots.
	if !settleButtonValue(f, shared) {
		return nil, nil
	}
	for i, w := range widgets {
		w.Field = f
		d.addWidget(res, w, widgetRefs[i])
	}
	if p := c.Path(); p != nil {
		d.addField(res, f, p.Ref, joinName(ctx.name, f.PartialName()))
	}
	return f, nil
}

// buildTerminal constructs a terminal field from its dictionary, flattening the
// inheritable attributes against ctx. It returns nil if the dictionary is not a
// field of one of the four defined types.
//
// Fields which share a fully qualified name must agree on type and value
// (12.7.4.2). byName holds the first field decoded under each name: a later
// field of a different type is dropped (its widgets survive through the page's
// /Annots), and one of the same type takes the value and default value of the
// first, which is the value a viewer shows for all of them. One which cannot
// hold that value is dropped too; see [shareValue]. The repair is made before
// the field is linked to its widgets and published. The second return value
// reports that the field took its value from the first: a check box or radio
// button then still has to be checked against its widgets, by
// [settleButtonValue]. A nil byName skips the check; this is used on the page
// side, which does not see the whole tree.
func buildTerminal(c pdf.Cursor, dict pdf.Dict, ctx inherited, byName map[string]acroform.Field) (acroform.Field, bool, error) {
	f, err := buildTerminalField(c, dict, ctx)
	if f == nil || err != nil {
		return nil, false, err
	}
	if byName == nil {
		return f, false, nil
	}
	fqn := joinName(ctx.name, f.PartialName())
	first, ok := byName[fqn]
	if !ok {
		byName[fqn] = f
		return f, false, nil
	}
	if !shareValue(f, first) {
		return nil, false, nil
	}
	return f, true, nil
}

// buildTerminalField constructs a terminal field from its dictionary,
// flattening the inheritable attributes against ctx. It returns nil if the
// dictionary is not a field of one of the four defined types.
func buildTerminalField(c pdf.Cursor, dict pdf.Dict, ctx inherited) (acroform.Field, error) {
	eff, err := applyOwnContext(ctx, c, dict)
	if err != nil {
		return nil, err
	}
	if !isValidFieldType(eff.ft) {
		return nil, nil
	}

	// a dictionary without a partial name is not a field (12.7.4.2)
	name, err := partialName(c, dict)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, nil
	}
	tu, err := pdf.Optional(c.TextString(dict["TU"]))
	if err != nil {
		return nil, err
	}
	tm, err := pdf.Optional(c.TextString(dict["TM"]))
	if err != nil {
		return nil, err
	}
	aa, err := decodeFieldAA(c, dict)
	if err != nil {
		return nil, err
	}

	switch eff.ft {
	case "Tx":
		f := acroform.NewTextField(name)
		f.AltName, f.ExportName, f.Flags, f.AA = tu, tm.String(), eff.ff.Normalize(eff.ft), aa
		if err := fillVariableText(c, dict, eff, &f.VariableText); err != nil {
			return nil, err
		}
		requireDefaultAppearance(&f.VariableText)
		if f.V, err = stringOrStreamPtr(c, eff.v); err != nil {
			return nil, err
		}
		if f.DV, err = stringOrStreamPtr(c, eff.dv); err != nil {
			return nil, err
		}
		f.MaxLen = eff.maxLen
		// the value of a password field must never be stored in the file
		if f.Flags&acroform.FieldPassword != 0 {
			f.V, f.RichValue = nil, nil
		}
		// the Comb flag is valid only with a MaxLen and with Multiline, Password
		// and FileSelect all clear; drop an invalid one so the field stays
		// writable
		if f.Flags&acroform.FieldComb != 0 {
			conflict := f.Flags & (acroform.FieldMultiline | acroform.FieldPassword | acroform.FieldFileSelect)
			if f.MaxLen == 0 || conflict != 0 {
				f.Flags &^= acroform.FieldComb
			}
		}
		return f, nil

	case "Btn":
		f := acroform.NewButtonField(name)
		f.AltName, f.ExportName, f.Flags, f.AA = tu, tm.String(), eff.ff.Normalize(eff.ft), aa
		if err := fillVariableText(c, dict, eff, &f.VariableText); err != nil {
			return nil, err
		}
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
		f.AltName, f.ExportName, f.Flags, f.AA = tu, tm.String(), eff.ff.Normalize(eff.ft), aa
		if err := fillVariableText(c, dict, eff, &f.VariableText); err != nil {
			return nil, err
		}
		requireDefaultAppearance(&f.VariableText)
		if f.V, err = choiceFieldValue(c, eff.v); err != nil {
			return nil, err
		}
		if f.DV, err = choiceFieldValue(c, eff.dv); err != nil {
			return nil, err
		}
		// the choice /Opt is not inheritable; read it from the field itself
		var dropped []int // original indices of unusable /Opt elements
		if arr, err := pdf.Optional(c.Array(dict["Opt"])); err != nil {
			return nil, err
		} else {
			for i, el := range arr {
				opt, ok, err := decodeChoiceOption(c, el)
				if err != nil {
					return nil, err
				}
				if ok {
					f.Opt = append(f.Opt, opt)
				} else {
					dropped = append(dropped, i)
				}
			}
		}
		// /TI and /I index the original /Opt array; move them with the
		// options that remain and drop those left without a target
		if ti, err := pdf.Optional(c.Integer(dict["TI"])); err != nil {
			return nil, err
		} else if idx, ok := remapOptIndex(int(ti), len(f.Opt), dropped); ok {
			f.TopIndex = idx
		}
		if arr, err := pdf.Optional(c.Array(dict["I"])); err != nil {
			return nil, err
		} else {
			for _, el := range arr {
				if idx, err := pdf.Optional(c.Integer(el)); err != nil {
					return nil, err
				} else if idx, ok := remapOptIndex(int(idx), len(f.Opt), dropped); ok {
					f.Selected = append(f.Selected, idx)
				}
			}
			slices.Sort(f.Selected)
			f.Selected = slices.Compact(f.Selected)
		}
		// several selections in a single-selection field leave the intended
		// one unknown; a bad /V takes its /I along, which only restated it
		if f.Flags&acroform.FieldMultiSelect == 0 {
			// only a multi-select field has an explicit empty selection
			if len(f.V) == 0 {
				f.V = nil
			}
			if len(f.DV) == 0 {
				f.DV = nil
			}
			if len(f.V) > 1 {
				f.V, f.Selected = nil, nil
			}
			if len(f.Selected) > 1 {
				f.Selected = nil
			}
			if len(f.DV) > 1 {
				f.DV = nil
			}
		}
		// where /V and /I disagree, /V is authoritative; without /V, /I is
		// the only record of the selection
		if f.V == nil {
			for _, idx := range f.Selected {
				f.V = append(f.V, f.Opt[idx].Display)
			}
		} else if !f.SelectionAgrees() {
			f.Selected = nil
		}
		return f, nil

	case "Sig":
		f := acroform.NewSignatureField(name)
		f.AltName, f.ExportName, f.Flags, f.AA = tu, tm.String(), eff.ff.Normalize(eff.ft), aa
		f.V = sigValue(c, eff.v)
		f.DV = sigValue(c, eff.dv)
		if lock, err := c.DecodeOptional(dict["Lock"], sigFieldLock); err != nil {
			return nil, err
		} else {
			f.Lock = lock
		}
		if sv, err := c.DecodeOptional(dict["SV"], sigSeedValue); err != nil {
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
	aa, err := c.DecodeOptional(dict["AA"], triggers.DecodeForm)
	if err != nil {
		return nil, err
	}
	if aa != nil && aa.IsEmpty() {
		aa = nil
	}
	return aa, nil
}

// sigValue wraps the value of a signature field, which must be a signature
// dictionary.  Any other value is dropped.
func sigValue(c pdf.Cursor, obj pdf.Object) *opaque.Object {
	if d, err := c.Dict(obj); err != nil || d == nil {
		return nil
	}
	return opaque.Extract(c.Extractor(), obj)
}

// nonNull returns obj unchanged, or nil if obj is absent, null, or a reference
// which cannot be resolved.  Read errors are returned.
func nonNull(c pdf.Cursor, obj pdf.Object) (pdf.Object, error) {
	resolved, err := pdf.Optional(c.Resolve(obj))
	if err != nil || resolved == nil {
		return nil, err
	}
	return obj, nil
}

// stringOrStreamPtr decodes an optional "text string or stream" value, returning
// nil when the entry is absent or is neither a string nor a stream.
func stringOrStreamPtr(c pdf.Cursor, obj pdf.Object) (*pdf.StringOrStream, error) {
	resolved, err := pdf.Optional(c.Resolve(obj))
	if err != nil {
		return nil, err
	}
	switch resolved.(type) {
	case pdf.String, *pdf.Stream:
	default:
		return nil, nil
	}
	sos, err := pdf.Optional(c.StringOrStream(resolved))
	if err != nil {
		return nil, err
	}
	return &sos, nil
}

// fillVariableText fills the variable-text attributes of a field. The default
// appearance and quadding come from the effective context; the rich-text
// entries (DS, RV) are not inheritable and are read from the field's own dict.
func fillVariableText(c pdf.Cursor, dict pdf.Dict, eff inherited, v *acroform.VariableText) error {
	v.DefaultAppearance = eff.da
	v.Align = eff.q
	if ds, err := pdf.Optional(c.TextString(dict["DS"])); err != nil {
		return err
	} else {
		v.DefaultStyle = ds.String()
	}
	rv, err := stringOrStreamPtr(c, dict["RV"])
	if err != nil {
		return err
	}
	v.RichValue = rv
	return nil
}

// fieldDefaultAppearance replaces a missing default appearance string, which text
// and choice fields require: auto-sized Helvetica in black.
const fieldDefaultAppearance = "/Helv 0 Tf 0 g"

// requireDefaultAppearance repairs a missing default appearance string.
func requireDefaultAppearance(v *acroform.VariableText) {
	if v.DefaultAppearance == "" {
		v.DefaultAppearance = fieldDefaultAppearance
	}
}

// joinName appends a partial name to a fully qualified name prefix.
func joinName(prefix, partial string) string {
	switch {
	case partial == "":
		return prefix
	case prefix == "":
		return partial
	default:
		return prefix + "." + partial
	}
}

// partialName reads a field's partial name (/T), stripping any period so the
// name can be written back (a partial name must not contain the separator used
// in fully qualified names).
func partialName(c pdf.Cursor, dict pdf.Dict) (string, error) {
	t, err := pdf.Optional(c.TextString(dict["T"]))
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(t.String(), ".", ""), nil
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
		opt = append(opt, s.String())
	}
	*out = opt
	return nil
}

// decodeChoiceOption reads a single /Opt entry, which is either a string (used
// for both export and display) or a two-element [export, display] array. An
// entry that is neither is skipped (ok is false).
func decodeChoiceOption(c pdf.Cursor, el pdf.Object) (acroform.ChoiceOption, bool, error) {
	arr, err := pdf.Optional(c.Array(el))
	if err != nil {
		return acroform.ChoiceOption{}, false, err
	}
	if len(arr) == 2 {
		export, ok1, err := choiceOptionString(c, arr[0])
		if err != nil {
			return acroform.ChoiceOption{}, false, err
		}
		display, ok2, err := choiceOptionString(c, arr[1])
		if err != nil {
			return acroform.ChoiceOption{}, false, err
		}
		if ok1 && ok2 {
			return acroform.ChoiceOption{Export: export, Display: display}, true, nil
		}
		return acroform.ChoiceOption{}, false, nil
	}
	s, ok, err := choiceOptionString(c, el)
	if err != nil || !ok {
		return acroform.ChoiceOption{}, false, err
	}
	return acroform.ChoiceOption{Export: s, Display: s}, true, nil
}

// remapOptIndex translates an index into the original /Opt array to an index
// into the decoded options, given the ascending original indices of the
// elements that were dropped. It reports false if the index pointed at a
// dropped element or lies outside the array.
func remapOptIndex(idx, numOpt int, dropped []int) (int, bool) {
	pos, found := slices.BinarySearch(dropped, idx)
	if found {
		return 0, false
	}
	idx -= pos
	if idx < 0 || idx >= numOpt {
		return 0, false
	}
	return idx, true
}

// choiceFieldValue reads a choice field's /V or /DV entry, which is a bare text
// string for a single selection or an array of text strings for multiple
// selections. A malformed length-one array is accepted as a single selection;
// non-string entries are skipped. The result is nil when the entry is absent
// or unusable, and non-nil but empty for an empty array.
func choiceFieldValue(c pdf.Cursor, obj pdf.Object) ([]pdf.TextString, error) {
	if obj == nil {
		return nil, nil
	}
	resolved, err := c.Resolve(obj)
	if err != nil {
		return nil, err
	}
	if arr, ok := resolved.(pdf.Array); ok {
		if len(arr) == 0 {
			// an explicit empty selection
			return []pdf.TextString{}, nil
		}
		var vals []pdf.TextString
		for _, el := range arr {
			s, err := c.Resolve(el)
			if err != nil {
				return nil, err
			}
			if str, ok := s.(pdf.String); ok {
				vals = append(vals, str.AsTextString())
			}
		}
		return vals, nil
	}
	if str, ok := resolved.(pdf.String); ok {
		return []pdf.TextString{str.AsTextString()}, nil
	}
	return nil, nil
}

// choiceOptionString reads obj as a text string. It returns false if obj is
// absent or not a string, so that a non-string /Opt entry is skipped rather
// than silently turned into an empty option.
func choiceOptionString(c pdf.Cursor, obj pdf.Object) (pdf.TextString, bool, error) {
	resolved, err := pdf.Optional(c.Resolve(obj))
	if err != nil {
		return pdf.TextString{}, false, err
	}
	s, ok := resolved.(pdf.String)
	if !ok {
		return pdf.TextString{}, false, nil
	}
	return s.AsTextString(), true, nil
}

// acroFormDefaults returns the interactive form dictionary's document-wide /DA
// and /Q defaults, the root of field-attribute inheritance. It is used on the
// page side, where the field tree's top-down context is not available.
func acroFormDefaults(c pdf.Cursor) (da string, q pdf.TextAlign, err error) {
	meta := c.Getter().GetMeta()
	if meta == nil || meta.Catalog == nil {
		return "", pdf.TextAlignLeft, nil
	}
	form, err := pdf.Optional(c.Dict(meta.Catalog.AcroForm))
	if err != nil || form == nil {
		return "", pdf.TextAlignLeft, err
	}
	if s, err := pdf.Optional(c.String(form["DA"])); err != nil {
		return "", pdf.TextAlignLeft, err
	} else {
		da = string(s)
	}
	if v, err := pdf.Optional(c.Integer(form["Q"])); err != nil {
		return "", pdf.TextAlignLeft, err
	} else if v >= 0 && v <= 2 {
		q = pdf.TextAlign(v)
	}
	return da, q, nil
}

// inheritedFromChain reconstructs a field's inherited context from its
// /Parent chain, seeded with the interactive form dictionary's /DA and /Q
// defaults. It is used on the page side to flatten a merged field/widget
// reached from a page's /Annots, where the tree's top-down context is
// unavailable. It computes the same values as the top-down walk, so both
// directions produce identical flattened fields.
//
// Each ancestor's context is decoded through [pdf.Cursor.Decode], so it is computed
// once per ancestor and shared by every widget below it, the chain is bounded
// by the extract depth limit, and a cycle ends the chain.  A malformed
// ancestor also ends the chain; read errors are returned.
func inheritedFromChain(c pdf.Cursor, dict pdf.Dict) (inherited, error) {
	if ref, ok := dict["Parent"].(pdf.Reference); ok {
		ctx, err := pdf.Optional(c.Decode(ref, ancestorContext))
		if err != nil {
			return inherited{}, err
		}
		if ctx != nil {
			return *ctx, nil
		}
	}
	da, q, err := acroFormDefaults(c)
	if err != nil {
		return inherited{}, err
	}
	return inherited{da: da, q: q}, nil
}

// ancestorContext decodes the context a field dictionary passes on to its
// descendants: its own inheritable entries applied over the context of its own
// ancestors. A chain cut short by a cycle or the depth limit starts from the
// form defaults at the cut, so the result is always a context and is cached.
func ancestorContext(c pdf.Cursor, obj pdf.Object, _ bool) (*inherited, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	}
	parent, err := inheritedFromChain(c, dict)
	if err != nil {
		return nil, err
	}
	ctx, err := applyOwnContext(parent, c, dict)
	if err != nil {
		return nil, err
	}
	return &ctx, nil
}

// decodeMergedField decodes one dictionary that is both a form field and its
// single widget annotation (12.5.6.19) into a linked field+widget pair, and
// publishes both typed views under ref so that the page's /Annots entry and the
// field tree share one widget object. It builds both halves directly from the
// dictionary — never resolving ref recursively — so there is no self-cycle. The
// field's inheritable attributes are flattened against ctx; it returns a nil
// field (but a decoded widget) when the effective field type is unknown.
func decodeMergedField(c pdf.Cursor, ref pdf.Reference, dict pdf.Dict, ctx inherited, byName map[string]acroform.Field) (acroform.Field, *annotation.Widget, error) {
	w, err := decodeWidgetBody(c, dict)
	if err != nil {
		return nil, nil, err
	}

	f, shared, err := buildTerminal(c, dict, ctx, byName)
	if err != nil {
		return nil, nil, err
	}
	// the merged path returns before decodeAnnotation's own repairs, so the
	// widget half is repaired here
	repairMissingAppearance(w, pdf.GetVersion(c.Getter()))
	if err := repairMissingAppearanceState(c, w, dict); err != nil {
		return nil, nil, err
	}
	if f == nil {
		// not a recognisable field: decode as a plain widget only
		return nil, w, nil
	}

	// link the pair before publishing: StoreOrLoadPair publishes both halves
	// atomically, so the winner's already-linked f/w become the shared pair and
	// a losing concurrent decoder adopts them without mutating shared state.
	linkWidget(f, w)
	if !settleButtonValue(f, shared) {
		// the field cannot hold the value of its namesake: keep the widget
		w.Field = nil
		return nil, w, nil
	}
	fc, ac := c.Extractor().StoreOrLoadPair[acroform.Field, annotation.Annotation](ref, f, w)
	return fc, ac.(*annotation.Widget), nil
}

// isMergedFieldDict reports whether a dictionary is a form field merged with its
// single widget annotation: a Widget annotation that also carries field entries
// and omits /Kids (12.5.6.19, 12.7.4.1).
func isMergedFieldDict(c pdf.Cursor, dict pdf.Dict) (bool, error) {
	if isWidget, err := isWidgetSubtype(c, dict); err != nil || !isWidget {
		return false, err
	}
	if has, err := hasEntry(c, dict, "Kids"); err != nil || has {
		return false, err
	}
	for _, key := range []pdf.Name{"FT", "T", "TU", "TM", "Ff", "V", "DV", "DA", "Q", "MaxLen", "Opt", "Lock", "SV"} {
		if has, err := hasEntry(c, dict, key); err != nil || has {
			return has, err
		}
	}
	return false, nil
}

// isWidgetKid reports whether a child dictionary is a widget annotation of its
// parent rather than a (possibly merged) sub-field. A widget has the Widget
// subtype and neither a partial name (T) nor Kids. A dictionary without T is
// not a field (12.7.4.2), so any other field entries it carries, such as a
// redundant FT, do not make it one.
func isWidgetKid(c pdf.Cursor, dict pdf.Dict) (bool, error) {
	if isWidget, err := isWidgetSubtype(c, dict); err != nil || !isWidget {
		return false, err
	}
	for _, key := range []pdf.Name{"T", "Kids"} {
		if has, err := hasEntry(c, dict, key); err != nil || has {
			return false, err
		}
	}
	return true, nil
}

// isWidgetSubtype reports whether the dictionary's Subtype entry is Widget.
// The entry is resolved, since a name may be written as an indirect object.
func isWidgetSubtype(c pdf.Cursor, dict pdf.Dict) (bool, error) {
	subtype, err := pdf.Optional(c.Name(dict["Subtype"]))
	return subtype == "Widget", err
}

// hasEntry reports whether the dictionary has the given key with a non-null
// value.  An entry whose value is null, or a reference which cannot be
// resolved, counts as absent.
func hasEntry(c pdf.Cursor, dict pdf.Dict, key pdf.Name) (bool, error) {
	v, err := nonNull(c, dict[key])
	return v != nil, err
}
