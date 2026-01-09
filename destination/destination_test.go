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

package destination

import (
	"bytes"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

type testCase struct {
	Name string
	Dest Destination
}

// decodeTestCase represents a test case for decoding raw PDF arrays.
// These may include malformed inputs, as long as the decode function
// has code to fix them up so they can round-trip.
type decodeTestCase struct {
	Name string
	Obj  pdf.Object // raw PDF object to decode
}

var decodeTestCases = []decodeTestCase{
	// FitR with null coordinates - nulls decode as 0, creating invalid rectangle
	{"FitR with nulls", pdf.Array{pdf.Reference(10), pdf.Name("FitR"), nil, nil, nil, nil}},
}

var testCases = []testCase{
	{"XYZ", &XYZ{Page: Target(pdf.Reference(10)), Left: 100, Top: 200, Zoom: 1.5}},
	{"XYZ with Unset", &XYZ{Page: Target(pdf.Reference(10)), Left: Unset, Top: Unset, Zoom: Unset}},
	{"Fit", &Fit{Page: Target(pdf.Reference(10))}},
	{"FitH", &FitH{Page: Target(pdf.Reference(10)), Top: 500}},
	{"FitV", &FitV{Page: Target(pdf.Reference(10)), Left: 100}},
	{"FitR", &FitR{Page: Target(pdf.Reference(10)), Left: 100, Bottom: 200, Right: 400, Top: 500}},
	{"FitB", &FitB{Page: Target(pdf.Reference(10))}},
	{"FitBH", &FitBH{Page: Target(pdf.Reference(10)), Top: 600}},
	{"FitBV", &FitBV{Page: Target(pdf.Reference(10)), Left: 50}},
	{"Named", &Named{Name: pdf.String("Chapter6")}},
}

// testRoundTrip encodes a destination, decodes it back, and verifies the result
// matches the original using cmp.Diff.
func testRoundTrip(t *testing.T, d Destination) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	// encode
	obj, err := d.Encode(rm)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	// decode
	x := pdf.NewExtractor(w)
	decoded, err := Decode(x, obj)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// compare with custom NaN comparer
	opts := cmp.Options{
		cmp.Comparer(func(a, b float64) bool {
			return a == b || (math.IsNaN(a) && math.IsNaN(b))
		}),
	}
	if diff := cmp.Diff(d, decoded, opts); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestXYZ(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pageRef := w.Alloc()

	dest := &XYZ{
		Page: Target(pageRef),
		Left: 100,
		Top:  200,
		Zoom: 1.5,
	}

	obj, err := dest.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}

	arr, ok := obj.(pdf.Array)
	if !ok {
		t.Fatalf("expected Array, got %T", obj)
	}

	if len(arr) != 5 {
		t.Fatalf("expected 5 elements, got %d", len(arr))
	}

	if arr[0] != pageRef {
		t.Errorf("page: got %v, want %v", arr[0], pageRef)
	}

	if arr[1] != pdf.Name("XYZ") {
		t.Errorf("type: got %v, want XYZ", arr[1])
	}

	if arr[2] != pdf.Number(100) {
		t.Errorf("left: got %v, want 100", arr[2])
	}

	if arr[3] != pdf.Number(200) {
		t.Errorf("top: got %v, want 200", arr[3])
	}

	if arr[4] != pdf.Number(1.5) {
		t.Errorf("zoom: got %v, want 1.5", arr[4])
	}
}

func TestXYZWithUnset(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pageRef := w.Alloc()

	dest := &XYZ{
		Page: Target(pageRef),
		Left: Unset,
		Top:  Unset,
		Zoom: Unset,
	}

	obj, err := dest.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}

	arr := obj.(pdf.Array)

	if arr[2] != nil {
		t.Errorf("left: got %v, want nil", arr[2])
	}

	if arr[3] != nil {
		t.Errorf("top: got %v, want nil", arr[3])
	}

	if arr[4] != nil {
		t.Errorf("zoom: got %v, want nil", arr[4])
	}
}

func TestXYZInvalidValues(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pageRef := w.Alloc()

	tests := []struct {
		name string
		dest *XYZ
	}{
		{
			name: "infinite left",
			dest: &XYZ{Page: Target(pageRef), Left: math.Inf(1), Top: 0, Zoom: 1},
		},
		{
			name: "infinite top",
			dest: &XYZ{Page: Target(pageRef), Left: 0, Top: math.Inf(-1), Zoom: 1},
		},
		{
			name: "infinite zoom",
			dest: &XYZ{Page: Target(pageRef), Left: 0, Top: 0, Zoom: math.Inf(1)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.dest.Encode(rm)
			if err == nil {
				t.Error("expected error for infinite value, got nil")
			}
		})
	}
}

func TestFit(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pageRef := w.Alloc()

	dest := &Fit{
		Page: Target(pageRef),
	}

	obj, err := dest.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}

	arr, ok := obj.(pdf.Array)
	if !ok {
		t.Fatalf("expected Array, got %T", obj)
	}

	if len(arr) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr))
	}

	if arr[0] != pageRef {
		t.Errorf("page: got %v, want %v", arr[0], pageRef)
	}

	if arr[1] != pdf.Name("Fit") {
		t.Errorf("type: got %v, want Fit", arr[1])
	}
}

func TestFitH(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pageRef := w.Alloc()

	dest := &FitH{
		Page: Target(pageRef),
		Top:  500,
	}

	obj, err := dest.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}

	arr := obj.(pdf.Array)
	if len(arr) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr))
	}

	if arr[1] != pdf.Name("FitH") {
		t.Errorf("type: got %v, want FitH", arr[1])
	}

	if arr[2] != pdf.Number(500) {
		t.Errorf("top: got %v, want 500", arr[2])
	}
}

func TestFitV(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pageRef := w.Alloc()

	dest := &FitV{
		Page: Target(pageRef),
		Left: 100,
	}

	obj, err := dest.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}

	arr := obj.(pdf.Array)
	if len(arr) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr))
	}

	if arr[1] != pdf.Name("FitV") {
		t.Errorf("type: got %v, want FitV", arr[1])
	}

	if arr[2] != pdf.Number(100) {
		t.Errorf("left: got %v, want 100", arr[2])
	}
}

func TestFitR(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pageRef := w.Alloc()

	dest := &FitR{
		Page:   Target(pageRef),
		Left:   100,
		Bottom: 200,
		Right:  400,
		Top:    500,
	}

	obj, err := dest.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}

	arr := obj.(pdf.Array)
	if len(arr) != 6 {
		t.Fatalf("expected 6 elements, got %d", len(arr))
	}

	if arr[1] != pdf.Name("FitR") {
		t.Errorf("type: got %v, want FitR", arr[1])
	}
}

func TestFitRInvalidRectangle(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pageRef := w.Alloc()

	tests := []struct {
		name string
		dest *FitR
	}{
		{
			name: "left >= right",
			dest: &FitR{Page: Target(pageRef), Left: 400, Bottom: 200, Right: 100, Top: 500},
		},
		{
			name: "bottom >= top",
			dest: &FitR{Page: Target(pageRef), Left: 100, Bottom: 500, Right: 400, Top: 200},
		},
		{
			name: "infinite coordinate",
			dest: &FitR{Page: Target(pageRef), Left: math.Inf(1), Bottom: 200, Right: 400, Top: 500},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.dest.Encode(rm)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestFitB(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pageRef := w.Alloc()

	dest := &FitB{Page: Target(pageRef)}

	obj, err := dest.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}

	arr := obj.(pdf.Array)
	if arr[1] != pdf.Name("FitB") {
		t.Errorf("type: got %v, want FitB", arr[1])
	}
}

func TestFitBVersionCheck(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_0, nil)
	rm := pdf.NewResourceManager(w)
	pageRef := w.Alloc()

	dest := &FitB{Page: Target(pageRef)}

	_, err := dest.Encode(rm)
	if err == nil {
		t.Error("expected version error for PDF 1.0, got nil")
	}
}

func TestFitBH(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pageRef := w.Alloc()

	dest := &FitBH{Page: Target(pageRef), Top: 600}

	obj, err := dest.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}

	arr := obj.(pdf.Array)
	if arr[1] != pdf.Name("FitBH") {
		t.Errorf("type: got %v, want FitBH", arr[1])
	}
}

func TestFitBV(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pageRef := w.Alloc()

	dest := &FitBV{Page: Target(pageRef), Left: 50}

	obj, err := dest.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}

	arr := obj.(pdf.Array)
	if arr[1] != pdf.Name("FitBV") {
		t.Errorf("type: got %v, want FitBV", arr[1])
	}
}

func TestNamed(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	dest := &Named{Name: pdf.String("Chapter6")}

	obj, err := dest.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}

	str, ok := obj.(pdf.String)
	if !ok {
		t.Fatalf("expected String, got %T", obj)
	}

	if string(str) != "Chapter6" {
		t.Errorf("got %q, want %q", str, "Chapter6")
	}
}

func TestNamedEmptyName(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	dest := &Named{Name: pdf.String("")}

	_, err := dest.Encode(rm)
	if err == nil {
		t.Error("expected error for empty name, got nil")
	}
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			testRoundTrip(t, tc.Dest)
		})
	}
}

func TestDecodeRoundTrip(t *testing.T) {
	for _, tc := range decodeTestCases {
		t.Run(tc.Name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

			// decode
			x := pdf.NewExtractor(w)
			dest, err := Decode(x, tc.Obj)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			// round-trip
			testRoundTrip(t, dest)
		})
	}
}

func TestDecodeNamedFromName(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	// Old-style PDF 1.1 named destination using pdf.Name
	obj := pdf.Name("Chapter6")

	dest, err := Decode(x, obj)
	if err != nil {
		t.Fatal(err)
	}

	named, ok := dest.(*Named)
	if !ok {
		t.Fatalf("expected *Named, got %T", dest)
	}

	if string(named.Name) != "Chapter6" {
		t.Errorf("got %q, want %q", named.Name, "Chapter6")
	}
}

func TestDecodeDictionaryWrapper(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	pageRef := w.Alloc()

	// Dictionary wrapper with D entry
	dict := pdf.Dict{
		"D": pdf.Array{pageRef, pdf.Name("Fit")},
	}

	x := pdf.NewExtractor(w)
	dest, err := Decode(x, dict)
	if err != nil {
		t.Fatal(err)
	}

	fit, ok := dest.(*Fit)
	if !ok {
		t.Fatalf("expected *Fit, got %T", dest)
	}

	if fit.Page != Target(pageRef) {
		t.Errorf("page mismatch")
	}
}

func FuzzRoundTrip(f *testing.F) {
	// seed corpus with test cases
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}

	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(pdf.V1_7, opt)
		rm := pdf.NewResourceManager(w)

		obj, err := tc.Dest.Encode(rm)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = obj
		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	for _, tc := range decodeTestCases {
		w, buf := memfile.NewPDFWriter(pdf.V1_7, opt)

		w.GetMeta().Trailer["Quir:E"] = tc.Obj
		err := w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing test object")
		}

		x := pdf.NewExtractor(r)
		dest, err := Decode(x, obj)
		if err != nil {
			t.Skip("malformed destination")
		}

		// round-trip test
		testRoundTrip(t, dest)
	})
}
