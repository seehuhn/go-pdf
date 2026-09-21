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

package annotation

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestAnnotationTypeVersion checks that an annotation type refuses to be
// written into a file older than the type itself, and agrees to be written
// into one as old as the type.  A file which named a subtype it predates
// would be one no reader of that version could make sense of.
func TestAnnotationTypeVersion(t *testing.T) {
	cases := []struct {
		name string
		// first is the earliest version the type may be written at
		first pdf.Version
		make  func() Annotation
	}{
		{"Circle", pdf.V1_3, func() Annotation { return &Circle{} }},
		{"Line", pdf.V1_3, func() Annotation { return &Line{} }},
		{"Square", pdf.V1_3, func() Annotation { return &Square{} }},
		{"Polygon", pdf.V1_5, func() Annotation {
			return &Polygon{Vertices: []float64{0, 0, 1, 0, 1, 1}}
		}},
		{"PolyLine", pdf.V1_5, func() Annotation {
			return &PolyLine{Vertices: []float64{0, 0, 1, 0}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// one version below the type's own
			w, _ := memfile.NewPDFWriter(t, tc.first-1, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := tc.make().Encode(rm); err == nil {
				t.Errorf("written into a %v file, which predates the type", tc.first-1)
			}

			w, _ = memfile.NewPDFWriter(t, tc.first, nil)
			rm = pdf.NewResourceManager(w)
			if _, err := tc.make().Encode(rm); err != nil {
				t.Errorf("not written into a %v file, where the type belongs: %v",
					tc.first, err)
			}
		})
	}
}
