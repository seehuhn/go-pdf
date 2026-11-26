// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"maps"
	"slices"
	"strings"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/annotation/decode"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
)

// valueKeys lists the entries of a field which hold its value.
// The order matches fieldNode.sup.
var valueKeys = [...]pdf.Name{"V", "RV", "DV", "I"}

// positions of the entries in valueKeys
const (
	keyV  = 0
	keyRV = 1
	keyDV = 2
	keyI  = 3
)

// formChanges holds the changes to the interactive form.
type formChanges struct {
	// acro is the new AcroForm dictionary, with references into the input,
	// or nil if the form is copied unchanged
	acro pdf.Dict

	rewrites []*formRewrite
}

// formRewrite is a field or widget dictionary which is written again.
type formRewrite struct {
	src, newRef pdf.Reference
	dict        pdf.Dict // source references

	// regen is set if the appearance is replaced by ap and as
	regen bool
	ap    pdf.Native
	as    pdf.Name
}

// fieldNode is a node of the field tree of the input.
type fieldNode struct {
	ref    pdf.Reference // 0 for a direct dictionary
	dict   pdf.Dict
	parent *fieldNode
	kids   []fieldKid
	name   string // fully qualified name
	named  bool   // the dictionary has a partial name
	ft     pdf.Name

	terminal bool
	merged   bool // the field dictionary is also the widget

	// sup holds, for each of valueKeys, the nearest node from this one
	// upwards which has the entry
	sup [len(valueKeys)]*fieldNode

	removed bool

	// lost records, for each of valueKeys, that the field loses the entry
	// it has or inherits, and clear that the entry is removed from this node
	lost  [len(valueKeys)]bool
	clear [len(valueKeys)]bool

	// regen is set if the remaining widgets get a new appearance
	regen bool

	// optSup is the nearest node from this one upwards which has /Opt.  If
	// optSet is set, the node gets the /Opt entry opt, or none for nil;
	// clearOpt is set if the /Opt entry of the node is removed.
	optSup   *fieldNode
	optSet   bool
	opt      pdf.Array
	clearOpt bool
}

// fieldKid is a child of a field.  At most one of field and widget is set;
// an entry with neither is kept as it is.  An orphan is not listed in the
// /Kids array of its parent, but names the parent in its /Parent entry.
type fieldKid struct {
	obj    pdf.Object
	field  *fieldNode
	widget *widgetNode
	orphan bool
}

// widgetNode is a widget annotation which is a kid of a field.
type widgetNode struct {
	ref     pdf.Reference
	dict    pdf.Dict
	deleted bool
}

// formBuilder reads the field tree.
type formBuilder struct {
	red     *redactor
	cur     pdf.Cursor
	deleted map[pdf.Reference]bool
	seen    map[pdf.Reference]bool
	nodes   map[pdf.Reference]*fieldNode
	widgets map[pdf.Reference]*widgetNode

	// all lists every node, parents before their kids
	all []*fieldNode
}

// prepareForms decides how the interactive form changes.
//
// The field tree is read from the /Fields entry of the form.  The fields of
// widgets which this misses, among the deleted annotations and the
// annotations of all pages, are found by following /Parent.  Such fields are
// changed in the same way as the others, but are not added to /Fields.
//
// A field loses its value if one of its widgets is deleted.  The value
// consists of /V, /RV, /DV and /I.  An entry which a field inherits is
// removed from the ancestor which supplies it, and every field which inherits
// the entry from there loses it as well.  Fields with the same fully
// qualified name share their value: an entry which one of them loses, all of
// them lose.  A field whose widgets are all deleted is removed, and so is a
// group whose fields are all removed.  A signature field which has a value
// loses its value in the same way, and the form then loses /SigFlags.  If a
// region was applied on any page, the form loses /XFA, which can hold a copy
// of the field values.
//
// A check box or radio button field whose /Kids lose an entry loses the
// corresponding export value in /Opt, see [formBuilder.filterOpts].
//
// Deleted fields are added to red.deleted.  The remaining widgets of a field
// which loses /V, and of a choice field which loses /I, are written again
// with a new appearance.
func (red *redactor) prepareForms() error {
	cur := pdf.CursorAt(red.x, nil)
	var acro pdf.Dict
	if obj := red.r.GetMeta().Catalog.AcroForm; obj != nil {
		var err error
		acro, err = pdf.Optional(cur.Dict(obj))
		if err != nil {
			return err
		}
	}

	fb := &formBuilder{
		red:     red,
		cur:     cur,
		deleted: map[pdf.Reference]bool{},
		seen:    map[pdf.Reference]bool{},
		nodes:   map[pdf.Reference]*fieldNode{},
		widgets: map[pdf.Reference]*widgetNode{},
	}
	for _, ref := range red.deleted {
		fb.deleted[ref] = true
	}

	var rootKids []fieldKid
	if acro != nil {
		roots, err := pdf.Optional(cur.Array(acro["Fields"]))
		if err != nil {
			return err
		}
		for _, obj := range roots {
			n, err := fb.node(obj, nil)
			if err != nil {
				return err
			}
			rootKids = append(rootKids, fieldKid{obj: obj, field: n})
		}
	}

	// widgets which /Fields does not reach
	for _, ref := range red.deleted {
		if err := fb.attach(ref); err != nil {
			return err
		}
	}
	for _, pg := range red.pages {
		annots, err := pdf.Optional(cur.Array(pg.src["Annots"]))
		if err != nil {
			return err
		}
		for _, obj := range annots {
			if ref, ok := obj.(pdf.Reference); ok {
				if err := fb.attach(ref); err != nil {
					return err
				}
			}
		}
	}

	touched, signed := fb.decide()
	xfa := red.regions && acro["XFA"] != nil
	if !touched && !signed && !xfa {
		return nil
	}

	fb.prune()
	if err := fb.filterOpts(); err != nil {
		return err
	}

	removedRefs := map[pdf.Reference]bool{}
	for _, n := range fb.all {
		if n.removed && n.ref != 0 {
			removedRefs[n.ref] = true
			if !fb.deleted[n.ref] {
				red.deleted = append(red.deleted, n.ref)
			}
		}
	}

	regen, err := fb.regenerate()
	if err != nil {
		return err
	}

	// rewrites of the fields; direct fields are part of their parent
	for _, n := range fb.all {
		if n.removed || n.ref == 0 || !n.dirty() {
			continue
		}
		rw := &formRewrite{src: n.ref, dict: fb.output(n)}
		if r := regen[n]; r != nil && n.merged {
			rw.regen, rw.ap, rw.as = true, r.ap, r.as
			delete(rw.dict, "AP")
			delete(rw.dict, "AS")
		}
		if prev := red.takeAnnotRewrite(n.ref); prev != nil {
			rw.newRef = prev.newRef
			if _, ok := prev.dict["Popup"]; !ok {
				delete(rw.dict, "Popup")
			}
		} else {
			rw.newRef = red.out.Alloc()
			red.redirect(n.ref, rw.newRef)
		}
		red.forms.rewrites = append(red.forms.rewrites, rw)
	}

	// widgets of fields which get a new appearance
	for _, n := range fb.all {
		r := regen[n]
		if r == nil {
			continue
		}
		for _, kid := range n.kids {
			w := kid.widget
			if w == nil || w.deleted || w.ref == 0 {
				continue
			}
			rw := &formRewrite{
				src: w.ref, dict: maps.Clone(w.dict), regen: true,
				ap: r.byWidget[w.ref], as: r.asByWidget[w.ref],
			}
			if prev := red.takeAnnotRewrite(w.ref); prev != nil {
				rw.newRef = prev.newRef
				rw.dict = maps.Clone(prev.dict)
			} else {
				rw.newRef = red.out.Alloc()
				red.redirect(w.ref, rw.newRef)
			}
			delete(rw.dict, "AP")
			delete(rw.dict, "AS")
			red.forms.rewrites = append(red.forms.rewrites, rw)
		}
	}

	if acro == nil {
		return nil
	}

	// the form dictionary
	newFields := pdf.Array{}
	for _, kid := range rootKids {
		n := kid.field
		switch {
		case n == nil:
			newFields = append(newFields, kid.obj)
		case n.removed:
		case n.ref == 0 && n.dirty():
			newFields = append(newFields, fb.output(n))
		default:
			newFields = append(newFields, kid.obj)
		}
	}
	newAcro := maps.Clone(acro)
	newAcro["Fields"] = newFields
	if co, err := pdf.Optional(cur.Array(acro["CO"])); err != nil {
		return err
	} else if co != nil {
		var kept pdf.Array
		for _, obj := range co {
			if ref, ok := obj.(pdf.Reference); ok && removedRefs[ref] {
				continue
			}
			kept = append(kept, obj)
		}
		newAcro["CO"] = kept
	}
	if xfa {
		delete(newAcro, "XFA")
	}
	if signed {
		delete(newAcro, "SigFlags")
	}
	red.forms.acro = newAcro
	return nil
}

// node reads the field tree below obj, whose parent is parent.  It returns
// nil if obj is not a field dictionary, or if it has been read before.  The
// tree is read with an explicit stack, so that its depth is limited only by
// the work budget.
func (fb *formBuilder) node(obj pdf.Object, parent *fieldNode) (*fieldNode, error) {
	// a field kid which is still to be read: kid idx of parent, or the
	// root of the tree for idx < 0
	type pending struct {
		obj    pdf.Object
		parent *fieldNode
		idx    int
	}

	var root *fieldNode
	var created []*fieldNode
	stack := []pending{{obj: obj, parent: parent, idx: -1}}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		n, fieldKids, err := fb.readNode(p.obj, p.parent)
		if err != nil {
			return nil, err
		}
		if n == nil {
			continue
		}
		created = append(created, n)
		if p.idx < 0 {
			root = n
		} else {
			p.parent.kids[p.idx].field = n
		}
		// the kids are pushed in reverse, so that they are read in order
		for _, i := range slices.Backward(fieldKids) {
			stack = append(stack, pending{obj: n.kids[i].obj, parent: n, idx: i})
		}
	}

	for _, n := range created {
		n.terminal = !slices.ContainsFunc(n.kids, func(k fieldKid) bool { return k.field != nil })
		if n.terminal {
			st, err := pdf.Optional(fb.cur.Name(n.dict["Subtype"]))
			if err != nil {
				return nil, err
			}
			n.merged = st == "Widget" && !n.hasWidgets()
		}
	}
	return root, nil
}

// readNode reads the field dictionary obj, whose parent is parent, for
// [formBuilder.node].  The kids of the result which are widgets are set, and
// fieldKids lists the indices of the other kids which are dictionaries.  The
// result is nil if obj is not a dictionary, or if it has been read before.
func (fb *formBuilder) readNode(obj pdf.Object, parent *fieldNode) (*fieldNode, []int, error) {
	ref, _ := obj.(pdf.Reference)
	if ref != 0 {
		if fb.seen[ref] {
			return nil, nil, nil
		}
		fb.seen[ref] = true
	}
	dict, err := pdf.Optional(fb.cur.Dict(obj))
	if err != nil {
		return nil, nil, err
	}
	if dict == nil {
		return nil, nil, nil
	}

	n := &fieldNode{ref: ref, dict: dict, parent: parent}
	if ref != 0 {
		fb.nodes[ref] = n
	}
	fb.all = append(fb.all, n)

	t, err := pdf.Optional(fb.cur.TextString(dict["T"]))
	if err != nil {
		return nil, nil, err
	}
	// a partial name cannot contain the separator
	partial := strings.ReplaceAll(t.String(), ".", "")
	n.named = partial != ""
	switch {
	case parent == nil || parent.name == "":
		n.name = partial
	case partial == "":
		n.name = parent.name
	default:
		n.name = parent.name + "." + partial
	}
	for i, key := range valueKeys {
		has, err := fb.has(dict, key)
		if err != nil {
			return nil, nil, err
		}
		switch {
		case has:
			n.sup[i] = n
		case parent != nil:
			n.sup[i] = parent.sup[i]
		}
	}
	if has, err := fb.has(dict, "Opt"); err != nil {
		return nil, nil, err
	} else if has {
		n.optSup = n
	} else if parent != nil {
		n.optSup = parent.optSup
	}
	if ft, err := pdf.Optional(fb.cur.Name(dict["FT"])); err != nil {
		return nil, nil, err
	} else if ft != "" {
		n.ft = ft
	} else if parent != nil {
		n.ft = parent.ft
	}

	kids, err := pdf.Optional(fb.cur.Array(dict["Kids"]))
	if err != nil {
		return nil, nil, err
	}
	// the qualified names make the work grow with the depth of the tree
	if err := fb.red.charge(int64(len(n.name)) + int64(len(kids)) + 1); err != nil {
		return nil, nil, err
	}
	var fieldKids []int
	for _, kobj := range kids {
		kid := fieldKid{obj: kobj}
		kdict, err := pdf.Optional(fb.cur.Dict(kobj))
		if err != nil {
			return nil, nil, err
		}
		if kdict != nil {
			isWidget, err := fb.isWidgetKid(kdict)
			if err != nil {
				return nil, nil, err
			}
			if isWidget {
				kref, _ := kobj.(pdf.Reference)
				kid.widget = fb.widget(kref, kdict)
			} else {
				fieldKids = append(fieldKids, len(n.kids))
			}
		}
		n.kids = append(n.kids, kid)
	}
	return n, fieldKids, nil
}

// widget records a widget annotation which is a kid of a field.
func (fb *formBuilder) widget(ref pdf.Reference, dict pdf.Dict) *widgetNode {
	w := &widgetNode{ref: ref, dict: dict, deleted: ref != 0 && fb.deleted[ref]}
	if ref != 0 {
		fb.widgets[ref] = w
	}
	return w
}

// attach adds the field of the annotation ref to the field tree, if the
// annotation is a widget which the tree does not reach yet.  The field and
// its ancestors are found by following /Parent.  An ancestor which the tree
// does not reach becomes an orphan kid of its parent, or the root of an extra
// tree.
func (fb *formBuilder) attach(ref pdf.Reference) error {
	if fb.seen[ref] || fb.widgets[ref] != nil {
		return nil
	}
	if err := fb.red.charge(1); err != nil {
		return err
	}
	dict, err := pdf.Optional(fb.cur.Dict(ref))
	if err != nil || dict == nil {
		return err
	}
	if st, err := pdf.Optional(fb.cur.Name(dict["Subtype"])); err != nil || st != "Widget" {
		return err
	}

	// chain[i+1] is the parent of chain[i]
	chain := []pdf.Reference{ref}
	dicts := []pdf.Dict{dict}
	inChain := map[pdf.Reference]bool{ref: true}
	var anchor *fieldNode
	for {
		p, ok := dicts[len(dicts)-1]["Parent"].(pdf.Reference)
		if !ok || inChain[p] || fb.widgets[p] != nil {
			break
		}
		if n := fb.nodes[p]; n != nil {
			anchor = n
			break
		}
		if err := fb.red.charge(1); err != nil {
			return err
		}
		pd, err := pdf.Optional(fb.cur.Dict(p))
		if err != nil {
			return err
		}
		if pd == nil {
			break
		}
		chain = append(chain, p)
		dicts = append(dicts, pd)
		inChain[p] = true
	}

	parent := anchor
	for i, r := range slices.Backward(chain) {
		if n := fb.nodes[r]; n != nil {
			// reached through /Kids of an ancestor attached before
			parent = n
			continue
		}
		if fb.seen[r] || fb.widgets[r] != nil {
			return nil
		}

		if parent == nil {
			if i == 0 {
				// a widget without a parent is a field only if it is merged
				isField, err := fb.hasFieldEntries(dicts[0])
				if err != nil || !isField {
					return err
				}
			}
			n, err := fb.node(r, nil)
			if err != nil || n == nil {
				return err
			}
			parent = n
			continue
		}

		kid := fieldKid{obj: r, orphan: true}
		isWidget := false
		if i == 0 {
			isWidget, err = fb.isWidgetKid(dicts[0])
			if err != nil {
				return err
			}
		}
		if isWidget {
			kid.widget = fb.widget(r, dicts[0])
		} else {
			kid.field, err = fb.node(r, parent)
			if err != nil || kid.field == nil {
				return err
			}
		}
		parent.kids = append(parent.kids, kid)
		parent = kid.field
	}
	return nil
}

// isWidgetKid reports whether a child of a field is a widget annotation
// rather than a field: a widget has the Widget subtype, and neither /T nor
// /Kids nor an entry of a field value.  A kid without a partial name which
// gives a value is a field dictionary merged with its widget: it stands for
// the same field as its parent, and its value is changed in the same way.
func (fb *formBuilder) isWidgetKid(dict pdf.Dict) (bool, error) {
	st, err := pdf.Optional(fb.cur.Name(dict["Subtype"]))
	if err != nil || st != "Widget" {
		return false, err
	}
	for _, key := range append([]pdf.Name{"T", "Kids"}, valueKeys[:]...) {
		if has, err := fb.has(dict, key); err != nil || has {
			return false, err
		}
	}
	return true, nil
}

// hasFieldEntries reports whether a dictionary names a field, or gives its
// type or value.
func (fb *formBuilder) hasFieldEntries(dict pdf.Dict) (bool, error) {
	for _, key := range append([]pdf.Name{"T", "FT"}, valueKeys[:]...) {
		if has, err := fb.has(dict, key); err != nil || has {
			return has, err
		}
	}
	return false, nil
}

// has reports whether dict has an entry for key which is not null.
func (fb *formBuilder) has(dict pdf.Dict, key pdf.Name) (bool, error) {
	if dict[key] == nil {
		return false, nil
	}
	v, err := pdf.Optional(fb.cur.Resolve(dict[key]))
	return v != nil, err
}

// decide marks the fields which lose entries of their value or are removed.
// It reports whether any field is affected by a deleted widget, and whether
// any signature is removed.
//
// Here a field is a terminal field, or another node which has widgets.  The
// entries are shared by name between all nodes, since a node whose kids have
// no partial names stands for the same field as these kids.
func (fb *formBuilder) decide() (touched, signed bool) {
	type loss struct {
		t   *fieldNode
		key int
	}
	var queue []loss
	lose := func(t *fieldNode, i int) {
		if !t.lost[i] {
			t.lost[i] = true
			queue = append(queue, loss{t, i})
		}
	}
	loseAll := func(t *fieldNode) {
		for i := range valueKeys {
			lose(t, i)
		}
	}

	var fields []*fieldNode
	byName := map[string][]*fieldNode{}
	var bySup [len(valueKeys)]map[*fieldNode][]*fieldNode
	var doneName [len(valueKeys)]map[string]bool
	for i := range valueKeys {
		bySup[i] = map[*fieldNode][]*fieldNode{}
		doneName[i] = map[string]bool{}
	}
	for _, n := range fb.all {
		if n.terminal || n.hasWidgets() {
			fields = append(fields, n)
		}
		if n.name != "" {
			byName[n.name] = append(byName[n.name], n)
		}
		for i, s := range n.sup {
			if s != nil {
				bySup[i][s] = append(bySup[i][s], n)
			}
		}
	}

	for _, t := range fields {
		nDel, nKeep := 0, 0
		if t.merged {
			if t.ref != 0 && fb.deleted[t.ref] {
				nDel++
			} else {
				nKeep++
			}
		}
		for _, kid := range t.kids {
			switch {
			case kid.widget == nil:
			case kid.widget.deleted:
				nDel++
			default:
				nKeep++
			}
		}
		if nDel > 0 {
			touched = true
			t.removed = t.terminal && nKeep == 0
			loseAll(t)
		}
		if t.terminal && t.ft == "Sig" && t.sup[keyV] != nil {
			signed = true
			loseAll(t)
		}
	}

	for len(queue) > 0 {
		l := queue[0]
		queue = queue[1:]
		t, i := l.t, l.key
		if s := t.sup[i]; s != nil && !s.clear[i] {
			s.clear[i] = true
			for _, o := range bySup[i][s] {
				lose(o, i)
			}
		}
		if t.name != "" && !doneName[i][t.name] {
			doneName[i][t.name] = true
			for _, o := range byName[t.name] {
				lose(o, i)
			}
		}
	}

	for _, t := range fields {
		shown := t.lost[keyV] || t.lost[keyI] && t.ft == "Ch"
		t.regen = shown && !t.removed && t.ft != "Sig"
	}
	return touched, signed
}

// hasWidgets reports whether the node has a widget among its kids.
func (n *fieldNode) hasWidgets() bool {
	return slices.ContainsFunc(n.kids, func(k fieldKid) bool { return k.widget != nil })
}

// prune marks groups which have nothing left.  A group is removed if all its
// kids, including those which /Kids does not list, are removed fields or
// deleted widgets, so that no kept object names a removed group in /Parent.
func (fb *formBuilder) prune() {
	// fb.all lists the parents before their kids
	for _, n := range slices.Backward(fb.all) {
		if n.terminal {
			continue
		}
		all := true
		for _, kid := range n.kids {
			switch {
			case kid.field != nil:
				all = all && kid.field.removed
			case kid.widget != nil:
				all = all && kid.widget.deleted
			default:
				all = false
			}
		}
		if all {
			n.removed = true
		}
	}
}

// filterOpts removes from the /Opt entries of check box and radio button
// fields the export values of the /Kids entries which are removed.  /Opt has
// one entry for each entry of /Kids (PDF 2.0 section 12.7.5.2.4).  An /Opt
// entry which does not match /Kids is removed.  A field which inherits /Opt
// gets its own; the ancestor loses its /Opt if no other button field uses
// it.
func (fb *formBuilder) filterOpts() error {
	// the button fields which use the /Opt of each node
	users := map[*fieldNode][]*fieldNode{}
	for _, n := range fb.all {
		if n.optSup != nil && !n.removed && n.isButtonField() {
			users[n.optSup] = append(users[n.optSup], n)
		}
	}

	var inherited []*fieldNode
	for _, n := range fb.all {
		if n.removed || n.optSup == nil || !n.isButtonField() {
			continue
		}
		// keep[i] is set if the i-th entry of /Kids is kept
		var keep []bool
		nKept := 0
		for _, kid := range n.kids {
			if !isListed(kid) {
				continue
			}
			k := !(kid.field != nil && kid.field.removed || kid.widget != nil && kid.widget.deleted)
			keep = append(keep, k)
			if k {
				nKept++
			}
		}
		if nKept == len(keep) {
			continue
		}
		if err := fb.red.charge(int64(len(keep))); err != nil {
			return err
		}
		opt, err := pdf.Optional(fb.cur.Array(n.optSup.dict["Opt"]))
		if err != nil {
			return err
		}
		n.optSet = true
		if len(opt) == len(keep) {
			for i, k := range keep {
				if k {
					n.opt = append(n.opt, opt[i])
				}
			}
		}
		if n.optSup != n {
			inherited = append(inherited, n)
		}
	}
	for _, n := range inherited {
		s := n.optSup
		if !slices.ContainsFunc(users[s], func(u *fieldNode) bool { return !u.optSet }) {
			s.clearOpt = true
		}
	}
	return nil
}

// isButtonField reports whether the node is a check box or radio button
// field whose /Kids lists its buttons: widgets, or field dictionaries without
// a partial name.
func (n *fieldNode) isButtonField() bool {
	return n.ft == "Btn" && slices.ContainsFunc(n.kids, func(k fieldKid) bool {
		return isListed(k) && (k.widget != nil || k.field != nil && !k.field.named)
	})
}

// isListed reports whether a kid is listed in the /Kids entry of its parent.
func isListed(kid fieldKid) bool {
	return !kid.orphan
}

// output returns the new dictionary of a field, with references into the
// input.
func (fb *formBuilder) output(n *fieldNode) pdf.Dict {
	d := maps.Clone(n.dict)
	for i, key := range valueKeys {
		if n.clear[i] {
			delete(d, key)
		}
	}
	switch {
	case n.optSet && n.opt != nil:
		d["Opt"] = n.opt
	case n.optSet || n.clearOpt:
		delete(d, "Opt")
	}
	if n.regen && n.merged {
		dropAppearance(d, n.ft)
	}
	if slices.ContainsFunc(n.kids, isListed) {
		var kids pdf.Array
		for _, kid := range n.kids {
			switch {
			case kid.orphan:
			case kid.field != nil && kid.field.removed:
			case kid.widget != nil && kid.widget.deleted:
			case kid.field != nil && kid.field.ref == 0 && kid.field.dirty():
				kids = append(kids, fb.output(kid.field))
			case kid.widget != nil && kid.widget.ref == 0 && n.regen:
				w := maps.Clone(kid.widget.dict)
				dropAppearance(w, n.ft)
				kids = append(kids, w)
			default:
				kids = append(kids, kid.obj)
			}
		}
		d["Kids"] = kids
	}
	return d
}

// dropAppearance removes the appearance of a widget dictionary of a field
// of type ft.
func dropAppearance(d pdf.Dict, ft pdf.Name) {
	delete(d, "AP")
	delete(d, "AS")
	if as := offState(ft); as != "" {
		d["AS"] = as
	}
}

// offState returns the appearance state of a widget of a field of type ft
// which has lost its appearance and value: Off for a button, and none
// otherwise, since only buttons give the state a meaning.
func offState(ft pdf.Name) pdf.Name {
	if ft == "Btn" {
		return "Off"
	}
	return ""
}

// dirty reports whether the dictionary of the node, or of a direct
// descendant, changes.
func (n *fieldNode) dirty() bool {
	if slices.Contains(n.clear[:], true) || n.regen && n.merged || n.optSet || n.clearOpt {
		return true
	}
	for _, kid := range n.kids {
		switch {
		case kid.orphan:
		case kid.field != nil:
			if kid.field.removed || kid.field.ref == 0 && kid.field.dirty() {
				return true
			}
		case kid.widget != nil:
			if kid.widget.deleted || kid.widget.ref == 0 && n.regen {
				return true
			}
		}
	}
	return false
}

// regenAppearance holds the new appearances of the widgets of a field.
type regenAppearance struct {
	ap         pdf.Native // for a merged field
	as         pdf.Name
	byWidget   map[pdf.Reference]pdf.Native
	asByWidget map[pdf.Reference]pdf.Name
}

// regenerate builds the new appearances for the remaining widgets of the
// fields marked by decide.  The fields are read with [decode.FieldAt], so
// that fields which /Fields does not reach get new appearances as well.  A
// widget whose field cannot be read is drawn without field contents.  A
// widget without an area to draw on gets an appearance which draws nothing
// if the output version requires an appearance, and none otherwise.
func (fb *formBuilder) regenerate() (map[*fieldNode]*regenAppearance, error) {
	red := fb.red
	res := map[*fieldNode]*regenAppearance{}
	var todo []*fieldNode
	for _, n := range fb.all {
		if n.regen {
			todo = append(todo, n)
		}
	}
	if len(todo) == 0 {
		return res, nil
	}

	cur := pdf.CursorAt(red.x, nil)
	gen, err := red.generator()
	if err != nil {
		return nil, err
	}
	for _, t := range todo {
		r := &regenAppearance{
			byWidget:   map[pdf.Reference]pdf.Native{},
			asByWidget: map[pdf.Reference]pdf.Name{},
		}
		res[t] = r

		type target struct {
			ref pdf.Reference
			dst func(ap pdf.Native, as pdf.Name)
		}
		var targets []target
		if t.merged && t.ref != 0 {
			targets = append(targets, target{t.ref, func(ap pdf.Native, as pdf.Name) { r.ap, r.as = ap, as }})
		}
		for _, kid := range t.kids {
			if w := kid.widget; w != nil && !w.deleted && w.ref != 0 {
				targets = append(targets, target{w.ref, func(ap pdf.Native, as pdf.Name) {
					r.byWidget[w.ref], r.asByWidget[w.ref] = ap, as
				}})
			}
		}

		var fc *fieldCopy
		widgetAt := map[pdf.Reference]*annotation.Widget{}
		// A dictionary without a partial name is not a field: its widgets
		// are widgets of the nearest ancestor which has one.
		src := t
		for src != nil && !src.named {
			src = src.parent
		}
		if src != nil && src.ref != 0 {
			f, widgets, err := decode.FieldAt(red.x, src.ref)
			if err != nil && !pdf.IsMalformed(err) {
				return nil, err
			}
			if f != nil {
				fc = copyField(f)
				clearField(fc.f, t.lost)
				for w, ref := range widgets {
					if aw := fc.widgets[w]; aw != nil && ref != 0 {
						widgetAt[ref] = aw
					}
				}
				fb.dropDeletedButtons(fc, widgets)
			}
		}
		for _, tg := range targets {
			w := widgetAt[tg.ref]
			if w == nil && fc != nil {
				w, err = fc.orphanWidget(cur, tg.ref)
				if err != nil {
					return nil, err
				}
			}
			if w == nil {
				w, err = plainWidget(cur, tg.ref)
				if err != nil {
					return nil, err
				}
			}
			if w == nil {
				// not readable as a widget annotation
				tg.dst(nil, offState(t.ft))
				continue
			}

			// Every appearance is generated afresh: the rollover and down
			// appearances, and the states of a button, may show the removed
			// value.
			w.Appearance = nil
			w.AppearanceState = ""
			if err := red.charge(nestCost); err != nil {
				return nil, err
			}
			if err := gen.AddAppearance(w); err != nil {
				return nil, err
			}
			if w.Appearance == nil {
				// nothing visible, e.g. a widget without a usable /Rect
				if !annotation.AppearanceRequired("Widget", w.Rect, red.version) {
					tg.dst(nil, offState(t.ft))
					continue
				}
				w.Appearance = emptyAppearance(w.Rect)
			}
			ap, err := red.rm.Embed(w.Appearance)
			if err != nil {
				return nil, err
			}
			tg.dst(ap, w.AppearanceState)
		}
	}
	return res, nil
}

// dropDeletedButtons removes the deleted widgets, and their export values,
// from a copied check box or radio button field; refs gives the references of
// the widgets of the original.  If a widget is removed, the remaining widgets
// lose their appearances in the copy.  The new appearances then number the
// on-states of a radio button field by the positions of the remaining
// widgets, which the /Opt entry made by [formBuilder.filterOpts] expects.
func (fb *formBuilder) dropDeletedButtons(fc *fieldCopy, refs map[acroform.Widget]pdf.Reference) {
	f, ok := fc.f.(*acroform.ButtonField)
	if !ok {
		return
	}
	orig := fc.orig.GetCommon().Widgets
	matched := len(f.ExportValues) == len(orig)
	var widgets []acroform.Widget
	var values []string
	for i, w := range orig {
		if ref := refs[w]; ref != 0 && fb.deleted[ref] {
			continue
		}
		widgets = append(widgets, f.Widgets[i])
		if matched {
			values = append(values, f.ExportValues[i])
		}
	}
	if len(widgets) == len(orig) {
		return
	}
	f.Widgets = widgets
	f.ExportValues = values
	for _, w := range widgets {
		if aw, ok := w.(*annotation.Widget); ok {
			aw.Appearance = nil
			aw.AppearanceState = ""
		}
	}
}

// plainWidget reads the widget annotation ref and returns a copy which
// belongs to no field, so that its appearance shows no field contents.  It
// returns nil if ref is not a widget annotation.
func plainWidget(cur pdf.Cursor, ref pdf.Reference) (*annotation.Widget, error) {
	a, err := pdf.Optional(cur.Decode(ref, decode.Annotation))
	if err != nil {
		return nil, err
	}
	w, ok := a.(*annotation.Widget)
	if !ok {
		return nil, nil
	}
	c := *w
	c.Field = nil
	return &c, nil
}

// emptyAppearance returns an appearance dictionary which draws nothing over
// the given rectangle.
func emptyAppearance(rect pdf.Rectangle) *appearance.Dict {
	return &appearance.Dict{
		Normal: &form.Form{
			BBox:   rect,
			Res:    &content.Resources{},
			Matrix: matrix.Identity,
		},
		SingleUse: true,
	}
}

// fieldCopy is a copy of a decoded field together with its widgets.  The
// decoded values are shared with other readers of the extractor, so
// redaction changes the copies only.
type fieldCopy struct {
	orig acroform.Field
	f    acroform.Field

	// widgets maps each widget of orig to its copy, which is a widget of f
	widgets map[acroform.Widget]*annotation.Widget
}

// copyField copies a decoded field and its widgets.  Values the field and
// the widgets refer to are shared with the originals.
func copyField(orig acroform.Field) *fieldCopy {
	var f acroform.Field
	switch orig := orig.(type) {
	case *acroform.TextField:
		c := *orig
		f = &c
	case *acroform.ButtonField:
		c := *orig
		f = &c
	case *acroform.PushButtonField:
		c := *orig
		f = &c
	case *acroform.ChoiceField:
		c := *orig
		f = &c
	case *acroform.SignatureField:
		c := *orig
		f = &c
	}
	fc := &fieldCopy{orig: orig, f: f, widgets: map[acroform.Widget]*annotation.Widget{}}
	common := f.GetCommon()
	common.Widgets = slices.Clone(common.Widgets)
	for i, w := range common.Widgets {
		if aw, ok := w.(*annotation.Widget); ok {
			c := *aw
			c.Field = f
			fc.widgets[w] = &c
			common.Widgets[i] = &c
		}
	}
	return fc
}

// orphanWidget reads the widget annotation ref, which names the field's
// dictionary in /Parent but which the /Kids of the field do not list, and
// returns a copy of it which is a widget of the copied field.  It returns nil
// if ref is no widget, or a widget of another field.
func (fc *fieldCopy) orphanWidget(cur pdf.Cursor, ref pdf.Reference) (*annotation.Widget, error) {
	a, err := pdf.Optional(cur.Decode(ref, decode.Annotation))
	if err != nil {
		return nil, err
	}
	w, ok := a.(*annotation.Widget)
	if !ok {
		return nil, nil
	}
	if c := fc.widgets[w]; c != nil {
		return c, nil
	}
	if w.Field != nil && w.Field != fc.orig {
		return nil, nil
	}
	c := *w
	c.Field = fc.f
	if w.Field == nil {
		common := fc.f.GetCommon()
		common.Widgets = append(common.Widgets, &c)
	}
	fc.widgets[w] = &c
	return &c, nil
}

// clearField removes from a field the entries of its value which the field
// loses, as marked in lost.
func clearField(f acroform.Field, lost [len(valueKeys)]bool) {
	v, rv, dv, i := lost[keyV], lost[keyRV], lost[keyDV], lost[keyI]
	switch f := f.(type) {
	case *acroform.TextField:
		if v {
			f.Text = nil
		}
		if dv {
			f.DefaultText = nil
		}
	case *acroform.ChoiceField:
		if v {
			f.Value = nil
		}
		if dv {
			f.DefaultValue = nil
		}
		if i {
			f.Selected = nil
		}
	case *acroform.ButtonField:
		if v {
			f.State = ""
		}
		if dv {
			f.DefaultState = ""
		}
	}
	if vt, ok := f.(interface{ GetVariableText() *acroform.VariableText }); ok && rv {
		vt.GetVariableText().RichText = nil
	}
}

// takeAnnotRewrite removes and returns the pending rewrite of a kept
// annotation, so that the caller can write the annotation instead.
func (red *redactor) takeAnnotRewrite(ref pdf.Reference) *annotRewrite {
	rw := red.rewritten[ref]
	if rw == nil || rw.taken {
		return nil
	}
	rw.taken = true
	return rw
}

// writeFormRewrites writes the changed fields and widgets.
func (red *redactor) writeFormRewrites() error {
	for _, rw := range red.forms.rewrites {
		var edit apEdit
		if !rw.regen {
			edit = red.apEdits[annotID{ref: rw.src}]
		}
		dict, err := red.copyAnnotDict(rw.dict, edit)
		if err != nil {
			return err
		}
		if rw.regen {
			if rw.ap != nil {
				dict["AP"] = rw.ap
			}
			if rw.as != "" {
				dict["AS"] = rw.as
			}
		}
		if err := red.out.Put(rw.newRef, dict); err != nil {
			return err
		}
	}
	return nil
}

// catalogOverride returns the replacement for a catalog entry which is not
// copied from the input.  The signatures which /Perms refers to do not match
// the output.  The form dictionary and a changed direct structure tree root
// are replaced by their new versions.
func (red *redactor) catalogOverride(name string) (pdf.Native, bool, error) {
	switch name {
	case "Perms":
		return nil, true, nil
	case "StructTreeRoot":
		if red.structRoot == nil {
			return nil, false, nil
		}
		dict, err := red.structEd.copyElem(red.structRoot)
		return dict, true, err
	case "AcroForm":
		if red.forms.acro == nil {
			return nil, false, nil
		}
		dict, err := red.copier.CopyDict(red.forms.acro)
		return dict, true, err
	}
	return nil, false, nil
}
