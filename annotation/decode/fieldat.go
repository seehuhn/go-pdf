// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation"
)

// FieldAt reads the terminal field whose dictionary is ref, together with the
// references of its widgets.
//
// If ref is a widget of a field of the document's form, FieldAt returns that
// field.  Any other dictionary without a partial name is not a field but a
// widget annotation, or a group of widget annotations, of the nearest
// ancestor on its /Parent chain which has a partial name.  For such a ref,
// FieldAt returns that ancestor's field, provided the field has ref among its
// widgets or above one of them.
//
// If the document's form contains the field, the field and its widgets are
// those of [DocumentForm].
//
// Otherwise the field is read on its own.  It inherits the attributes of
// the ancestors which its /Parent chain names.  Its widgets are the widget
// annotations among its /Kids and below kids without a partial name, or the
// dictionary itself for a field merged with its widget.  A kid whose /Parent
// entry names another dictionary which lists it belongs to that dictionary
// and is left out, as is a widget which belongs to a field of the form.  The
// kids of a kid without a partial name are used only if the /Parent entry of
// that kid names the dictionary which lists it.  A field read in this way
// belongs to no form, and its widgets are not the objects [Annotation]
// returns for their references.  A widget of such a field leads to the field
// only through its /Parent chain.
//
// The result is nil if ref is neither a terminal field of one of the defined
// types nor part of one.  Later calls through x for the same reference
// return the same result.
func FieldAt(x *pdf.Extractor, ref pdf.Reference) (acroform.Field, map[acroform.Widget]pdf.Reference, error) {
	idx, err := pdf.Optional(DocumentForm(x))
	if err != nil {
		return nil, nil, err
	}
	if f := idx.fieldOf(ref); f != nil {
		return f, idx.widgetsOf(f), nil
	}
	if f := idx.fieldOfWidget(ref); f != nil {
		return f, idx.widgetsOf(f), nil
	}

	// The field and its widgets are built as fresh values and published
	// together, as one cached value; a racing caller adopts the first.
	decode := func(c pdf.Cursor, obj pdf.Object, _ bool) (*detachedField, error) {
		return decodeDetached(c, obj, idx)
	}
	cur := pdf.CursorAt(x, nil)
	res, err := pdf.Optional(cur.DecodeOptional(ref, decode))
	if err != nil {
		return nil, nil, err
	}
	if res != nil && res.f != nil {
		return res.f, res.widgets, nil
	}

	// a widget, or a group of widgets, of the nearest named ancestor
	_, owner, err := unnamedChain(cur, ref)
	if err != nil || owner == 0 {
		return nil, nil, err
	}
	f, widgets, err := FieldAt(x, owner)
	if err != nil || f == nil {
		return nil, nil, err
	}
	for _, wref := range widgets {
		chain, _, err := unnamedChain(cur, wref)
		if err != nil {
			return nil, nil, err
		}
		if slices.Contains(chain, ref) {
			return f, widgets, nil
		}
	}
	return nil, nil, nil
}

// widgetsOf returns the widgets of f, a terminal field of the form idx,
// with their references.
func (idx *FormIndex) widgetsOf(f acroform.Field) map[acroform.Widget]pdf.Reference {
	widgets := map[acroform.Widget]pdf.Reference{}
	for _, w := range f.GetCommon().Widgets {
		widgets[w] = idx.WidgetRefs[w]
	}
	return widgets
}

// unnamedChain follows the /Parent chain from ref.  It returns the
// references of the dictionaries without a partial name, starting with ref,
// and the reference of the first dictionary above them which has a partial
// name.  The owner is zero if there is none, and also if ref has a partial
// name itself.
func unnamedChain(c pdf.Cursor, ref pdf.Reference) (chain []pdf.Reference, owner pdf.Reference, err error) {
	seen := map[pdf.Reference]bool{}
	for cur := ref; !seen[cur]; {
		seen[cur] = true
		dict, err := pdf.Optional(c.Dict(cur))
		if err != nil {
			return nil, 0, err
		}
		if dict == nil {
			return chain, 0, nil
		}
		name, err := partialName(c, dict)
		if err != nil {
			return nil, 0, err
		}
		if name != "" {
			if cur == ref {
				return nil, 0, nil
			}
			return chain, cur, nil
		}
		chain = append(chain, cur)
		parent, ok := dict["Parent"].(pdf.Reference)
		if !ok {
			break
		}
		cur = parent
	}
	return chain, 0, nil
}

// fieldOf returns the terminal field of the form read from ref, or nil.  The
// receiver may be nil, for a document without a form.
func (idx *FormIndex) fieldOf(ref pdf.Reference) acroform.Field {
	if idx == nil {
		return nil
	}
	return idx.fieldAt[ref]
}

// fieldOfWidget returns the field of the form which has the widget read
// from ref, or nil.  The receiver may be nil, for a document without a form.
func (idx *FormIndex) fieldOfWidget(ref pdf.Reference) acroform.Field {
	if idx == nil {
		return nil
	}
	return idx.widgetAt[ref]
}

// isFormWidget reports whether ref is a widget of a field of the form.  The
// receiver may be nil, for a document without a form.
func (idx *FormIndex) isFormWidget(ref pdf.Reference) bool {
	return idx.fieldOfWidget(ref) != nil
}

// detachedField is a terminal field read by [FieldAt] outside the document's
// form.  Once published, neither the field nor its widgets change.
type detachedField struct {
	f       acroform.Field
	widgets map[acroform.Widget]pdf.Reference
}

// decodeDetached reads a terminal field outside the document's form idx.
// The result holds a nil field if obj is not a terminal field.
func decodeDetached(c pdf.Cursor, obj pdf.Object, idx *FormIndex) (*detachedField, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	}
	res := &detachedField{}
	p := c.Path()
	if dict == nil || p == nil {
		return res, nil
	}
	// a kid which its parent takes for a widget is not a field
	if _, ok := dict["Parent"].(pdf.Reference); ok {
		if isWidget, err := isWidgetKid(c, dict); err != nil || isWidget {
			return res, err
		}
	}
	ctx, err := inheritedFromChain(c, dict)
	if err != nil {
		return nil, err
	}

	byName := map[string]*nameGroup{}
	merged, err := isMergedFieldDict(c, dict)
	if err != nil {
		return nil, err
	}
	if merged {
		f, w, err := decodeMergedField(c, dict, ctx, byName)
		if err != nil {
			return nil, err
		}
		if f != nil {
			res.f = f
			res.widgets = map[acroform.Widget]pdf.Reference{w: p.Ref}
		}
		return res, nil
	}

	fieldKids, widgetKids, err := partitionKids(c, dict, nil)
	if err != nil {
		return nil, err
	}
	if len(fieldKids) > 0 {
		return res, nil
	}
	f, shared, err := buildTerminal(c, dict, ctx, byName)
	if err != nil || f == nil {
		return res, err
	}
	widgets := map[acroform.Widget]pdf.Reference{}
	for _, ref := range widgetKids {
		if idx.isFormWidget(ref) {
			continue
		}
		w, err := freshWidget(c, ref)
		if err != nil {
			return nil, err
		}
		if w != nil {
			linkWidget(f, w)
			widgets[w] = ref
		}
	}
	settleButtonValue(f, shared)
	res.f, res.widgets = f, widgets
	return res, nil
}

// freshWidget decodes the widget annotation ref as a new value, which is not
// cached.  It returns nil if ref is not a widget annotation.
func freshWidget(c pdf.Cursor, ref pdf.Reference) (*annotation.Widget, error) {
	dict, err := pdf.Optional(c.DictTyped(ref, "Annot"))
	if err != nil || dict == nil {
		return nil, err
	}
	a, err := pdf.Optional(decodeAnnotation(c, dict))
	if err != nil {
		return nil, err
	}
	w, _ := a.(*annotation.Widget)
	return w, nil
}
