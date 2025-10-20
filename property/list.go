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

import "seehuhn.de/go/pdf"

// PDF 2.0 sections: 14.6

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
//   - The property list IS an OCG or OCMD dictionary, so it has those
//     dictionary keys (Type, Name, Intent, Usage for OCG; Type, OCGs, P,
//     VE for OCMD).

// List represents a marked-property list.
//
// TODO(voss): add methods to inspect the object.
type List interface {
	// Keys returns the dictionary keys present in the property list.
	Keys() []pdf.Name

	// Get retrieves the value associated with the given key.
	// If the key is not present, the error [ErrNoKey] is returned.
	Get(key pdf.Name) (*ResolvedObject, error)

	pdf.Embedder
}

type ResolvedObject struct {
	obj pdf.Object
	x   *pdf.Extractor
}

var _ pdf.Object = (*ResolvedObject)(nil)

func (r *ResolvedObject) AsPDF(opt pdf.OutputOptions) pdf.Native {
	obj := r.obj.AsPDF(opt)

	if ref, ok := obj.(pdf.Reference); ok {
		resolved, err := r.x.Resolve(ref)
		if err != nil {
			resolved = nil // TODO(voss): what to do on error?
		}
		obj = resolved
	}

	switch obj := obj.(type) {
	case pdf.Dict:
		res := make(pdf.Dict, len(obj))
		for k, v := range obj {
			res[k] = &ResolvedObject{v, r.x}
		}
		return res
	case pdf.Array:
		res := make(pdf.Array, len(obj))
		for i, v := range obj {
			res[i] = &ResolvedObject{v, r.x}
		}
		return res
	case *pdf.Stream:
		res := *obj
		res.Dict = make(pdf.Dict, len(obj.Dict))
		for k, v := range obj.Dict {
			res.Dict[k] = &ResolvedObject{v, r.x}
		}
		return &res
	default:
		return obj
	}
}

// ErrNoKey is returned by List.Get if the requested key is not present
// in the property list.
var ErrNoKey = pdf.Error("no such key in property list")
