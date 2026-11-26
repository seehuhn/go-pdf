// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics/content"
)

// share makes the resource manager embed v, a value decoded from the source
// object src, as the copy of src which the rest of the output uses.  Without
// this, the resource manager would write the objects of v anew, and the
// output would hold a second copy of every source object which src refers to.
//
// The objects which src refers to are redirected to output objects which are
// written as copies of the source objects once all redirections are known.
// An object which is already redirected keeps its redirection.  A value
// without references, which shares no object with the rest of the output, is
// left to the resource manager.  So is a value which the resource manager has
// embedded before.
func (red *redactor) share[T comparable](v T, src pdf.Object) error {
	if !hasRef(src) {
		return nil
	}
	_, err := red.rm.EmbedFunc(func(*pdf.EmbedHelper, T) (pdf.Native, error) {
		return red.outputOf(src), nil
	}, v)
	return err
}

// hasRef reports whether obj is or contains a reference.
func hasRef(obj pdf.Object) bool {
	switch o := obj.(type) {
	case pdf.Reference:
		return true
	case pdf.Dict:
		for _, val := range o {
			if hasRef(val) {
				return true
			}
		}
	case pdf.Array:
		return slices.ContainsFunc(o, hasRef)
	}
	return false
}

// outputOf returns the form of the source object obj in the output, as the
// copier will write it once all redirections are known.  A reference which is
// not yet redirected is redirected to a new output object, which
// [redactor.writeShared] writes as a copy of the source object.
func (red *redactor) outputOf(obj pdf.Object) pdf.Native {
	switch o := obj.(type) {
	case nil:
		return nil
	case pdf.Reference:
		if dst, ok := red.redirects[o]; ok {
			return dst
		}
		ref := red.out.Alloc()
		red.redirect(o, ref)
		red.shared[o] = ref
		red.sharedOrder = append(red.sharedOrder, o)
		return ref
	case pdf.Dict:
		// sorted keys, so that the object numbers do not depend on map order
		res := pdf.Dict{}
		for _, key := range o.SortedKeys() {
			if val := o[key]; val != nil {
				res[key] = red.outputOf(val)
			}
		}
		return res
	case pdf.Array:
		res := make(pdf.Array, len(o))
		for i, val := range o {
			res[i] = red.outputOf(val)
		}
		return res
	default:
		return o.AsPDF(red.out.GetOptions())
	}
}

// writeShared writes the copies of the source objects which
// [redactor.outputOf] redirected.  A copy whose source object was redirected
// elsewhere later is written as null, and a copy which [redactor.fixFor]
// took over is left to [redactor.writeFixes].
func (red *redactor) writeShared() error {
	for _, src := range red.sharedOrder {
		ref := red.shared[src]
		if job := red.fixes[src]; job != nil && job.ref == ref {
			continue
		}
		var obj pdf.Native
		if red.redirects[src] == ref {
			v, err := red.getNative(src)
			if err != nil {
				return err
			}
			obj, err = red.copier.Copy(v)
			if err != nil {
				return err
			}
		}
		if err := red.out.Put(ref, obj); err != nil {
			return err
		}
	}
	return nil
}

// noteFontSources records the source objects of the fonts of the resources
// res, which were decoded from the resource dictionary raw.  This covers the
// font resources and the fonts which the extended graphics states set.
func (red *redactor) noteFontSources(cur pdf.Cursor, raw pdf.Dict, res *content.Resources) error {
	if res == nil {
		return nil
	}
	note := func(f font.Instance, src pdf.Object) {
		if _, seen := red.fontSrc[f]; !seen {
			red.fontSrc[f] = src
		}
	}

	fonts, err := pdf.Optional(cur.Dict(raw["Font"]))
	if err != nil {
		return err
	}
	for name, obj := range fonts {
		if f := res.Font[name]; f != nil {
			note(f, obj)
		}
	}

	states, err := pdf.Optional(cur.Dict(raw["ExtGState"]))
	if err != nil {
		return err
	}
	for name, obj := range states {
		gs := res.ExtGState[name]
		if gs == nil || gs.TextFont == nil {
			continue
		}
		d, err := pdf.Optional(cur.Dict(obj))
		if err != nil {
			return err
		}
		a, err := pdf.Optional(cur.Array(d["Font"]))
		if err != nil {
			return err
		}
		if len(a) > 0 {
			note(gs.TextFont, a[0])
		}
	}
	return nil
}
