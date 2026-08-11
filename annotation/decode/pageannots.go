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

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
)

// PageAnnotations reads a page's /Annots array, applying the repairs that
// need the whole page rather than a single annotation:
//
//   - An IRT entry whose target is not an annotation on the same page is
//     cleared.  Table 172 requires both annotations to be on the same page,
//     and a reply whose parent is missing would be an annotation no renderer
//     draws and no comment thread accounts for; clearing the entry makes it
//     an ordinary annotation instead.
//   - If the page carries widget annotations, the document's interactive
//     form is read as well, which links each widget to its form field
//     ([annotation.Widget.Field]).
//
// Array entries that are not indirect references are skipped, as are entries
// that fail to decode.  The two slices are aligned: refs[i] is the reference
// annots[i] was read from.
//
// Every consumer of a page's annotations should obtain them through this
// function (or through the page decoder, which does).  An annotation decoded
// on its own misses the repairs, and since all consumers sharing an extractor
// see one Go value per annotation, a consumer skipping the repairs would
// disagree with the others about the same annotation.
//
// The IRT repair writes to those shared annotation values without
// synchronization.  The page decoder is safe because [pdf.DecodeExclusive]
// single-flights it, but callers invoking this function directly must not run
// it concurrently with other users of the same extractor.
func PageAnnotations(c pdf.Cursor, obj pdf.Object) (refs []pdf.Reference, annots []annotation.Annotation, err error) {
	arr, err := pdf.Optional(c.Array(obj))
	if err != nil {
		return nil, nil, err
	}

	pageRefs := map[pdf.Reference]bool{}
	for _, item := range arr {
		ref, ok := item.(pdf.Reference)
		if !ok {
			continue
		}
		a, err := pdf.Decode(c, item, Annotation)
		if err != nil {
			// permissive: skip invalid annotations
			continue
		}
		refs = append(refs, ref)
		annots = append(annots, a)
		pageRefs[ref] = true
	}

	// clear InReplyTo entries whose target is not on this page (table 172)
	hasWidget := false
	for _, a := range annots {
		if _, ok := a.(*annotation.Widget); ok {
			hasWidget = true
		}
		if m, ok := a.(annotation.MarkupAnnotation); ok {
			markup := m.GetMarkup()
			if markup.InReplyTo != 0 && !pageRefs[markup.InReplyTo] {
				markup.InReplyTo = 0
			}
		}
	}

	// A widget annotation belongs to a form field.  Reading the interactive
	// form links each widget to its field (annotation.Widget.Field); the
	// page's widgets are already cached, so the form's top-down walk links
	// them via cache hits without a cycle.  DecodeExclusive single-flights
	// so concurrent page decodes share one field tree.  Errors are non-fatal:
	// a malformed form must not break page decoding.
	if hasWidget {
		if m := c.Getter().GetMeta(); m != nil && m.Catalog != nil && m.Catalog.AcroForm != nil {
			_, _ = pdf.DecodeExclusive(pdf.CursorAt(c.Extractor(), nil), m.Catalog.AcroForm, Form)
		}
	}

	return refs, annots, nil
}
