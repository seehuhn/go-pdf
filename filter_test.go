// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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
	"fmt"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// All filter types currently implemented by this library.
var (
	_ Filter      = FilterASCII85{}
	_ Filter      = FilterASCIIHex{}
	_ Filter      = FilterRunLength{}
	_ Filter      = FilterFlate{}
	_ Filter      = FilterLZW{}
	_ Filter      = FilterCompress{}
	_ Filter      = FilterCCITTFax{}
	_ Filter      = FilterDCT{}
	_ Filter      = (*FilterJBIG2)(nil)
	_ Filter      = FilterJPX{}
	_ Filter      = FilterCryptIdentity{}
	_ Filter      = FilterCryptStandard{}
	_ Filter      = FilterCryptNamed{}
	_ CryptFilter = FilterCryptIdentity{}
	_ CryptFilter = FilterCryptStandard{}
	_ CryptFilter = FilterCryptNamed{}
)

func TestFilterChaining(t *testing.T) {
	F1 := FilterASCII85{}
	F2 := FilterASCIIHex{}
	F3 := FilterLZW{Predictor: FlatePredictorPNGOptimum}
	F4 := FilterCompress{}

	testData := "Hello, World!\n"

	testCases := [][]Filter{
		{F1, F2, F3},
		{F3, F2, F1},
		{F1, F3, F2},

		{F1, F2, F4},
		{F4, F2, F1},
		{F1, F4, F2},
	}
	for i, filters := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			buf := &bytes.Buffer{}
			w, err := NewWriter(buf, V2_0, nil)
			if err != nil {
				t.Fatal(err)
			}
			w.GetMeta().Catalog.Pages = w.Alloc() // pretend we have pages

			ref := w.Alloc()

			out, err := w.OpenStream(ref, nil, filters...)
			if err != nil {
				t.Fatal(err)
			}
			_, err = io.WriteString(out, testData)
			if err != nil {
				t.Fatal(err)
			}
			err = out.Close()
			if err != nil {
				t.Fatal(err)
			}

			err = w.Close()
			if err != nil {
				t.Fatal(err)
			}

			opt := &ReaderOptions{
				ErrorHandling: ErrorHandlingReport,
			}
			r, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), opt)
			if err != nil {
				t.Fatal(err)
			}
			stmObj, err := GetStream(r, ref)
			if err != nil {
				t.Fatal(err)
			}
			in, err := DecodeStream(r, nil, stmObj, 0)
			if err != nil {
				t.Fatal(err)
			}

			res, err := io.ReadAll(in)
			if err != nil {
				t.Fatal(err)
			}
			if string(res) != testData {
				t.Errorf("wrong result: %q vs %q", res, testData)
			}
		})
	}
}

// TestFlateLZWEncodeDecode exercises the Flate and LZW encoders against
// their decoders for a representative cross-product of parameter values.
func TestFlateLZWEncodeDecode(t *testing.T) {
	predictors := []FlatePredictor{
		FlatePredictorNone,
		FlatePredictorTIFF,
		FlatePredictorPNGUp,
	}
	inputs := []string{"", "12345", "1234567890"}

	encodeDecode := func(t *testing.T, f Filter, in string) {
		t.Helper()
		buf := &bytes.Buffer{}
		w, err := f.Encode(V2_0, withDummyClose{buf})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(in)); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		r, err := f.Decode(V2_0, buf)
		if err != nil {
			t.Fatal(err)
		}
		out, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != in {
			t.Errorf("round-trip mismatch: got %q, want %q", out, in)
		}
	}

	for _, isLZW := range []bool{false, true} {
		for _, predictor := range predictors {
			for _, in := range inputs {
				name := fmt.Sprintf("LZW=%v/Pred=%d/Len=%d", isLZW, predictor, len(in))
				t.Run(name, func(t *testing.T) {
					var f Filter
					if isLZW {
						f = FilterLZW{Predictor: predictor, OffByOne: true}
					} else {
						f = FilterFlate{Predictor: predictor}
					}
					encodeDecode(t, f, in)
				})
			}
		}
	}
}

// filterRoundTrip writes a stream using the given filter, reads back the
// corresponding /Filter and /DecodeParms entries via GetFilters, and
// returns the decoded Filter for comparison with the original.
func filterRoundTrip(t *testing.T, version Version, f Filter) Filter {
	t.Helper()

	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, version, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Catalog.Pages = w.Alloc() // pretend we have pages

	ref := w.Alloc()
	stm, err := w.OpenStream(ref, nil, f)
	if err != nil {
		if IsWrongVersion(err) {
			t.Skip("filter not supported in this PDF version")
		}
		t.Fatal(err)
	}
	if _, err := stm.Write([]byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	opt := &ReaderOptions{ErrorHandling: ErrorHandlingReport}
	r, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), opt)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := GetStream(r, ref)
	if err != nil {
		t.Fatal(err)
	}
	filters, err := GetFilters(r, nil, stream.Dict)
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(filters))
	}
	return filters[0]
}

// TestFilterFlateRoundTrip verifies write→read round-trips for canonical
// FilterFlate values (no zero-as-shorthand abbreviations).
func TestFilterFlateRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		f    FilterFlate
	}{
		{"no-predictor", FilterFlate{Predictor: FlatePredictorNone}},
		{"png-up-rgb", FilterFlate{
			Predictor:        FlatePredictorPNGUp,
			Colors:           3,
			BitsPerComponent: 8,
			Columns:          640,
		}},
		{"png-up-rgb-bpc16", FilterFlate{
			Predictor:        FlatePredictorPNGUp,
			Colors:           3,
			BitsPerComponent: 16,
			Columns:          640,
		}},
		{"tiff", FilterFlate{
			Predictor:        FlatePredictorTIFF,
			Colors:           1,
			BitsPerComponent: 8,
			Columns:          100,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, v := range []Version{V1_7, V2_0} {
				got := filterRoundTrip(t, v, tc.f)
				if diff := cmp.Diff(tc.f, got); diff != "" {
					t.Errorf("round trip failed (-want +got):\n%s", diff)
				}
			}
		})
	}
}

// TestFilterLZWRoundTrip verifies write→read round-trips for canonical
// FilterLZW values.
func TestFilterLZWRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		f    FilterLZW
	}{
		{"default-OffByOne", FilterLZW{
			Predictor: FlatePredictorNone,
			OffByOne:  true,
		}},
		{"corrected", FilterLZW{Predictor: FlatePredictorNone}},
		{"png-up-OffByOne", FilterLZW{
			Predictor:        FlatePredictorPNGUp,
			Colors:           3,
			BitsPerComponent: 8,
			Columns:          640,
			OffByOne:         true,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := filterRoundTrip(t, V1_7, tc.f)
			if diff := cmp.Diff(tc.f, got); diff != "" {
				t.Errorf("round trip failed (-want +got):\n%s", diff)
			}
		})
	}
}

// TestFilterCCITTFaxRoundTrip verifies write→read round-trips for canonical
// FilterCCITTFax values.
func TestFilterCCITTFaxRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		f    FilterCCITTFax
	}{
		{"minimal", FilterCCITTFax{Columns: 1728}},
		{"group4", FilterCCITTFax{K: -1, Columns: 1024, Rows: 768}},
		{"all-flags", FilterCCITTFax{
			K:                      -1,
			EndOfLine:              true,
			EncodedByteAlign:       true,
			Columns:                512,
			Rows:                   512,
			IgnoreEndOfBlock:       true,
			BlackIs1:               true,
			DamagedRowsBeforeError: 5,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := filterRoundTrip(t, V1_7, tc.f)
			if diff := cmp.Diff(tc.f, got); diff != "" {
				t.Errorf("round trip failed (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFilterDCTRoundTrip(t *testing.T) {
	// DCT cannot encode through OpenStream, so we test Info/MakeFilter
	// directly rather than going through a real stream.
	cases := []FilterDCT{
		{ColorTransform: DCTColorTransformAuto},
		{ColorTransform: DCTColorTransformNone},
		{ColorTransform: DCTColorTransformYCbCr},
	}
	for _, want := range cases {
		name, params, err := want.Info(V2_0)
		if err != nil {
			t.Fatal(err)
		}
		got, err := MakeFilter(name, params)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("DCT round trip (%v) failed (-want +got):\n%s",
				want.ColorTransform, diff)
		}
	}
}

func TestFilterJBIG2RoundTrip(t *testing.T) {
	// JBIG2 cannot encode through OpenStream and depends on an external
	// Globals stream; verify Info/MakeFilter symmetry only.
	cases := []*FilterJBIG2{
		{},                                    // no globals
		{Globals: []byte("not-roundtripped")}, // Globals is resolved separately
		{GlobalsRef: NewReference(42, 0)},     // a reference re-emits via Info
		{Globals: []byte("x"), GlobalsRef: NewReference(7, 0)},
	}
	for _, want := range cases {
		name, params, err := want.Info(V2_0)
		if err != nil {
			t.Fatal(err)
		}
		f, err := MakeFilter(name, params)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := f.(*FilterJBIG2)
		if !ok {
			t.Fatalf("MakeFilter returned %T, want *FilterJBIG2", f)
		}
		// Globals only round-trips through resolveJBIG2Globals (needs a Getter);
		// the wire form carries only GlobalsRef.
		opts := []cmp.Option{cmpopts.IgnoreFields(FilterJBIG2{}, "Globals")}
		if diff := cmp.Diff(want, got, opts...); diff != "" {
			t.Errorf("JBIG2 round trip failed (-want +got):\n%s", diff)
		}
	}
}

func TestFilterJPXRoundTrip(t *testing.T) {
	want := FilterJPX{}
	name, params, err := want.Info(V2_0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := MakeFilter(name, params)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("JPX round trip failed (-want +got):\n%s", diff)
	}
}

func TestFilterCryptIdentityRoundTrip(t *testing.T) {
	want := FilterCryptIdentity{}
	name, params, err := want.Info(V2_0)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Crypt" {
		t.Errorf("name = %q, want Crypt", name)
	}
	if params != nil {
		t.Errorf("params = %v, want nil (Identity is the default)", params)
	}
	got, err := MakeFilter(name, params)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Identity round trip failed (-want +got):\n%s", diff)
	}
}

func TestFilterCryptIdentityFromExplicitName(t *testing.T) {
	// /Name /Identity in DecodeParms must also dispatch to FilterCryptIdentity.
	got, err := MakeFilter("Crypt", Dict{"Name": Name("Identity")})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.(FilterCryptIdentity); !ok {
		t.Errorf("MakeFilter(Crypt, {Name: Identity}) = %T, want FilterCryptIdentity", got)
	}
}

func TestFilterCryptStandardRoundTrip(t *testing.T) {
	want := FilterCryptStandard{}
	name, params, err := want.Info(V2_0)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Crypt" {
		t.Errorf("name = %q, want Crypt", name)
	}
	if params["Name"] != Name("StdCF") {
		t.Errorf("params[Name] = %v, want StdCF", params["Name"])
	}
	got, err := MakeFilter(name, params)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Standard round trip failed (-want +got):\n%s", diff)
	}
}

func TestFilterCryptNamedRoundTrip(t *testing.T) {
	want := FilterCryptNamed{Name: "MyCF"}
	name, params, err := want.Info(V2_0)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Crypt" {
		t.Errorf("name = %q, want Crypt", name)
	}
	if params["Name"] != Name("MyCF") {
		t.Errorf("params[Name] = %v, want MyCF", params["Name"])
	}
	got, err := MakeFilter(name, params)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Named round trip failed (-want +got):\n%s", diff)
	}
}

// TestMakeFilterCryptWrongNameType verifies that MakeFilter returns a
// MalformedFileError when the /Crypt filter's /Name entry is present
// with a non-Name PDF type — there is no safe default fix-up.
func TestMakeFilterCryptWrongNameType(t *testing.T) {
	cases := []Object{
		String("StdCF"),
		Integer(0),
		Boolean(true),
		Array{Name("StdCF")},
	}
	for _, val := range cases {
		_, err := MakeFilter("Crypt", Dict{"Name": val})
		if err == nil {
			t.Errorf("expected error for /Name = %T, got nil", val)
			continue
		}
		if !IsMalformed(err) {
			t.Errorf("expected MalformedFileError for /Name = %T, got %T: %v", val, err, err)
		}
	}
}

// TestFilterCryptNamedInvalidName verifies that Info rejects names which
// have a more canonical representation as one of the other variants.
func TestFilterCryptNamedInvalidName(t *testing.T) {
	for _, name := range []Name{"", "Identity", "StdCF"} {
		f := FilterCryptNamed{Name: name}
		if _, _, err := f.Info(V2_0); err == nil {
			t.Errorf("FilterCryptNamed{Name:%q}.Info: expected error, got nil", name)
		}
	}
}

// TestFilterCryptVersionGate ensures Crypt filters require PDF 1.5+.
func TestFilterCryptVersionGate(t *testing.T) {
	for _, f := range []CryptFilter{
		FilterCryptIdentity{},
		FilterCryptStandard{},
		FilterCryptNamed{Name: "MyCF"},
	} {
		if _, _, err := f.Info(V1_4); !IsWrongVersion(err) {
			t.Errorf("%T.Info(V1_4): want VersionError, got %v", f, err)
		}
		if _, _, err := f.Info(V1_5); err != nil {
			t.Errorf("%T.Info(V1_5): unexpected error %v", f, err)
		}
	}
}

// TestCryptFilterPositionValidationOnRead verifies that GetFilters rejects
// streams whose /Filter array places /Crypt at a non-first position.
func TestCryptFilterPositionValidationOnRead(t *testing.T) {
	dict := Dict{
		"Filter": Array{Name("FlateDecode"), Name("Crypt")},
	}
	_, err := GetFilters(nil, nil, dict)
	if err == nil {
		t.Fatal("expected error for /Crypt at position 1, got nil")
	}
	if !IsMalformed(err) {
		t.Errorf("expected MalformedFileError, got %T: %v", err, err)
	}
}

// TestCryptFilterPositionValidationOnWrite verifies that OpenStream rejects
// a CryptFilter at any position other than 0 in the user-supplied filters.
func TestCryptFilterPositionValidationOnWrite(t *testing.T) {
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, V2_0, nil)
	if err != nil {
		t.Fatal(err)
	}
	ref := w.Alloc()
	_, err = w.OpenStream(ref, nil, FilterFlate{}, FilterCryptIdentity{})
	if err == nil {
		t.Fatal("expected error for CryptFilter at position 1, got nil")
	}
}

// TestFilterCryptStandardUnimplemented verifies that Encode/Decode return
// clear errors for the not-yet-implemented variants.
func TestFilterCryptStandardUnimplemented(t *testing.T) {
	f := FilterCryptStandard{}
	if _, err := f.Encode(V2_0, nil); err == nil {
		t.Error("FilterCryptStandard.Encode: expected error, got nil")
	}
	if _, err := f.Decode(V2_0, bytes.NewReader(nil)); err == nil {
		t.Error("FilterCryptStandard.Decode: expected error, got nil")
	}
}

// TestFilterCryptNamedUnimplemented verifies that Encode/Decode return
// clear errors for FilterCryptNamed.
func TestFilterCryptNamedUnimplemented(t *testing.T) {
	f := FilterCryptNamed{Name: "MyCF"}
	if _, err := f.Encode(V2_0, nil); err == nil {
		t.Error("FilterCryptNamed.Encode: expected error, got nil")
	}
	if _, err := f.Decode(V2_0, bytes.NewReader(nil)); err == nil {
		t.Error("FilterCryptNamed.Decode: expected error, got nil")
	}
}

// TestFilterFlateVersionGate ensures Info/Encode reject FlateDecode for
// PDF versions older than 1.2.
func TestFilterFlateVersionGate(t *testing.T) {
	f := FilterFlate{}
	if _, _, err := f.Info(V1_1); !IsWrongVersion(err) {
		t.Errorf("expected VersionError for PDF 1.1, got %v", err)
	}
	if _, _, err := f.Info(V1_2); err != nil {
		t.Errorf("unexpected error for PDF 1.2: %v", err)
	}
}

// TestFilterFlateBPC16Gate ensures BitsPerComponent=16 requires PDF 1.5+.
func TestFilterFlateBPC16Gate(t *testing.T) {
	f := FilterFlate{Predictor: FlatePredictorPNGUp, BitsPerComponent: 16, Columns: 1}
	if _, _, err := f.Info(V1_4); !IsWrongVersion(err) {
		t.Errorf("expected VersionError for BPC=16 in PDF 1.4, got %v", err)
	}
	if _, _, err := f.Info(V1_5); err != nil {
		t.Errorf("unexpected error for BPC=16 in PDF 1.5: %v", err)
	}
}

// TestFilterFlateRequiresPredictor verifies that Colors, BitsPerComponent,
// and Columns each error out when set without a predictor.  The same
// validation applies to FilterLZW and FilterCompress because they share
// validateFlateLZW.
func TestFilterFlateRequiresPredictor(t *testing.T) {
	cases := []struct {
		name string
		f    FilterFlate
	}{
		{"Colors", FilterFlate{Colors: 3}},
		{"BitsPerComponent", FilterFlate{BitsPerComponent: 8}},
		{"Columns", FilterFlate{Columns: 100}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := tc.f.Info(V2_0); err == nil {
				t.Errorf("expected error when %s is set without a predictor", tc.name)
			}
		})
	}
}

// TestFilterLZWRequiresPredictor mirrors TestFilterFlateRequiresPredictor
// for FilterLZW, which shares validateFlateLZW.
func TestFilterLZWRequiresPredictor(t *testing.T) {
	cases := []struct {
		name string
		f    FilterLZW
	}{
		{"Colors", FilterLZW{Colors: 3}},
		{"BitsPerComponent", FilterLZW{BitsPerComponent: 8}},
		{"Columns", FilterLZW{Columns: 100}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := tc.f.Info(V2_0); err == nil {
				t.Errorf("expected error when %s is set without a predictor", tc.name)
			}
		})
	}
}
