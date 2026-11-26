// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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

func TestCatalogMinVersion(t *testing.T) {
	cases := []struct {
		name string
		c    *Catalog
		want Version
	}{
		{"pages only", &Catalog{Pages: 1}, V1_0},
		{"dests", &Catalog{Pages: 1, Dests: Dict{}}, V1_1},
		{"acroform", &Catalog{Pages: 1, AcroForm: Dict{}}, V1_2},
		{"lang", &Catalog{Pages: 1, Lang: language.German}, V1_4},
		{"ocproperties", &Catalog{Pages: 1, OCProperties: Dict{}}, V1_5},
		{"needs rendering", &Catalog{Pages: 1, NeedsRendering: true}, V1_5},
		{"page mode use outlines", &Catalog{Pages: 1, PageMode: "UseOutlines"}, V1_0},
		{"page mode use OC", &Catalog{Pages: 1, PageMode: "UseOC"}, V1_5},
		{"page mode use attachments", &Catalog{Pages: 1, PageMode: "UseAttachments"}, V1_6},
		{"page layout two column left", &Catalog{Pages: 1, PageLayout: "TwoColumnLeft"}, V1_0},
		{"page layout two page left", &Catalog{Pages: 1, PageLayout: "TwoPageLeft"}, V1_5},
		{"page layout two page right", &Catalog{Pages: 1, PageLayout: "TwoPageRight"}, V1_5},
		{"af", &Catalog{Pages: 1, AF: Array{}}, V2_0},
		{"maximum", &Catalog{Pages: 1, AcroForm: Dict{}, AF: Array{}, Dests: Dict{}}, V2_0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.c.MinVersion(); got != tc.want {
				t.Errorf("MinVersion() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCatalogMinVersionEncode checks that a catalog can be encoded at
// exactly the version MinVersion reports, and not below.
func TestCatalogMinVersionEncode(t *testing.T) {
	cats := []*Catalog{
		{Pages: 1, AcroForm: Dict{}},
		{Pages: 1, OCProperties: Dict{}, Dests: Dict{}},
		{Pages: 1, Requirements: Array{}},
		{Pages: 1, PageMode: "UseOC"},
		{Pages: 1, PageMode: "UseAttachments"},
		{Pages: 1, PageLayout: "TwoPageLeft"},
		{Pages: 1, PageLayout: "TwoPageRight"},
	}
	for _, c := range cats {
		v := c.MinVersion()
		w, err := NewWriter(&bytes.Buffer{}, v, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Encode(NewResourceManager(w)); err != nil {
			t.Errorf("encode at %v: %v", v, err)
		}
		w, err = NewWriter(&bytes.Buffer{}, v-1, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Encode(NewResourceManager(w)); err == nil {
			t.Errorf("encode at %v succeeded", v-1)
		}
	}
}
