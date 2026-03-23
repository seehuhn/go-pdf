// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package property

import (
	"seehuhn.de/go/pdf"
)

// Standard tags (from Table 352a and spec):
//   1. AF - Associated files (14.13.5)
//   2. Artifact - Artifacts (14.8.2.2.2)
//   3. OC - Optional content (8.11.3.2)
//   4. ReversedChars - Reverse order show strings (14.8.2.5.3)
//   5. Span - Alternate descriptions, replacement text, expansion (14.9.3, 14.9.4, 14.9.5)
//   6. Tx - Variable text field replacement (12.7.4.3)
//
// Property list keys with defined meanings:
//
// Generic (any tag):
//   - MCID (integer) - Marked-content identifier for structure (14.7.5)
//
// For AF tag:
//   - MCAF (array) - Array of file specification dictionaries (Table 409a)
//
// For Artifact tag (Table 363):
//   - Type (name) - Pagination, Layout, Page, Background
//   - BBox (rectangle) - Bounding box
//   - Attached (array) - Page edges (Top, Bottom, Left, Right)
//   - Subtype (name) - Header, Footer, Watermark, PageNum, LineNum, Redaction, Bates
//   - Alt, ActualText, E, Lang (text strings)
//
// For Span tag:
//   - Alt, ActualText, E, Lang (text strings) - from 14.9.x
//
// For OC tag:
//   - The property list itself is an OCG or OCMD dictionary, so it has those
//     dictionary keys (Type, Name, Intent, Usage for OCG; Type, OCGs, P,
//     VE for OCMD).

// PDF 2.0 sections: 14.6

// List represents a marked-property list.
type List interface {
	// AsDirectDict returns the property list as a direct PDF dictionary
	// if it can be embedded inline in a content stream.
	// Returns nil if the property list must be referenced via the
	// Properties resource dictionary.
	AsDirectDict() pdf.Dict

	// Equal reports whether two property lists are semantically equal.
	Equal(other List) bool

	pdf.Embedder
}

// ListGet resolves indirect references and extracts a typed object.
//
// This only works for property lists that have been extracted from a PDF file.
//
// Once Go allows generic methods, this function can be made a method of List.
func ListGet[T any](l List, extract func(*pdf.Extractor, *pdf.CycleCheck, pdf.Object, bool) (T, error)) (T, error) {
	p, ok := l.(*proxyList)
	if !ok {
		var zero T
		return zero, pdf.Error("ListGet only works for property lists extracted from PDF files")
	}
	return pdf.ExtractorGet(p.x, p.path, p.obj, extract)
}

// ListsEqual compares two property lists for semantic equality.
func ListsEqual(a, b List) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(b)
}
