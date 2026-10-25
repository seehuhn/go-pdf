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

package extract

import (
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func decodeFormWith(t *testing.T, extras pdf.Dict) *form.Form {
	t.Helper()
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)

	dict := pdf.Dict{
		"Type":    pdf.Name("XObject"),
		"Subtype": pdf.Name("Form"),
		"BBox":    pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(10), pdf.Integer(10)},
	}
	maps.Copy(dict, extras)
	ref := w.Alloc()
	body, err := w.OpenStream(ref, dict)
	if err != nil {
		t.Fatal(err)
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	f, err := Form(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// TestFormDanglingReference checks that a reference to a free object reads
// the same as an absent entry.
func TestFormDanglingReference(t *testing.T) {
	keys := []pdf.Name{
		"Name", "Matrix", "Group", "Ref", "OPI", "Metadata", "PieceInfo",
		"LastModified", "OC", "Measure", "PtData", "StructParent", "AF",
		"Resources", "PCM", "SeparationColorNames", "TrapRegions",
		"TrapStyles", "MarkStyle", "Colorants",
	}
	dangling := pdf.NewReference(99, 0)
	opt := cmpopts.IgnoreFields(form.Form{}, "Content")
	want := decodeFormWith(t, nil)
	for _, key := range keys {
		t.Run(string(key), func(t *testing.T) {
			got := decodeFormWith(t, pdf.Dict{key: dangling})
			if diff := cmp.Diff(want, got, opt); diff != "" {
				t.Errorf("dangling reference differs from absent entry (-want +got):\n%s", diff)
			}
		})
	}
}
