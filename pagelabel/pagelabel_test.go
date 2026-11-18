// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pagelabel

import (
	"math"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
	"seehuhn.de/go/pdf/internal/limits"
)

func TestStyleParseInvertsFormat(t *testing.T) {
	values := []int{-3, 0, 4000, 5000,
		maxAlphaValue, maxAlphaValue + 1, 260_000_000}
	for n := 1; n <= maxRomanValue; n++ {
		values = append(values, n)
	}
	for _, style := range []Style{Decimal, UpperRoman, LowerRoman, UpperAlpha, LowerAlpha} {
		for _, n := range values {
			str := style.Format(n)
			got, ok := style.Parse(str)
			if !ok || got != n {
				t.Errorf("style %d: Parse(%q) = (%d, %t), want (%d, true)",
					style, str, got, ok, n)
			}
		}
	}
}

func TestStyleParseNone(t *testing.T) {
	if _, ok := None.Parse(""); !ok {
		t.Error("None rejects the empty string")
	}
	if _, ok := None.Parse("1"); ok {
		t.Error("None accepts a non-empty string")
	}
}

func TestStyleParseIgnoresCase(t *testing.T) {
	tests := []struct {
		style Style
		str   string
		want  int
	}{
		{LowerRoman, "MCMxlIX", 1949},
		{UpperRoman, "iv", 4},
		{LowerAlpha, "BB", 28},
		{UpperAlpha, "bb", 28},
	}
	for _, tc := range tests {
		got, ok := tc.style.Parse(tc.str)
		if !ok || got != tc.want {
			t.Errorf("Parse(%q) = (%d, %t), want (%d, true)", tc.str, got, ok, tc.want)
		}
	}
}

func TestStyleParseRejects(t *testing.T) {
	tests := []struct {
		style Style
		str   string
	}{
		{Decimal, ""},
		{Decimal, "x"},
		{LowerRoman, ""},
		{LowerRoman, "ivx1"},
		{LowerRoman, "5"}, // Format never gives this
		{LowerAlpha, ""},
		{LowerAlpha, "ab"},
		{LowerAlpha, "a1"},
		{LowerAlpha, "5"}, // Format never gives this
		{LowerAlpha, strings.Repeat("a", limits.MaxPageLabelLength+1)},
		{Decimal, "99999999999999999999"}, // overflows int
	}
	for _, tc := range tests {
		if n, ok := tc.style.Parse(tc.str); n != 0 || ok {
			t.Errorf("style %d: Parse(%.20q) = (%d, %t), want (0, false)",
				tc.style, tc.str, n, ok)
		}
	}
}

func TestStyleParseRomanNonCanonical(t *testing.T) {
	accept := []struct {
		str  string
		want int
	}{
		{"iiii", 4},
		{"ic", 99},
		{"im", 999},
		{"viiii", 9},
		{"mmmm", 4000},
	}
	for _, tc := range accept {
		if n, ok := LowerRoman.Parse(tc.str); !ok || n != tc.want {
			t.Errorf("Parse(%q) = (%d, %t), want (%d, true)", tc.str, n, ok, tc.want)
		}
	}
	for _, str := range []string{"iic", "iix", "ivx", "xxc", "vix", "ivi", "cmc", "xcx"} {
		if n, ok := LowerRoman.Parse(str); ok {
			t.Errorf("Parse(%q) = %d, want failure", str, n)
		}
	}
}

func TestStyleParseLenientDecimal(t *testing.T) {
	tests := []struct {
		style Style
		str   string
		want  int
	}{
		{Decimal, "+007", 7},
		{Decimal, "-0", 0},
		{LowerRoman, "0004000", 4000},
		{UpperAlpha, "+0", 0},
	}
	for _, tc := range tests {
		if n, ok := tc.style.Parse(tc.str); !ok || n != tc.want {
			t.Errorf("Parse(%q) = (%d, %t), want (%d, true)", tc.str, n, ok, tc.want)
		}
	}
}

func TestRangeFormat(t *testing.T) {
	tests := []struct {
		r      Range
		offset int
		want   string
	}{
		{Range{Decimal, pdf.Text(""), 1}, 0, "1"},
		{Range{Decimal, pdf.Text(""), 1}, 4, "5"},
		{Range{LowerRoman, pdf.Text(""), 1}, 0, "i"},
		{Range{LowerRoman, pdf.Text(""), 1}, 3, "iv"},
		{Range{UpperRoman, pdf.Text(""), 1}, 0, "I"},
		{Range{Decimal, pdf.Text("A-"), 8}, 0, "A-8"},
		{Range{Decimal, pdf.Text("A-"), 8}, 2, "A-10"},
		{Range{None, pdf.Text("Contents"), 1}, 0, "Contents"},
		{Range{None, pdf.Text("Contents"), 1}, 5, "Contents"},
		{Range{LowerAlpha, pdf.Text(""), 1}, 0, "a"},
		{Range{LowerAlpha, pdf.Text(""), 1}, 25, "z"},
		{Range{LowerAlpha, pdf.Text(""), 1}, 26, "aa"},
		{Range{UpperAlpha, pdf.Text(""), 1}, 0, "A"},
		{Range{UpperAlpha, pdf.Text(""), 1}, 26, "AA"},
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

// TestExtractCyclicPageLabels guards against the stack-overflow attack
// path documented in the security report: a self-referencing /Kids
// dict planted at Catalog/PageLabels.
func TestExtractCyclicPageLabels(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)

	rootRef := w.Alloc()
	root := pdf.Dict{
		"Kids":   pdf.Array{rootRef},
		"Limits": pdf.Array{pdf.Integer(0), pdf.Integer(1 << 30)},
	}
	if err := w.Put(rootRef, root); err != nil {
		t.Fatal(err)
	}

	labels, err := Extract(w, rootRef)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// no real entries in the cyclic tree; Extract installs the default
	// page-0 range, so we expect exactly one range.
	if len(labels.Ranges) != 1 {
		t.Errorf("got %d ranges, want 1", len(labels.Ranges))
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
				"P":  pdf.Text("A-"),
				"St": pdf.Integer(8),
			},
		},
	}

	labels, err := Extract(mock.Getter, obj)
	if err != nil {
		t.Fatal(err)
	}

	want := []Entry{
		{FirstPage: 0, Style: LowerRoman, Start: 1},
		{FirstPage: 4, Style: Decimal, Start: 1},
		{FirstPage: 7, Style: Decimal, Prefix: pdf.Text("A-"), Start: 8},
	}
	if diff := cmp.Diff(want, labels.Ranges); diff != "" {
		t.Errorf("wrong ranges (-want +got):\n%s", diff)
	}

	if got := labels.Format(0); got != "i" {
		t.Errorf("Format(0) = %q, want %q", got, "i")
	}
	if got := labels.Format(7); got != "A-8" {
		t.Errorf("Format(7) = %q, want %q", got, "A-8")
	}
}

// TestExtractLargeStart checks that start values up to 2³¹ − 1 are kept.
func TestExtractLargeStart(t *testing.T) {
	for _, st := range []int{1_500_000_000, maxStart} {
		obj := pdf.Dict{
			"Nums": pdf.Array{
				pdf.Integer(0),
				pdf.Dict{"S": pdf.Name("a"), "St": pdf.Integer(st)},
			},
		}

		labels, err := Extract(mock.Getter, obj)
		if err != nil {
			t.Fatal(err)
		}
		if got := labels.Ranges[0].Start; got != st {
			t.Errorf("Start = %d, want %d", got, st)
		}
	}
}

// TestExtractStartTooLarge checks that a file with a start value beyond
// 2³¹ − 1 can be read, and that the result survives a write-read cycle.
func TestExtractStartTooLarge(t *testing.T) {
	obj := pdf.Dict{
		"Nums": pdf.Array{
			pdf.Integer(0),
			pdf.Dict{"S": pdf.Name("a"), "St": pdf.Integer(26_000_000_000_000)},
		},
	}

	labels, err := Extract(mock.Getter, obj)
	if err != nil {
		t.Fatal(err)
	}

	labels2 := writeAndRead(t, labels)
	if diff := cmp.Diff(labels.Ranges, labels2.Ranges); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// TestNewStartTooLarge checks that a start value beyond 2³¹ − 1 is rejected.
func TestNewStartTooLarge(t *testing.T) {
	start := int64(maxStart) + 1
	if start > math.MaxInt {
		t.Skip("int cannot hold the start value")
	}
	if _, err := New(Entry{0, Range{Decimal, pdf.Text(""), int(start)}}); err == nil {
		t.Error("start value beyond 2³¹ − 1 accepted")
	}
}

// TestPrefixAtLimit checks that a prefix of the maximal length can be
// written and read back.
func TestPrefixAtLimit(t *testing.T) {
	prefix := pdf.Text(strings.Repeat("ä", limits.MaxPageLabelPrefixLength))
	labels, err := New(Entry{0, Range{Decimal, prefix, 1}})
	if err != nil {
		t.Fatal(err)
	}
	labels2 := writeAndRead(t, labels)
	if diff := cmp.Diff(labels.Ranges, labels2.Ranges); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// TestNewPrefixTooLong checks that a prefix beyond the length limit is
// rejected.
func TestNewPrefixTooLong(t *testing.T) {
	prefix := pdf.Text(strings.Repeat("x", limits.MaxPageLabelPrefixLength+1))
	if _, err := New(Entry{0, Range{Decimal, prefix, 1}}); err == nil {
		t.Error("prefix beyond the length limit accepted")
	}
}

// TestExtractPrefixTooLong checks that a file with a prefix beyond the
// length limit can be read, and that the result survives a write-read cycle.
func TestExtractPrefixTooLong(t *testing.T) {
	obj := pdf.Dict{
		"Nums": pdf.Array{
			pdf.Integer(0), pdf.Dict{
				"S": pdf.Name("D"),
				"P": pdf.String(strings.Repeat("x", limits.MaxPageLabelPrefixLength+1)),
			},
		},
	}

	labels, err := Extract(mock.Getter, obj)
	if err != nil {
		t.Fatal(err)
	}

	labels2 := writeAndRead(t, labels)
	if diff := cmp.Diff(labels.Ranges, labels2.Ranges); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// TestExtractNegativeKey checks that a key which is not a valid page index
// is dropped, so that the ranges stay sorted and can be fed back into New.
func TestExtractNegativeKey(t *testing.T) {
	obj := pdf.Dict{
		"Nums": pdf.Array{
			pdf.Integer(-5), pdf.Dict{"S": pdf.Name("A")},
			pdf.Integer(3), pdf.Dict{"S": pdf.Name("r")},
		},
	}

	labels, err := Extract(mock.Getter, obj)
	if err != nil {
		t.Fatal(err)
	}

	want := []Entry{
		{FirstPage: 0, Style: Decimal, Start: 1},
		{FirstPage: 3, Style: LowerRoman, Start: 1},
	}
	if diff := cmp.Diff(want, labels.Ranges); diff != "" {
		t.Errorf("wrong ranges (-want +got):\n%s", diff)
	}

	if got := labels.Format(0); got != "1" {
		t.Errorf("Format(0) = %q, want %q", got, "1")
	}

	if _, err := New(labels.Ranges...); err != nil {
		t.Errorf("the extracted ranges cannot be fed back into New: %v", err)
	}
}

// TestRoundTripLargeStart checks that the largest valid start value survives
// a write-read cycle.
func TestRoundTripLargeStart(t *testing.T) {
	labels, err := New(Entry{0, Range{LowerAlpha, pdf.Text(""), maxStart}})
	if err != nil {
		t.Fatal(err)
	}

	labels2 := writeAndRead(t, labels)
	if diff := cmp.Diff(labels.Ranges, labels2.Ranges); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestRoundTrip(t *testing.T) {
	labels := newTestLabels(t)

	labels2 := writeAndRead(t, labels)
	if diff := cmp.Diff(labels.Ranges, labels2.Ranges); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// writeAndRead embeds labels in a new PDF file and reads them back.
func writeAndRead(t *testing.T, labels *Labels) *Labels {
	t.Helper()

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(labels)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// the writer can be used as a getter after closing
	labels2, err := Extract(w, ref)
	if err != nil {
		t.Fatal(err)
	}
	return labels2
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
	labels, err := New(
		Entry{0, Range{LowerRoman, pdf.Text(""), 1}},
		Entry{4, Range{Decimal, pdf.Text(""), 1}},
		Entry{7, Range{Decimal, pdf.Text("A-"), 8}},
	)
	if err != nil {
		t.Fatal(err)
	}
	return labels
}

// The Ranges of a Labels can be fed straight back into New.
func TestRangesRoundTrip(t *testing.T) {
	labels := newTestLabels(t)

	again, err := New(labels.Ranges...)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(labels, again); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// TestExtractValidates checks the promise made by validate's doc comment:
// whatever Extract makes of a malformed number tree can be written back out.
func TestExtractValidates(t *testing.T) {
	tests := []struct {
		name string
		obj  pdf.Object
	}{
		{"empty", pdf.Dict{"Nums": pdf.Array{}}},
		{"no page 0", pdf.Dict{"Nums": pdf.Array{
			pdf.Integer(7), pdf.Dict{"S": pdf.Name("D")},
		}}},
		{"negative key", pdf.Dict{"Nums": pdf.Array{
			pdf.Integer(-5), pdf.Dict{"S": pdf.Name("D")},
		}}},
		{"duplicate keys", pdf.Dict{"Nums": pdf.Array{
			pdf.Integer(0), pdf.Dict{"S": pdf.Name("D")},
			pdf.Integer(0), pdf.Dict{"S": pdf.Name("r")},
		}}},
		{"unsorted keys", pdf.Dict{"Nums": pdf.Array{
			pdf.Integer(4), pdf.Dict{"S": pdf.Name("D")},
			pdf.Integer(0), pdf.Dict{"S": pdf.Name("r")},
		}}},
		{"unknown style", pdf.Dict{"Nums": pdf.Array{
			pdf.Integer(0), pdf.Dict{"S": pdf.Name("nonsense")},
		}}},
		{"zero start", pdf.Dict{"Nums": pdf.Array{
			pdf.Integer(0), pdf.Dict{"S": pdf.Name("D"), "St": pdf.Integer(0)},
		}}},
		{"negative start", pdf.Dict{"Nums": pdf.Array{
			pdf.Integer(0), pdf.Dict{"S": pdf.Name("D"), "St": pdf.Integer(-3)},
		}}},
		{"wrong value type", pdf.Dict{"Nums": pdf.Array{
			pdf.Integer(0), pdf.Integer(42),
		}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			labels, err := Extract(mock.Getter, tc.obj)
			if err != nil {
				t.Fatal(err)
			}
			if err := labels.validate(); err != nil {
				t.Errorf("Extract returned an unwritable value: %v", err)
			}

			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := rm.Embed(labels); err != nil {
				t.Errorf("re-embedding a value read from a file failed: %v", err)
			}
		})
	}
}
