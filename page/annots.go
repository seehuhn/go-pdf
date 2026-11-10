// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package page

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/decode"
)

// Annots is the list of annotations of a page.
type Annots struct {
	// List holds the page's annotations, in the order they are drawn and
	// scanned for input.
	List []annotation.Annotation

	// SingleUse can be set if the list is written inline into the page
	// dictionary instead of as an indirect object.  Indirect lists are the
	// default: a later incremental update can then replace the list without
	// rewriting the page.
	SingleUse bool
}

// Add appends annotations to the list.
//
// Do not call this on a list decoded through the Writer you are writing to:
// decoded values are immutable and the addition would be silently lost.
// Copy the list and use [pdf.ResourceManager.Replace] instead.
func (a *Annots) Add(annots ...annotation.Annotation) {
	a.List = append(a.List, annots...)
}

// Embed writes the annotations and returns the array of their references,
// inline or as an indirect object.  An empty list embeds as nil.
//
// This implements the [pdf.Embedder] interface.
func (a *Annots) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if len(a.List) == 0 {
		return nil, nil
	}
	arr := make(pdf.Array, len(a.List))
	for i, ai := range a.List {
		ref, err := e.Store(ai)
		if err != nil {
			return nil, err
		}
		arr[i] = ref
	}
	if a.SingleUse {
		return arr, nil
	}
	ref := e.AllocSelf()
	if err := e.Out().Put(ref, arr); err != nil {
		return nil, err
	}
	return ref, nil
}

// decodeAnnots reads an /Annots array.  An empty or malformed array reads as
// nil, matching [Annots.Embed], which writes nothing for an empty list; this
// keeps a read-write-read cycle stable.
func decodeAnnots(c pdf.Cursor, obj pdf.Object, isDirect bool) (*Annots, error) {
	_, list, err := decode.PageAnnotations(c, obj)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return &Annots{List: list, SingleUse: isDirect}, nil
}
