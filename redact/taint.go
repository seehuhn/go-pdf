// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"io"
	"maps"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/page"
)

// taintedCategories are the resource categories whose entries can refer to
// tainted streams: directly for XObject and tiling patterns, and through the
// soft mask group for ExtGState and shading patterns.
var taintedCategories = []pdf.Name{"XObject", "Pattern", "ExtGState"}

// taint records that the source stream ref is tainted.  See
// [redactor.decideRedirects].
func (red *redactor) taint(ref pdf.Reference) {
	if ref != 0 {
		red.tainted[ref] = true
	}
}

// A fixJob is a copy of a source stream or Type 3 font which is written as a
// new object of the output.  For a tainted stream, the copy is the alias
// which the uses that paint the stream refer to.  For an object which is not
// tainted, all references to the object are redirected to the copy.
type fixJob struct {
	src, ref pdf.Reference

	// plan describes the resources of the copy, or is nil if the copy keeps
	// the resources of the source
	plan *resPlan

	// image is set if the source is an image, and plan describes its masks
	// in place of resources
	image bool
}

// fixFor returns the copy of the source object src, which is made on first
// use.  An object which is not tainted and which [redactor.outputOf] has
// redirected already keeps its output object, which then holds the copy.
func (red *redactor) fixFor(src pdf.Reference) *fixJob {
	if job := red.fixes[src]; job != nil {
		return job
	}
	job := &fixJob{src: src}
	red.fixes[src] = job
	red.fixOrder = append(red.fixOrder, job)
	switch {
	case red.tainted[src]:
		job.ref = red.out.Alloc()
	case red.shared[src] != 0 && red.redirects[src] == red.shared[src]:
		job.ref = red.shared[src]
	default:
		job.ref = red.out.Alloc()
		red.redirect(src, job.ref)
	}
	return job
}

// writeFixes writes the copies which [redactor.fixFor] made.
func (red *redactor) writeFixes() error {
	// writing a copy can add more copies
	for i := 0; i < len(red.fixOrder); i++ {
		job := red.fixOrder[i]
		obj, err := red.fixCopy(job)
		if err != nil {
			return err
		}
		if err := red.out.Put(job.ref, obj); err != nil {
			return err
		}
	}
	return nil
}

// fixCopy returns the copy which a job writes.  The copy of an object which
// is neither a stream nor a dictionary is null.
func (red *redactor) fixCopy(job *fixJob) (pdf.Native, error) {
	v, err := red.getNative(job.src)
	if err != nil {
		return nil, err
	}
	return red.planCopy(v, job.plan, job.image)
}

// planCopy returns a copy of the source object v, a stream or a dictionary,
// whose resources are described by the plan, or which keeps the resources of
// the source if the plan is nil.  If image is set, v is an image and the plan
// describes its masks in place of resources.  The copy of an object which is
// neither a stream nor a dictionary is null.
func (red *redactor) planCopy(v pdf.Native, plan *resPlan, image bool) (pdf.Native, error) {
	var src pdf.Dict
	switch v := v.(type) {
	case *pdf.Stream:
		src = v.Dict
	case pdf.Dict:
		src = v
	default:
		return nil, nil
	}
	if plan == nil {
		return red.copier.Copy(v)
	}

	// the entries which the plan describes
	keys := []pdf.Name{"Resources"}
	if image {
		keys = maskKeys
	}
	rest := maps.Clone(src)
	for _, key := range keys {
		delete(rest, key)
	}
	var obj pdf.Native
	var dict pdf.Dict
	if stm, ok := v.(*pdf.Stream); ok {
		cp := *stm
		cp.Dict = rest
		c, err := red.copier.Copy(&cp)
		if err != nil {
			return nil, err
		}
		obj, dict = c, c.(*pdf.Stream).Dict
	} else {
		var err error
		if dict, err = red.copier.CopyDict(rest); err != nil {
			return nil, err
		}
		obj = dict
	}

	res, err := red.buildResources(plan)
	if err != nil {
		return nil, err
	}
	if image {
		masks, _ := res["XObject"].(pdf.Dict)
		maps.Copy(dict, masks)
	} else {
		dict["Resources"] = res
	}
	return obj, nil
}

// inlineFonts writes the direct Type 3 fonts of the category dictionary
// catDict of the output, for which the plan rp describes new resources: in
// the Font category, the fonts themselves, and in the ExtGState category, the
// fonts which the extended graphics states set.
func (red *redactor) inlineFonts(cur pdf.Cursor, rp *resPlan, category pdf.Name, catDict pdf.Dict) error {
	plans := rp.fonts[category]
	if len(plans) == 0 {
		return nil
	}
	src, err := pdf.Optional(cur.Dict(rp.named[category]))
	if err != nil {
		return err
	}
	for _, name := range slices.Sorted(maps.Keys(plans)) {
		val, ok := catDict[name]
		if !ok {
			continue
		}
		if category == "Font" {
			fd, _ := src[name].(pdf.Dict)
			if catDict[name], err = red.planCopy(fd, plans[name], false); err != nil {
				return err
			}
			continue
		}

		gsSrc, err := pdf.Optional(cur.Dict(src[name]))
		if err != nil {
			return err
		}
		fontArr, err := pdf.Optional(cur.Array(gsSrc["Font"]))
		if err != nil {
			return err
		}
		gs, ok := val.(pdf.Dict)
		if ok {
			gs = maps.Clone(gs)
		} else if gs, err = red.copier.CopyDict(gsSrc); err != nil {
			return err
		}
		arr := make(pdf.Array, len(fontArr))
		for i, obj := range fontArr {
			if i == 0 {
				fd, _ := obj.(pdf.Dict)
				arr[i], err = red.planCopy(fd, plans[name], false)
			} else if obj != nil {
				arr[i], err = red.copier.Copy(obj.AsPDF(red.out.GetOptions()))
			}
			if err != nil {
				return err
			}
		}
		gs["Font"] = arr
		catDict[name] = gs
	}
	return nil
}

// taintedEntry returns the tainted stream which an entry of a resource
// category refers to, or 0.
func (red *redactor) taintedEntry(cur pdf.Cursor, category pdf.Name, obj pdf.Object) (pdf.Reference, error) {
	switch category {
	case "XObject", "Pattern":
		if ref, ok := obj.(pdf.Reference); ok && red.tainted[ref] {
			return ref, nil
		}
		if category == "Pattern" {
			ref, err := shadingGroupRef(cur, obj)
			if err != nil {
				return 0, err
			}
			if red.tainted[ref] {
				return ref, nil
			}
		}
	case "ExtGState":
		ref, err := gsGroupRef(cur, obj)
		if err != nil {
			return 0, err
		}
		if red.tainted[ref] {
			return ref, nil
		}
	}
	return 0, nil
}

// entryStream returns the reference to the stream which an entry of a
// resource category refers to, or 0: the soft-mask group of an extended
// graphics state or of a shading pattern, or the XObject or tiling pattern.
func entryStream(cur pdf.Cursor, category pdf.Name, obj pdf.Object) (pdf.Reference, error) {
	switch category {
	case "ExtGState":
		return gsGroupRef(cur, obj)
	case "Pattern":
		ref, err := shadingGroupRef(cur, obj)
		if err != nil || ref != 0 {
			return ref, err
		}
	}
	ref, _ := obj.(pdf.Reference)
	return ref, nil
}

// gsGroupRef returns the reference to the soft mask group of an extended
// graphics state, or 0.
func gsGroupRef(cur pdf.Cursor, obj pdf.Object) (pdf.Reference, error) {
	d, err := pdf.Optional(cur.Dict(obj))
	if err != nil {
		return 0, err
	}
	smask, err := pdf.Optional(cur.Dict(d["SMask"]))
	if err != nil {
		return 0, err
	}
	ref, _ := smask["G"].(pdf.Reference)
	return ref, nil
}

// aliasEntry returns the copy of a resource entry which refers to the tainted
// stream t, for a use which paints the stream.  The copy refers to the alias
// of t.
func (red *redactor) aliasEntry(cur pdf.Cursor, category pdf.Name, obj pdf.Object, t pdf.Reference) (pdf.Native, error) {
	alias := red.fixFor(t).ref
	if ref, _ := obj.(pdf.Reference); category == "Pattern" && ref != t {
		return red.aliasShading(cur, obj, alias)
	}
	if category != "ExtGState" {
		return alias, nil
	}
	gs, err := pdf.Optional(cur.Dict(obj))
	if err != nil {
		return nil, err
	}
	smask, err := pdf.Optional(cur.Dict(gs["SMask"]))
	if err != nil {
		return nil, err
	}
	rest := maps.Clone(gs)
	delete(rest, "SMask")
	d, err := red.copier.CopyDict(rest)
	if err != nil {
		return nil, err
	}
	mrest := maps.Clone(smask)
	delete(mrest, "G")
	sm, err := red.copier.CopyDict(mrest)
	if err != nil {
		return nil, err
	}
	sm["G"] = alias
	d["SMask"] = sm
	return d, nil
}

// nameSet maps resource categories to sets of resource names.
type nameSet map[pdf.Name]map[pdf.Name]bool

// fontStates is the key of a [nameSet] for the names of the ExtGState
// resources whose font is a Type 3 font and shows text.
const fontStates pdf.Name = "ExtGState/Font"

func (s nameSet) add(category, name pdf.Name) {
	if s[category] == nil {
		s[category] = map[pdf.Name]bool{}
	}
	s[category][name] = true
}

// A nameScan finds the names of the XObject, ExtGState and Pattern resources
// which a content stream uses, and the names of the Font and ExtGState
// resources which set Type 3 fonts that show text.  The scan follows the
// forms and soft-mask groups which have no resources of their own, and the
// glyph procedures of the Type 3 fonts without resources which show text,
// since these use the names of the stream.  All glyph procedures of such a
// font are followed.
type nameScan struct {
	red *redactor
	cur pdf.Cursor

	xobj, gs, fonts pdf.Dict
	names           nameSet

	// seen lists the streams without resources which were scanned, with
	// the Type 3 font which they inherit, and the fonts whose glyph
	// procedures were scanned
	seen map[scanKey]bool
}

// A scanKey identifies a stream scanned by a [nameScan], together with the
// font it starts with.
type scanKey struct {
	ref  pdf.Reference
	font fontKey
}

// A fontKey identifies a font in a [nameScan]: by its reference, or for a
// direct font dictionary, by the name of the Font or ExtGState resource which
// holds it.
type fontKey struct {
	ref    pdf.Reference
	tf, gs pdf.Name
}

// A type3Font is a Type 3 font which a [nameScan] meets.
type type3Font struct {
	key  fontKey
	dict pdf.Dict

	// tf is the name of the Font resource, for a font which Tf selects, and
	// gs the name of the ExtGState resource, for a font which gs sets
	tf, gs pdf.Name
}

// paintedNames returns the names of the XObject, ExtGState and Pattern
// resources which the stream that open reads uses, with the resources res,
// and the names of the Font and ExtGState resources which set Type 3 fonts
// that show text, the latter under the key [fontStates].
// If p is not nil, the stream is taken as it is rewritten by the plan p.
// Read errors end the scan, since a reader paints only the operators before
// the error.
func (red *redactor) paintedNames(open func() (io.ReadCloser, error), res pdf.Dict, p *plan) (nameSet, error) {
	ns, err := red.newNameScan(res)
	if err != nil {
		return nil, err
	}
	if err := ns.scan(open, p, 0, nil); err != nil {
		return nil, err
	}
	return ns.names, nil
}

// newNameScan returns a scan for streams with the resources res.
func (red *redactor) newNameScan(res pdf.Dict) (*nameScan, error) {
	cur := pdf.CursorAt(red.x, nil)
	ns := &nameScan{red: red, cur: cur, names: nameSet{}, seen: map[scanKey]bool{}}
	var err error
	if ns.xobj, err = pdf.Optional(cur.Dict(res["XObject"])); err != nil {
		return nil, err
	}
	if ns.gs, err = pdf.Optional(cur.Dict(res["ExtGState"])); err != nil {
		return nil, err
	}
	if ns.fonts, err = pdf.Optional(cur.Dict(res["Font"])); err != nil {
		return nil, err
	}
	return ns, nil
}

// scan implements paintedNames for one stream, which starts with the Type 3
// font f, or with no such font if f is nil.
func (ns *nameScan) scan(open func() (io.ReadCloser, error), p *plan, depth int, f *type3Font) error {
	// the fonts which q saves
	var saved []*type3Font

	do := func(name content.OpName, args []pdf.Object) error {
		switch name {
		case content.OpPushGraphicsState:
			saved = append(saved, f)
			return nil
		case content.OpPopGraphicsState:
			if n := len(saved); n > 0 {
				f, saved = saved[n-1], saved[:n-1]
			}
			return nil
		}
		next, err := ns.op(name, args, depth, f)
		f = next
		return err
	}

	it := content.NewScanner(open).NewIter()
	idx := -1
	for name, args := range it.All() {
		idx++
		if err := ns.red.charge(opCost); err != nil {
			return err
		}
		if op := p.op(idx); op != nil {
			if op.drop {
				continue
			}
			if op.repl != nil {
				for _, r := range op.repl {
					if err := do(r.Name, r.Args); err != nil {
						return err
					}
				}
				continue
			}
		}
		if err := do(name, args); err != nil {
			return err
		}
	}
	return nil
}

// op records the names which one operator uses.  The Type 3 font f is the
// current font, or nil, and the result is the current font after the
// operator.
func (ns *nameScan) op(name content.OpName, args []pdf.Object, depth int, f *type3Font) (*type3Font, error) {
	if len(args) == 0 {
		return f, nil
	}
	var inner pdf.Object
	switch name {
	case content.OpXObject:
		n, ok := args[0].(pdf.Name)
		if !ok {
			return f, nil
		}
		ns.names.add("XObject", n)
		inner = ns.xobj[n]
	case content.OpSetExtGState:
		n, ok := args[0].(pdf.Name)
		if !ok {
			return f, nil
		}
		ns.names.add("ExtGState", n)
		ref, err := gsGroupRef(ns.cur, ns.gs[n])
		if err != nil {
			return f, err
		}
		fontObj, set, err := gsFont(ns.cur, ns.gs[n])
		if err != nil {
			return f, err
		}
		if set {
			if f, err = ns.type3(fontObj, fontKey{gs: n}); err != nil {
				return f, err
			}
			if f != nil {
				f.gs = n
			}
		}
		inner = ref
	case content.OpSetFillColorN, content.OpSetStrokeColorN:
		if n, ok := args[len(args)-1].(pdf.Name); ok {
			ns.names.add("Pattern", n)
		}
		return f, nil
	case content.OpTextSetFont:
		n, ok := args[0].(pdf.Name)
		if !ok {
			return f, nil
		}
		f, err := ns.type3(ns.fonts[n], fontKey{tf: n})
		if f != nil {
			f.tf = n
		}
		return f, err
	case content.OpTextShow, content.OpTextShowArray,
		content.OpTextShowMoveNextLine, content.OpTextShowMoveNextLineSetSpacing:
		return f, ns.show(f, depth)
	default:
		return f, nil
	}

	ref, ok := inner.(pdf.Reference)
	if !ok || ref == 0 || depth >= maxNestDepth {
		return f, nil
	}
	key := scanKey{ref: ref}
	if f != nil {
		key.font = f.key
	}
	if ns.seen[key] {
		return f, nil
	}
	stm, err := pdf.Optional(ns.cur.Stream(ref))
	if err != nil || stm == nil {
		return f, err
	}
	if _, has := stm.Dict["Resources"]; has {
		return f, nil
	}
	ns.seen[key] = true
	return f, ns.scan(ns.red.contentOpener(ref), nil, depth+1, f)
}

// type3 returns the Type 3 font which the font object obj gives, or nil if
// obj is not a Type 3 font.  The key identifies a direct font dictionary.
func (ns *nameScan) type3(obj pdf.Object, key fontKey) (*type3Font, error) {
	d, err := type3Dict(ns.cur, obj)
	if err != nil || d == nil {
		return nil, err
	}
	if ref, ok := obj.(pdf.Reference); ok {
		key = fontKey{ref: ref}
	}
	return &type3Font{key: key, dict: d}, nil
}

// show records that text is shown in the Type 3 font f.  The glyph
// procedures of a font without resources are scanned.  The font f can be
// nil, for a font which is not a Type 3 font.
func (ns *nameScan) show(f *type3Font, depth int) error {
	if f == nil {
		return nil
	}
	if f.tf != "" {
		ns.names.add("Font", f.tf)
	}
	if f.gs != "" {
		ns.names.add(fontStates, f.gs)
	}
	if _, has := f.dict["Resources"]; has || depth >= maxNestDepth {
		return nil
	}
	key := scanKey{font: f.key}
	if ns.seen[key] {
		return nil
	}
	ns.seen[key] = true
	return ns.glyphs(f.dict, depth+1)
}

// glyphs scans the glyph procedures of the Type 3 font dictionary fd which
// have no resources of their own.
func (ns *nameScan) glyphs(fd pdf.Dict, depth int) error {
	procs, err := glyphProcs(ns.red, ns.cur, fd)
	if err != nil {
		return err
	}
	for _, ref := range procs {
		stm, err := pdf.Optional(ns.cur.Stream(ref))
		if err != nil {
			return err
		}
		if stm == nil {
			continue
		}
		if _, has := stm.Dict["Resources"]; has {
			continue
		}
		if err := ns.scan(ns.red.contentOpener(ref), nil, depth, nil); err != nil {
			return err
		}
	}
	return nil
}

// type3Dict returns the dictionary of the font object obj, if it is a Type 3
// font, or nil.
func type3Dict(cur pdf.Cursor, obj pdf.Object) (pdf.Dict, error) {
	d, err := pdf.Optional(cur.Dict(obj))
	if err != nil || d == nil {
		return nil, err
	}
	subtype, err := pdf.Optional(cur.Name(d["Subtype"]))
	if err != nil || subtype != "Type3" {
		return nil, err
	}
	return d, nil
}

// glyphProcs returns the references to the glyph procedures of the Type 3
// font dictionary fd, in the order of the glyph names.
func glyphProcs(red *redactor, cur pdf.Cursor, fd pdf.Dict) ([]pdf.Reference, error) {
	procs, err := pdf.Optional(cur.Dict(fd["CharProcs"]))
	if err != nil {
		return nil, err
	}
	var res []pdf.Reference
	for _, name := range procs.SortedKeys() {
		if err := red.charge(1); err != nil {
			return nil, err
		}
		if ref, ok := procs[name].(pdf.Reference); ok {
			res = append(res, ref)
		}
	}
	return res, nil
}

// gsFont returns the font object which an extended graphics state sets.  The
// result set is false if the state sets no font.
func gsFont(cur pdf.Cursor, obj pdf.Object) (_ pdf.Object, set bool, err error) {
	d, err := pdf.Optional(cur.Dict(obj))
	if err != nil {
		return nil, false, err
	}
	a, err := pdf.Optional(cur.Array(d["Font"]))
	if err != nil || len(a) == 0 {
		return nil, false, err
	}
	return a[0], true, nil
}

// A resNode is a stream with resources of its own, a Type 3 font, an image
// with masks, or a page or rewritten stream, in the graph of resources which
// [redactor.planFixes] builds.
type resNode struct {
	// res are the resources.  For an image, res holds the masks of the image
	// as XObject entries, under the keys of the image dictionary.
	res pdf.Dict

	// direct is set if res has an entry which refers to a tainted stream
	direct bool

	// reach is set if a tainted stream can be reached from res, through
	// the resources of the streams which res holds
	reach bool

	// font is the dictionary of a Type 3 font, and image is set for an
	// image
	font  pdf.Dict
	image bool

	// inline holds the nodes of the direct Type 3 fonts with resources of
	// their own which res holds, by category and name: Font for the font
	// resources, and ExtGState for the fonts which the extended graphics
	// states set
	inline map[pdf.Name]map[pdf.Name]*resNode

	kids    []pdf.Reference
	parents []*resNode
}

// A resGraph is the graph of the resources of the painted streams.  An edge
// leads from a resource dictionary to each stream with resources of its own,
// Type 3 font and image with masks which an entry of the dictionary refers
// to, and from a Type 3 font to its glyph procedures with resources of their
// own.  A direct Type 3 font dictionary without resources counts as part of
// the resource dictionary which holds it.  A direct one with resources is a
// node of its own, whose parent is the node of the resource dictionary which
// holds it.
type resGraph struct {
	red   *redactor
	cur   pdf.Cursor
	nodes map[pdf.Reference]*resNode
	all   []*resNode

	// depth is the nesting depth of the direct Type 3 fonts in [resGraph.node]
	depth int
}

// node adds a resource dictionary to the graph.
func (g *resGraph) node(res pdf.Dict) (*resNode, error) {
	n := &resNode{res: res}
	g.all = append(g.all, n)
	for _, category := range taintedCategories {
		d, err := pdf.Optional(g.cur.Dict(res[category]))
		if err != nil {
			return nil, err
		}
		for _, name := range d.SortedKeys() {
			if err := g.red.charge(1); err != nil {
				return nil, err
			}
			obj := d[name]
			t, err := g.red.taintedEntry(g.cur, category, obj)
			if err != nil {
				return nil, err
			}
			n.direct = n.direct || t != 0
			kid, err := entryStream(g.cur, category, obj)
			if err != nil {
				return nil, err
			}
			if category == "ExtGState" {
				fontObj, set, err := gsFont(g.cur, obj)
				if err != nil {
					return nil, err
				}
				if set {
					if err := g.fontKids(n, category, name, fontObj); err != nil {
						return nil, err
					}
				}
			}
			if kid != 0 {
				n.kids = append(n.kids, kid)
			}
		}
	}

	fonts, err := pdf.Optional(g.cur.Dict(res["Font"]))
	if err != nil {
		return nil, err
	}
	for _, name := range fonts.SortedKeys() {
		if err := g.red.charge(1); err != nil {
			return nil, err
		}
		if err := g.fontKids(n, "Font", name, fonts[name]); err != nil {
			return nil, err
		}
	}
	return n, nil
}

// fontKids adds the edges for the font obj of the resources of n, which the
// entry name of the category gives: to the font, if it is an indirect Type 3
// font, to the node of a direct one with resources, and to the glyph
// procedures of a direct one without.
func (g *resGraph) fontKids(n *resNode, category, name pdf.Name, obj pdf.Object) error {
	if ref, ok := obj.(pdf.Reference); ok {
		n.kids = append(n.kids, ref)
		return nil
	}
	fd, err := type3Dict(g.cur, obj)
	if err != nil || fd == nil {
		return err
	}
	procs, err := glyphProcs(g.red, g.cur, fd)
	if err != nil {
		return err
	}
	// the resources of a direct font can hold the font again, through an
	// indirect resource dictionary
	if _, has := fd["Resources"]; !has || g.depth >= maxNestDepth {
		n.kids = append(n.kids, procs...)
		return nil
	}
	res, err := pdf.Optional(g.cur.Dict(fd["Resources"]))
	if err != nil {
		return err
	}
	g.depth++
	sub, err := g.node(res)
	g.depth--
	if err != nil {
		return err
	}
	sub.font = fd
	sub.kids = append(sub.kids, procs...)
	sub.parents = append(sub.parents, n)
	n.direct = n.direct || sub.direct
	if n.inline == nil {
		n.inline = map[pdf.Name]map[pdf.Name]*resNode{}
	}
	if n.inline[category] == nil {
		n.inline[category] = map[pdf.Name]*resNode{}
	}
	n.inline[category][name] = sub
	return nil
}

// stream returns the node of a stream which has resources of its own, of a
// Type 3 font or of an image with masks, or nil.
func (g *resGraph) stream(ref pdf.Reference) (*resNode, error) {
	if n, ok := g.nodes[ref]; ok {
		return n, nil
	}
	g.nodes[ref] = nil
	obj, err := pdf.Optional(g.cur.Resolve(ref))
	if err != nil {
		return nil, err
	}

	var n *resNode
	switch obj := obj.(type) {
	case *pdf.Stream:
		if _, has := obj.Dict["Resources"]; has {
			res, err := pdf.Optional(g.cur.Dict(obj.Dict["Resources"]))
			if err != nil {
				return nil, err
			}
			if n, err = g.node(res); err != nil {
				return nil, err
			}
		} else if masks, err := imageMasks(g.cur, obj.Dict); err != nil {
			return nil, err
		} else if masks != nil {
			if n, err = g.node(pdf.Dict{"XObject": masks}); err != nil {
				return nil, err
			}
			n.image = true
		}
	case pdf.Dict:
		fd, err := type3Dict(g.cur, obj)
		if err != nil || fd == nil {
			return nil, err
		}
		res, err := pdf.Optional(g.cur.Dict(fd["Resources"]))
		if err != nil {
			return nil, err
		}
		if n, err = g.node(res); err != nil {
			return nil, err
		}
		n.font = fd
		procs, err := glyphProcs(g.red, g.cur, fd)
		if err != nil {
			return nil, err
		}
		n.kids = append(n.kids, procs...)
	}
	g.nodes[ref] = n
	return n, nil
}

// maskKeys are the entries of an image dictionary which can refer to a mask
// image.
var maskKeys = []pdf.Name{"SMask", "Mask"}

// imageMasks returns the references to the mask images of an image
// dictionary, by key, or nil if there are none.
func imageMasks(cur pdf.Cursor, dict pdf.Dict) (pdf.Dict, error) {
	if subtype, err := pdf.Optional(cur.Name(dict["Subtype"])); err != nil || subtype != "Image" {
		return nil, err
	}
	var masks pdf.Dict
	for _, key := range maskKeys {
		if ref, ok := dict[key].(pdf.Reference); ok {
			if masks == nil {
				masks = pdf.Dict{}
			}
			masks[key] = ref
		}
	}
	return masks, nil
}

// complete adds the streams which can be reached from the nodes, and
// decides which nodes can reach a tainted stream.
func (g *resGraph) complete() error {
	for i := 0; i < len(g.all); i++ {
		n := g.all[i]
		for _, ref := range n.kids {
			kid, err := g.stream(ref)
			if err != nil {
				return err
			}
			if kid != nil {
				kid.parents = append(kid.parents, n)
			}
		}
	}
	var queue []*resNode
	for _, n := range g.all {
		if n.direct {
			n.reach = true
			queue = append(queue, n)
		}
	}
	for len(queue) > 0 {
		n := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		for _, p := range n.parents {
			if !p.reach {
				p.reach = true
				queue = append(queue, p)
			}
		}
	}
	return nil
}

// A paintRoot is a resource dictionary of the output whose content is
// painted: the resources of a page, of a rewritten stream, of an appearance
// stream of a kept annotation, or of a stream which one of these paints.
type paintRoot struct {
	node *resNode

	// for a page without regions
	pg *pageState

	// for a rewritten stream
	plan *resPlan

	// for a stream
	ref pdf.Reference

	// for a direct Type 3 font with resources: inline is set, and the font is
	// the entry name of the category of the resources whose plan is owner,
	// or owner is nil if these resources are copied unchanged
	inline         bool
	owner          *resPlan
	category, name pdf.Name
}

// planFixes decides which uses of tainted streams paint the streams, and
// which streams need a copy with new resources.  See
// [redactor.decideRedirects].
func (red *redactor) planFixes() error {
	if len(red.tainted) == 0 {
		return nil
	}
	cur := pdf.CursorAt(red.x, nil)
	g := &resGraph{red: red, cur: cur, nodes: map[pdf.Reference]*resNode{}}

	var roots []paintRoot
	addPlan := func(rp *resPlan) error {
		n, err := g.node(rp.named)
		if err != nil {
			return err
		}
		roots = append(roots, paintRoot{node: n, plan: rp})
		return nil
	}
	for _, pg := range red.pages {
		if pg.res != nil {
			if err := addPlan(pg.res); err != nil {
				return err
			}
			continue
		}
		refs, err := contentStreams(cur, pg.src["Contents"])
		if err != nil {
			return err
		}
		for _, ref := range refs {
			if red.tainted[ref] {
				red.fixFor(ref)
				pg.aliasContents = true
			}
		}
		res, err := pdf.Optional(cur.Dict(pg.src["Resources"]))
		if err != nil {
			return err
		}
		n, err := g.node(res)
		if err != nil {
			return err
		}
		roots = append(roots, paintRoot{node: n, pg: pg})
	}
	for _, ps := range red.pending {
		if ps.res != nil {
			if err := addPlan(ps.res); err != nil {
				return err
			}
		}
	}
	done := map[pdf.Reference]bool{}
	if err := red.planAppearances(cur, g, &roots, done); err != nil {
		return err
	}
	if err := g.complete(); err != nil {
		return err
	}

	// visit adds the node of a stream, a Type 3 font or an image which is
	// painted to the roots
	visit := func(ref pdf.Reference) error {
		if done[ref] {
			return nil
		}
		done[ref] = true
		n, err := g.stream(ref)
		if err != nil {
			return err
		}
		if n != nil {
			roots = append(roots, paintRoot{node: n, ref: ref})
		}
		return nil
	}

	// visitFont visits the Type 3 font which shows text, or the glyph
	// procedures of a direct one
	visitFont := func(obj pdf.Object) error {
		if ref, ok := obj.(pdf.Reference); ok {
			return visit(ref)
		}
		fd, err := type3Dict(cur, obj)
		if err != nil || fd == nil {
			return err
		}
		procs, err := glyphProcs(red, cur, fd)
		if err != nil {
			return err
		}
		for _, ref := range procs {
			if err := visit(ref); err != nil {
				return err
			}
		}
		return nil
	}

	for len(roots) > 0 {
		r := roots[len(roots)-1]
		roots = roots[:len(roots)-1]
		if !r.node.reach {
			continue
		}

		// own is the plan of the resources of the root, if they are written
		// again
		var painted nameSet
		var own *resPlan
		var err error
		switch {
		case r.plan != nil:
			painted, own = r.plan.painted, r.plan
		case r.pg != nil:
			painted, err = red.paintedNames(red.contentOpener(r.pg.src["Contents"]), r.node.res, nil)
			if err == nil && r.node.direct {
				own = newResPlan(r.node.res, painted)
				r.pg.res = own
			}
		case r.inline:
			painted, err = red.nodeNames(r.node, 0)
			if err == nil && r.node.direct {
				own = newResPlan(r.node.res, painted)
				r.owner.setFont(r.category, r.name, own)
			}
		default:
			painted, err = red.nodeNames(r.node, r.ref)
			if err == nil && r.node.direct {
				own = newResPlan(r.node.res, painted)
				job := red.fixFor(r.ref)
				job.plan = own
				job.image = r.node.image
			}
		}
		if err != nil {
			return err
		}

		// inline adds the node of a direct Type 3 font with resources to the
		// roots, and reports whether there is one
		inline := func(category, name pdf.Name) bool {
			sub := r.node.inline[category][name]
			if sub != nil {
				roots = append(roots, paintRoot{node: sub, inline: true, owner: own, category: category, name: name})
			}
			return sub != nil
		}

		for _, category := range taintedCategories {
			d, err := pdf.Optional(cur.Dict(r.node.res[category]))
			if err != nil {
				return err
			}
			for _, name := range slices.Sorted(maps.Keys(painted[category])) {
				obj, ok := d[name]
				if !ok || (r.plan != nil && r.plan.drops(category, name)) {
					continue
				}
				ref, err := entryStream(cur, category, obj)
				if err != nil {
					return err
				}
				if category == "ExtGState" && painted[fontStates][name] && !inline(category, name) {
					fontObj, set, err := gsFont(cur, obj)
					if err != nil {
						return err
					}
					if set {
						if err := visitFont(fontObj); err != nil {
							return err
						}
					}
				}
				if ref == 0 {
					continue
				}
				if red.tainted[ref] {
					red.fixFor(ref)
				}
				if err := visit(ref); err != nil {
					return err
				}
			}
		}

		fonts, err := pdf.Optional(cur.Dict(r.node.res["Font"]))
		if err != nil {
			return err
		}
		for _, name := range slices.Sorted(maps.Keys(painted["Font"])) {
			if inline("Font", name) {
				continue
			}
			if err := visitFont(fonts[name]); err != nil {
				return err
			}
		}
		if r.node.font != nil {
			// the glyph procedures with resources of their own
			procs, err := glyphProcs(red, cur, r.node.font)
			if err != nil {
				return err
			}
			for _, ref := range procs {
				if err := visit(ref); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// nodeNames returns the names of the resources of the node n, for the
// object ref, which are painted when the object is: for a stream, the names
// which it uses, for a Type 3 font, the names which its glyph procedures
// use, and for an image, its masks.
func (red *redactor) nodeNames(n *resNode, ref pdf.Reference) (nameSet, error) {
	switch {
	case n.image:
		masks, _ := n.res["XObject"].(pdf.Dict)
		names := nameSet{}
		for key := range masks {
			names.add("XObject", key)
		}
		return names, nil
	case n.font != nil:
		ns, err := red.newNameScan(n.res)
		if err != nil {
			return nil, err
		}
		if err := ns.glyphs(n.font, 0); err != nil {
			return nil, err
		}
		return ns.names, nil
	}
	return red.paintedNames(red.contentOpener(ref), n.res, nil)
}

// contentStreams returns the references to the streams of the /Contents
// entry obj of a page.
func contentStreams(cur pdf.Cursor, obj pdf.Object) ([]pdf.Reference, error) {
	ok, err := isStreamRef(cur, obj)
	if err != nil {
		return nil, err
	} else if ok {
		return []pdf.Reference{obj.(pdf.Reference)}, nil
	}
	a, err := pdf.Optional(cur.Array(obj))
	if err != nil {
		return nil, err
	}
	var res []pdf.Reference
	for _, item := range a {
		ok, err := isStreamRef(cur, item)
		if err != nil {
			return nil, err
		}
		if ok {
			res = append(res, item.(pdf.Reference))
		}
	}
	return res, nil
}

// copyContents returns the copy of the /Contents entry obj of a page without
// regions, in which the tainted streams are replaced by their aliases.
func (red *redactor) copyContents(obj pdf.Object) (pdf.Native, error) {
	cur := pdf.CursorAt(red.x, nil)
	alias := func(obj pdf.Object) (pdf.Native, error) {
		if ref, ok := obj.(pdf.Reference); ok && red.tainted[ref] && red.fixes[ref] != nil {
			return red.fixes[ref].ref, nil
		}
		if obj == nil {
			return nil, nil
		}
		return red.copier.Copy(obj.AsPDF(red.out.GetOptions()))
	}
	ok, err := isStreamRef(cur, obj)
	if err != nil {
		return nil, err
	} else if ok {
		return alias(obj)
	}
	a, err := pdf.Optional(cur.Array(obj))
	if err != nil {
		return nil, err
	}
	res := make(pdf.Array, len(a))
	for i, item := range a {
		if res[i], err = alias(item); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// contentOpener returns a function which opens the content of a page or a
// content stream.
func (red *redactor) contentOpener(obj pdf.Object) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		segments, err := page.ExtractContents(pdf.CursorAt(red.x, nil), obj)
		if err != nil {
			return nil, err
		}
		return page.SegmentsReader(segments), nil
	}
}

// taintAppearances taints the appearance streams of the deleted annotations,
// and the replaced appearance streams of the widgets which get a new
// appearance, except those which a kept annotation shows.
func (red *redactor) taintAppearances() error {
	cur := pdf.CursorAt(red.x, nil)

	// the sources of the replaced appearances
	formByRef := red.formRewritesByRef()
	var replaced []pdf.Reference
	for _, src := range slices.Sorted(maps.Keys(red.redirects)) {
		if rw := formByRef[red.redirects[src]]; rw != nil && rw.regen {
			replaced = append(replaced, src)
		}
	}

	kept := map[pdf.Reference]bool{}
	err := red.keptAnnots(cur, func(_ *pageState, id annotID, d pdf.Dict) error {
		refs, err := red.apStreams(cur, d)
		if err != nil {
			return err
		}
		for _, ref := range refs {
			if _, edited := red.apEdits[id][ref]; !edited {
				kept[ref] = true
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	for _, ref := range slices.Concat(red.deleted, replaced) {
		d, err := pdf.Optional(cur.Dict(ref))
		if err != nil {
			return err
		}
		refs, err := red.apStreams(cur, d)
		if err != nil {
			return err
		}
		for _, ap := range refs {
			if !kept[ap] {
				red.taint(ap)
			}
		}
	}
	return nil
}

// keptAnnotDict returns the dictionary of a kept annotation as it is
// written to the output, with references into the input.  The appearance of
// a widget which gets a new appearance is left out.
func (red *redactor) keptAnnotDict(cur pdf.Cursor, obj pdf.Object, formByRef map[pdf.Reference]*formRewrite) (pdf.Dict, error) {
	if ref, ok := obj.(pdf.Reference); ok {
		if rw := formByRef[red.redirects[ref]]; rw != nil {
			if rw.regen {
				return nil, nil
			}
			return rw.dict, nil
		}
		if rw := red.rewritten[ref]; rw != nil {
			return rw.dict, nil
		}
	}
	return pdf.Optional(cur.Dict(obj))
}

// apStreams returns the references to the appearance streams of an
// annotation dictionary.
func (red *redactor) apStreams(cur pdf.Cursor, annot pdf.Dict) ([]pdf.Reference, error) {
	ap, err := pdf.Optional(cur.Dict(annot["AP"]))
	if err != nil || ap == nil {
		return nil, err
	}
	var res []pdf.Reference
	for _, key := range []pdf.Name{"N", "R", "D"} {
		if err := red.charge(1); err != nil {
			return nil, err
		}
		obj := ap[key]
		ok, err := isStreamRef(cur, obj)
		if err != nil {
			return nil, err
		}
		if ok {
			res = append(res, obj.(pdf.Reference))
			continue
		}
		states, err := pdf.Optional(cur.Dict(obj))
		if err != nil {
			return nil, err
		}
		for _, name := range states.SortedKeys() {
			if err := red.charge(1); err != nil {
				return nil, err
			}
			ok, err := isStreamRef(cur, states[name])
			if err != nil {
				return nil, err
			}
			if ok {
				res = append(res, states[name].(pdf.Reference))
			}
		}
	}
	return res, nil
}

// isStreamRef reports whether obj is a reference to a stream.
func isStreamRef(cur pdf.Cursor, obj pdf.Object) (bool, error) {
	if _, ok := obj.(pdf.Reference); !ok {
		return false, nil
	}
	stm, err := pdf.Optional(cur.Stream(obj))
	return stm != nil, err
}
