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

package pdf

import (
	"bytes"
	"testing"

	"golang.org/x/text/language"
)

func TestDecodeCatalogLang(t *testing.T) {
	cases := []struct {
		in   string
		want language.Tag
	}{
		{"en-US", language.AmericanEnglish},
		{"en-XYZ", language.English},
	}
	for _, tc := range cases {
		dict := Dict{
			"Type":  Name("Catalog"),
			"Pages": NewReference(1, 0),
			"Lang":  Text(tc.in),
		}
		cat, err := DecodeCatalog(NewCursor(mockGetter), dict, false)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if cat.Lang != tc.want {
			t.Errorf("%q: got Lang %v, want %v", tc.in, cat.Lang, tc.want)
		}
	}
}

// TestDecodeCatalogWrongType checks that a catalog with a missing or wrong
// /Type entry is read, and that /Type /Catalog is written back.
func TestDecodeCatalogWrongType(t *testing.T) {
	for _, tp := range []Object{nil, Name("catalog"), Name("Pages"), Integer(1)} {
		dict := Dict{"Pages": NewReference(1, 0)}
		if tp != nil {
			dict["Type"] = tp
		}
		cat, err := DecodeCatalog(NewCursor(mockGetter), dict, false)
		if err != nil {
			t.Fatalf("/Type %v: %v", tp, err)
		}
		if cat.Pages != NewReference(1, 0) {
			t.Errorf("/Type %v: got Pages %v", tp, cat.Pages)
		}

		w, err := NewWriter(&bytes.Buffer{}, V1_7, nil)
		if err != nil {
			t.Fatal(err)
		}
		obj, err := cat.Encode(NewResourceManager(w))
		if err != nil {
			t.Fatal(err)
		}
		if got := obj.(Dict)["Type"]; got != Name("Catalog") {
			t.Errorf("/Type %v: written as %v", tp, got)
		}
	}
}

// TestCatalogEncodeUnsupportedVersion verifies that a Version the library
// cannot write itself is written through unchanged.
func TestCatalogEncodeUnsupportedVersion(t *testing.T) {
	w, err := NewWriter(&bytes.Buffer{}, V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}
	rm := NewResourceManager(w)
	v18, err := ParseVersion("1.8")
	if err != nil {
		t.Fatal(err)
	}
	c := &Catalog{Pages: w.Alloc(), Version: v18}
	obj, err := c.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}
	if got := obj.(Dict)["Version"]; got != Name("1.8") {
		t.Errorf("Version = %v, want /1.8", got)
	}
}
