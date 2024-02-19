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
)

// TestGetResourceName1 tests that resources of all categories can be
// added to the resource dictionary.
func TestGetResourceName1(t *testing.T) {
	w := NewWriter(io.Discard, pdf.V1_7)
	ref := pdf.NewReference(1, 2)
	r := &pdf.Res{
		DefName: "Q",
		Ref:     ref,
	}
	var allCats = []resourceCategory{
		catExtGState,
		catColorSpace,
		catPattern,
		catShading,
		catXObject,
		catFont,
		catProperties,
	}
	for _, cat := range allCats {
		// test name generation
		name := w.getResourceName(cat, r)
		if name != "Q" {
			t.Errorf("expected Q, got %s", name)
		}

		// test caching
		name = w.getResourceName(cat, r)
		if name != "Q" {
			t.Errorf("expected Q, got %s", name)
		}
	}

	if w.Resources.ExtGState["Q"] != ref {
		t.Errorf("expected Q, got %s", w.Resources.ExtGState["Q"])
	}
	if w.Resources.ColorSpace["Q"] != ref {
		t.Errorf("expected Q, got %s", w.Resources.ColorSpace["Q"])
	}
	if w.Resources.Pattern["Q"] != ref {
		t.Errorf("expected Q, got %s", w.Resources.Pattern["Q"])
	}
	if w.Resources.Shading["Q"] != ref {
		t.Errorf("expected Q, got %s", w.Resources.Shading["Q"])
	}
	if w.Resources.XObject["Q"] != ref {
		t.Errorf("expected Q, got %s", w.Resources.XObject["Q"])
	}
	if w.Resources.Font["Q"] != ref {
		t.Errorf("expected Q, got %s", w.Resources.Font["Q"])
	}
	if w.Resources.Properties["Q"] != ref {
		t.Errorf("expected Q, got %s", w.Resources.Properties["Q"])
	}
}

// TestGetResourceName2 tests that names are generated correctly when
// there is more than one resource of the same category.
func TestGetResourceName2(t *testing.T) {
	defName := pdf.Name("F3")
	w := NewWriter(io.Discard, pdf.V1_7)

	// allocate 9 resources
	var names []pdf.Name
	for i := 0; i < 9; i++ {
		r := &pdf.Res{
			DefName: defName,
			Ref:     pdf.Reference(i + 1),
		}
		names = append(names, w.getResourceName(catFont, r))
	}

	if names[0] != defName {
		t.Errorf("expected %s, got %s", defName, names[0])
	}
	for i, name := range names {
		if len(name) > 2 {
			t.Errorf("name too long: %q", name)
		}
		ref, ok := w.Resources.Font[name]
		if !ok {
			t.Errorf("expected %s, got %s", defName, name)
			continue
		}
		if ref != pdf.Reference(i+1) {
			t.Errorf("expected %d, got %d", i+1, ref)
		}
	}
}

// TestGetResourceName3 tests that the reserved color space names
// are always returned when the resource is the corresponding color space.
func TestGetResourceName3(t *testing.T) {
	w := NewWriter(io.Discard, pdf.V1_7)

	for _, name := range []pdf.Name{"DeviceGray", "DeviceRGB", "DeviceCMYK", "Pattern"} {
		r := &pdf.Res{
			Ref: name,
		}
		got := w.getResourceName(catColorSpace, r)
		if got != name {
			t.Errorf("expected %s, got %s", name, got)
		}
	}
}

// TestGetResourceName4 tests that the reserved color space names
// are never added to the resource dictionary.
func TestGetResourceName4(t *testing.T) {
	w := NewWriter(io.Discard, pdf.V1_7)

	for _, name := range []pdf.Name{"DeviceGray", "DeviceRGB", "DeviceCMYK", "Pattern"} {
		r := &pdf.Res{
			DefName: name,
			Ref:     pdf.NewReference(1, 0),
		}
		w.getResourceName(catColorSpace, r)
	}

	for name := range w.Resources.ColorSpace {
		if name == "DeviceGray" || name == "DeviceRGB" || name == "DeviceCMYK" || name == "Pattern" {
			t.Errorf("unexpected name: %s", name)
		}
	}
}
