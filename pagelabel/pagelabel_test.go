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

package pagelabel

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

func TestFormatDecimal(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{1, "1"},
		{10, "10"},
		{999, "999"},
	}
	for _, tc := range tests {
		got := formatDecimal(tc.n)
		if got != tc.want {
			t.Errorf("formatDecimal(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestFormatRoman(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{1, "i"},
		{2, "ii"},
		{3, "iii"},
		{4, "iv"},
		{5, "v"},
		{9, "ix"},
		{10, "x"},
		{14, "xiv"},
		{40, "xl"},
		{49, "xlix"},
		{90, "xc"},
		{99, "xcix"},
		{400, "cd"},
		{500, "d"},
		{900, "cm"},
		{1000, "m"},
		{1994, "mcmxciv"},
		{3999, "mmmcmxcix"},
	}
	for _, tc := range tests {
		got := formatRoman(tc.n)
		if got != tc.want {
			t.Errorf("formatRoman(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestFormatAlpha(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{1, "a"},
		{2, "b"},
		{26, "z"},
		{27, "aa"},
		{28, "bb"},
		{52, "zz"},
		{53, "aaa"},
		{78, "zzz"},
	}
	for _, tc := range tests {
		got := formatAlpha(tc.n)
		if got != tc.want {
			t.Errorf("formatAlpha(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestRangeFormat(t *testing.T) {
	tests := []struct {
		r      Range
		offset int
		want   string
	}{
		{Range{Decimal, "", 1}, 0, "1"},
		{Range{Decimal, "", 1}, 4, "5"},
		{Range{LowerRoman, "", 1}, 0, "i"},
		{Range{LowerRoman, "", 1}, 3, "iv"},
		{Range{UpperRoman, "", 1}, 0, "I"},
		{Range{Decimal, "A-", 8}, 0, "A-8"},
		{Range{Decimal, "A-", 8}, 2, "A-10"},
		{Range{None, "Contents", 1}, 0, "Contents"},
		{Range{None, "Contents", 1}, 5, "Contents"},
		{Range{LowerAlpha, "", 1}, 0, "a"},
		{Range{LowerAlpha, "", 1}, 25, "z"},
		{Range{LowerAlpha, "", 1}, 26, "aa"},
		{Range{UpperAlpha, "", 1}, 0, "A"},
		{Range{UpperAlpha, "", 1}, 26, "AA"},
	}
	for _, tc := range tests {
		got := tc.r.Format(tc.offset)
		if got != tc.want {
			t.Errorf("Range%+v.Format(%d) = %q, want %q", tc.r, tc.offset, got, tc.want)
		}
	}
}

func TestLabelsFormat(t *testing.T) {
	// spec example: i, ii, iii, iv, 1, 2, 3, A-8, A-9, ...
	labels := newTestLabels(t)

	tests := []struct {
		pageIndex int
		want      string
	}{
		{0, "i"},
		{1, "ii"},
		{2, "iii"},
		{3, "iv"},
		{4, "1"},
		{5, "2"},
		{6, "3"},
		{7, "A-8"},
		{8, "A-9"},
		{100, "A-101"}, // well beyond the last range
	}
	for _, tc := range tests {
		got := labels.Format(tc.pageIndex)
		if got != tc.want {
			t.Errorf("Format(%d) = %q, want %q", tc.pageIndex, got, tc.want)
		}
	}
}

func TestExtractNil(t *testing.T) {
	_, err := Extract(mock.Getter, nil)
	if err == nil {
		t.Fatal("expected error for nil input")
	}
}

func TestExtract(t *testing.T) {
	// build a PageLabels number tree as raw PDF objects
	obj := pdf.Dict{
		"Nums": pdf.Array{
			pdf.Integer(0), pdf.Dict{"S": pdf.Name("r")},
			pdf.Integer(4), pdf.Dict{"S": pdf.Name("D")},
			pdf.Integer(7), pdf.Dict{
				"S":  pdf.Name("D"),
				"P":  pdf.TextString("A-"),
				"St": pdf.Integer(8),
			},
		},
	}

	labels, err := Extract(mock.Getter, obj)
	if err != nil {
		t.Fatal(err)
	}

	if labels.NumRanges() != 3 {
		t.Fatalf("got %d ranges, want 3", labels.NumRanges())
	}

	fp, rng := labels.GetRange(0)
	if fp != 0 || rng.Style != LowerRoman || rng.Prefix != "" || rng.Start != 1 {
		t.Errorf("range 0: firstPage=%d, range=%+v", fp, rng)
	}

	fp, rng = labels.GetRange(2)
	if fp != 7 || rng.Style != Decimal || rng.Prefix != "A-" || rng.Start != 8 {
		t.Errorf("range 2: firstPage=%d, range=%+v", fp, rng)
	}

	if got := labels.Format(0); got != "i" {
		t.Errorf("Format(0) = %q, want %q", got, "i")
	}
	if got := labels.Format(7); got != "A-8" {
		t.Errorf("Format(7) = %q, want %q", got, "A-8")
	}
}

func TestRoundTrip(t *testing.T) {
	labels := newTestLabels(t)

	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)

	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(labels)
	if err != nil {
		t.Fatal(err)
	}
	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// the writer can be used as a getter after closing
	labels2, err := Extract(w, ref)
	if err != nil {
		t.Fatal(err)
	}

	if labels.NumRanges() != labels2.NumRanges() {
		t.Fatalf("round trip: %d ranges → %d ranges", labels.NumRanges(), labels2.NumRanges())
	}
	for i := range labels.NumRanges() {
		fp1, r1 := labels.GetRange(i)
		fp2, r2 := labels2.GetRange(i)
		if diff := cmp.Diff(fp1, fp2); diff != "" {
			t.Errorf("range %d firstPage (-want +got):\n%s", i, diff)
		}
		if diff := cmp.Diff(r1, r2); diff != "" {
			t.Errorf("range %d (-want +got):\n%s", i, diff)
		}
	}
}

func TestRangeAt(t *testing.T) {
	labels := newTestLabels(t)

	tests := []struct {
		pageIndex  int
		wantRange  int
		wantOffset int
	}{
		{0, 0, 0},
		{3, 0, 3},
		{4, 1, 0},
		{6, 1, 2},
		{7, 2, 0},
		{10, 2, 3},
	}
	for _, tc := range tests {
		ri, offset := labels.RangeAt(tc.pageIndex)
		if ri != tc.wantRange || offset != tc.wantOffset {
			t.Errorf("RangeAt(%d) = (%d, %d), want (%d, %d)",
				tc.pageIndex, ri, offset, tc.wantRange, tc.wantOffset)
		}
	}
}

// newTestLabels creates the spec example:
// pages 0–3: lowercase Roman (i, ii, iii, iv)
// pages 4–6: decimal (1, 2, 3)
// pages 7+: decimal with prefix "A-", starting at 8 (A-8, A-9, ...)
func newTestLabels(t *testing.T) *Labels {
	t.Helper()
	entries := func(yield func(int, Range) bool) {
		if !yield(0, Range{LowerRoman, "", 1}) {
			return
		}
		if !yield(4, Range{Decimal, "", 1}) {
			return
		}
		yield(7, Range{Decimal, "A-", 8})
	}
	labels, err := New(entries)
	if err != nil {
		t.Fatal(err)
	}
	return labels
}
