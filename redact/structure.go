// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"cmp"
	"iter"
	"maps"
	"math"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/nametree"
	"seehuhn.de/go/pdf/numtree"
)

// elemKeys lists the entries which a structure element keeps when it loses
// its text: the entries of PDF 2.0 Table 355 except those which can repeat
// removed content (ActualText, Alt, E, Phoneme, T and AF).
var elemKeys = map[pdf.Name]bool{
	"Type": true, "S": true, "P": true, "ID": true, "Ref": true, "Pg": true,
	"K": true, "A": true, "C": true, "R": true, "Lang": true, "NS": true,
	"PhoneticAlphabet": true,
}

// keptOwners lists the standard attribute owners whose attribute objects
// describe geometry or semantics, with the attributes they define (PDF 2.0
// Tables 378 to 385) except those which hold text (Desc, Summary and
// Short).  An element which loses its text keeps the attribute objects of
// these owners, with only the /O entry and the listed attributes, and drops
// all other attribute objects.
var keptOwners = map[pdf.Name]map[pdf.Name]bool{
	"Layout": setOf(
		"Placement", "WritingMode", "BackgroundColor", "BorderColor",
		"BorderStyle", "BorderThickness", "Padding", "Color",
		"SpaceBefore", "SpaceAfter", "StartIndent", "EndIndent",
		"TextIndent", "TextAlign", "BBox", "Width", "Height",
		"BlockAlign", "InlineAlign", "TBorderStyle", "TPadding",
		"BaselineShift", "LineHeight", "TextPosition",
		"TextDecorationColor", "TextDecorationThickness",
		"TextDecorationType", "RubyAlign", "RubyPosition",
		"GlyphOrientationVertical", "ColumnCount", "ColumnGap",
		"ColumnWidths",
	),
	"List":       setOf("ListNumbering", "ContinuedList", "ContinuedFrom"),
	"PrintField": setOf("Role", "Checked", "checked"),
	"Table":      setOf("RowSpan", "ColSpan", "Headers", "Scope"),
	"Artifact":   setOf("Type", "BBox", "Subtype"),
}

func setOf(keys ...pdf.Name) map[pdf.Name]bool {
	res := make(map[pdf.Name]bool, len(keys))
	for _, k := range keys {
		res[k] = true
	}
	return res
}

// A parentReq is a parent tree entry of removed content.  If mcid is
// negative, the entry for key is the structure element itself, otherwise it
// is an array which is indexed by the marked-content identifier.
type parentReq struct {
	key  pdf.Integer
	mcid int
}

// An arrayEdit is an indirect array of kids which is written again.
type arrayEdit struct {
	ref pdf.Reference
	arr pdf.Array
}

// An attrFix is the replacement for an indirect attribute object in the /A
// entry of an element which loses its text.  If obj is nil, the attribute
// object is dropped.
type attrFix struct {
	obj     pdf.Object
	changed bool
}

// An elemEdit is a structure element which is written again.
type elemEdit struct {
	dict    pdf.Dict
	changed bool
}

// structEditor collects the changes to the structure tree.
type structEditor struct {
	red     *redactor
	rootRef pdf.Reference

	edits map[pdf.Reference]*elemEdit
	order []pdf.Reference

	// stripped lists the elements which have lost their text entries
	stripped map[pdf.Reference]bool

	// classes lists the attribute classes which the elements that lose their
	// text use
	classes map[pdf.Name]bool

	// attrFixes holds the replacements for the indirect attribute objects
	// which were processed
	attrFixes map[pdf.Reference]attrFix

	// arrays lists the indirect arrays of kids which are written again
	arrays []arrayEdit

	// entries holds the parent tree entries which are needed, and
	// entryArrays the resolved arrays among them.  A key can have several
	// entries in a malformed tree.
	entries     map[pdf.Integer][]pdf.Object
	entryArrays map[pdf.Integer][]pdf.Array

	// lostMC lists the marked-content sequences which lost content, and
	// lostObj the /StructParent keys of the objects which lost content.
	// pageKeys maps the pages to their /StructParents keys.
	lostMC   map[parentReq]bool
	lostObj  map[pdf.Integer]bool
	pageKeys map[pdf.Reference]pdf.Integer

	// deleted lists the deleted annotations and the removed fields
	deleted map[pdf.Reference]bool

	// failed is set if the structure element for removed content could not
	// be found
	failed bool

	// newRefs and arrayRefs are the output objects of the changed elements
	// and of the changed arrays of kids
	newRefs   map[pdf.Reference]pdf.Reference
	arrayRefs []pdf.Reference
}

// redactStructure removes text which describes removed content from the
// structure tree.  The structure elements which hold removed content, and
// all their ancestors, keep only the entries listed in elemKeys, and lose the
// text held in their own attribute objects and in the attribute classes they
// use (see stripAttrs).  The elements are found through the parent tree and
// /P, and through the content items of the elements which /K reaches from
// the root.
//
// Content items which refer to a deleted object are removed, and so are
// those which refer to a tainted stream: an object reference to such an
// object, and a marked-content reference whose stream or stream owner is
// such an object.  The element of a removed content item loses its text.  A
// content item of a tainted stream is replaced by one content item for each
// redacted copy of the stream, and for its alias.
//
// If the element of some removed content cannot be found, the text entries
// are removed from every element of the tree.  The source references of the
// changed elements are redirected to new objects, so that the copy of the
// structure tree uses them.
func (red *redactor) redactStructure() error {
	rootObj := red.r.GetMeta().Catalog.StructTreeRoot
	if rootObj == nil {
		return nil
	}
	ed := &structEditor{
		red:      red,
		edits:    map[pdf.Reference]*elemEdit{},
		stripped: map[pdf.Reference]bool{},
		lostMC:   map[parentReq]bool{},
		lostObj:  map[pdf.Integer]bool{},
		pageKeys: map[pdf.Reference]pdf.Integer{},
		deleted:  map[pdf.Reference]bool{},
	}
	red.structEd = ed
	ed.rootRef, _ = rootObj.(pdf.Reference)
	root, err := ed.dict(rootObj)
	if err != nil || root == nil {
		return err
	}
	var pt *numtree.FromFile
	if root["ParentTree"] != nil {
		pt, err = numtree.ExtractFromFile(red.r, root["ParentTree"])
		if err != nil {
			if !pdf.IsMalformed(err) {
				return err
			}
			pt = nil
		}
	}
	if err := ed.collect(pt); err != nil {
		return err
	}
	if ed.failed {
		if err := ed.stripAll(root, pt); err != nil {
			return err
		}
	} else if len(red.nulled) > 0 || len(ed.lostMC) > 0 || len(ed.stripped) > 0 {
		if err := ed.walkTree(root); err != nil {
			return err
		}
	}
	if err := ed.stripClasses(root); err != nil {
		return err
	}
	if err := ed.setNextKey(root); err != nil {
		return err
	}
	ed.redirect()
	return nil
}

// collect records the removed content, and edits the structure elements
// which the parent tree gives for it.
func (ed *structEditor) collect(pt *numtree.FromFile) error {
	red := ed.red

	for _, ref := range red.deleted {
		ed.deleted[ref] = true
	}
	for _, pg := range red.pages {
		key, ok, err := red.parentKey(pg.src["StructParents"])
		if err != nil {
			return err
		}
		if ok {
			ed.pageKeys[pg.oldRef] = key
		}
	}

	// the parent tree entries of the removed content, without repetitions
	var reqs []parentReq
	keys := map[pdf.Integer]bool{}
	add := func(key pdf.Integer, mcid int) {
		r := parentReq{key: key, mcid: mcid}
		if mcid >= 0 {
			if ed.lostMC[r] {
				return
			}
			ed.lostMC[r] = true
		} else {
			if ed.lostObj[key] {
				return
			}
			ed.lostObj[key] = true
		}
		reqs = append(reqs, r)
		keys[key] = true
	}
	for _, pg := range red.pages {
		if len(pg.lostMCIDs) > 0 {
			key, ok := ed.pageKeys[pg.oldRef]
			if ok {
				for _, mcid := range pg.lostMCIDs {
					add(key, mcid)
				}
			} else {
				ed.failed = true
			}
		}
		for _, rec := range pg.lostForms {
			if rec.key == unknownKey {
				ed.failed = true
				continue
			}
			add(pdf.Integer(rec.key), rec.mcid)
		}
		for _, key := range pg.lostObjs {
			if key == unknownKey {
				ed.failed = true
				continue
			}
			add(pdf.Integer(key), -1)
		}
	}

	// the parent tree keys of the deleted annotations, without repetitions
	var annotKeys []pdf.Integer
	seenAnnot := map[pdf.Integer]bool{}
	for _, ref := range red.deleted {
		if err := red.charge(1); err != nil {
			return err
		}
		d, err := ed.dict(ref)
		if err != nil {
			return err
		}
		obj := d["StructParent"]
		if obj == nil {
			continue
		}
		key, ok, err := red.parentKey(obj)
		if err != nil {
			return err
		}
		if !ok {
			ed.failed = true
			continue
		}
		if !seenAnnot[key] {
			seenAnnot[key] = true
			annotKeys = append(annotKeys, key)
			keys[key] = true
		}
	}

	if err := ed.readParentTree(pt, keys); err != nil {
		return err
	}

	for _, key := range annotKeys {
		reqs = append(reqs, parentReq{key: key, mcid: -1})
	}
	for _, r := range reqs {
		elems, err := ed.lookup(r)
		if err != nil {
			return err
		}
		for _, elem := range elems {
			if err := ed.stripChain(elem); err != nil {
				return err
			}
		}
	}
	return nil
}

// parentKey resolves the value of a /StructParent or /StructParents entry.
// The result is false if the value is not a valid parent tree key.
func (red *redactor) parentKey(obj pdf.Object) (pdf.Integer, bool, error) {
	v, err := red.getNative(obj)
	if err != nil {
		return 0, false, err
	}
	key, ok := v.(pdf.Integer)
	return key, ok && key >= 0 && key <= math.MaxInt32, nil
}

// readParentTree stores the parent tree entries for the given keys in
// ed.entries.  The tree is read once.  If a key occurs more than once, all
// its entries are stored.
func (ed *structEditor) readParentTree(pt *numtree.FromFile, keys map[pdf.Integer]bool) error {
	ed.entries = make(map[pdf.Integer][]pdf.Object, len(keys))
	ed.entryArrays = map[pdf.Integer][]pdf.Array{}
	if pt == nil || len(keys) == 0 {
		return nil
	}
	for key, val := range pt.All() {
		if err := ed.red.charge(1); err != nil {
			return err
		}
		if keys[key] {
			ed.entries[key] = append(ed.entries[key], val)
		}
	}
	return nil
}

// lookup returns the structure elements for a parent tree entry, which must
// have been read by readParentTree: one for each entry of the key.  If there
// is no such element, ed.failed is set.
func (ed *structEditor) lookup(r parentReq) ([]pdf.Reference, error) {
	vals := ed.entries[r.key]
	if r.mcid >= 0 && ed.entryArrays[r.key] == nil {
		arrays := make([]pdf.Array, len(vals))
		for i, val := range vals {
			v, err := ed.red.getNative(val)
			if err != nil {
				return nil, err
			}
			arrays[i], _ = v.(pdf.Array)
			if err := ed.red.charge(1 + int64(len(arrays[i]))); err != nil {
				return nil, err
			}
		}
		ed.entryArrays[r.key] = arrays
	}
	var res []pdf.Reference
	for i, val := range vals {
		if r.mcid >= 0 {
			a := ed.entryArrays[r.key][i]
			if r.mcid >= len(a) {
				continue
			}
			val = a[r.mcid]
		}
		if ref, _ := val.(pdf.Reference); ref != 0 {
			res = append(res, ref)
		}
	}
	if len(res) == 0 {
		ed.failed = true
	}
	return res, nil
}

// stripChain removes the text entries from an element and its ancestors,
// following /P.  It sets ed.failed if the chain does not lead to the root of
// the tree, or passes an object which is not a structure element.
func (ed *structEditor) stripChain(ref pdf.Reference) error {
	// every step marks a new element, so that a cycle ends the loop
	for {
		if ref == 0 {
			ed.failed = true
			return nil
		}
		if ref == ed.rootRef || ed.stripped[ref] {
			return nil
		}
		ed.stripped[ref] = true
		if err := ed.red.charge(1); err != nil {
			return err
		}
		e, err := ed.edit(ref)
		if err != nil {
			return err
		}
		if e == nil {
			ed.failed = true
			return nil
		}
		if err := ed.stripKeys(e); err != nil {
			return err
		}
		ref, _ = e.dict["P"].(pdf.Reference)
	}
}

// stripAll removes the text entries from all structure elements which can be
// reached from the root, through the /K entries, the parent tree pt and the
// ID tree.  The content items are changed as described at
// [redactor.redactStructure].
func (ed *structEditor) stripAll(root pdf.Dict, pt *numtree.FromFile) error {
	seen := map[pdf.Reference]bool{}
	var queue []pdf.Reference
	visit := func(r pdf.Reference) {
		if !seen[r] {
			seen[r] = true
			queue = append(queue, r)
		}
	}
	pass := &kidPass{
		visit:    func(r, _ pdf.Reference) { visit(r) },
		edit:     true,
		stripAll: true,
	}

	// the root keeps its own entries
	rootEdit := &elemEdit{dict: maps.Clone(root)}
	if ed.rootRef != 0 {
		seen[ed.rootRef] = true
		e, err := ed.editDict(ed.rootRef)
		if err != nil || e == nil {
			return err
		}
		rootEdit = e
	}
	if err := ed.cleanElem(rootEdit, pass, 0); err != nil {
		return err
	}
	if ed.rootRef == 0 && rootEdit.changed {
		ed.red.structRoot = rootEdit.dict
	}
	if err := ed.treeElems(root, pt, visit); err != nil {
		return err
	}

	for len(queue) > 0 {
		ref := queue[0]
		queue = queue[1:]
		if err := ed.red.charge(1); err != nil {
			return err
		}
		e, err := ed.editKid(ref)
		if err != nil {
			return err
		}
		if e == nil {
			// a content item, or a malformed kid
			continue
		}
		ed.stripped[ref] = true
		if err := ed.stripKeys(e); err != nil {
			return err
		}
		if err := ed.cleanElem(e, pass, 0); err != nil {
			return err
		}
	}
	return nil
}

// treeElems passes to visit the structure elements which the parent tree pt
// and the ID tree of the root refer to.  These can include elements which no
// /K entry reaches.
func (ed *structEditor) treeElems(root pdf.Dict, pt *numtree.FromFile, visit func(pdf.Reference)) error {
	red := ed.red
	elem := func(obj pdf.Object) error {
		ref, ok := obj.(pdf.Reference)
		if !ok {
			return nil
		}
		d, err := ed.dict(ref)
		if err != nil || d == nil {
			return err
		}
		isElem, err := ed.isElem(d)
		if isElem {
			visit(ref)
		}
		return err
	}
	// many keys of a parent tree can share one value
	seen := map[pdf.Reference]bool{}
	value := func(obj pdf.Object) error {
		if err := red.charge(1); err != nil {
			return err
		}
		if ref, isRef := obj.(pdf.Reference); isRef {
			if seen[ref] {
				return nil
			}
			seen[ref] = true
		}
		v, err := red.getNative(obj)
		if err != nil {
			return err
		}
		a, isArray := v.(pdf.Array)
		if !isArray {
			return elem(obj)
		}
		for _, item := range a {
			if err := red.charge(1); err != nil {
				return err
			}
			if err := elem(item); err != nil {
				return err
			}
		}
		return nil
	}

	if pt != nil {
		for _, val := range pt.All() {
			if err := value(val); err != nil {
				return err
			}
		}
	}
	if root["IDTree"] == nil {
		return nil
	}
	ids, err := nametree.ExtractFromFile(red.r, root["IDTree"])
	if err != nil {
		if !pdf.IsMalformed(err) {
			return err
		}
		return nil
	}
	if ids == nil {
		return nil
	}
	for _, val := range ids.All() {
		if err := value(val); err != nil {
			return err
		}
	}
	return nil
}

// cleanElem processes the kids of an element with [structEditor.cleanKids],
// and stores the result in e.  The kids are on page pg, unless the element
// names its own page.
func (ed *structEditor) cleanElem(e *elemEdit, pass *kidPass, pg pdf.Reference) error {
	k, ok := e.dict["K"]
	if !ok {
		return nil
	}
	if p, ok := e.dict["Pg"].(pdf.Reference); ok {
		pg = p
	}
	kids, changed, _, err := ed.cleanKids(k, pass, pg)
	if err != nil {
		return err
	}
	if changed {
		setKids(e.dict, kids)
		e.changed = true
	}
	return nil
}

// setKids sets the /K entry of an element, or removes it for nil.
func setKids(d pdf.Dict, kids pdf.Object) {
	if kids == nil {
		delete(d, "K")
	} else {
		d["K"] = kids
	}
}

// walkTree processes every structure element which /K reaches from the root.
// An element loses its text entries if one of its content items names
// removed content, if its element was found through the parent tree, or if
// one of its descendants loses them; the ancestors are those through which
// the walk reaches the element, whatever /P names.  The content items are
// changed as described at [redactor.redactStructure].
//
// The tree is walked twice: first to find the elements and their ancestors,
// then to change them.
func (ed *structEditor) walkTree(root pdf.Dict) error {
	type node struct {
		ref, pg pdf.Reference
		parents []int
		lost    bool
	}
	nodes := []*node{{ref: ed.rootRef}}
	index := map[pdf.Reference]int{}
	if ed.rootRef != 0 {
		index[ed.rootRef] = 0
	}
	cur := 0
	scan := &kidPass{visit: func(ref, pg pdf.Reference) {
		i, ok := index[ref]
		if !ok {
			i = len(nodes)
			index[ref] = i
			nodes = append(nodes, &node{ref: ref, pg: pg})
		}
		nodes[i].parents = append(nodes[i].parents, cur)
	}}
	elemDict := func(i int) (pdf.Dict, error) {
		if i == 0 && ed.rootRef == 0 {
			return root, nil
		}
		if e, ok := ed.edits[nodes[i].ref]; ok {
			return e.dict, nil
		}
		return ed.dict(nodes[i].ref)
	}

	for cur = 0; cur < len(nodes); cur++ {
		if err := ed.red.charge(1); err != nil {
			return err
		}
		n := nodes[cur]
		d, err := elemDict(cur)
		if err != nil {
			return err
		}
		if p, ok := d["Pg"].(pdf.Reference); ok {
			n.pg = p
		}
		k, ok := d["K"]
		if !ok {
			continue
		}
		_, _, n.lost, err = ed.cleanKids(k, scan, n.pg)
		if err != nil {
			return err
		}
	}

	// the elements which lose their text, and their ancestors
	strip := map[pdf.Reference]bool{}
	var queue []int
	mark := func(i int) {
		if i != 0 && !strip[nodes[i].ref] {
			strip[nodes[i].ref] = true
			queue = append(queue, i)
		}
	}
	for i, n := range nodes {
		if n.lost || ed.stripped[n.ref] {
			mark(i)
		}
	}
	for len(queue) > 0 {
		i := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		for _, p := range nodes[i].parents {
			mark(p)
		}
	}

	pass := &kidPass{visit: func(_, _ pdf.Reference) {}, edit: true, strip: strip}
	for i, n := range nodes {
		if err := ed.red.charge(1); err != nil {
			return err
		}
		var e *elemEdit
		if i > 0 && strip[n.ref] {
			var err error
			if e, err = ed.editKid(n.ref); err != nil {
				return err
			}
			if e != nil {
				ed.stripped[n.ref] = true
				if err := ed.stripKeys(e); err != nil {
					return err
				}
			}
		}
		d, err := elemDict(i)
		if err != nil {
			return err
		}
		k, ok := d["K"]
		if !ok {
			continue
		}
		kids, changed, _, err := ed.cleanKids(k, pass, n.pg)
		if err != nil {
			return err
		}
		if !changed {
			continue
		}
		if i == 0 && ed.rootRef == 0 {
			rd, err := ed.rootDict(root)
			if err != nil {
				return err
			}
			setKids(rd, kids)
			continue
		}
		if e == nil {
			if e, err = ed.editDict(n.ref); err != nil {
				return err
			}
		}
		setKids(e.dict, kids)
		e.changed = true
	}
	return nil
}

// rootDict returns the dictionary of the structure tree root root which is
// written to the output, and marks it as changed.
func (ed *structEditor) rootDict(root pdf.Dict) (pdf.Dict, error) {
	if ed.rootRef == 0 {
		if ed.red.structRoot == nil {
			ed.red.structRoot = maps.Clone(root)
		}
		return ed.red.structRoot, nil
	}
	e, err := ed.editDict(ed.rootRef)
	if err != nil {
		return nil, err
	}
	e.changed = true
	return e.dict, nil
}

// stripClasses removes the text from the attribute classes which the
// elements that lost their text use.  Other elements which use these
// classes are affected as well.
func (ed *structEditor) stripClasses(root pdf.Dict) error {
	if len(ed.classes) == 0 {
		return nil
	}
	if err := ed.red.charge(1); err != nil {
		return err
	}
	cm, err := ed.dict(root["ClassMap"])
	if err != nil || cm == nil {
		return err
	}
	var res pdf.Dict
	for _, name := range slices.Sorted(maps.Keys(ed.classes)) {
		val, ok := cm[name]
		if !ok {
			continue
		}
		obj, changed, err := ed.filterAttrs(val)
		if err != nil {
			return err
		}
		if !changed {
			continue
		}
		if res == nil {
			res = maps.Clone(cm)
		}
		if obj == nil {
			delete(res, name)
		} else {
			res[name] = obj
		}
	}
	if res == nil {
		return nil
	}
	rd, err := ed.rootDict(root)
	if err != nil {
		return err
	}
	rd["ClassMap"] = res
	return nil
}

// stripKeys removes the entries not listed in elemKeys from an element, and
// the text from its attribute objects.
func (ed *structEditor) stripKeys(e *elemEdit) error {
	changed, err := ed.stripElem(e.dict)
	if err != nil {
		return err
	}
	e.changed = e.changed || changed
	return nil
}

// stripElem removes the entries not listed in elemKeys from the structure
// element d, and the text from its attribute objects.  The attribute classes
// of d are recorded, to lose their text in [structEditor.stripClasses].  The
// result reports whether d was changed.
func (ed *structEditor) stripElem(d pdf.Dict) (bool, error) {
	if err := ed.red.charge(int64(len(d))); err != nil {
		return false, err
	}
	changed := false
	for k := range d {
		if !elemKeys[k] {
			delete(d, k)
			changed = true
		}
	}
	if err := ed.noteClasses(d["C"]); err != nil {
		return false, err
	}
	c, err := ed.stripAttrs(d)
	if err != nil {
		return false, err
	}
	return changed || c, nil
}

// noteClasses records the attribute classes which the /C entry obj of an
// element names: a class name, or an array of class names, each optionally
// followed by a revision number.
func (ed *structEditor) noteClasses(obj pdf.Object) error {
	if obj == nil {
		return nil
	}
	v, err := ed.red.getNative(obj)
	if err != nil {
		return err
	}
	names, isArray := v.(pdf.Array)
	if !isArray {
		names = pdf.Array{v}
	}
	if err := ed.red.charge(int64(len(names))); err != nil {
		return err
	}
	for _, item := range names {
		iv, err := ed.red.getNative(item)
		if err != nil {
			return err
		}
		if name, ok := iv.(pdf.Name); ok {
			if ed.classes == nil {
				ed.classes = map[pdf.Name]bool{}
			}
			ed.classes[name] = true
		}
	}
	return nil
}

// stripAttrs removes the text from the attribute objects which the /A entry
// of the structure element d holds, see [structEditor.filterAttrs].  The
// result reports whether d was changed.
func (ed *structEditor) stripAttrs(d pdf.Dict) (bool, error) {
	a, ok := d["A"]
	if !ok {
		return false, nil
	}
	obj, changed, err := ed.filterAttrs(a)
	if err != nil || !changed {
		return false, err
	}
	if obj == nil {
		delete(d, "A")
	} else {
		d["A"] = obj
	}
	return true, nil
}

// filterAttrs returns a copy without text of a, an attribute object or an
// array of attribute objects with optional revision numbers.  The attribute
// objects of the owners in keptOwners keep only /O and the attributes listed
// there, and all other attribute objects are dropped.  Malformed parts, and
// the revision numbers of dropped attribute objects, are dropped too.  The
// result is nil if nothing is left, and changed reports whether it differs
// from a.
//
// The attribute objects are not modified.  The result holds direct copies of
// those which change, so that an attribute object shared with other elements
// keeps its entries there.
func (ed *structEditor) filterAttrs(a pdf.Object) (pdf.Object, bool, error) {
	if err := ed.red.charge(1); err != nil {
		return nil, false, err
	}
	v, err := ed.red.getNative(a)
	if err != nil {
		return nil, false, err
	}
	arr, isArray := v.(pdf.Array)
	if !isArray {
		return ed.attr(a, v)
	}

	var res pdf.Array
	changed := false
	// set if the last item kept is an attribute object, which may be
	// followed by its revision number
	revOK := false
	for _, item := range arr {
		if err := ed.red.charge(1); err != nil {
			return nil, false, err
		}
		iv, err := ed.red.getNative(item)
		if err != nil {
			return nil, false, err
		}
		if rev, isInt := iv.(pdf.Integer); isInt {
			if revOK && rev >= 0 {
				res = append(res, item)
			} else {
				changed = true
			}
			revOK = false
			continue
		}
		obj, c, err := ed.attr(item, iv)
		if err != nil {
			return nil, false, err
		}
		changed = changed || c
		revOK = obj != nil
		if obj != nil {
			res = append(res, obj)
		}
	}
	if !changed {
		return a, false, nil
	}
	if len(res) == 0 {
		return nil, true, nil
	}
	return res, true, nil
}

// attr returns the replacement for an attribute object of an element which
// loses its text, where v is the resolved value of item.  The result is nil
// if the attribute object is dropped, and changed reports whether the result
// differs from item.
func (ed *structEditor) attr(item pdf.Object, v pdf.Native) (pdf.Object, bool, error) {
	ref, isRef := item.(pdf.Reference)
	if isRef {
		if fix, ok := ed.attrFixes[ref]; ok {
			return fix.obj, fix.changed, ed.chargeAttr(fix.obj, fix.changed)
		}
	}
	obj, changed, err := ed.fixAttr(item, v)
	if err != nil {
		return nil, false, err
	}
	if isRef {
		if ed.attrFixes == nil {
			ed.attrFixes = map[pdf.Reference]attrFix{}
		}
		ed.attrFixes[ref] = attrFix{obj: obj, changed: changed}
	}
	return obj, changed, ed.chargeAttr(obj, changed)
}

// chargeAttr charges for a copy of an attribute object, which is written
// once for every element using it.
func (ed *structEditor) chargeAttr(obj pdf.Object, changed bool) error {
	d, ok := obj.(pdf.Dict)
	if !changed || !ok {
		return nil
	}
	return ed.red.charge(int64(len(d)))
}

// fixAttr implements attr, without the cache for indirect attribute objects.
func (ed *structEditor) fixAttr(item pdf.Object, v pdf.Native) (pdf.Object, bool, error) {
	var dict pdf.Dict
	isStream := false
	switch v := v.(type) {
	case pdf.Dict:
		dict = v
	case *pdf.Stream:
		dict = v.Dict
		isStream = true
	default:
		return nil, true, nil
	}
	if err := ed.red.charge(int64(len(dict))); err != nil {
		return nil, false, err
	}
	o, err := ed.red.getNative(dict["O"])
	if err != nil {
		return nil, false, err
	}
	owner, _ := o.(pdf.Name)
	allowed, keep := keptOwners[owner]
	if !keep {
		return nil, true, nil
	}

	// The attributes of an attribute stream are the entries of its
	// dictionary, and the stream data has no defined meaning.  An attribute
	// stream is therefore replaced by a direct dictionary.
	changed := isStream
	res := pdf.Dict{"O": owner}
	for k, val := range dict {
		if allowed[k] {
			res[k] = val
		} else if k != "O" {
			changed = true
		}
	}
	if !changed {
		return item, false, nil
	}
	return res, true, nil
}

// A kidPass describes one traversal of the kids of structure elements with
// [structEditor.cleanKids].
type kidPass struct {
	// visit is called for every indirect kid which is not a content item,
	// together with the page which the kid inherits
	visit func(ref, pg pdf.Reference)

	// edit is set if the content items are changed and the changed arrays
	// of kids are recorded.  Otherwise the kids are only examined.
	edit bool

	// If stripAll is set, every direct structure element loses its text.
	// Otherwise a direct element loses its text if it holds removed content,
	// or an indirect element listed in strip.
	stripAll bool
	strip    map[pdf.Reference]bool

	// arrays holds the outcome for the indirect arrays of kids which were
	// processed, so that each is processed once
	arrays map[pdf.Reference]*arrayInfo
}

// An arrayInfo is the outcome of a [kidPass] for an indirect array of kids.
type arrayInfo struct {
	done bool
	lost bool

	// kids lists the calls of visit which the array caused, with the pages
	kids [][2]pdf.Reference
}

// cleanKids processes the kids k of a structure element on page pg.  Indirect
// kids which are not content items are passed to pass.visit.  The result
// lost reports whether a content item names removed content, see
// [structEditor.itemState], or whether a kid is listed in pass.strip.
//
// If pass.edit is set, the content items which refer to deleted objects are
// removed, those which refer to tainted streams are replaced by
// [itemCopies], and direct structure elements lose their text as pass says.
// The result is the new value of the entry, or nil if nothing is left, and
// changed is set if it differs from k.
//
// Nested arrays and direct elements are processed with a stack of frames
// instead of recursion, so that the depth of the nesting is not limited.
// Indirect arrays are processed once, which breaks cycles.
func (ed *structEditor) cleanKids(k pdf.Object, pass *kidPass, pg pdf.Reference) (pdf.Object, bool, bool, error) {
	v, c, l, f, err := ed.cleanKid(k, pass, pg)
	if err != nil || f == nil {
		return v, c, l, err
	}
	stack := []*kidFrame{f}
	for {
		top := stack[len(stack)-1]
		if top.next < len(top.items) {
			item := top.items[top.next]
			top.next++
			v, c, l, f, err := ed.cleanKid(item, pass, top.pg)
			if err != nil {
				return nil, false, false, err
			}
			if f != nil {
				stack = append(stack, f)
			} else {
				top.add(v, c, l)
			}
			continue
		}
		stack = stack[:len(stack)-1]
		v, c, l, err := top.finish()
		if err != nil {
			return nil, false, false, err
		}
		if len(stack) == 0 {
			return v, c, l, nil
		}
		stack[len(stack)-1].add(v, c, l)
	}
}

// A kidFrame is an array of kids or a direct element whose kids are being
// processed by [structEditor.cleanKids].  The kids are on page pg.  The
// results for the items are passed to add, and finish returns the result for
// the frame.
type kidFrame struct {
	items  []pdf.Object
	next   int
	pg     pdf.Reference
	add    func(v pdf.Object, changed, lost bool)
	finish func() (pdf.Object, bool, bool, error)
}

// cleanKid processes one value of the kids of a structure element on page
// pg, for [structEditor.cleanKids].  Either the result is given directly, or
// the frame which processes the nested values is returned.
func (ed *structEditor) cleanKid(k pdf.Object, pass *kidPass, pg pdf.Reference) (pdf.Object, bool, bool, *kidFrame, error) {
	if err := ed.red.charge(1); err != nil {
		return nil, false, false, nil, err
	}
	switch k := k.(type) {
	case pdf.Integer:
		key, ok := ed.pageKeys[pg]
		lost := ok && k >= 0 && k <= math.MaxInt32 && ed.lostMC[parentReq{key: key, mcid: int(k)}]
		return k, false, lost, nil, nil
	case pdf.Reference:
		v, err := ed.red.getNative(k)
		if err != nil {
			return nil, false, false, nil, err
		}
		switch v := v.(type) {
		case pdf.Dict:
			isItem, err := ed.isContentItem(v)
			if err != nil {
				return nil, false, false, nil, err
			}
			if !isItem {
				pass.visit(k, pg)
				return k, false, pass.strip[k], nil, nil
			}
			res, c, l, err := ed.cleanItem(v, pass, pg)
			if err != nil || !c {
				return k, false, l, nil, err
			}
			return res, true, l, nil, nil
		case pdf.Array:
			return ed.indirectArray(k, v, pass, pg)
		}
		return k, false, false, nil, nil
	case pdf.Array:
		return nil, false, false, kidArray(k, pg), nil
	case pdf.Dict:
		isItem, err := ed.isContentItem(k)
		if err != nil {
			return nil, false, false, nil, err
		}
		if isItem {
			res, c, l, err := ed.cleanItem(k, pass, pg)
			if err != nil || !c {
				return k, false, l, nil, err
			}
			return res, true, l, nil, nil
		}
		return ed.directElem(k, pass, pg)
	}
	return k, false, false, nil, nil
}

// indirectArray returns the frame for the indirect array of kids v, which ref
// refers to, for [structEditor.cleanKid].  If the array changes, the copy is
// recorded in ed.arrays.
func (ed *structEditor) indirectArray(ref pdf.Reference, v pdf.Array, pass *kidPass, pg pdf.Reference) (pdf.Object, bool, bool, *kidFrame, error) {
	if info := pass.arrays[ref]; info != nil {
		// an array reached again repeats its outcome, and one which is
		// being processed is part of a cycle
		if info.done {
			for _, kid := range info.kids {
				pass.visit(kid[0], kid[1])
			}
		}
		return ref, false, info.lost, nil, nil
	}
	if pass.arrays == nil {
		pass.arrays = map[pdf.Reference]*arrayInfo{}
	}
	info := &arrayInfo{}
	pass.arrays[ref] = info
	visit := pass.visit
	pass.visit = func(r, p pdf.Reference) {
		info.kids = append(info.kids, [2]pdf.Reference{r, p})
		visit(r, p)
	}
	f := kidArray(v, pg)
	result := f.finish
	f.finish = func() (pdf.Object, bool, bool, error) {
		pass.visit = visit
		res, changed, lost, err := result()
		if err != nil {
			return nil, false, false, err
		}
		info.done, info.lost = true, lost
		if changed && pass.edit {
			arr, _ := res.(pdf.Array)
			if arr == nil {
				arr = pdf.Array{}
			}
			ed.arrays = append(ed.arrays, arrayEdit{ref: ref, arr: arr})
		}
		return ref, false, lost, nil
	}
	return nil, false, false, f, nil
}

// directElem returns the frame for a structure element d which is a direct
// object, for [structEditor.cleanKid].
func (ed *structEditor) directElem(k pdf.Dict, pass *kidPass, pg pdf.Reference) (pdf.Object, bool, bool, *kidFrame, error) {
	d := k
	changed := false
	if pass.edit {
		d = maps.Clone(k)
		if pass.stripAll {
			var err error
			if changed, err = ed.stripElem(d); err != nil {
				return nil, false, false, nil, err
			}
		}
	}
	if p, ok := d["Pg"].(pdf.Reference); ok {
		pg = p
	}
	lost := false
	result := func() (pdf.Object, bool, bool, error) {
		if lost && pass.edit && !pass.stripAll {
			c, err := ed.stripElem(d)
			if err != nil {
				return nil, false, false, err
			}
			changed = changed || c
		}
		if !changed {
			return k, false, lost, nil
		}
		return d, true, lost, nil
	}
	sub, ok := d["K"]
	if !ok {
		v, c, l, err := result()
		return v, c, l, nil, err
	}
	f := &kidFrame{
		items: []pdf.Object{sub},
		pg:    pg,
		add: func(v pdf.Object, c, l bool) {
			if c {
				setKids(d, v)
				changed = true
			}
			lost = lost || l
		},
		finish: result,
	}
	return nil, false, false, f, nil
}

// kidArray returns the frame for an array of kids on page pg.  Items which
// are removed are left out, and an array with nothing left gives nil.
func kidArray(k pdf.Array, pg pdf.Reference) *kidFrame {
	var res pdf.Array
	changed, lost := false, false
	return &kidFrame{
		items: k,
		pg:    pg,
		add: func(v pdf.Object, c, l bool) {
			if v != nil || !c {
				res = append(res, v)
			}
			changed = changed || c
			lost = lost || l
		},
		finish: func() (pdf.Object, bool, bool, error) {
			if !changed {
				return k, false, lost, nil
			}
			if len(res) == 0 {
				return nil, true, lost, nil
			}
			return res, true, lost, nil
		},
	}
}

// cleanItem processes a content item d of an element on page pg, for
// [structEditor.cleanKid].  If pass.edit is set and the item changes, the
// result is its replacement, nil for an item which is removed; changed
// reports whether this is the case.  The result lost is set if the item
// names removed content, see [structEditor.itemState].
func (ed *structEditor) cleanItem(d pdf.Dict, pass *kidPass, pg pdf.Reference) (pdf.Object, bool, bool, error) {
	drop, key, lost, err := ed.itemState(d, pg)
	if err != nil || !pass.edit {
		return nil, false, lost, err
	}
	switch {
	case drop:
		return nil, true, lost, nil
	case key != "":
		src, _ := d[key].(pdf.Reference)
		return &itemCopies{dict: d, key: key, src: src}, true, lost, nil
	}
	return nil, false, lost, nil
}

// itemState examines a content item d of an element on page pg.  The result
// drop is set if the item refers to a deleted object: an object reference to
// a deleted annotation or field, or a marked-content reference whose stream
// owner is deleted.  The result key is the entry, /Obj or /Stm, by which the
// item refers to a tainted stream.  The result lost is set if the item names
// removed content: if it is dropped, if the stream it refers to has no
// redacted copy, if its marked-content sequence lost content, or if its
// object lost content or is tainted without a valid /StructParent key.
func (ed *structEditor) itemState(d pdf.Dict, pg pdf.Reference) (drop bool, key pdf.Name, lost bool, err error) {
	red := ed.red
	tp, err := red.getNative(d["Type"])
	if err != nil {
		return false, "", false, err
	}
	switch tp {
	case pdf.Name("MCR"):
		if own, ok := d["StmOwn"].(pdf.Reference); ok && red.nulled[own] {
			return true, "", true, nil
		}
		var pk pdf.Integer
		var ok bool
		if stmObj, present := d["Stm"]; present {
			stm, _ := stmObj.(pdf.Reference)
			if red.nulled[stm] {
				if ed.deleted[stm] {
					return true, "", true, nil
				}
				key = "Stm"
				lost = len(red.copiesOf[stm]) == 0
			}
			sd, err := ed.streamDict(stmObj)
			if err != nil {
				return false, "", false, err
			}
			if pk, ok, err = red.parentKey(sd["StructParents"]); err != nil {
				return false, "", false, err
			}
		} else {
			if p, isRef := d["Pg"].(pdf.Reference); isRef {
				pg = p
			}
			pk, ok = ed.pageKeys[pg]
		}
		mcid, err := red.getNative(d["MCID"])
		if err != nil {
			return false, "", false, err
		}
		if m, isInt := mcid.(pdf.Integer); ok && isInt && m >= 0 && m <= math.MaxInt32 {
			lost = lost || ed.lostMC[parentReq{key: pk, mcid: int(m)}]
		}
		return false, key, lost, nil

	case pdf.Name("OBJR"):
		obj, _ := d["Obj"].(pdf.Reference)
		if ed.deleted[obj] {
			return true, "", true, nil
		}
		od, err := ed.streamDict(d["Obj"])
		if err != nil {
			return false, "", false, err
		}
		pk, ok, err := red.parentKey(od["StructParent"])
		if err != nil {
			return false, "", false, err
		}
		lost = ok && ed.lostObj[pk]
		if red.nulled[obj] {
			key = "Obj"
			lost = lost || !ok || len(red.copiesOf[obj]) == 0
		}
		return false, key, lost, nil
	}
	return false, "", false, nil
}

// streamDict returns the dictionary of the stream or dictionary obj, or nil.
func (ed *structEditor) streamDict(obj pdf.Object) (pdf.Dict, error) {
	if err := ed.red.charge(1); err != nil {
		return nil, err
	}
	v, err := ed.red.getNative(obj)
	if err != nil {
		return nil, err
	}
	switch v := v.(type) {
	case pdf.Dict:
		return v, nil
	case *pdf.Stream:
		return v.Dict, nil
	}
	return nil, nil
}

// An itemCopies is a content item d whose entry key, /Stm or /Obj, refers to
// the tainted stream src.  In the output it is replaced by one content item
// for each redacted copy of the stream, and for its alias, see
// [structEditor.copyKids].  Since the parent tree entries of the copies
// repeat that of the stream, the copies belong to the same element.
type itemCopies struct {
	dict pdf.Dict
	key  pdf.Name
	src  pdf.Reference
}

// AsPDF implements the [pdf.Object] interface.  An itemCopies refers to
// objects of the output, and only [structEditor.copyKids] can write it.
func (c *itemCopies) AsPDF(pdf.OutputOptions) pdf.Native {
	panic("redact: content item copied without copyKids")
}

// noteCopy records that dst is a redacted copy of the source stream src.
func (red *redactor) noteCopy(src, dst pdf.Reference) {
	if src == 0 {
		return
	}
	if red.copiesOf == nil {
		red.copiesOf = map[pdf.Reference][]pdf.Reference{}
	}
	red.copiesOf[src] = append(red.copiesOf[src], dst)
}

// edit returns the editable copy of a structure element.  The result is nil
// if the reference does not lead to a structure element.
func (ed *structEditor) edit(ref pdf.Reference) (*elemEdit, error) {
	if e, ok := ed.edits[ref]; ok {
		return e, nil
	}
	d, err := ed.dict(ref)
	if err != nil || d == nil {
		return nil, err
	}
	isElem, err := ed.isElem(d)
	if err != nil || !isElem {
		return nil, err
	}
	return ed.editDict(ref)
}

// editKid returns the editable copy of a dictionary reached as a kid of the
// structure tree.  Every such dictionary except a content item is treated as
// a structure element, so that a malformed element still loses its text.
// The result is nil if the reference does not lead to a dictionary, or leads
// to a content item.
func (ed *structEditor) editKid(ref pdf.Reference) (*elemEdit, error) {
	if e, ok := ed.edits[ref]; ok {
		return e, nil
	}
	d, err := ed.dict(ref)
	if err != nil || d == nil {
		return nil, err
	}
	isItem, err := ed.isContentItem(d)
	if err != nil || isItem {
		return nil, err
	}
	return ed.editDict(ref)
}

// isContentItem reports whether the kid dictionary d is not to be treated as
// a structure element: a marked-content reference, an object reference, or
// an object which names another type and has no structure type.  A kid with
// a structure type, or with neither entry, counts as a structure element even
// where other entries are malformed.
func (ed *structEditor) isContentItem(d pdf.Dict) (bool, error) {
	tp, err := ed.red.getNative(d["Type"])
	if err != nil {
		return false, err
	}
	switch tp {
	case nil, pdf.Name("StructElem"):
		return false, nil
	case pdf.Name("MCR"), pdf.Name("OBJR"):
		return true, nil
	}
	s, err := ed.red.getNative(d["S"])
	if err != nil {
		return false, err
	}
	_, hasS := s.(pdf.Name)
	return !hasS, nil
}

// isElem reports whether d is a structure element dictionary, rather than a
// content item or some other object.
func (ed *structEditor) isElem(d pdf.Dict) (bool, error) {
	s, err := ed.red.getNative(d["S"])
	if err != nil {
		return false, err
	}
	if _, ok := s.(pdf.Name); !ok {
		return false, nil
	}
	tp, err := ed.red.getNative(d["Type"])
	if err != nil {
		return false, err
	}
	return tp == nil || tp == pdf.Name("StructElem"), nil
}

// editDict returns the editable copy of a dictionary of the structure tree.
// The result is nil if the reference does not lead to a dictionary.
func (ed *structEditor) editDict(ref pdf.Reference) (*elemEdit, error) {
	if e, ok := ed.edits[ref]; ok {
		return e, nil
	}
	d, err := ed.dict(ref)
	if err != nil || d == nil {
		return nil, err
	}
	e := &elemEdit{dict: maps.Clone(d)}
	ed.edits[ref] = e
	ed.order = append(ed.order, ref)
	return e, nil
}

// dict resolves obj to a dictionary.  The result is nil for a missing or
// malformed object, or for a different type.
func (ed *structEditor) dict(obj pdf.Object) (pdf.Dict, error) {
	v, err := ed.red.getNative(obj)
	if err != nil {
		return nil, err
	}
	d, _ := v.(pdf.Dict)
	return d, nil
}

// getNative resolves references, and turns malformed objects into nil.
func (red *redactor) getNative(obj pdf.Object) (pdf.Native, error) {
	v, err := pdf.Resolve(red.r, obj)
	if err != nil {
		if pdf.IsMalformed(err) {
			return nil, nil
		}
		return nil, err
	}
	return v, nil
}

// redirect allocates the output objects of the changed elements and of the
// changed arrays of kids, and redirects the source objects to them.
func (ed *structEditor) redirect() {
	red := ed.red
	ed.newRefs = map[pdf.Reference]pdf.Reference{}
	for _, ref := range ed.order {
		if !ed.edits[ref].changed {
			continue
		}
		newRef := red.out.Alloc()
		ed.newRefs[ref] = newRef
		red.redirect(ref, newRef)
	}
	ed.arrayRefs = make([]pdf.Reference, len(ed.arrays))
	for i, a := range ed.arrays {
		ed.arrayRefs[i] = red.out.Alloc()
		red.redirect(a.ref, ed.arrayRefs[i])
	}
}

// write writes the objects which redirect allocated, and the parent tree
// with the keys of the redacted copies.
func (ed *structEditor) write() error {
	red := ed.red
	for i, a := range ed.arrays {
		arr, err := ed.copyKids(a.arr)
		if err != nil {
			return err
		}
		if arr == nil {
			arr = pdf.Array{}
		}
		if err := red.out.Put(ed.arrayRefs[i], arr); err != nil {
			return err
		}
	}
	tree, err := ed.writeParentTree()
	if err != nil {
		return err
	}
	if tree != 0 && ed.rootRef == 0 {
		// the parent tree of a direct root is a reference, see readKeys
		red.copier.Redirect(red.keys.treeRef, tree)
	}
	for _, ref := range ed.order {
		newRef, ok := ed.newRefs[ref]
		if !ok {
			continue
		}
		d := ed.edits[ref].dict
		if ref == ed.rootRef && tree != 0 {
			d = maps.Clone(d)
			delete(d, "ParentTree")
		}
		dict, err := ed.copyElem(d)
		if err != nil {
			return err
		}
		if ref == ed.rootRef && tree != 0 {
			dict["ParentTree"] = tree
		}
		if err := red.out.Put(newRef, dict); err != nil {
			return err
		}
	}
	return nil
}

// copyElem copies a dictionary of the structure tree to the output, with
// the content items in /K which [structEditor.copyKids] writes.
func (ed *structEditor) copyElem(d pdf.Dict) (pdf.Dict, error) {
	k, hasK := d["K"]
	if !hasK {
		return ed.red.copier.CopyDict(d)
	}
	rest := maps.Clone(d)
	delete(rest, "K")
	res, err := ed.red.copier.CopyDict(rest)
	if err != nil {
		return nil, err
	}
	kids, err := ed.copyKids(k)
	if err != nil {
		return nil, err
	}
	if kids != nil {
		res["K"] = kids
	}
	return res, nil
}

// copyKids copies the value of a /K entry to the output.  An [itemCopies]
// gives one content item for each redacted copy and for the alias of its
// stream; the result is nil if the value is an itemCopies whose stream has
// neither.  Direct values are copied recursively: their nesting is bounded
// by the parser.
func (ed *structEditor) copyKids(k pdf.Object) (pdf.Native, error) {
	switch k := k.(type) {
	case nil:
		return nil, nil
	case *itemCopies:
		items, err := ed.itemCopies(k)
		if err != nil {
			return nil, err
		}
		switch len(items) {
		case 0:
			return nil, nil
		case 1:
			return items[0].(pdf.Dict), nil
		}
		return items, nil
	case pdf.Array:
		res := pdf.Array{}
		for _, item := range k {
			if c, ok := item.(*itemCopies); ok {
				items, err := ed.itemCopies(c)
				if err != nil {
					return nil, err
				}
				res = append(res, items...)
				continue
			}
			v, err := ed.copyKids(item)
			if err != nil {
				return nil, err
			}
			res = append(res, v)
		}
		return res, nil
	case pdf.Dict:
		d, err := ed.copyElem(k)
		if err != nil {
			return nil, err
		}
		return d, nil
	}
	return ed.red.copier.Copy(k.AsPDF(ed.red.out.GetOptions()))
}

// itemCopies returns the content items of the output which replace c.
func (ed *structEditor) itemCopies(c *itemCopies) (pdf.Array, error) {
	red := ed.red
	targets := slices.Clone(red.copiesOf[c.src])
	if job := red.fixes[c.src]; job != nil && red.tainted[c.src] {
		targets = append(targets, job.ref)
	}
	if len(targets) == 0 {
		return nil, nil
	}
	rest := maps.Clone(c.dict)
	delete(rest, c.key)
	base, err := red.copier.CopyDict(rest)
	if err != nil {
		return nil, err
	}
	res := make(pdf.Array, len(targets))
	for i, ref := range targets {
		if err := red.charge(int64(len(base))); err != nil {
			return nil, err
		}
		d := maps.Clone(base)
		d[c.key] = ref
		res[i] = d
	}
	return res, nil
}

// A keyCopy is a parent tree key of a redacted copy of a stream.  The entry
// for dst repeats the entry for src, the key of the source stream.
type keyCopy struct {
	src, dst pdf.Integer
}

// parentKeys hands out the parent tree keys of the redacted copies of
// streams.
type parentKeys struct {
	// ready is set once the parent tree was read
	ready bool

	// tree is the parent tree, or nil if no keys can be added.  treeRef is
	// the reference to the tree, or 0 if the tree is a direct object.
	tree    *numtree.FromFile
	treeRef pdf.Reference

	// sorted is set if the tree lists its keys in increasing order, without
	// repetitions
	sorted bool

	// next is the key which is given out next
	next pdf.Integer

	copies []keyCopy
}

// copyParentKey returns a new parent tree key for a redacted copy of a
// stream, whose source has the key src.  The parent tree of the output has
// an entry for the new key which repeats the entry for src.  The result is
// false if no key can be added to the parent tree.
func (red *redactor) copyParentKey(src pdf.Integer) (pdf.Integer, bool, error) {
	pk := &red.keys
	if !pk.ready {
		pk.ready = true
		if err := red.readKeys(); err != nil {
			return 0, false, err
		}
	}
	if pk.tree == nil || pk.next > math.MaxInt32 {
		return 0, false, nil
	}
	key := pk.next
	pk.next++
	pk.copies = append(pk.copies, keyCopy{src: src, dst: key})
	return key, true, nil
}

// readKeys finds the first key after all keys of the parent tree and after
// /ParentTreeNextKey.  New keys are given out only if the output can refer to
// a new parent tree: if the structure tree root or the parent tree is an
// indirect object.
func (red *redactor) readKeys() error {
	pk := &red.keys
	rootObj := red.r.GetMeta().Catalog.StructTreeRoot
	rv, err := red.getNative(rootObj)
	if err != nil {
		return err
	}
	root, _ := rv.(pdf.Dict)
	if root == nil {
		return nil
	}
	ptObj := root["ParentTree"]
	_, rootIsRef := rootObj.(pdf.Reference)
	pk.treeRef, _ = ptObj.(pdf.Reference)
	if ptObj == nil || !rootIsRef && pk.treeRef == 0 {
		return nil
	}
	pt, err := numtree.ExtractFromFile(red.r, ptObj)
	if err != nil {
		if pdf.IsMalformed(err) {
			return nil
		}
		return err
	}
	if pt == nil {
		return nil
	}

	// keys beyond the range of valid keys leave no key to give out
	const limit = math.MaxInt32 + 1
	v, err := red.getNative(root["ParentTreeNextKey"])
	if err != nil {
		return err
	}
	if n, ok := v.(pdf.Integer); ok && n > 0 {
		pk.next = min(n, limit)
	}
	pk.sorted = true
	first := true
	var last pdf.Integer
	for key := range pt.All() {
		if err := red.charge(1); err != nil {
			return err
		}
		if !first && key <= last {
			pk.sorted = false
		}
		first, last = false, key
		pk.next = max(pk.next, min(key, limit-1)+1)
	}
	pk.tree = pt
	return nil
}

// setNextKey sets /ParentTreeNextKey in the structure tree root, if
// redacted copies have new parent tree keys.
func (ed *structEditor) setNextKey(root pdf.Dict) error {
	pk := &ed.red.keys
	if len(pk.copies) == 0 {
		return nil
	}
	rd, err := ed.rootDict(root)
	if err != nil {
		return err
	}
	rd["ParentTreeNextKey"] = pk.next
	return nil
}

// writeParentTree writes the parent tree of the output, if redacted copies
// have new keys, and returns the reference to it.  Otherwise the result is 0,
// and the parent tree is copied with the root.
func (ed *structEditor) writeParentTree() (pdf.Reference, error) {
	red, pk := ed.red, &ed.red.keys
	if len(pk.copies) == 0 {
		return 0, nil
	}
	want := map[pdf.Integer]bool{}
	for _, c := range pk.copies {
		want[c.src] = true
	}
	entries, err := ed.sortedEntries()
	if err != nil {
		return 0, err
	}

	// the new keys come after all keys of the tree
	var copyErr error
	data := func(yield func(pdf.Integer, pdf.Object) bool) {
		put := func(key pdf.Integer, val pdf.Object) bool {
			if copyErr = red.charge(1); copyErr != nil {
				return false
			}
			n, _ := val.(pdf.Native)
			var cp pdf.Native
			if cp, copyErr = red.copier.Copy(n); copyErr != nil {
				return false
			}
			return yield(key, cp)
		}
		found := map[pdf.Integer]pdf.Object{}
		for key, val := range entries {
			if want[key] {
				found[key] = val
			}
			if !put(key, val) {
				return
			}
		}
		for _, c := range pk.copies {
			if val, ok := found[c.src]; ok && !put(c.dst, val) {
				return
			}
		}
	}
	ref, err := numtree.Write(red.out, data)
	if copyErr != nil {
		return 0, copyErr
	}
	return ref, err
}

// sortedEntries returns the entries of the parent tree in increasing order
// of the keys.  Where a key occurs more than once, the first entry is used.
func (ed *structEditor) sortedEntries() (iter.Seq2[pdf.Integer, pdf.Object], error) {
	pk := &ed.red.keys
	if pk.sorted {
		return pk.tree.All(), nil
	}
	type entry struct {
		key pdf.Integer
		val pdf.Object
	}
	var all []entry
	for key, val := range pk.tree.All() {
		if err := ed.red.charge(1); err != nil {
			return nil, err
		}
		all = append(all, entry{key, val})
	}
	slices.SortStableFunc(all, func(a, b entry) int { return cmp.Compare(a.key, b.key) })
	all = slices.CompactFunc(all, func(a, b entry) bool { return a.key == b.key })
	return func(yield func(pdf.Integer, pdf.Object) bool) {
		for _, e := range all {
			if !yield(e.key, e.val) {
				return
			}
		}
	}, nil
}
