// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"maps"
	"math"
	"math/rand/v2"
	"runtime"
	"slices"
	"testing"
	"unsafe"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
	"seehuhn.de/go/postscript/cid"
)

func TestCompositeWidthsValid(t *testing.T) {
	// range form for CIDs 1..3, individual form for CIDs 10..11
	w := pdf.Array{
		pdf.Integer(1), pdf.Integer(3), pdf.Integer(500),
		pdf.Integer(10), pdf.Array{pdf.Integer(700), pdf.Integer(800)},
	}
	got, err := decodeCompositeWidths(pdf.NewCursor(mock.Getter), w, false)
	if err != nil {
		t.Fatal(err)
	}
	want := dict.CIDMetrics[float64]{
		wRange(1, 3, 500),
		wList(10, 700, 800),
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("widths mismatch (-want +got):\n%s", diff)
	}
}

// the maximal valid range covers all 65536 CIDs and must be accepted
func TestCompositeWidthsFullRange(t *testing.T) {
	w := pdf.Array{pdf.Integer(0), pdf.Integer(65535), pdf.Integer(1000)}
	got, err := decodeCompositeWidths(pdf.NewCursor(mock.Getter), w, false)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(dict.CIDMetrics[float64]{wRange(0, 65535, 1000)}, got); diff != "" {
		t.Errorf("widths mismatch (-want +got):\n%s", diff)
	}
}

// two disjoint ranges which together cover all CIDs
func TestCompositeWidthsAllCIDs(t *testing.T) {
	w := pdf.Array{
		pdf.Integer(0), pdf.Integer(32767), pdf.Integer(500),
		pdf.Integer(32768), pdf.Integer(65535), pdf.Integer(600),
	}
	got, err := decodeCompositeWidths(pdf.NewCursor(mock.Getter), w, false)
	if err != nil {
		t.Fatal(err)
	}
	want := dict.CIDMetrics[float64]{wRange(0, 32767, 500), wRange(32768, 65535, 600)}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("widths mismatch (-want +got):\n%s", diff)
	}
}

// for CIDs covered by several groups, the first group gives the width
func TestCompositeWidthsOverlapFirstWins(t *testing.T) {
	w := pdf.Array{
		pdf.Integer(0), pdf.Integer(10), pdf.Integer(500),
		pdf.Integer(5), pdf.Integer(15), pdf.Integer(600),
	}
	got, err := decodeCompositeWidths(pdf.NewCursor(mock.Getter), w, false)
	if err != nil {
		t.Fatal(err)
	}
	want := dict.CIDMetrics[float64]{wRange(0, 10, 500), wRange(11, 15, 600)}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("widths mismatch (-want +got):\n%s", diff)
	}
}

// Malformed groups and CIDs above 65535 end the W array: the groups before
// are kept.
func TestCompositeWidthsTruncate(t *testing.T) {
	keep := pdf.Array{pdf.Integer(1), pdf.Integer(2), pdf.Integer(500)}
	kept := dict.CIDMetrics[float64]{wRange(1, 2, 500)}
	full := pdf.Array{pdf.Integer(0), pdf.Integer(65535), pdf.Integer(1000)}
	cases := map[string]struct {
		w    pdf.Object
		want dict.CIDMetrics[float64]
	}{
		"not an array":         {pdf.Integer(5), nil},
		"range high CID":       {append(slices.Clone(keep), pdf.Integer(70000), pdf.Integer(70100), pdf.Integer(1000)), kept},
		"range end past limit": {append(slices.Clone(keep), pdf.Integer(65535), pdf.Integer(65536), pdf.Integer(1000)), kept},
		"range reversed":       {append(slices.Clone(keep), pdf.Integer(9), pdf.Integer(8), pdf.Integer(1000)), kept},
		"range short":          {append(slices.Clone(keep), pdf.Integer(8), pdf.Integer(9)), kept},
		"range bad width":      {append(slices.Clone(keep), pdf.Integer(8), pdf.Integer(9), pdf.Name("x")), kept},
		"indiv high CID":       {append(slices.Clone(keep), pdf.Integer(70000), pdf.Array{pdf.Integer(1000)}), kept},
		"indiv crosses limit":  {append(slices.Clone(keep), pdf.Integer(65535), pdf.Array{pdf.Integer(1), pdf.Integer(2)}), kept},
		"indiv bad width": {
			append(slices.Clone(keep), pdf.Integer(8), pdf.Array{pdf.Integer(1), pdf.Name("x")}),
			kept,
		},
		"bad CID":        {append(slices.Clone(keep), pdf.Name("x"), pdf.Array{pdf.Integer(1)}), kept},
		"bad second":     {append(slices.Clone(keep), pdf.Integer(8), pdf.Name("x")), kept},
		"null second":    {append(slices.Clone(keep), pdf.Integer(8), nil, pdf.Integer(1)), kept},
		"missing object": {append(slices.Clone(keep), pdf.Integer(8), pdf.NewReference(99, 0), pdf.Integer(1)), kept},
		"trailing":       {append(slices.Clone(keep), pdf.Integer(8)), kept},
		"non-finite":     {append(slices.Clone(keep), pdf.Integer(8), pdf.Array{pdf.Real(math.Inf(1))}), kept},
		"first invalid":  {pdf.Array{pdf.Integer(1)}, nil},
		"empty":          {pdf.Array{}, nil},
		// maximal range, followed by a range beyond the largest CID
		"disjoint ranges": {
			append(slices.Clone(full), pdf.Integer(65536), pdf.Integer(131071), pdf.Integer(1000)),
			nil, // checked below
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := decodeCompositeWidths(pdf.NewCursor(mock.Getter), tc.w, false)
			if err != nil {
				t.Fatal(err)
			}
			if name == "disjoint ranges" {
				if diff := cmp.Diff(dict.CIDMetrics[float64]{wRange(0, 65535, 1000)}, got); diff != "" {
					t.Errorf("want the first range only (-want +got):\n%s", diff)
				}
				return
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("widths mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestVMetricsValid(t *testing.T) {
	// individual form for CIDs 1..2, range form for CIDs 5..6
	a := pdf.Array{
		pdf.Integer(1), pdf.Array{
			pdf.Integer(-1000), pdf.Integer(250), pdf.Integer(880),
			pdf.Integer(-1000), pdf.Integer(260), pdf.Integer(890),
		},
		pdf.Integer(5), pdf.Integer(6), pdf.Integer(-900), pdf.Integer(300), pdf.Integer(800),
	}
	got, err := decodeVMetrics(pdf.NewCursor(mock.Getter), a, false)
	if err != nil {
		t.Fatal(err)
	}
	want := dict.CIDMetrics[dict.VMetrics]{
		vList(1, dict.VMetrics{DeltaY: -1000, OffsX: 250, OffsY: 880}, dict.VMetrics{DeltaY: -1000, OffsX: 260, OffsY: 890}),
		vRange(5, 6, dict.VMetrics{DeltaY: -900, OffsX: 300, OffsY: 800}),
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("vmetrics mismatch (-want +got):\n%s", diff)
	}
}

// the maximal valid range covers all 65536 CIDs and must be accepted
func TestVMetricsFullRange(t *testing.T) {
	a := pdf.Array{
		pdf.Integer(0), pdf.Integer(65535),
		pdf.Integer(-1000), pdf.Integer(0), pdf.Integer(880),
	}
	got, err := decodeVMetrics(pdf.NewCursor(mock.Getter), a, false)
	if err != nil {
		t.Fatal(err)
	}
	want := dict.CIDMetrics[dict.VMetrics]{vRange(0, 65535, dict.VMetrics{DeltaY: -1000, OffsY: 880})}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("vmetrics mismatch (-want +got):\n%s", diff)
	}
}

// Malformed groups and CIDs above 65535 end the W2 array: the groups before
// are kept.
func TestVMetricsTruncate(t *testing.T) {
	v := func(dy, ox, oy int) []pdf.Object {
		return []pdf.Object{pdf.Integer(dy), pdf.Integer(ox), pdf.Integer(oy)}
	}
	keep := append(pdf.Array{pdf.Integer(1), pdf.Integer(2)}, v(-900, 250, 800)...)
	kept := dict.CIDMetrics[dict.VMetrics]{vRange(1, 2, dict.VMetrics{DeltaY: -900, OffsX: 250, OffsY: 800})}
	with := func(tail ...pdf.Object) pdf.Array {
		return append(slices.Clone(keep), tail...)
	}
	full := append(pdf.Array{pdf.Integer(0), pdf.Integer(65535)}, v(-1000, 0, 880)...)
	cases := map[string]struct {
		a    pdf.Object
		want dict.CIDMetrics[dict.VMetrics]
	}{
		"not an array":         {pdf.Integer(1), nil},
		"empty":                {pdf.Array{}, nil},
		"first invalid":        {pdf.Array{pdf.Integer(1)}, nil},
		"empty list":           {pdf.Array{pdf.Integer(1), pdf.Array{}}, nil},
		"range high CID":       {with(append(pdf.Array{pdf.Integer(70000), pdf.Integer(70100)}, v(-1000, 0, 880)...)...), kept},
		"range end past limit": {with(append(pdf.Array{pdf.Integer(65535), pdf.Integer(65536)}, v(-1000, 0, 880)...)...), kept},
		"range reversed":       {with(append(pdf.Array{pdf.Integer(9), pdf.Integer(8)}, v(-1000, 0, 880)...)...), kept},
		"range short":          {with(pdf.Integer(8), pdf.Integer(9), pdf.Integer(-1000)), kept},
		"range bad number":     {with(pdf.Integer(8), pdf.Integer(9), pdf.Integer(-1000), pdf.Name("x"), pdf.Integer(880)), kept},
		"indiv not triples":    {with(pdf.Integer(8), pdf.Array{pdf.Integer(-1000), pdf.Integer(0)}), kept},
		"indiv bad number":     {with(pdf.Integer(8), pdf.Array{pdf.Integer(-1000), pdf.Integer(0), pdf.Name("x")}), kept},
		"indiv crosses limit":  {with(pdf.Integer(65535), append(pdf.Array(v(-1000, 0, 880)), v(-1000, 0, 880)...)), kept},
		"non-finite":           {with(pdf.Integer(8), pdf.Array{pdf.Real(math.NaN()), pdf.Integer(0), pdf.Integer(0)}), kept},
		"bad CID":              {with(pdf.Name("x"), pdf.Array{}), kept},
		"bad second":           {with(pdf.Integer(8), pdf.Name("x")), kept},
		"null second":          {with(pdf.Integer(8), nil, pdf.Integer(1), pdf.Integer(0), pdf.Integer(0), pdf.Integer(0)), kept},
		"trailing":             {with(pdf.Integer(8)), kept},
		// maximal range, followed by a range beyond the largest CID
		"disjoint ranges": {
			append(slices.Clone(full), append(pdf.Array{pdf.Integer(65536), pdf.Integer(131071)}, v(-1000, 0, 880)...)...),
			nil, // checked below
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := decodeVMetrics(pdf.NewCursor(mock.Getter), tc.a, false)
			if err != nil {
				t.Fatal(err)
			}
			if name == "disjoint ranges" {
				want := dict.CIDMetrics[dict.VMetrics]{vRange(0, 65535, dict.VMetrics{DeltaY: -1000, OffsY: 880})}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("want the first range only (-want +got):\n%s", diff)
				}
				return
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("vmetrics mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// Where groups overlap, earlier groups take precedence.
func TestCompositeWidthsOverlap(t *testing.T) {
	w := pdf.Array{
		pdf.Integer(5), pdf.Array{pdf.Integer(1), pdf.Integer(2), pdf.Integer(3)},
		pdf.Integer(6), pdf.Integer(6), pdf.Integer(9),
		pdf.Integer(0), pdf.Integer(10), pdf.Integer(500),
		pdf.Integer(0), pdf.Integer(65535), pdf.Integer(100), // repeated below
		pdf.Integer(0), pdf.Integer(65535), pdf.Integer(200),
		pdf.Integer(20), pdf.Integer(20), pdf.Integer(300),
	}
	got, err := decodeCompositeWidths(pdf.NewCursor(mock.Getter), w, false)
	if err != nil {
		t.Fatal(err)
	}
	want := dict.CIDMetrics[float64]{
		wRange(0, 4, 500),
		wList(5, 1, 2, 3),
		wRange(8, 10, 500),
		wRange(11, 65535, 100),
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("widths mismatch (-want +got):\n%s", diff)
	}

	got, err = decodeCompositeWidths(pdf.NewCursor(mock.Getter), w[:8], false)
	if err != nil {
		t.Fatal(err)
	}
	want = dict.CIDMetrics[float64]{
		wRange(0, 4, 500),
		wList(5, 1, 2, 3),
		wRange(8, 10, 500),
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("widths mismatch (-want +got):\n%s", diff)
	}
}

// Where groups overlap, earlier groups take precedence.
func TestVMetricsOverlap(t *testing.T) {
	v := func(dy int) []pdf.Object {
		return []pdf.Object{pdf.Integer(dy), pdf.Integer(250), pdf.Integer(880)}
	}
	var a pdf.Array
	a = append(a, pdf.Integer(5), append(append(append(pdf.Array{}, v(-1)...), v(-2)...), v(-3)...))
	a = append(append(a, pdf.Integer(6), pdf.Integer(6)), v(-9)...)
	a = append(append(a, pdf.Integer(0), pdf.Integer(10)), v(-500)...)
	got, err := decodeVMetrics(pdf.NewCursor(mock.Getter), a, false)
	if err != nil {
		t.Fatal(err)
	}
	vm := func(dy float64) dict.VMetrics { return dict.VMetrics{DeltaY: dy, OffsX: 250, OffsY: 880} }
	want := dict.CIDMetrics[dict.VMetrics]{
		vRange(0, 4, vm(-500)),
		vList(5, vm(-1), vm(-2), vm(-3)),
		vRange(8, 10, vm(-500)),
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("vmetrics mismatch (-want +got):\n%s", diff)
	}
}

// countingGetter counts the objects read.
type countingGetter struct {
	pdf.Getter
	count int
}

func (g *countingGetter) Get(ref pdf.Reference, canObjStm bool) (pdf.Native, error) {
	g.count++
	return g.Getter.Get(ref, canObjStm)
}

// An array referenced by many groups of W or W2 is read only once.
func TestMetricsRepeatedReference(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	widths := make(pdf.Array, 3*1000)
	for i := range widths {
		widths[i] = pdf.Integer(i)
	}
	ref := w.Alloc()
	if err := w.Put(ref, widths); err != nil {
		t.Fatal(err)
	}

	var a pdf.Array
	for range 1000 {
		a = append(a, pdf.Integer(0), ref)
	}

	g := &countingGetter{Getter: w}
	got, err := decodeCompositeWidths(pdf.NewCursor(g), a, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].First != 0 || got[0].Last != 2999 || got[0].Values[2999] != 2999 {
		t.Errorf("unexpected widths %v", got)
	}
	if g.count != 1 {
		t.Errorf("W: %d objects read, want 1", g.count)
	}

	g.count = 0
	vm, err := decodeVMetrics(pdf.NewCursor(g), a, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(vm) != 1 || vm[0].Last != 999 || vm[0].Values[999] != (dict.VMetrics{DeltaY: 2997, OffsX: 2998, OffsY: 2999}) {
		t.Errorf("unexpected vertical metrics %v", vm)
	}
	if g.count != 1 {
		t.Errorf("W2: %d objects read, want 1", g.count)
	}
}

// Arrays reached through chains of aliases ("N 0 obj M 0 R endobj") are read
// and decoded once, no matter how many groups or aliases lead to them.
func TestMetricsAliasedReference(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	widths := make(pdf.Array, 3*1000)
	for i := range widths {
		widths[i] = pdf.Integer(i)
	}
	target := w.Alloc()
	if err := w.Put(target, widths); err != nil {
		t.Fatal(err)
	}
	numObjects := 1

	// a shared chain of aliases leading to the array
	chain := target
	for range 50 {
		ref := w.Alloc()
		if err := w.Put(ref, chain); err != nil {
			t.Fatal(err)
		}
		chain = ref
		numObjects++
	}

	// many distinct aliases, some pointing at the array, some at the chain,
	// each used twice
	var a pdf.Array
	for i := range 1000 {
		ref := w.Alloc()
		dest := target
		if i%2 == 1 {
			dest = chain
		}
		if err := w.Put(ref, dest); err != nil {
			t.Fatal(err)
		}
		numObjects++
		a = append(a, pdf.Integer(0), ref, pdf.Integer(0), ref)
	}

	g := &countingGetter{Getter: w}
	got, err := decodeCompositeWidths(pdf.NewCursor(g), a, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].First != 0 || got[0].Last != 2999 || got[0].Values[2999] != 2999 {
		t.Errorf("unexpected widths %v", got)
	}
	if g.count > numObjects {
		t.Errorf("W: %d objects read, want at most %d", g.count, numObjects)
	}

	g.count = 0
	vm, err := decodeVMetrics(pdf.NewCursor(g), a, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(vm) != 1 || vm[0].Last != 999 || vm[0].Values[999] != (dict.VMetrics{DeltaY: 2997, OffsX: 2998, OffsY: 2999}) {
		t.Errorf("unexpected vertical metrics %v", vm)
	}
	if g.count > numObjects {
		t.Errorf("W2: %d objects read, want at most %d", g.count, numObjects)
	}
}

// A cycle of aliases ends the W array.
func TestMetricsAliasCycle(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	r1, r2 := w.Alloc(), w.Alloc()
	if err := w.Put(r1, r2); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(r2, r1); err != nil {
		t.Fatal(err)
	}
	a := pdf.Array{
		pdf.Integer(1), pdf.Array{pdf.Integer(500)},
		pdf.Integer(2), r1,
		pdf.Integer(3), pdf.Array{pdf.Integer(600)},
	}
	got, err := decodeCompositeWidths(pdf.NewCursor(w), a, false)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(dict.CIDMetrics[float64]{wList(1, 500)}, got); diff != "" {
		t.Errorf("widths mismatch (-want +got):\n%s", diff)
	}
}

func TestVDefault(t *testing.T) {
	cases := map[string]struct {
		a    pdf.Object
		want dict.DefaultVMetrics
	}{
		"absent":        {nil, dict.DefaultVMetricsDefault},
		"valid":         {pdf.Array{pdf.Integer(700), pdf.Integer(-900)}, dict.DefaultVMetrics{OffsY: 700, DeltaY: -900}},
		"extra element": {pdf.Array{pdf.Integer(700), pdf.Integer(-900), pdf.Integer(1)}, dict.DefaultVMetrics{OffsY: 700, DeltaY: -900}},
		"not an array":  {pdf.Integer(5), dict.DefaultVMetricsDefault},
		"too short":     {pdf.Array{pdf.Integer(700)}, dict.DefaultVMetricsDefault},
		"bad first":     {pdf.Array{pdf.Name("x"), pdf.Integer(-900)}, dict.DefaultVMetricsDefault},
		"bad second":    {pdf.Array{pdf.Integer(880), pdf.Name("x")}, dict.DefaultVMetricsDefault},
		"non-finite":    {pdf.Array{pdf.Integer(880), pdf.Real(math.Inf(-1))}, dict.DefaultVMetricsDefault},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := decodeVDefault(pdf.NewCursor(mock.Getter), tc.a)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEarliest(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for range 200 {
		ranges := make([]intRange, rng.IntN(20))
		for k := range ranges {
			a, b := rng.IntN(100), rng.IntN(100)
			ranges[k] = intRange{first: min(a, b), last: max(a, b)}
			if rng.IntN(5) == 0 {
				ranges[k].last = ranges[k].first - 1 // empty
			}
		}
		want := make(map[int]int)
		for k, r := range ranges {
			for c := r.first; c <= r.last; c++ {
				if _, seen := want[c]; !seen {
					want[c] = k
				}
			}
		}
		got := make(map[int]int)
		earliest(ranges, func(k, c int) {
			if _, seen := got[c]; seen {
				t.Fatalf("CID %d assigned twice", c)
			}
			got[c] = k
		})
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("ranges %v: mismatch (-want +got):\n%s", ranges, diff)
		}
	}
}

// Real numbers in place of CIDs are rounded, for the start and the end of a
// group alike.
func TestMetricsRealCID(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)

	for name, tc := range map[string]struct {
		w    pdf.Array
		want dict.CIDMetrics[float64]
	}{
		"range": {pdf.Array{pdf.Real(1.6), pdf.Real(3.2), pdf.Integer(500)}, dict.CIDMetrics[float64]{wRange(2, 3, 500)}},
		"indiv": {pdf.Array{pdf.Real(1.6), pdf.Array{pdf.Integer(500), pdf.Integer(500)}}, dict.CIDMetrics[float64]{wList(2, 500, 500)}},
	} {
		got, err := decodeCompositeWidths(c, tc.w, false)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("%s: widths mismatch (-want +got):\n%s", name, diff)
		}
	}

	v := dict.VMetrics{DeltaY: -900, OffsX: 250, OffsY: 800}
	triple := pdf.Array{pdf.Integer(-900), pdf.Integer(250), pdf.Integer(800)}
	for name, tc := range map[string]struct {
		a    pdf.Array
		want dict.CIDMetrics[dict.VMetrics]
	}{
		"range": {append(pdf.Array{pdf.Real(1.6), pdf.Real(3.2)}, triple...), dict.CIDMetrics[dict.VMetrics]{vRange(2, 3, v)}},
		"indiv": {pdf.Array{pdf.Real(1.6), append(slices.Clone(triple), triple...)}, dict.CIDMetrics[dict.VMetrics]{vList(2, v, v)}},
	} {
		got, err := decodeVMetrics(c, tc.a, false)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("%s: vertical metrics mismatch (-want +got):\n%s", name, diff)
		}
	}
}

func wRange(first, last cid.CID, v float64) dict.CIDRun[float64] {
	return dict.CIDRun[float64]{First: first, Last: last, Value: v}
}

func wList(first cid.CID, vv ...float64) dict.CIDRun[float64] {
	return dict.CIDRun[float64]{First: first, Last: first + cid.CID(len(vv)) - 1, Values: vv}
}

func vRange(first, last cid.CID, v dict.VMetrics) dict.CIDRun[dict.VMetrics] {
	return dict.CIDRun[dict.VMetrics]{First: first, Last: last, Value: v}
}

func vList(first cid.CID, vv ...dict.VMetrics) dict.CIDRun[dict.VMetrics] {
	return dict.CIDRun[dict.VMetrics]{First: first, Last: first + cid.CID(len(vv)) - 1, Values: vv}
}

// The last CID is at least 256 in both cases, so that boxing the integers into
// interface values costs the same.
func TestCIDMetricsScaling(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	ints := func(xx ...int) pdf.Array {
		var a pdf.Array
		for _, x := range xx {
			a = append(a, pdf.Integer(x))
		}
		return a
	}
	allocs := func(f func()) float64 { return testing.AllocsPerRun(10, f) }

	// allocated bytes must not depend on the range size either
	var b [2]uint64
	for i, last := range []int{300, 65535} {
		w := ints(0, last, 1)
		var m0, m1 runtime.MemStats
		runtime.ReadMemStats(&m0)
		for range 100 {
			if _, err := decodeCompositeWidths(c, w, false); err != nil {
				t.Fatal(err)
			}
		}
		runtime.ReadMemStats(&m1)
		b[i] = (m1.TotalAlloc - m0.TotalAlloc) / 100
	}
	if b[1] > 2*b[0] {
		t.Errorf("W: allocated bytes depend on range size: %d vs %d", b[0], b[1])
	}

	var a [2]float64
	for i, last := range []int{300, 65535} {
		w := ints(0, last, 1)
		a[i] = allocs(func() {
			got, err := decodeCompositeWidths(c, w, false)
			if err != nil || len(got) != 1 {
				t.Fatalf("W: got %d runs, err %v", len(got), err)
			}
		})
	}
	if a[0] != a[1] {
		t.Errorf("W: allocations depend on range size: %g vs %g", a[0], a[1])
	}

	for i, last := range []int{300, 65535} {
		w2 := ints(0, last, -1000, 500, 880)
		a[i] = allocs(func() {
			got, err := decodeVMetrics(c, w2, false)
			if err != nil || len(got) != 1 {
				t.Fatalf("W2: got %d runs, err %v", len(got), err)
			}
		})
	}
	if a[0] != a[1] {
		t.Errorf("W2: allocations depend on range size: %g vs %g", a[0], a[1])
	}
}

func overlapWidths(n int) pdf.Array {
	var w pdf.Array
	for k := range n { // staggered
		w = append(w, pdf.Integer(10*k), pdf.Integer(10*k+15), pdf.Integer(k))
	}
	for k := n - 1; k >= 0; k-- { // nested, innermost first
		w = append(w, pdf.Integer(k), pdf.Integer(65535-k), pdf.Integer(k))
	}
	return w
}

func TestCIDMetricsOverlapLinear(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	const n = 1000
	w := overlapWidths(n)
	got, err := decodeCompositeWidths(c, w, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) <= n || len(got) > 2*(len(w)/3) {
		t.Errorf("got %d runs, want between %d and %d", len(got), n+1, 2*(len(w)/3))
	}
	for i, r := range got {
		if r.First > r.Last || i > 0 && got[i-1].Last >= r.First {
			t.Fatalf("invalid run %d: %v", i, r)
		}
	}

	want := make(map[cid.CID]float64)
	for g := 0; g+2 < len(w); g += 3 {
		first, last, v := int(w[g].(pdf.Integer)), int(w[g+1].(pdf.Integer)), float64(w[g+2].(pdf.Integer))
		for c := first; c <= last; c++ {
			if _, ok := want[cid.CID(c)]; !ok {
				want[cid.CID(c)] = v
			}
		}
	}
	if diff := cmp.Diff(want, maps.Collect(got.All())); diff != "" {
		t.Errorf("widths mismatch (-want +got):\n%s", diff)
	}

	// 2-D scaling: this checks memory (allocation count), not time
	var a [2]float64
	for i, n := range []int{1000, 4000} {
		w := overlapWidths(n)
		a[i] = testing.AllocsPerRun(3, func() {
			if _, err := decodeCompositeWidths(c, w, false); err != nil {
				t.Fatal(err)
			}
		})
	}
	if a[1] > 5*a[0] {
		t.Errorf("allocations grow faster than linear: %g for n=1000, %g for n=4000", a[0], a[1])
	}
}

func TestCIDMetricsRandomOverlap(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	rng := rand.New(rand.NewPCG(3, 4))
	for range 300 {
		var w, w2 pdf.Array
		want := make(map[cid.CID]float64)
		want2 := make(map[cid.CID]dict.VMetrics)
		claim := func(first, last int, v float64, vm dict.VMetrics) {
			for x := first; x <= last; x++ {
				if _, ok := want[cid.CID(x)]; !ok {
					want[cid.CID(x)] = v
				}
				if _, ok := want2[cid.CID(x)]; !ok {
					want2[cid.CID(x)] = vm
				}
			}
		}
		for g := range rng.IntN(13) {
			first := rng.IntN(200)
			v := float64(g + 1)
			vm := dict.VMetrics{DeltaY: float64(g + 1), OffsX: float64(2 * g), OffsY: float64(3 * g)}
			if rng.IntN(2) == 0 {
				last := first + rng.IntN(60)
				w = append(w, pdf.Integer(first), pdf.Integer(last), pdf.Number(v))
				w2 = append(w2, pdf.Integer(first), pdf.Integer(last),
					pdf.Number(vm.DeltaY), pdf.Number(vm.OffsX), pdf.Number(vm.OffsY))
				claim(first, last, v, vm)
			} else {
				m := 1 + rng.IntN(8)
				var l, l2 pdf.Array
				for i := range m {
					l = append(l, pdf.Number(v+float64(i)/8))
					l2 = append(l2, pdf.Number(vm.DeltaY+float64(i)), pdf.Number(vm.OffsX), pdf.Number(vm.OffsY))
					if _, ok := want[cid.CID(first+i)]; !ok {
						want[cid.CID(first+i)] = v + float64(i)/8
					}
					if _, ok := want2[cid.CID(first+i)]; !ok {
						want2[cid.CID(first+i)] = dict.VMetrics{DeltaY: vm.DeltaY + float64(i), OffsX: vm.OffsX, OffsY: vm.OffsY}
					}
				}
				w = append(w, pdf.Integer(first), l)
				w2 = append(w2, pdf.Integer(first), l2)
			}
		}

		got, err := decodeCompositeWidths(c, w, false)
		if err != nil {
			t.Fatal(err)
		}
		checkRuns(t, got)
		if diff := cmp.Diff(want, maps.Collect(got.All())); diff != "" {
			t.Fatalf("widths mismatch for %v (-want +got):\n%s", w, diff)
		}

		got2, err := decodeVMetrics(c, w2, false)
		if err != nil {
			t.Fatal(err)
		}
		checkRuns(t, got2)
		if diff := cmp.Diff(want2, maps.Collect(got2.All())); diff != "" {
			t.Fatalf("vmetrics mismatch for %v (-want +got):\n%s", w2, diff)
		}
	}
}

func checkRuns[T any](t *testing.T, m dict.CIDMetrics[T]) {
	t.Helper()
	for i, r := range m {
		if r.First > r.Last || i > 0 && m[i-1].Last >= r.First {
			t.Fatalf("invalid run %d: %v", i, r)
		}
		if r.Values != nil && len(r.Values) != int(r.Last-r.First)+1 {
			t.Fatalf("run %d: %d values for %d CIDs", i, len(r.Values), int(r.Last-r.First)+1)
		}
	}
}

func TestCIDMetricsListSplit(t *testing.T) {
	w := pdf.Array{
		pdf.Integer(10), pdf.Integer(12), pdf.Integer(7),
		pdf.Integer(8), pdf.Array{
			pdf.Integer(1), pdf.Integer(2), pdf.Integer(3), pdf.Integer(4),
			pdf.Integer(5), pdf.Integer(6), pdf.Integer(7), pdf.Integer(8),
		},
	}
	got, err := decodeCompositeWidths(pdf.NewCursor(mock.Getter), w, false)
	if err != nil {
		t.Fatal(err)
	}
	want := dict.CIDMetrics[float64]{wList(8, 1, 2), wRange(10, 12, 7), wList(13, 6, 7, 8)}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("widths mismatch (-want +got):\n%s", diff)
	}
}

func TestCIDMetricsListShared(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Array{pdf.Integer(1), pdf.Integer(2), pdf.Integer(3)}); err != nil {
		t.Fatal(err)
	}
	a := pdf.Array{pdf.Integer(0), ref, pdf.Integer(100), ref, pdf.Integer(200), ref}
	got, err := decodeCompositeWidths(pdf.NewCursor(w), a, false)
	if err != nil {
		t.Fatal(err)
	}
	want := dict.CIDMetrics[float64]{wList(0, 1, 2, 3), wList(100, 1, 2, 3), wList(200, 1, 2, 3)}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("widths mismatch (-want +got):\n%s", diff)
	}
	for _, r := range got {
		if unsafe.SliceData(r.Values) != unsafe.SliceData(got[0].Values) {
			t.Errorf("run %d does not share the list", r.First)
		}
	}
}
