// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"maps"
	"slices"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/redactmark"
	"seehuhn.de/go/pdf/page"
)

// An annotID identifies a kept annotation: an indirect one by its source
// reference, a direct one by the page and its index in the page's /Annots.
type annotID struct {
	ref       pdf.Reference
	page, idx int
}

// An apEdit maps appearance streams of a kept annotation to the output
// streams which replace them: a redacted copy, or an alias of a tainted
// stream.  The value 0 removes the appearance.
type apEdit map[pdf.Reference]pdf.Reference

// editFor returns the changes to the appearance of an annotation, which are
// made on first use.
func (red *redactor) editFor(id annotID) apEdit {
	e := red.apEdits[id]
	if e == nil {
		e = apEdit{}
		red.apEdits[id] = e
		red.apOrder = append(red.apOrder, id)
	}
	return e
}

// formRewritesByRef returns the rewrites of the interactive form, by output
// reference.
func (red *redactor) formRewritesByRef() map[pdf.Reference]*formRewrite {
	res := map[pdf.Reference]*formRewrite{}
	for _, rw := range red.forms.rewrites {
		res[rw.newRef] = rw
	}
	return res
}

// keptAnnots calls fn for every kept annotation on the pages, with the
// annotation dictionary as it is written to the output, with references
// into the input.  Widgets which get a new appearance are left out.
func (red *redactor) keptAnnots(cur pdf.Cursor, fn func(pg *pageState, id annotID, d pdf.Dict) error) error {
	formByRef := red.formRewritesByRef()
	deleted := map[pdf.Reference]bool{}
	for _, ref := range red.deleted {
		deleted[ref] = true
	}
	for _, pg := range red.pages {
		annots, err := pdf.Optional(cur.Array(pg.src["Annots"]))
		if err != nil {
			return err
		}
		for i, obj := range annots {
			if err := red.charge(1); err != nil {
				return err
			}
			id := annotID{page: pg.index, idx: i}
			if ref, ok := obj.(pdf.Reference); ok {
				if deleted[ref] {
					continue
				}
				id = annotID{ref: ref}
			}
			d, err := red.keptAnnotDict(cur, obj, formByRef)
			if err != nil {
				return err
			}
			if d == nil {
				continue
			}
			if err := fn(pg, id, d); err != nil {
				return err
			}
		}
	}
	return nil
}

// analyzeAppearances rewrites the appearance streams of the kept annotations
// on the pages with regions.  Each appearance stream is a form placed at the
// annotation rectangle (PDF 2.0 section 12.5.5), and is analysed like a form
// which the page content paints.  An appearance which cannot be placed, since
// the annotation has no rectangle or the stream's bounding box encloses no
// area, is removed.
//
// An annotation listed on several pages with regions shows the same
// appearance on each of them, which is analysed for the regions of all these
// pages.  The appearance of an annotation which is also shown turned is
// analysed for both placements, see [apHulls].
func (red *redactor) analyzeAppearances() error {
	cur := pdf.CursorAt(red.x, nil)

	type placed struct {
		id    annotID
		dict  pdf.Dict
		pages []*pageState
	}
	byID := map[annotID]*placed{}
	var order []*placed
	err := red.keptAnnots(cur, func(pg *pageState, id annotID, d pdf.Dict) error {
		if pg.hulls == nil {
			return nil
		}
		a := byID[id]
		if a == nil {
			a = &placed{id: id, dict: d, pages: []*pageState{pg}}
			byID[id] = a
			order = append(order, a)
		}
		// the pages are visited in order
		if a.pages[len(a.pages)-1] != pg {
			a.pages = append(a.pages, pg)
		}
		return nil
	})
	if err != nil {
		return err
	}

	type pageScope struct {
		pg *pageState
		sc *scope
	}
	var scopes []pageScope
	byPage := map[*pageState]*scope{}
	for _, a := range order {
		rect, err := pdf.Optional(cur.Rectangle(a.dict["Rect"]))
		if err != nil {
			return err
		}
		pg := a.pages[0]
		hulls := pg.hulls
		turned := false
		if rect != nil {
			hulls = nil
			for _, q := range a.pages {
				hs, t, err := apHulls(cur, a.dict, *rect, q)
				if err != nil {
					return err
				}
				hulls = append(hulls, hs...)
				turned = turned || t
			}
		}
		sc := byPage[pg]
		if len(a.pages) > 1 || turned {
			// the hulls differ from those of the page
			sc = newScope()
			scopes = append(scopes, pageScope{pg, sc})
		} else if sc == nil {
			sc = newScope()
			byPage[pg] = sc
			scopes = append(scopes, pageScope{pg, sc})
		}
		if err := red.analyzeAnnotAP(cur, sc, a.id, a.dict, rect, hulls); err != nil {
			return err
		}
	}
	for _, s := range scopes {
		s.pg.lostForms = append(s.pg.lostForms, s.sc.lost...)
		s.pg.lostObjs = append(s.pg.lostObjs, s.sc.lostObjs...)
	}
	return nil
}

// apHulls returns the regions of the page pg for the appearance of the
// annotation with dictionary d and rectangle rect, in the space of the
// unturned rectangle.  For an annotation which is shown turned, see
// [annotTurn], the result holds the regions moved by the inverse turn as
// well, and turned is set: content which the turn moves into a region lies
// in such a moved region.
func apHulls(cur pdf.Cursor, d pdf.Dict, rect pdf.Rectangle, pg *pageState) (hulls []poly, turned bool, err error) {
	turn, err := annotTurn(cur, d, rect, pg.src["Rotate"])
	if err != nil {
		return nil, false, err
	}
	if turn == nil {
		return pg.hulls, false, nil
	}
	inv, _ := turn.Inv()
	hulls = slices.Clone(pg.hulls)
	for _, h := range pg.hulls {
		hulls = append(hulls, h.transform(inv))
	}
	return hulls, true, nil
}

// analyzeAnnotAP rewrites the appearance streams of one kept annotation with
// the rectangle rect, which can be nil, for the regions hulls.  See
// [redactor.analyzeAppearances].
func (red *redactor) analyzeAnnotAP(cur pdf.Cursor, sc *scope, id annotID, d pdf.Dict, rect *pdf.Rectangle, hulls []poly) error {
	refs, err := red.apStreams(cur, d)
	if err != nil {
		return err
	}
	seen := map[pdf.Reference]bool{}
	for _, ref := range refs {
		if seen[ref] {
			continue
		}
		seen[ref] = true
		stm, err := pdf.Optional(cur.Stream(ref))
		if err != nil {
			return err
		}
		if stm == nil {
			continue
		}
		src := &streamSrc{ref: ref, stm: stm}

		res := &copyResult{fail: true}
		if rect != nil {
			base, ok, err := appearanceMatrix(cur, stm.Dict, *rect)
			if err != nil {
				return err
			}
			if ok {
				res, err = red.rewriteNested(sc, &nestSpec{
					src:   src,
					key:   copyKey{ref: ref, base: base, kind: content.Form},
					hulls: hulls,
					depth: 1,
				})
				if err != nil {
					return err
				}
			}
		}
		switch {
		case res.fail:
			red.taint(ref)
			red.editFor(id)[ref] = 0
			if err := red.scanDropped(sc, src, &resSources{}, map[visit]bool{}); err != nil {
				return err
			}
		case res.changed:
			red.editFor(id)[ref] = res.ref
			if res.lost {
				if err := red.noteObj(sc, stm.Dict); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// appearanceMatrix returns the matrix which maps the space of an appearance
// stream with dictionary dict to default user space, for the annotation
// rectangle rect (PDF 2.0 section 12.5.5).  The result is false if the
// stream has no usable bounding box, or if the box transformed by the form
// matrix has no area.
func appearanceMatrix(cur pdf.Cursor, dict pdf.Dict, rect pdf.Rectangle) (matrix.Matrix, bool, error) {
	m, err := matrixOf(cur, dict["Matrix"])
	if err != nil {
		return matrix.Matrix{}, false, err
	}
	bbox, err := pdf.Optional(cur.Rectangle(dict["BBox"]))
	if err != nil || bbox == nil {
		return matrix.Matrix{}, false, err
	}
	box := bbox.ToRect().Transform(m)
	w, h := box.Dx(), box.Dy()
	if !(w > 0 && h > 0 && isFinite(w) && isFinite(h)) {
		return matrix.Matrix{}, false, nil
	}
	sx := (rect.URx - rect.LLx) / w
	sy := (rect.URy - rect.LLy) / h
	a := matrix.Matrix{sx, 0, 0, sy, rect.LLx - box.LLx*sx, rect.LLy - box.LLy*sy}
	return m.Mul(a), true, nil
}

// annotTurn returns the rotation with which the annotation with dictionary
// d and rectangle rect is shown on a page whose /Rotate entry is rotate, or
// nil if it is shown at its rectangle.  An annotation with the NoRotate flag
// stays upright on the screen while the page is turned, and pivots around the
// upper-left corner of its rectangle (PDF 2.0 section 12.5.3).  Text
// annotations always have the flag (section 12.5.6.4).
func annotTurn(cur pdf.Cursor, d pdf.Dict, rect pdf.Rectangle, rotate pdf.Object) (*matrix.Matrix, error) {
	flags, err := pdf.Optional(cur.Integer(d["F"]))
	if err != nil {
		return nil, err
	}
	subtype, err := pdf.Optional(cur.Name(d["Subtype"]))
	if err != nil {
		return nil, err
	}
	if annotation.Flags(flags)&annotation.FlagNoRotate == 0 && subtype != "Text" {
		return nil, nil
	}
	deg, err := pdf.Optional(cur.Integer(rotate))
	if err != nil {
		return nil, err
	}

	// the page is shown turned clockwise by deg, so that the annotation
	// turns counter-clockwise by deg in default user space
	var r matrix.Matrix
	switch page.RotationFromDegrees(int(deg % 360)).Degrees() {
	case 90:
		r = matrix.Matrix{0, 1, -1, 0, 0, 0}
	case 180:
		r = matrix.Matrix{-1, 0, 0, -1, 0, 0}
	case 270:
		r = matrix.Matrix{0, -1, 1, 0, 0, 0}
	default:
		return nil, nil
	}
	x, y := rect.LLx, rect.URy
	turn := matrix.Translate(-x, -y).Mul(r).Mul(matrix.Translate(x, y))
	return &turn, nil
}

// annotMeetsRegion reports whether the annotation with dictionary d and
// rectangle rect is shown over one of the hulls, on a page whose /Rotate
// entry is rotate.  The annotation is tested at its rectangle and, if it is
// shown turned, see [annotTurn], also at the turned rectangle.  An annotation
// with the NoZoom flag is tested at its size for the default magnification.
func annotMeetsRegion(cur pdf.Cursor, d pdf.Dict, rect pdf.Rectangle, rotate pdf.Object, hulls []poly) (bool, error) {
	turn, err := annotTurn(cur, d, rect, rotate)
	if err != nil {
		return false, err
	}
	q := redactmark.RectQuad(rect)
	shapes := []poly{convexHull(q[:])}
	if turn != nil {
		shapes = append(shapes, convexHull(transformPoints(q[:], *turn)))
	}
	for _, a := range shapes {
		for _, h := range hulls {
			if intersects(a, h, touchTol) {
				return true, nil
			}
		}
	}
	return false, nil
}

// planAppearances adds the appearance streams of the kept annotations to
// the roots of [redactor.planFixes].  A tainted appearance stream which a
// kept annotation shows unchanged is replaced by its alias.
func (red *redactor) planAppearances(cur pdf.Cursor, g *resGraph, roots *[]paintRoot, done map[pdf.Reference]bool) error {
	return red.keptAnnots(cur, func(pg *pageState, id annotID, d pdf.Dict) error {
		refs, err := red.apStreams(cur, d)
		if err != nil {
			return err
		}
		for _, ref := range refs {
			if _, replaced := red.apEdits[id][ref]; replaced {
				continue
			}
			if red.tainted[ref] {
				red.editFor(id)[ref] = red.fixFor(ref).ref
			}
			if done[ref] {
				continue
			}
			done[ref] = true
			n, err := g.stream(ref)
			if err != nil {
				return err
			}
			if n != nil {
				*roots = append(*roots, paintRoot{node: n, ref: ref})
			}
		}
		return nil
	})
}

// rewriteEditedAnnots arranges for the kept annotations whose appearance
// changes to be written again.  An indirect annotation is written by
// [redactor.writeAnnotRewrites], unless the form code writes it, and the
// /Annots of a page with a direct one by [redactor.copyAnnots].
func (red *redactor) rewriteEditedAnnots() error {
	cur := pdf.CursorAt(red.x, nil)
	formByRef := red.formRewritesByRef()
	for _, id := range red.apOrder {
		if len(red.apEdits[id]) == 0 {
			continue
		}
		if id.ref == 0 {
			red.pages[id.page].annotEdits = true
			continue
		}
		if formByRef[red.redirects[id.ref]] != nil || red.rewritten[id.ref] != nil {
			continue
		}
		dict, err := pdf.Optional(cur.Dict(id.ref))
		if err != nil {
			return err
		}
		rw := &annotRewrite{oldRef: id.ref, newRef: red.out.Alloc(), dict: dict}
		red.rewritten[id.ref] = rw
		red.annotRewrites = append(red.annotRewrites, rw)
		red.redirect(rw.oldRef, rw.newRef)
	}
	return nil
}

// copyAnnots returns the /Annots array of a page with a direct annotation
// dictionary whose appearance changes.
func (red *redactor) copyAnnots(pg *pageState) (pdf.Native, error) {
	cur := pdf.CursorAt(red.x, nil)
	annots, err := pdf.Optional(cur.Array(pg.src["Annots"]))
	if err != nil {
		return nil, err
	}
	res := make(pdf.Array, 0, len(annots))
	for i, obj := range annots {
		edit := red.apEdits[annotID{page: pg.index, idx: i}]
		if obj == nil {
			res = append(res, nil)
			continue
		}
		if _, isRef := obj.(pdf.Reference); isRef || len(edit) == 0 {
			cp, err := red.copier.Copy(obj.AsPDF(red.out.GetOptions()))
			if err != nil {
				return nil, err
			}
			res = append(res, cp)
			continue
		}
		d, err := pdf.Optional(cur.Dict(obj))
		if err != nil {
			return nil, err
		}
		dict, err := red.copyAnnotDict(d, edit)
		if err != nil {
			return nil, err
		}
		res = append(res, dict)
	}
	return res, nil
}

// copyAnnotDict copies an annotation dictionary d, with the changes edit to
// its appearance.  An appearance dictionary which loses its normal
// appearance is left out.
func (red *redactor) copyAnnotDict(d pdf.Dict, edit apEdit) (pdf.Dict, error) {
	if len(edit) == 0 {
		return red.copier.CopyDict(d)
	}
	rest := maps.Clone(d)
	delete(rest, "AP")
	dict, err := red.copier.CopyDict(rest)
	if err != nil {
		return nil, err
	}
	ap, err := red.copyAP(d["AP"], edit)
	if err != nil {
		return nil, err
	}
	if ap != nil {
		dict["AP"] = ap
	}
	return dict, nil
}

// copyAP returns the copy of an appearance dictionary with the changes
// edit, or nil if the copy has no normal appearance.
func (red *redactor) copyAP(obj pdf.Object, edit apEdit) (pdf.Dict, error) {
	cur := pdf.CursorAt(red.x, nil)
	ap, err := pdf.Optional(cur.Dict(obj))
	if err != nil {
		return nil, err
	}
	res := pdf.Dict{}
	for _, key := range ap.SortedKeys() {
		obj := ap[key]
		if obj == nil {
			continue
		}
		var val pdf.Native
		switch key {
		case "N", "R", "D":
			val, err = red.copyAppearance(cur, obj, edit)
		default:
			val, err = red.copier.Copy(obj.AsPDF(red.out.GetOptions()))
		}
		if err != nil {
			return nil, err
		}
		if val != nil {
			res[key] = val
		}
	}
	if res["N"] == nil {
		return nil, nil
	}
	return res, nil
}

// copyAppearance returns the copy of an entry of an appearance dictionary, a
// stream or a dictionary of streams by appearance state, with the changes
// edit.  The result is nil if no appearance is left.
func (red *redactor) copyAppearance(cur pdf.Cursor, obj pdf.Object, edit apEdit) (pdf.Native, error) {
	// replaced returns the replacement of a stream which the edit changes
	replaced := func(obj pdf.Object) (pdf.Native, bool) {
		ref, _ := obj.(pdf.Reference)
		r, ok := edit[ref]
		if !ok || r == 0 {
			return nil, ok
		}
		return r, true
	}
	if val, ok := replaced(obj); ok {
		return val, nil
	}
	isStream, err := isStreamRef(cur, obj)
	if err != nil {
		return nil, err
	}
	var states pdf.Dict
	if !isStream {
		states, err = pdf.Optional(cur.Dict(obj))
		if err != nil {
			return nil, err
		}
	}
	changed := false
	for _, val := range states {
		_, ok := replaced(val)
		changed = changed || ok
	}
	if !changed {
		return red.copier.Copy(obj.AsPDF(red.out.GetOptions()))
	}
	res := pdf.Dict{}
	for _, name := range states.SortedKeys() {
		val, ok := replaced(states[name])
		if !ok && states[name] != nil {
			val, err = red.copier.Copy(states[name].AsPDF(red.out.GetOptions()))
			if err != nil {
				return nil, err
			}
		}
		if val != nil {
			res[name] = val
		}
	}
	if len(res) == 0 {
		return nil, nil
	}
	return res, nil
}
