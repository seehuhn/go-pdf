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

package image

import (
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func extractMaskWith(t *testing.T, extras pdf.Dict) *Mask {
	t.Helper()
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	dict := pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(8),
		"Height":           pdf.Integer(8),
		"ImageMask":        pdf.Boolean(true),
		"BitsPerComponent": pdf.Integer(1),
	}
	maps.Copy(dict, extras)
	ref := w.Alloc()
	body, err := w.OpenStream(ref, dict)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := body.Write(make([]byte, 8)); err != nil {
		t.Fatal(err)
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	mask, err := ExtractMask(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	return mask
}

// TestExtractMaskDanglingReference checks that a reference to a free object
// reads the same as an absent entry.
func TestExtractMaskDanglingReference(t *testing.T) {
	keys := []pdf.Name{
		"ColorSpace", "Mask", "BitsPerComponent", "Decode", "Interpolate",
		"Alternates", "Name", "Metadata", "OC", "Measure", "PtData",
		"StructParent", "AF", "ID",
	}
	dangling := pdf.NewReference(99, 0)
	opt := cmpopts.IgnoreFields(Mask{}, "Source")
	want := extractMaskWith(t, nil)
	for _, key := range keys {
		t.Run(string(key), func(t *testing.T) {
			got := extractMaskWith(t, pdf.Dict{key: dangling})
			if diff := cmp.Diff(want, got, opt); diff != "" {
				t.Errorf("dangling reference differs from absent entry (-want +got):\n%s", diff)
			}
		})
	}
}

// TestExtractMaskMalformedStructParent checks that a StructParent entry of
// the wrong type reads the same as an absent entry.
func TestExtractMaskMalformedStructParent(t *testing.T) {
	opt := cmpopts.IgnoreFields(Mask{}, "Source")
	want := extractMaskWith(t, nil)
	got := extractMaskWith(t, pdf.Dict{"StructParent": pdf.String("bad")})
	if diff := cmp.Diff(want, got, opt); diff != "" {
		t.Errorf("malformed value differs from absent entry (-want +got):\n%s", diff)
	}
}

// TestExtractMaskMalformedOptional checks that Metadata and Measure entries
// of the wrong type read the same as absent entries.
func TestExtractMaskMalformedOptional(t *testing.T) {
	opt := cmpopts.IgnoreFields(Mask{}, "Source")
	want := extractMaskWith(t, nil)
	for _, key := range []pdf.Name{"Metadata", "Measure"} {
		for _, bad := range []pdf.Object{pdf.Integer(1), pdf.String("bad")} {
			got := extractMaskWith(t, pdf.Dict{key: bad})
			if diff := cmp.Diff(want, got, opt); diff != "" {
				t.Errorf("%s %v: malformed value differs from absent entry (-want +got):\n%s", key, bad, diff)
			}
		}
	}
}

// TestExtractMaskForbiddenEntries checks that ColorSpace and Mask entries,
// which are not allowed for image masks, are ignored.
func TestExtractMaskForbiddenEntries(t *testing.T) {
	opt := cmpopts.IgnoreFields(Mask{}, "Source")
	want := extractMaskWith(t, nil)
	for name, extras := range map[string]pdf.Dict{
		"ColorSpace": {"ColorSpace": pdf.Name("DeviceGray")},
		"colour key": {"Mask": pdf.Array{pdf.Integer(0), pdf.Integer(0)}},
		"both": {
			"ColorSpace": pdf.Name("DeviceGray"),
			"Mask":       pdf.Array{pdf.Integer(0), pdf.Integer(0)},
		},
	} {
		got := extractMaskWith(t, extras)
		if diff := cmp.Diff(want, got, opt); diff != "" {
			t.Errorf("%s: entry not ignored (-want +got):\n%s", name, diff)
		}
	}
}

// TestExtractMaskMalformedDecode checks that a Decode array with a
// malformed element does not invert the mask.
func TestExtractMaskMalformedDecode(t *testing.T) {
	for _, bad := range []pdf.Array{
		{pdf.Integer(1), nil},
		{pdf.Integer(1), pdf.Name("x")},
		{pdf.String("1"), pdf.Integer(0)},
	} {
		got := extractMaskWith(t, pdf.Dict{"Decode": bad})
		if got.Inverted {
			t.Errorf("Decode %v: mask inverted", bad)
		}
	}
	if got := extractMaskWith(t, pdf.Dict{"Decode": pdf.Array{pdf.Integer(1), pdf.Integer(0)}}); !got.Inverted {
		t.Error("Decode [1 0]: mask not inverted")
	}
}
