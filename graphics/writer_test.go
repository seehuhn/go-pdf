// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package graphics

import (
	"io"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

type dummyResource pdf.Reference

func (r dummyResource) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	return pdf.Reference(r), nil
}

// TestGetResourceName1 tests that resources of all categories can be
// added to the resource dictionary.
func TestGetResourceName1(t *testing.T) {
	data, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(data)

	ref := data.Alloc()
	r := dummyResource(ref)

	w := NewWriter(io.Discard, rm)
	var allCats = []resourceCategory{
		catExtGState,
		catColorSpace,
		catPattern,
		catShading,
		catXObject,
		catFont,
		catProperties,
	}
	var allNames []pdf.Name
	for _, cat := range allCats {
		// test name generation
		name1, err := writerGetResourceName(w, cat, r)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// test caching
		name2, err := writerGetResourceName(w, cat, r)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		} else if name1 != name2 {
			t.Errorf("expected %s, got %s", name1, name2)
		}

		allNames = append(allNames, name1)
	}

	if obj := w.Resources.ExtGState[allNames[0]]; obj != ref {
		t.Errorf("expected %s, got %s", ref, obj)
	}
	if obj := w.Resources.ColorSpace[allNames[1]]; obj != ref {
		t.Errorf("expected %s, got %s", ref, obj)
	}
	if obj := w.Resources.Pattern[allNames[2]]; obj != ref {
		t.Errorf("expected %s, got %s", ref, obj)
	}
	if obj := w.Resources.Shading[allNames[3]]; obj != ref {
		t.Errorf("expected %s, got %s", ref, obj)
	}
	if obj := w.Resources.XObject[allNames[4]]; obj != ref {
		t.Errorf("expected %s, got %s", ref, obj)
	}
	if obj := w.Resources.Font[allNames[5]]; obj != ref {
		t.Errorf("expected %s, got %s", ref, obj)
	}
	if obj := w.Resources.Properties[allNames[6]]; obj != ref {
		t.Errorf("expected %s, got %s", ref, obj)
	}
}
