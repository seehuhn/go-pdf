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
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// annotWithoutAppearance writes a0 to a file of the given version, strips the
// AP and AS entries from the resulting dictionary, and reads it back.  Going
// through a real encode keeps the dictionary complete, so that decoders which
// insist on their required entries still succeed.
func annotWithoutAppearance(t *testing.T, v pdf.Version, a0 annotation.Annotation, rect pdf.Rectangle) annotation.Annotation {
	t.Helper()

	w, _ := memfile.NewPDFWriter(v, nil)
	rm := pdf.NewResourceManager(w)

	a := shallowCopy(a0)
	c := a.GetCommon()
	c.Rect = rect
	c.Appearance = appearanceFor(a)
	c.AppearanceState = ""

	obj, err := a.Encode(rm)
	if pdf.IsWrongVersion(err) {
		t.Skip()
	} else if err != nil {
		t.Fatalf("cannot write: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("expected a dictionary, got %T", obj)
	}
	delete(dict, "AP")
	delete(dict, "AS")

	x := pdf.NewExtractor(w)
	res, err := Annotation(pdf.CursorAt(x, nil), dict, false)
	if err != nil {
		t.Fatalf("cannot read: %v", err)
	}
	return res
}

// TestMissingAppearanceIsWritable checks that an annotation which needs an
// appearance dictionary but has none is repaired on read, so that it can be
// written back.  Before the repair existed, reading such an annotation
// succeeded and writing it failed with "missing appearance dictionary".
func TestMissingAppearanceIsWritable(t *testing.T) {
	rect := pdf.Rectangle{URx: 100, URy: 100}

	for subtype, cases := range testCases {
		for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
			t.Run(string(subtype)+"-"+v.String(), func(t *testing.T) {
				a := annotWithoutAppearance(t, v, cases[0].annotation, rect)

				out, _ := memfile.NewPDFWriter(v, nil)
				rm := pdf.NewResourceManager(out)
				if _, err := a.Encode(rm); err != nil && !pdf.IsWrongVersion(err) {
					t.Errorf("cannot write back: %v", err)
				}
			})
		}
	}
}

// TestMissingAppearanceRepairIsExact checks that the read side supplies an
// appearance exactly when the write side insists on one.  If the two rules
// drift apart, either annotations become unwritable or we invent appearances
// nobody asked for.
func TestMissingAppearanceRepairIsExact(t *testing.T) {
	rects := map[string]pdf.Rectangle{
		"area":  {URx: 100, URy: 100},
		"point": {LLx: 7, LLy: 9, URx: 7, URy: 9},
	}

	for subtype, cases := range testCases {
		for rectName, rect := range rects {
			for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
				name := string(subtype) + "-" + rectName + "-" + v.String()
				t.Run(name, func(t *testing.T) {
					a := annotWithoutAppearance(t, v, cases[0].annotation, rect)

					want := annotation.AppearanceRequired(subtype, rect, v)
					got := a.GetCommon().Appearance != nil
					if got != want {
						t.Errorf("appearance supplied = %v, required = %v", got, want)
					}
				})
			}
		}
	}
}

// TestSuppliedAppearanceIsEmpty checks that the supplied appearance draws
// nothing.  A renderer treats it as absent and generates its own fallback, so
// reading a file must not fix the annotation's appearance in place.
func TestSuppliedAppearanceIsEmpty(t *testing.T) {
	rect := pdf.Rectangle{URx: 100, URy: 100}
	a := annotWithoutAppearance(t, pdf.V2_0, testCases["Square"][0].annotation, rect)

	ap := a.GetCommon().Appearance
	if ap == nil {
		t.Fatal("expected an appearance to be supplied")
	}
	if ap.Normal == nil {
		t.Fatal("expected a normal appearance")
	}
	if ap.Normal.Content != nil {
		t.Error("expected the supplied appearance to have no content")
	}
	if ap.Normal.BBox != rect {
		t.Errorf("expected the BBox to match Rect, got %v", ap.Normal.BBox)
	}
}
