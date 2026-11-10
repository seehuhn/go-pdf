// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
// The walk links every widget to its field, and sets the appearance states of
// button widgets, before any other reader can obtain the widget: widgets are
// decoded under a private type, and [fieldTreeDecoder.publishWidgets] moves
// them into the cache entry [Annotation] reads once the walk is done.
//
// A tree is walked at most once per extractor.  The nodes are cached without
// what decoding them added to the walk's state, so a second walk would find
// them decoded already and record no field or widget references.
type fieldTreeDecoder struct {
	seen  map[pdf.Reference]bool
	byRef map[pdf.Reference]acroform.Field

	// widgetRefs holds the reference of each widget linked to a field
	widgetRefs map[acroform.Widget]pdf.Reference

	// byName holds the terminal fields decoded under each fully qualified
	// name
	byName map[string]*nameGroup
}

// nameGroup holds the terminal fields decoded under one fully qualified name,
// in order.
type nameGroup struct {
	fields []acroform.Field

	// flat records that the text fields of the group hold no line breaks,
	// because one of them is a single-line field
	flat bool
}

func newFieldTreeDecoder() *fieldTreeDecoder {
	return &fieldTreeDecoder{
		seen:       map[pdf.Reference]bool{},
		byRef:      map[pdf.Reference]acroform.Field{},
		widgetRefs: map[acroform.Widget]pdf.Reference{},
		byName:     map[string]*nameGroup{},
	}
}

// treeNode wraps a decoded tree node. The wrapper lets the node itself be nil
// (a dropped field) while keeping the value passed through [pdf.Cursor.Decode]
// a non-nil concrete pointer.  Since the type is private, no decoder outside
// the walk finds the node in the extractor's cache.
type treeNode struct {
	node acroform.Node
}

// widgetKid wraps a widget kid of a terminal field, as decoded by the walk.
// Since the type is private, the widget is cached apart from the value
// [Annotation] returns for the same reference until the walk publishes it.
type widgetKid struct {
	w *annotation.Widget
}

// decodeWidgetKid decodes a widget kid of a terminal field.
func decodeWidgetKid(c pdf.Cursor, obj pdf.Object, _ bool) (*widgetKid, error) {
	dict, err := c.DictTyped(obj, "Annot")
	if err != nil {
		return nil, err
	}
	a, err := decodeAnnotation(c, dict)
	if err != nil {
		return nil, err
	}
	w, _ := a.(*annotation.Widget)
	return &widgetKid{w: w}, nil
}

// publishWidgets publishes the widgets linked by the walk under their
// references, as [Annotation] returns them.  A widget merged with its field is
// published together with the field.
func (d *fieldTreeDecoder) publishWidgets(x *pdf.Extractor) {
	for w, ref := range d.widgetRefs {
		// a merged dictionary is the only one which is both field and widget
		if f, merged := d.byRef[ref]; merged {
			x.StoreOrLoadPair[acroform.Field, annotation.Annotation](ref, f, w.(*annotation.Widget))
		} else {
			x.StoreOrLoad[annotation.Annotation](ref, w.(*annotation.Widget))
		}
	}
}

// decodeOne decodes the tree node at ref.
func (d *fieldTreeDecoder) decodeOne(c pdf.Cursor, ref pdf.Reference, ctx inherited) (acroform.Node, error) {
	res, err := c.DecodeOptional(ref, d.nodeFunc(ctx))
	if err != nil || res == nil {
		return nil, err
	}
	return res.node, nil
}

// nodeFunc returns an extractor function that decodes a tree node with the given
// inherited context. The context is captured per call, so a node reached from
// two contexts keeps the first (the matching duplicate is dropped by seen).
func (d *fieldTreeDecoder) nodeFunc(ctx inherited) func(pdf.Cursor, pdf.Object, bool) (*treeNode, error) {
	return func(c pdf.Cursor, obj pdf.Object, _ bool) (*treeNode, error) {
		node, err := d.decodeNode(c, obj, ctx)
		if err != nil {
			return nil, err
		}
		return &treeNode{node: node}, nil
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
		node, err := d.decodeOne(c, ref, ctx)
		if err != nil {
			return nil, err
		}
		if node != nil {
			roots = append(roots, node)
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
// detection covers the field hierarchy.
func (d *fieldTreeDecoder) decodeNode(c pdf.Cursor, obj pdf.Object, ctx inherited) (acroform.Node, error) {
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
		f, w, err := decodeMergedField(c, dict, ctx, d.byName)
		if err != nil {
			return nil, err
		}
		if f == nil {
			return nil, nil
		}
		d.byRef[p.Ref] = f
		d.widgetRefs[w] = p.Ref
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
		return d.decodeGroup(c, dict, ctx, fieldKids)
	}
	return d.decodeTerminal(c, dict, ctx, widgetKids)
}

// decodeGroup decodes a non-terminal field into a [acroform.Group]. The group's
// own inheritable entries extend the context for its descendants but are
// otherwise dropped (its TU/TM/AA and value entries are not represented). A
// group whose children all drop out is itself dropped.
func (d *fieldTreeDecoder) decodeGroup(c pdf.Cursor, dict pdf.Dict, ctx inherited, fieldKids []pdf.Reference) (acroform.Node, error) {
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
		d.seen[ref] = true
		kid, err := d.decodeOne(c, ref, childCtx)
		if err != nil {
			return nil, err
		}
		if kid != nil {
			g.Children = append(g.Children, kid)
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
func (d *fieldTreeDecoder) decodeTerminal(c pdf.Cursor, dict pdf.Dict, ctx inherited, widgetKids []pdf.Reference) (acroform.Node, error) {
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
		d.seen[ref] = true
		kid, err := pdf.Optional(c.Decode(ref, decodeWidgetKid))
		if err != nil {
			return nil, err
		}
		if kid != nil && kid.w != nil {
			fc.Widgets = append(fc.Widgets, kid.w)
			widgets = append(widgets, kid.w)
			widgetRefs = append(widgetRefs, ref)
		}
	}
	settleButtonValue(f, shared)
	for i, w := range widgets {
		w.Field = f
		d.widgetRefs[w] = widgetRefs[i]
	}
	if p := c.Path(); p != nil {
		d.byRef[p.Ref] = f
	}
	return f, nil
}

// buildTerminal constructs a terminal field from its dictionary, flattening the
// inheritable attributes against ctx. It returns nil if the dictionary is not a
// field of one of the four defined types.
//
// Fields which share a fully qualified name must agree on type and value
// (12.7.4.2). byName holds the fields decoded under each name: a later field
// of a different type than the first is dropped (its widgets survive through
// the page's /Annots), and one of the same type takes the value and default
// value of the first, which is the value a viewer shows for all of them. One
// which cannot hold that value is dropped too; see [shareValue].  A text value
// shared with a single-line field loses its line breaks in all fields of the
// name, so that they keep one value.  The repair is made before the field is
// linked to its widgets and published.  The second return value
// reports that the field took its value from the first: a check box or radio
// button then still has to set its widgets from it, by [settleButtonValue].
func buildTerminal(c pdf.Cursor, dict pdf.Dict, ctx inherited, byName map[string]*nameGroup) (acroform.Field, bool, error) {
	f, err := buildTerminalField(c, dict, ctx)
	if f == nil || err != nil {
		return nil, false, err
	}
	fqn := joinName(ctx.name, f.PartialName())
	group := byName[fqn]
	if group == nil {
		byName[fqn] = &nameGroup{fields: []acroform.Field{f}, flat: isSingleLine(f)}
		return f, false, nil
	}
	if !shareValue(f, group.fields[0]) {
		return nil, false, nil
	}
	group.fields = append(group.fields, f)
	// the fields of a flat group already hold the flattened value
	if !group.flat && isSingleLine(f) {
		for _, g := range group.fields {
			flattenText(g.(*acroform.TextField))
		}
		group.flat = true
	}
	return f, true, nil
}

// isSingleLine reports whether f is a text field without the Multiline flag.
func isSingleLine(f acroform.Field) bool {
	tf, ok := f.(*acroform.TextField)
	return ok && tf.Flags&acroform.FieldMultiline == 0
}

// flattenText replaces each line break in the text and default text of f by
// a space: CR LF, CR and LF each count as one line break.
func flattenText(f *acroform.TextField) {
	for _, v := range []*pdf.StringOrStream{f.Text, f.DefaultText} {
		if v != nil {
			v.Value = v.Value.ReplaceAll("\r\n", " ").ReplaceAll("\r", " ").ReplaceAll("\n", " ")
		}
	}
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
		if f.Text, err = stringOrStreamPtr(c, eff.v); err != nil {
			return nil, err
		}
		if f.DefaultText, err = stringOrStreamPtr(c, eff.dv); err != nil {
			return nil, err
		}
		f.MaxLen = eff.maxLen
		// the value of a password field must never be stored in the file,
		// nor a default value
		if f.Flags&acroform.FieldPassword != 0 {
			f.Text, f.RichText, f.DefaultText = nil, nil, nil
		}
		// a single-line field holds no line breaks
		if f.Flags&acroform.FieldMultiline == 0 {
			flattenText(f)
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
		ff := eff.ff.Normalize(eff.ft)
		// a push button has no value, so /V, /DV and /Opt do not apply
		if ff&acroform.FieldPushbutton != 0 {
			f := &acroform.PushButtonField{}
			f.Name = name
			f.AltName, f.ExportName, f.Flags, f.AA = tu, tm.String(), ff, aa
			if err := fillVariableText(c, dict, eff, &f.VariableText); err != nil {
				return nil, err
			}
			return f, nil
		}
		f := &acroform.ButtonField{}
		f.Name = name
		f.AltName, f.ExportName, f.Flags, f.AA = tu, tm.String(), ff, aa
		if err := fillVariableText(c, dict, eff, &f.VariableText); err != nil {
			return nil, err
		}
		if v, err := pdf.Optional(c.Name(eff.v)); err != nil {
			return nil, err
		} else {
			f.State = v
		}
		if dv, err := pdf.Optional(c.Name(eff.dv)); err != nil {
			return nil, err
		} else {
			f.DefaultState = dv
		}
		if err := decodeExportValues(c, eff.opt, &f.ExportValues); err != nil {
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
		if f.Value, err = choiceFieldValue(c, eff.v); err != nil {
			return nil, err
		}
		if f.DefaultValue, err = choiceFieldValue(c, eff.dv); err != nil {
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
					f.Options = append(f.Options, opt)
				} else {
					dropped = append(dropped, i)
				}
			}
		}
		// /TI and /I index the original /Opt array; move them with the
		// options that remain and drop those left without a target
		if ti, err := pdf.Optional(c.Integer(dict["TI"])); err != nil {
			return nil, err
		} else if idx, ok := remapOptIndex(int(ti), len(f.Options), dropped); ok {
			f.TopIndex = idx
		}
		if arr, err := pdf.Optional(c.Array(dict["I"])); err != nil {
			return nil, err
		} else {
			for _, el := range arr {
				if idx, err := pdf.Optional(c.Integer(el)); err != nil {
					return nil, err
				} else if idx, ok := remapOptIndex(int(idx), len(f.Options), dropped); ok {
					f.Selected = append(f.Selected, idx)
				}
			}
			slices.Sort(f.Selected)
			f.Selected = slices.Compact(f.Selected)
		}
		// a value must name options, except in an editable combo box
		f.Value = keepOptions(f, f.Value)
		f.DefaultValue = keepOptions(f, f.DefaultValue)
		// several selections in a single-selection field leave the intended
		// one unknown; a bad /V takes its /I along, which only restated it
		if f.Flags&acroform.FieldMultiSelect == 0 {
			// only a multi-select field has an explicit empty selection
			if len(f.Value) == 0 {
				f.Value = nil
			}
			if len(f.DefaultValue) == 0 {
				f.DefaultValue = nil
			}
			if len(f.Value) > 1 {
				f.Value, f.Selected = nil, nil
			}
			if len(f.Selected) > 1 {
				f.Selected = nil
			}
			if len(f.DefaultValue) > 1 {
				f.DefaultValue = nil
			}
		}
		// where /V and /I disagree, /V is authoritative; without /V, /I is
		// the only record of the selection
		if f.Value == nil {
			for _, idx := range f.Selected {
				f.Value = append(f.Value, f.Options[idx].Display)
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
	v.RichText = rv
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

// decodeExportValues reads a button field's /Opt array of export values from the
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
			return acroform.ChoiceOption{Export: export.String(), Display: display}, true, nil
		}
		return acroform.ChoiceOption{}, false, nil
	}
	s, ok, err := choiceOptionString(c, el)
	if err != nil || !ok {
		return acroform.ChoiceOption{}, false, err
	}
	return acroform.ChoiceOption{Export: s.String(), Display: s}, true, nil
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

// keepOptions returns the entries of a choice value which are display
// strings of options of f.  A multiple-selection field keeps the entries
// which name an option; in other fields, or if no entry names an option, an
// entry naming none drops the whole value.  An editable combo box keeps any
// value.
func keepOptions(f *acroform.ChoiceField, v []pdf.TextString) []pdf.TextString {
	if f.IsEditable() || len(v) == 0 {
		return v
	}
	kept := slices.DeleteFunc(slices.Clone(v), func(s pdf.TextString) bool {
		return !namesOption(f, s)
	})
	if len(kept) == len(v) {
		return v
	}
	if f.Flags&acroform.FieldMultiSelect == 0 || len(kept) == 0 {
		return nil
	}
	return kept
}

// namesOption reports whether s is the display string of an option of f.
func namesOption(f *acroform.ChoiceField, s pdf.TextString) bool {
	return slices.ContainsFunc(f.Options, func(o acroform.ChoiceOption) bool {
		return o.Display == s
	})
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
// and /Q defaults, the root of field-attribute inheritance. It is used for a
// widget read without its field, where the field tree's top-down context is
// not available.
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
// defaults. It is used for a widget read without its field, where the tree's
// top-down context is unavailable, to find the value a check box or radio
// button shows.  It computes the same values as the top-down walk.
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
// single widget annotation (12.5.6.19) into a linked field+widget pair.  It
// builds both halves directly from the dictionary, without decoding the
// dictionary's own reference again, so there is no self-cycle.  The field's
// inheritable attributes are flattened against ctx; it returns a nil field
// (but a decoded widget) when the effective field type is unknown, or when
// the field cannot hold the value of its namesake.
//
// The pair is not published in the extractor's cache:
// [fieldTreeDecoder.publishWidgets] publishes it once the walk is done, so
// that the page's /Annots entry and the field tree share one widget object.
func decodeMergedField(c pdf.Cursor, dict pdf.Dict, ctx inherited, byName map[string]*nameGroup) (acroform.Field, *annotation.Widget, error) {
	a, err := decodeAnnotation(c, dict)
	if err != nil {
		return nil, nil, err
	}
	w := a.(*annotation.Widget)

	f, shared, err := buildTerminal(c, dict, ctx, byName)
	if err != nil {
		return nil, nil, err
	}
	if f == nil {
		// not a recognisable field: decode as a plain widget only
		return nil, w, nil
	}

	linkWidget(f, w)
	settleButtonValue(f, shared)
	return f, w, nil
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
