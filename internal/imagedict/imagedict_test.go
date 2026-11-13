// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package imagedict

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/limits"
)

func TestSampleBits(t *testing.T) {
	flate, err := pdf.MakeFilter("FlateDecode", nil)
	if err != nil {
		t.Fatal(err)
	}
	dct, err := pdf.MakeFilter("DCTDecode", nil)
	if err != nil {
		t.Fatal(err)
	}
	ccf, err := pdf.MakeFilter("CCITTFaxDecode", nil)
	if err != nil {
		t.Fatal(err)
	}
	rl, err := pdf.MakeFilter("RunLengthDecode", nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		filters []pdf.Filter
		bpc     pdf.Integer
		want    int
		wantOK  bool
	}{
		{nil, 1, 1, true},
		{nil, 2, 2, true},
		{nil, 4, 4, true},
		{nil, 8, 8, true},
		{nil, 16, 16, true},
		{nil, 0, 0, false},
		{nil, 3, 0, false},
		{nil, -1, 0, false},
		{nil, 32, 0, false},
		{[]pdf.Filter{flate}, 4, 4, true},
		{[]pdf.Filter{flate}, 0, 0, false},
		{[]pdf.Filter{rl}, 0, 0, false},
		{[]pdf.Filter{dct}, 0, 8, true},
		{[]pdf.Filter{dct}, 3, 8, true},
		{[]pdf.Filter{ccf}, 0, 1, true},
		{[]pdf.Filter{ccf}, 8, 1, true},
		{[]pdf.Filter{dct, flate}, 0, 0, false},
		{[]pdf.Filter{flate, dct}, 0, 8, true},
	} {
		got, ok := SampleBits(tc.filters, tc.bpc)
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("%v, %d: got %d, %t, want %d, %t",
				tc.filters, tc.bpc, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestReadBitsPerComponent(t *testing.T) {
	c := pdf.NewCursor(nil)
	for _, tc := range []struct {
		val     pdf.Object
		want    int
		wantErr bool
	}{
		{pdf.Integer(8), 8, false},
		{pdf.Real(4), 4, false},
		{nil, 0, true},
		{pdf.Name("8"), 0, true},
		{pdf.Integer(5), 0, true},
	} {
		dict := pdf.Dict{}
		if tc.val != nil {
			dict["BitsPerComponent"] = tc.val
		}
		got, err := ReadBitsPerComponent(c, dict, nil)
		if (err != nil) != tc.wantErr {
			t.Errorf("%v: got error %v, want error %t", tc.val, err, tc.wantErr)
		} else if got != tc.want {
			t.Errorf("%v: got %d, want %d", tc.val, got, tc.want)
		}
	}
}

func TestReadDimensions(t *testing.T) {
	c := pdf.NewCursor(nil)
	for _, tc := range []struct {
		dict    pdf.Dict
		wantErr bool
	}{
		{pdf.Dict{"Width": pdf.Integer(3), "Height": pdf.Integer(2)}, false},
		{pdf.Dict{"Height": pdf.Integer(2)}, true},
		{pdf.Dict{"Width": pdf.Integer(3)}, true},
		{pdf.Dict{"Width": pdf.Integer(0), "Height": pdf.Integer(2)}, true},
		{pdf.Dict{"Width": pdf.Integer(3), "Height": pdf.Integer(-1)}, true},
		{pdf.Dict{"Width": pdf.Integer(limits.MaxImageWidth + 1), "Height": pdf.Integer(1)}, true},
		{pdf.Dict{"Width": pdf.Integer(1), "Height": pdf.Integer(limits.MaxImageHeight + 1)}, true},
		{pdf.Dict{"Width": pdf.Integer(limits.MaxImageWidth), "Height": pdf.Integer(limits.MaxImageHeight)}, true},
		{pdf.Dict{"Width": pdf.Integer(1) << 40, "Height": pdf.Integer(1)}, true},
	} {
		w, h, err := ReadDimensions(c, tc.dict)
		if (err != nil) != tc.wantErr {
			t.Errorf("%v: got error %v, want error %t", tc.dict, err, tc.wantErr)
		} else if !tc.wantErr && (w != 3 || h != 2) {
			t.Errorf("%v: got %d×%d, want 3×2", tc.dict, w, h)
		}
	}
}

func TestCheckDataSize(t *testing.T) {
	if err := CheckDataSize(100, 100, 3, 8); err != nil {
		t.Errorf("small image rejected: %v", err)
	}
	// 2^14 × 2^14 × 4 channels × 16 bits = 2 GiB of samples
	if err := CheckDataSize(1<<14, 1<<14, 4, 16); err == nil {
		t.Error("oversized image accepted")
	}
	// 16384 × 8193 × 4 channels as float32 is just over the decoded cap
	if err := CheckDataSize(16384, 8193, 4, 1); err == nil {
		t.Error("oversized decoded buffer accepted")
	}
}

func TestExpandInline(t *testing.T) {
	in := pdf.Dict{
		"W":                pdf.Integer(2),
		"Width":            pdf.Integer(7),
		"Height":           pdf.Integer(1),
		"BPC":              pdf.Integer(8),
		"CS":               pdf.Name("RGB"),
		"F":                pdf.Name("Fl"),
		"DP":               pdf.Dict{"Predictor": pdf.Integer(1)},
		"IM":               pdf.Boolean(false),
		"I":                pdf.Boolean(true),
		"D":                pdf.Array{pdf.Integer(0), pdf.Integer(1)},
		"L":                pdf.Integer(6),
		"Intent":           pdf.Name("Perceptual"),
		"Foo":              pdf.Integer(1),
		"BitsPerComponent": pdf.Integer(1),
	}
	want := pdf.Dict{
		"Width":            pdf.Integer(2),
		"Height":           pdf.Integer(1),
		"BitsPerComponent": pdf.Integer(8),
		"ColorSpace":       pdf.Name("RGB"),
		"Filter":           pdf.Name("Fl"),
		"DecodeParms":      pdf.Dict{"Predictor": pdf.Integer(1)},
		"ImageMask":        pdf.Boolean(false),
		"Interpolate":      pdf.Boolean(true),
		"Decode":           pdf.Array{pdf.Integer(0), pdf.Integer(1)},
		"Length":           pdf.Integer(6),
		"Intent":           pdf.Name("Perceptual"),
	}
	if diff := cmp.Diff(want, ExpandInline(in)); diff != "" {
		t.Errorf("ExpandInline (-want +got):\n%s", diff)
	}
}

func TestInlineKey(t *testing.T) {
	dict := pdf.Dict{
		"W":      pdf.Integer(2),
		"Width":  pdf.Integer(7),
		"Height": pdf.Integer(1),
		"Intent": pdf.Name("Perceptual"),
	}
	for _, tc := range []struct {
		full    pdf.Name
		wantKey pdf.Name
		wantVal pdf.Object
	}{
		{"Width", "W", pdf.Integer(2)},
		{"Height", "Height", pdf.Integer(1)},
		{"Intent", "Intent", pdf.Name("Perceptual")},
		{"Length", "", nil},
	} {
		key, ok := InlineKey(dict, tc.full)
		if key != tc.wantKey || ok != (tc.wantKey != "") {
			t.Errorf("InlineKey(%s) = %q, %t, want %q", tc.full, key, ok, tc.wantKey)
		}
		if got := InlineValue(dict, tc.full); got != tc.wantVal {
			t.Errorf("InlineValue(%s) = %v, want %v", tc.full, got, tc.wantVal)
		}
	}
}
