// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"maps"
	"slices"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/decode"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/redactmark"
)

// defaultMediaBox is used for a page without a usable /MediaBox.
var defaultMediaBox = pdf.Rectangle{URx: 612, URy: 792}

// markNamePrefix starts the name of the resource which draws the marks.
const markNamePrefix = "RdMk"

// annotEntry is one entry of a page's /Annots array.
type annotEntry struct {
	obj  pdf.Object
	ref  pdf.Reference // 0 for a direct dictionary
	dict pdf.Dict      // nil if the entry is not a readable dictionary

	redact bool // the entry is a redaction annotation
	del    bool // the entry is deleted
}

// quadWork is the work charged for one quad of a region on one page: its
// hull, its part of the clip and its part of the marks.
const quadWork = 128

// annotRewrite is a kept annotation which is written again, without its
// pop-up.
type annotRewrite struct {
	oldRef, newRef pdf.Reference
	dict           pdf.Dict

	// taken is set if the form code writes the annotation instead
	taken bool
}

// redactPage applies the redactions of one page.  A page without
// redaction annotations is left alone.
func (red *redactor) redactPage(pg *pageState) error {
	cur := pdf.CursorAt(red.x, nil)
	entries, marks, err := red.readAnnots(cur, pg.index, pg.src["Annots"])
	if err != nil {
		return err
	}
	pg.annots = entries
	if len(marks) == 0 {
		return nil
	}
	pg.marked = true

	var quads []redactmark.Quad
	for _, m := range marks {
		quads = append(quads, m.Quads...)
	}
	hulls := regionHulls(quads)

	pg.src = maps.Clone(pg.src)
	if err := red.dropAnnots(cur, entries, pg.src["Rotate"], hulls); err != nil {
		return err
	}

	// the page may carry copies of the original artwork
	for _, key := range []pdf.Name{"Thumb", "PieceInfo", "Metadata", "AF"} {
		pg.repl[key] = nil
	}

	red.regions = true
	pg.hulls = hulls
	return red.rewritePageContent(pg, cur, marks, hulls)
}

// readAnnots reads the /Annots array obj of the page with the given index.
// It returns the entries of the array, each reference listed once, and the
// marks of the redaction annotations among them in the order of the array.
// A redaction annotation which cannot be read, or whose region encloses no
// area, gives an [UnusableRedactionError].
func (red *redactor) readAnnots(cur pdf.Cursor, index int, obj pdf.Object) ([]*annotEntry, []*redactmark.Mark, error) {
	annots, err := pdf.Optional(cur.Array(obj))
	if err != nil {
		return nil, nil, err
	}
	entries := make([]*annotEntry, 0, len(annots))
	seen := map[pdf.Reference]bool{}
	var marks []*redactmark.Mark
	for _, obj := range annots {
		e := &annotEntry{obj: obj}
		e.ref, _ = obj.(pdf.Reference)
		if e.ref != 0 {
			if seen[e.ref] {
				continue
			}
			seen[e.ref] = true
		}
		e.dict, err = red.annotDict(cur, obj)
		if err != nil {
			return nil, nil, err
		}
		if e.dict != nil {
			name, err := pdf.Optional(cur.Name(e.dict["Subtype"]))
			if err != nil {
				return nil, nil, err
			}
			e.redact = name == "Redact"
		}
		entries = append(entries, e)
		if !e.redact {
			continue
		}

		m, err := red.mark(cur, index, e)
		if err != nil {
			return nil, nil, err
		}
		if err := red.charge(quadWork * int64(len(m.Quads))); err != nil {
			return nil, nil, err
		}
		if !slices.ContainsFunc(regionHulls(m.Quads), func(h poly) bool { return len(h) >= 3 }) {
			// a repaired /Rect may enclose no area
			return nil, nil, &UnusableRedactionError{Page: index}
		}
		marks = append(marks, m)
	}
	return entries, marks, nil
}

// annotDict returns the dictionary obj, or nil if obj is not a dictionary.
// The dictionaries of references are read once.
func (red *redactor) annotDict(cur pdf.Cursor, obj pdf.Object) (pdf.Dict, error) {
	ref, isRef := obj.(pdf.Reference)
	if !isRef {
		return pdf.Optional(cur.Dict(obj))
	}
	if d, seen := red.dicts[ref]; seen {
		return d, nil
	}
	d, err := pdf.Optional(cur.Dict(ref))
	if err != nil {
		return nil, err
	}
	red.dicts[ref] = d
	return d, nil
}

// mark returns the mark of the redaction annotation e on the page with the
// given index.  The marks of references are made once.
func (red *redactor) mark(cur pdf.Cursor, index int, e *annotEntry) (*redactmark.Mark, error) {
	if m := red.marks[e.ref]; m != nil {
		return m, nil
	}
	gen, err := red.generator()
	if err != nil {
		return nil, err
	}
	// deleting an unusable annotation would leave its content in the output
	a, err := pdf.Optional(cur.Decode(e.obj, decode.Annotation))
	if err != nil {
		return nil, err
	}
	r, ok := a.(*annotation.Redact)
	if !ok {
		return nil, &UnusableRedactionError{Page: index}
	}
	m := redactmark.FromAnnotation(r, gen.ContentFont)
	if e.ref != 0 {
		red.marks[e.ref] = m
	}
	return m, nil
}

// generator returns the generator which supplies the font of the overlay
// text.
func (red *redactor) generator() (*fallback.Generator, error) {
	if red.gen == nil {
		style := red.style
		if style == nil {
			style = fallback.NewStyle()
		}
		gen, err := style.New(red.version)
		if err != nil {
			return nil, err
		}
		red.gen = gen
	}
	return red.gen, nil
}

// dropAnnots marks the entries of the /Annots array of a page with regions
// which are deleted: the entries which are not dictionaries, the redaction
// annotations and the annotations which meet a region.  The pop-ups and
// replies of these are found by [redactor.settleAnnots].
func (red *redactor) dropAnnots(cur pdf.Cursor, entries []*annotEntry, rotate pdf.Object, hulls []poly) error {
	for _, e := range entries {
		switch {
		case e.dict == nil, e.redact:
			e.del = true
		default:
			if err := red.charge(int64(len(hulls)) + 1); err != nil {
				return err
			}
			rect, err := pdf.Optional(cur.Rectangle(e.dict["Rect"]))
			if err != nil {
				return err
			}
			if rect != nil {
				if e.del, err = annotMeetsRegion(cur, e.dict, *rect, rotate, hulls); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// settleAnnots completes the deletion of annotations, once the entries which
// [redactor.dropAnnots] marks are known for all pages.  An annotation which
// is deleted is deleted on every page which lists it, together with its
// pop-up and the replies to it, transitively.  On every page whose /Annots
// loses an entry, pg.src gets the remaining entries.  A kept annotation which
// loses its pop-up is written again without /Popup.
//
// Only annotation dictionaries are redirected to null: a page, an object
// which is redirected already, or a dictionary of another type which a page
// lists in /Annots is removed from the array of a page with regions, but is
// kept.
func (red *redactor) settleAnnots() error {
	cur := pdf.CursorAt(red.x, nil)

	byRef := map[pdf.Reference][]*annotEntry{}
	dependents := map[pdf.Reference][]*annotEntry{}
	for _, pg := range red.pages {
		for _, e := range pg.annots {
			if err := red.charge(1); err != nil {
				return err
			}
			if e.ref != 0 {
				byRef[e.ref] = append(byRef[e.ref], e)
			}
			if irt, ok := e.dict["IRT"].(pdf.Reference); ok {
				dependents[irt] = append(dependents[irt], e)
			}
			if parent, ok := e.dict["Parent"].(pdf.Reference); ok && isPopup(cur, e.dict) {
				dependents[parent] = append(dependents[parent], e)
			}
		}
	}

	deleted := map[pdf.Reference]bool{}
	var queue []pdf.Reference
	enqueue := func(ref pdf.Reference) {
		if ref != 0 && !deleted[ref] {
			deleted[ref] = true
			queue = append(queue, ref)
		}
	}
	mark := func(e *annotEntry) error {
		e.del = true
		if !red.isAnnot(cur, e.ref, e.dict, e.redact) {
			return nil
		}
		enqueue(e.ref)
		popup, err := red.popupOf(cur, e.dict)
		if err != nil {
			return err
		}
		enqueue(popup)
		return nil
	}
	for _, pg := range red.pages {
		for _, e := range pg.annots {
			if e.del {
				if err := mark(e); err != nil {
					return err
				}
			}
		}
	}
	for len(queue) > 0 {
		ref := queue[0]
		queue = queue[1:]
		if err := red.charge(1); err != nil {
			return err
		}
		for _, e := range slices.Concat(byRef[ref], dependents[ref]) {
			if err := mark(e); err != nil {
				return err
			}
		}
	}
	red.deleted = append(red.deleted, slices.Sorted(maps.Keys(deleted))...)

	for _, pg := range red.pages {
		changed := false
		var kept pdf.Array
		for _, e := range pg.annots {
			if e.del {
				changed = true
				continue
			}
			obj := e.obj
			if popup, ok := e.dict["Popup"].(pdf.Reference); ok && deleted[popup] {
				dict := maps.Clone(e.dict)
				delete(dict, "Popup")
				if e.ref == 0 {
					obj = dict
					changed = true
				} else if red.rewritten[e.ref] == nil {
					rw := &annotRewrite{oldRef: e.ref, newRef: red.out.Alloc(), dict: dict}
					red.rewritten[e.ref] = rw
					red.annotRewrites = append(red.annotRewrites, rw)
					red.redirect(rw.oldRef, rw.newRef)
				}
			}
			kept = append(kept, obj)
		}
		if !changed {
			continue
		}
		if !pg.marked {
			pg.src = maps.Clone(pg.src)
		}
		if len(kept) > 0 {
			pg.src["Annots"] = kept
		} else {
			delete(pg.src, "Annots")
		}
	}
	return nil
}

// isAnnot reports whether the object with reference ref and dictionary d is
// an annotation which can be redirected to null: d is a dictionary, ref is 0
// or not redirected already, and d is a redaction annotation or has no /Type
// other than Annot.  The references of the pages are redirected.
func (red *redactor) isAnnot(cur pdf.Cursor, ref pdf.Reference, d pdf.Dict, redact bool) bool {
	if d == nil {
		return false
	}
	if ref != 0 {
		if _, redirected := red.redirects[ref]; redirected {
			return false
		}
	}
	if redact {
		return true
	}
	tp, _ := pdf.Optional(cur.Name(d["Type"]))
	return tp == "" || tp == "Annot"
}

// popupOf returns the reference to the pop-up annotation of the annotation
// dictionary d, or 0 if /Popup does not refer to a pop-up annotation which
// can be redirected to null.
func (red *redactor) popupOf(cur pdf.Cursor, d pdf.Dict) (pdf.Reference, error) {
	ref := popupRef(d)
	if ref == 0 {
		return 0, nil
	}
	if err := red.charge(1); err != nil {
		return 0, err
	}
	popup, err := red.annotDict(cur, ref)
	if err != nil {
		return 0, err
	}
	if !red.isAnnot(cur, ref, popup, false) || !isPopup(cur, popup) {
		return 0, nil
	}
	return ref, nil
}

// isPopup reports whether the annotation dictionary describes a pop-up
// annotation.
func isPopup(cur pdf.Cursor, d pdf.Dict) bool {
	name, _ := cur.Name(d["Subtype"])
	return name == "Popup"
}

// popupRef returns the reference to the pop-up of an annotation, or 0.
func popupRef(d pdf.Dict) pdf.Reference {
	ref, _ := d["Popup"].(pdf.Reference)
	return ref
}

// rewritePageContent writes the new content stream and resources of a page
// which has regions.
func (red *redactor) rewritePageContent(pg *pageState, cur pdf.Cursor, marks []*redactmark.Mark, hulls []poly) error {
	// the extent of the clip: everything which can be seen, and the regions
	box, err := pdf.Optional(cur.Rectangle(pg.src["MediaBox"]))
	if err != nil {
		return err
	}
	if box == nil {
		r := defaultMediaBox
		box = &r
	}
	big := *box
	for _, h := range hulls {
		hb := pdf.RectangleFromPoints(h...)
		big.Extend(&hb)
	}
	// the clip and the bounding box of the marks agree
	big = big.Grow(1).Round(clipDigits)

	markRef, err := red.embedMarks(marks, big)
	if err != nil {
		return err
	}

	srcRes, err := pdf.Optional(cur.Dict(pg.src["Resources"]))
	if err != nil {
		return err
	}
	if err := red.chargeResources(cur, srcRes); err != nil {
		return err
	}
	_, isRef := pg.src["Resources"].(pdf.Reference)
	contentRes, err := pdf.Optional(extract.Resources(cur, pg.src["Resources"], !isRef))
	if err != nil {
		return err
	}
	rs, err := newResSources(cur, srcRes, contentRes)
	if err != nil {
		return err
	}
	if err := red.noteFontSources(cur, srcRes, contentRes); err != nil {
		return err
	}
	var next int
	name := freshName(markNamePrefix, &next, func(name pdf.Name) bool { return rs.xnames[name] })

	// content; the analysis embeds the redacted images, and must run
	// before the content stream is opened
	sc := newScope()
	wk, p, err := red.analyzeContent(pg.src["Contents"], rs, sc, hulls)
	if err != nil {
		return err
	}
	ref := red.out.Alloc()
	stm, err := red.out.OpenStream(ref, pdf.Dict{}, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	w := bufio.NewWriter(stm)
	err = red.writePageContent(w, wk, p, hulls, big, name)
	if err == nil {
		err = w.Flush()
	}
	if err != nil {
		stm.Close()
		return err
	}
	if err := stm.Close(); err != nil {
		return err
	}
	pg.repl["Contents"] = ref
	// a page without regions which shares a content stream gets an alias
	refs, err := contentStreams(cur, pg.src["Contents"])
	if err != nil {
		return err
	}
	for _, ref := range refs {
		red.taint(ref)
	}
	pg.lostMCIDs = append(pg.lostMCIDs, wk.lostMCIDs...)
	pg.lostForms = append(pg.lostForms, sc.lost...)
	pg.lostObjs = append(pg.lostObjs, sc.lostObjs...)

	res, err := red.planResources(cur, srcRes, wk, p, map[pdf.Name]pdf.Native{name: markRef})
	if err != nil {
		return err
	}
	pg.res = res
	return nil
}

// chargeResources accounts for the work of reading the resource dictionary
// res: an amount for each entry of each category.
func (red *redactor) chargeResources(cur pdf.Cursor, res pdf.Dict) error {
	n := int64(1)
	for _, obj := range res {
		d, err := pdf.Optional(cur.Dict(obj))
		if err != nil {
			return err
		}
		n += int64(len(d))
	}
	return red.charge(n)
}

// resource categories which the rewrite changes
var changedCategories = []pdf.Name{"Font", "ExtGState", "Pattern", "Properties", "XObject"}

// A resPlan describes the resources of a rewritten stream.  The resources
// are written by [redactor.buildResources], once all redirections are known.
type resPlan struct {
	// src are the source resources, which can be nil
	src pdf.Dict

	// add holds the entries which are added, by category.  The values live
	// in the output file.
	add map[pdf.Name]map[pdf.Name]pdf.Object

	// drop lists the entries of the source resources which are left out,
	// by category
	drop map[pdf.Name][]pdf.Name

	// props are the stripped property lists, and gstates the extended
	// graphics states with a new soft mask, which are added by name.  Their
	// values refer to the input.
	props   map[pdf.Name]pdf.Dict
	gstates map[pdf.Name]*gsPlan

	// painted lists the names of the source resources which the stream
	// uses, see [redactor.paintedNames]
	painted nameSet

	// named are the resources which the names of painted refer to: src,
	// together with the source objects of the fonts which add holds
	named pdf.Dict

	// fonts holds the plans of the resources of the direct Type 3 fonts
	// which are written with new resources, by category and name: Font for
	// the font resources, and ExtGState for the fonts which the extended
	// graphics states set
	fonts map[pdf.Name]map[pdf.Name]*resPlan
}

// newResPlan returns the plan which copies the source resources src, with
// the painted names, except that the entries which refer to tainted streams
// are kept only if they are painted.
func newResPlan(src pdf.Dict, painted nameSet) *resPlan {
	return &resPlan{src: src, painted: painted, named: src}
}

// setFont records the plan fp of the resources of a direct Type 3 font,
// which the entry name of the category gives.
func (rp *resPlan) setFont(category, name pdf.Name, fp *resPlan) {
	if rp.fonts == nil {
		rp.fonts = map[pdf.Name]map[pdf.Name]*resPlan{}
	}
	if rp.fonts[category] == nil {
		rp.fonts[category] = map[pdf.Name]*resPlan{}
	}
	rp.fonts[category][name] = fp
}

// drops reports whether the plan leaves out an entry of the source
// resources.
func (rp *resPlan) drops(category, name pdf.Name) bool {
	return slices.Contains(rp.drop[category], name)
}

// planResources describes the resources of a rewritten stream, after the
// analysis of the stream is complete.  The source resources srcRes can be
// nil.  The plan p is the outcome of the analysis, and the entries extraX are
// additional XObject resources.
func (red *redactor) planResources(cur pdf.Cursor, srcRes pdf.Dict, wk *walker, p *plan, extraX map[pdf.Name]pdf.Native) (*resPlan, error) {
	named := srcRes
	if len(wk.extraFonts) > 0 {
		fonts, err := pdf.Optional(cur.Dict(srcRes["Font"]))
		if err != nil {
			return nil, err
		}
		fonts = maps.Clone(fonts)
		if fonts == nil {
			fonts = pdf.Dict{}
		}
		for name, f := range wk.extraFonts {
			fonts[name] = red.fontSrc[f]
		}
		named = maps.Clone(srcRes)
		if named == nil {
			named = pdf.Dict{}
		}
		named["Font"] = fonts
	}
	painted, err := red.paintedNames(wk.open, named, p)
	if err != nil {
		return nil, err
	}
	rp := &resPlan{
		src:     srcRes,
		add:     map[pdf.Name]map[pdf.Name]pdf.Object{},
		drop:    map[pdf.Name][]pdf.Name{},
		props:   wk.extraProps,
		gstates: wk.extraGStates,
		painted: painted,
		named:   named,
	}
	add, drop := rp.add, rp.drop

	for _, name := range slices.Sorted(maps.Keys(wk.extraFonts)) {
		f := wk.extraFonts[name]
		if err := red.share(f, red.fontSrc[f]); err != nil {
			return nil, err
		}
		fontRef, err := red.rm.Embed(f)
		if err != nil {
			return nil, err
		}
		addTo(add, "Font", name, fontRef)
	}
	drop["Properties"] = wk.strippedProps()

	for name, obj := range extraX {
		addTo(add, "XObject", name, obj)
	}
	for name, obj := range wk.extraXObjects {
		addTo(add, "XObject", name, obj)
	}
	// images which the rewritten content does not use are not copied: they
	// would carry the original samples
	for name := range wk.images {
		if !wk.keptImages[name] {
			drop["XObject"] = append(drop["XObject"], name)
		}
	}
	drop["XObject"] = append(drop["XObject"], dropNames(wk.touchedForms, wk.keptForms)...)

	for name, obj := range wk.patRepl {
		addTo(add, "Pattern", name, obj)
		drop["Pattern"] = append(drop["Pattern"], name)
	}
	drop["ExtGState"] = dropNames(wk.touchedGS, wk.keptGS)

	// the glyph procedures of Type 3 fonts without resources can still use
	// these entries
	for _, category := range []pdf.Name{"XObject", "ExtGState"} {
		drop[category] = slices.DeleteFunc(drop[category], func(name pdf.Name) bool {
			return painted[category][name]
		})
	}

	if wk.inherited {
		// the source resources belong to the stream which paints this one:
		// keep only the entries which this stream paints
		for _, category := range taintedCategories {
			d, err := pdf.Optional(cur.Dict(srcRes[category]))
			if err != nil {
				return nil, err
			}
			for name := range d {
				if !painted[category][name] {
					drop[category] = append(drop[category], name)
				}
			}
		}
		for name := range wk.props {
			if !wk.keptProps[name] {
				drop["Properties"] = append(drop["Properties"], name)
			}
		}
	}
	return rp, nil
}

// buildResources writes the resources which a plan describes, and returns
// the resource dictionary.
func (red *redactor) buildResources(rp *resPlan) (pdf.Dict, error) {
	cur := pdf.CursorAt(red.x, nil)
	rest := maps.Clone(rp.src)
	for _, category := range changedCategories {
		delete(rest, category)
	}
	if err := red.charge(int64(len(rest)) + 1); err != nil {
		return nil, err
	}
	res := pdf.Dict{}
	for _, key := range rest.SortedKeys() {
		if rest[key] == nil {
			continue
		}
		val, err := red.copyPageValue(rest[key])
		if err != nil {
			return nil, err
		}
		res[key] = val
	}
	for _, category := range changedCategories {
		entries := maps.Clone(rp.add[category])
		if entries == nil {
			entries = map[pdf.Name]pdf.Object{}
		}
		switch category {
		case "Properties":
			for _, name := range slices.Sorted(maps.Keys(rp.props)) {
				cp, err := red.copier.CopyDict(rp.props[name])
				if err != nil {
					return nil, err
				}
				entries[name] = cp
			}
		case "ExtGState":
			for _, name := range slices.Sorted(maps.Keys(rp.gstates)) {
				d, err := red.copyGS(rp.gstates[name])
				if err != nil {
					return nil, err
				}
				entries[name] = d
			}
		}
		if _, present := rp.src[category]; !present && len(entries) == 0 {
			continue
		}
		if err := red.extendResources(cur, rp, res, category, entries); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// xobjectSources returns the XObjects among the XObject resources of a
// stream, by name: the images, the forms, and the streams of all other
// subtypes, including PostScript XObjects and streams without a valid
// subtype.  The decoded images come from res.
func xobjectSources(cur pdf.Cursor, srcXObj pdf.Dict, res *content.Resources) (images map[pdf.Name]*imageSrc, forms, others map[pdf.Name]*streamSrc, err error) {
	images = map[pdf.Name]*imageSrc{}
	forms = map[pdf.Name]*streamSrc{}
	others = map[pdf.Name]*streamSrc{}
	for name, obj := range srcXObj {
		stm, err := streamOf(cur, obj)
		if err != nil {
			return nil, nil, nil, err
		}
		if stm == nil {
			continue
		}
		subtype, err := pdf.Optional(cur.Name(stm.stm.Dict["Subtype"]))
		if err != nil {
			return nil, nil, nil, err
		}
		switch subtype {
		case "Image":
			src := &imageSrc{ref: stm.ref, dict: stm.stm.Dict}
			if res != nil {
				src.img, _ = res.XObject[name].(graphics.Image)
			}
			images[name] = src
		case "Form":
			forms[name] = stm
		default:
			others[name] = stm
		}
	}
	return images, forms, others, nil
}

// keySet returns the keys of d as a set.
func keySet(d pdf.Dict) map[pdf.Name]bool {
	return setOf(slices.Collect(maps.Keys(d))...)
}

// addTo records the resource name of the given category in add.
func addTo(add map[pdf.Name]map[pdf.Name]pdf.Object, category, name pdf.Name, obj pdf.Object) {
	if add[category] == nil {
		add[category] = map[pdf.Name]pdf.Object{}
	}
	add[category][name] = obj
}

// extendResources writes a resource category of the output resources res
// for the plan rp.  The category dictionary holds the entries of the source
// resources which rp does not drop, and the given entries.  A source entry
// which refers to a tainted stream is kept only if the stream uses it, and
// then refers to the alias of the tainted stream.  A category which is not
// changed is copied as a whole, so that the pages which share a direct
// category dictionary share its copy.
func (red *redactor) extendResources(cur pdf.Cursor, rp *resPlan, res pdf.Dict, category pdf.Name, entries map[pdf.Name]pdf.Object) error {
	srcCat, err := pdf.Optional(cur.Dict(rp.src[category]))
	if err != nil {
		return err
	}
	if err := red.charge(int64(len(srcCat)) + 1); err != nil {
		return err
	}
	dropped := map[pdf.Name]bool{}
	for _, name := range rp.drop[category] {
		dropped[name] = true
	}
	taints := map[pdf.Name]pdf.Reference{}
	for name, obj := range srcCat {
		if obj == nil || dropped[name] {
			continue
		}
		t, err := red.taintedEntry(cur, category, obj)
		if err != nil {
			return err
		}
		if t != 0 {
			taints[name] = t
		}
	}

	if len(dropped) == 0 && len(taints) == 0 && len(entries) == 0 && len(rp.fonts[category]) == 0 && srcCat != nil {
		val, err := red.copyPageValue(rp.src[category])
		if err != nil {
			return err
		}
		res[category] = val
		return nil
	}

	catDict := pdf.Dict{}
	for _, name := range srcCat.SortedKeys() {
		obj := srcCat[name]
		if obj == nil || dropped[name] {
			continue
		}
		t := taints[name]
		var val pdf.Native
		var err error
		switch {
		case t != 0 && !rp.painted[category][name]:
			continue
		case t != 0:
			val, err = red.aliasEntry(cur, category, obj, t)
		default:
			val, err = red.copier.Copy(obj.AsPDF(red.out.GetOptions()))
		}
		if err != nil {
			return err
		}
		catDict[name] = val
	}
	maps.Copy(catDict, entries)
	if err := red.inlineFonts(cur, rp, category, catDict); err != nil {
		return err
	}
	res[category] = catDict
	return nil
}

// namedProperties returns the property lists of a resource dictionary, by
// name.  Entries which are not dictionaries are left out.
func namedProperties(cur pdf.Cursor, res pdf.Dict) (map[pdf.Name]pdf.Dict, error) {
	srcProps, err := pdf.Optional(cur.Dict(res["Properties"]))
	if err != nil {
		return nil, err
	}
	props := map[pdf.Name]pdf.Dict{}
	for name, obj := range srcProps {
		d, err := pdf.Optional(cur.Dict(obj))
		if err != nil {
			return nil, err
		}
		if d != nil {
			props[name] = d
		}
	}
	return props, nil
}

// markCost is the work charged for each glyph and quadrilateral of the marks.
const markCost = 16

// embedMarks writes a form XObject which draws all marks, and returns its
// reference.
func (red *redactor) embedMarks(marks []*redactmark.Mark, bbox pdf.Rectangle) (pdf.Native, error) {
	b := builder.New(content.Form, nil, red.version)
	charge := func(n int64) error { return red.charge(markCost * n) }
	for _, m := range marks {
		if err := redactmark.DrawCharged(b, m, charge); err != nil {
			return nil, err
		}
	}
	ops, err := b.Harvest()
	if err != nil {
		return nil, err
	}
	return red.rm.Embed(&form.Form{
		Content: ops,
		Res:     b.Resources,
		BBox:    bbox,
	})
}

// freshName returns the first name prefix+n which used rejects not, for n
// counting up from *next.  *next is left behind the number chosen.
func freshName(prefix string, next *int, used func(pdf.Name) bool) pdf.Name {
	for {
		name := pdf.Name(fmt.Sprintf("%s%d", prefix, *next))
		*next++
		if !used(name) {
			return name
		}
	}
}

// writePageContent writes the new page content to w.  It consists of the
// region-exclusion clip, the original content and the operators which draw
// the marks.  The path object, graphics states, text objects and
// marked-content sequences which the content leaves open are closed before
// the marks.
func (red *redactor) writePageContent(w io.Writer, wk *walker, p *plan, hulls []poly, big pdf.Rectangle, markName pdf.Name) error {
	if err := red.writeOps(w, exclusionClip(hulls, big)); err != nil {
		return err
	}

	tail, err := emit(w, wk.open, p, red.out.GetOptions())
	if err != nil {
		return err
	}
	tail = append(tail,
		content.Operator{Name: content.OpPopGraphicsState},
		content.Operator{
			Name: content.OpBeginMarkedContentWithProperties,
			Args: []pdf.Object{
				pdf.Name("Artifact"),
				pdf.Dict{"Subtype": pdf.Name("Redaction")},
			},
		},
		content.Operator{Name: content.OpXObject, Args: []pdf.Object{markName}},
		content.Operator{Name: content.OpEndMarkedContent},
	)
	return red.writeOps(w, tail)
}

// clipDigits is the number of decimal digits of the coordinates of the
// region-exclusion clip.
const clipDigits = 2

// exclusionClip returns the operators which restrict painting to the page
// outside all hulls.  Clips intersect, so one clip per hull leaves exactly
// the area outside the union of the hulls.
func exclusionClip(hulls []poly, big pdf.Rectangle) []content.Operator {
	num := func(x float64) pdf.Object { return pdf.Number(pdf.Round(x, clipDigits)) }
	ops := []content.Operator{{Name: content.OpPushGraphicsState}}
	for _, h := range hulls {
		if len(h) < 3 {
			// no area, nothing to exclude
			continue
		}
		ops = append(ops, content.Operator{
			Name: content.OpRectangle,
			Args: []pdf.Object{num(big.LLx), num(big.LLy), num(big.URx - big.LLx), num(big.URy - big.LLy)},
		})
		for i, p := range h {
			name := content.OpLineTo
			if i == 0 {
				name = content.OpMoveTo
			}
			ops = append(ops, content.Operator{Name: name, Args: []pdf.Object{num(p.X), num(p.Y)}})
		}
		ops = append(ops,
			content.Operator{Name: content.OpClosePath},
			content.Operator{Name: content.OpClipEvenOdd},
			content.Operator{Name: content.OpEndPath},
		)
	}
	return ops
}

// analyzeContent runs the analysis pass over the content of a page, and
// returns the walker for the content together with the plan for the emission
// pass.  The hulls and the content are both in page space.  The redacted
// images which the content uses are written during the analysis, and the
// redacted copies of nested streams are added to red.pending.
func (red *redactor) analyzeContent(contents pdf.Object, rs *resSources, sc *scope, hulls []poly) (*walker, *plan, error) {
	wk := newWalker(red.contentOpener(contents), rs.res, matrix.Identity, hulls, red.fonts)
	wk.rs = rs
	wk.useResources(rs)
	wk.charge = red.charge
	wk.red, wk.sc = red, sc
	p, err := wk.analyze()
	return wk, p, err
}

// writeOps writes operators to w, one per line.
func (red *redactor) writeOps(w io.Writer, ops []content.Operator) error {
	var buf bytes.Buffer
	for _, op := range ops {
		buf.Reset()
		if err := op.Format(&buf, red.out.GetOptions()); err != nil {
			return err
		}
		buf.WriteByte('\n')
		if _, err := w.Write(buf.Bytes()); err != nil {
			return err
		}
	}
	return nil
}
