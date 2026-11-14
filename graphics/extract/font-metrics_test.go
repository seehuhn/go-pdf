// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"

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
	got, err := decodeCompositeWidths(pdf.NewCursor(mock.Getter), w)
	if err != nil {
		t.Fatal(err)
	}
	want := map[cid.CID]float64{
		1: 500, 2: 500, 3: 500,
		10: 700, 11: 800,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("widths mismatch (-want +got):\n%s", diff)
	}
}

// the maximal valid range covers all 65536 CIDs and must be accepted
func TestCompositeWidthsFullRange(t *testing.T) {
	w := pdf.Array{pdf.Integer(0), pdf.Integer(65535), pdf.Integer(1000)}
	got, err := decodeCompositeWidths(pdf.NewCursor(mock.Getter), w)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 65536 {
		t.Errorf("got %d entries, want 65536", len(got))
	}
}

// two disjoint ranges which together cover all CIDs
func TestCompositeWidthsAllCIDs(t *testing.T) {
	w := pdf.Array{
		pdf.Integer(0), pdf.Integer(32767), pdf.Integer(500),
		pdf.Integer(32768), pdf.Integer(65535), pdf.Integer(600),
	}
	got, err := decodeCompositeWidths(pdf.NewCursor(mock.Getter), w)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 65536 {
		t.Errorf("got %d entries, want 65536", len(got))
	}
	if got[0] != 500 || got[65535] != 600 {
		t.Errorf("got widths %v/%v, want 500/600", got[0], got[65535])
	}
}

// for CIDs covered by several groups, the first group gives the width
func TestCompositeWidthsOverlapFirstWins(t *testing.T) {
	w := pdf.Array{
		pdf.Integer(0), pdf.Integer(10), pdf.Integer(500),
		pdf.Integer(5), pdf.Integer(15), pdf.Integer(600),
	}
	got, err := decodeCompositeWidths(pdf.NewCursor(mock.Getter), w)
	if err != nil {
		t.Fatal(err)
	}
	want := map[cid.CID]float64{
		0: 500, 1: 500, 2: 500, 3: 500, 4: 500,
		5: 500, 6: 500, 7: 500, 8: 500, 9: 500, 10: 500,
		11: 600, 12: 600, 13: 600, 14: 600, 15: 600,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("widths mismatch (-want +got):\n%s", diff)
	}
}

// Malformed groups and CIDs above 65535 end the W array: the groups before
// are kept.
func TestCompositeWidthsTruncate(t *testing.T) {
	keep := pdf.Array{pdf.Integer(1), pdf.Integer(2), pdf.Integer(500)}
	kept := map[cid.CID]float64{1: 500, 2: 500}
	full := pdf.Array{pdf.Integer(0), pdf.Integer(65535), pdf.Integer(1000)}
	cases := map[string]struct {
		w    pdf.Object
		want map[cid.CID]float64
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
			got, err := decodeCompositeWidths(pdf.NewCursor(mock.Getter), tc.w)
			if err != nil {
				t.Fatal(err)
			}
			if name == "disjoint ranges" {
				if len(got) != 65536 || got[0] != 1000 || got[65535] != 1000 {
					t.Errorf("got %d entries, want the first range only", len(got))
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
	got, err := decodeVMetrics(pdf.NewCursor(mock.Getter), a)
	if err != nil {
		t.Fatal(err)
	}
	want := map[cid.CID]dict.VMetrics{
		1: {DeltaY: -1000, OffsX: 250, OffsY: 880},
		2: {DeltaY: -1000, OffsX: 260, OffsY: 890},
		5: {DeltaY: -900, OffsX: 300, OffsY: 800},
		6: {DeltaY: -900, OffsX: 300, OffsY: 800},
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
	got, err := decodeVMetrics(pdf.NewCursor(mock.Getter), a)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 65536 {
		t.Errorf("got %d entries, want 65536", len(got))
	}
}

// Malformed groups and CIDs above 65535 end the W2 array: the groups before
// are kept.
func TestVMetricsTruncate(t *testing.T) {
	v := func(dy, ox, oy int) []pdf.Object {
		return []pdf.Object{pdf.Integer(dy), pdf.Integer(ox), pdf.Integer(oy)}
	}
	keep := append(pdf.Array{pdf.Integer(1), pdf.Integer(2)}, v(-900, 250, 800)...)
	kept := map[cid.CID]dict.VMetrics{
		1: {DeltaY: -900, OffsX: 250, OffsY: 800},
		2: {DeltaY: -900, OffsX: 250, OffsY: 800},
	}
	with := func(tail ...pdf.Object) pdf.Array {
		return append(slices.Clone(keep), tail...)
	}
	full := append(pdf.Array{pdf.Integer(0), pdf.Integer(65535)}, v(-1000, 0, 880)...)
	cases := map[string]struct {
		a    pdf.Object
		want map[cid.CID]dict.VMetrics
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
			got, err := decodeVMetrics(pdf.NewCursor(mock.Getter), tc.a)
			if err != nil {
				t.Fatal(err)
			}
			if name == "disjoint ranges" {
				if len(got) != 65536 || got[0].DeltaY != -1000 || got[65535].DeltaY != -1000 {
					t.Errorf("got %d entries, want the first range only", len(got))
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
	got, err := decodeCompositeWidths(pdf.NewCursor(mock.Getter), w)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 65536 {
		t.Fatalf("got %d entries, want 65536", len(got))
	}
	want := map[cid.CID]float64{0: 500, 5: 1, 6: 2, 11: 100, 20: 100, 65535: 100}
	for c, wi := range want {
		if got[c] != wi {
			t.Errorf("CID %d: got %g, want %g", c, got[c], wi)
		}
	}

	got, err = decodeCompositeWidths(pdf.NewCursor(mock.Getter), w[:8])
	if err != nil {
		t.Fatal(err)
	}
	want = map[cid.CID]float64{
		0: 500, 1: 500, 2: 500, 3: 500, 4: 500,
		5: 1, 6: 2, 7: 3,
		8: 500, 9: 500, 10: 500,
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
	got, err := decodeVMetrics(pdf.NewCursor(mock.Getter), a)
	if err != nil {
		t.Fatal(err)
	}
	want := map[cid.CID]float64{
		0: -500, 1: -500, 2: -500, 3: -500, 4: -500,
		5: -1, 6: -2, 7: -3,
		8: -500, 9: -500, 10: -500,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d", len(got), len(want))
	}
	for c, dy := range want {
		if g := got[c]; g.DeltaY != dy || g.OffsX != 250 || g.OffsY != 880 {
			t.Errorf("CID %d: got %v, want DeltaY %g", c, g, dy)
		}
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
	got, err := decodeCompositeWidths(pdf.NewCursor(g), a)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(widths) || got[2999] != 2999 {
		t.Errorf("got %d widths, want %d", len(got), len(widths))
	}
	if g.count != 1 {
		t.Errorf("W: %d objects read, want 1", g.count)
	}

	g.count = 0
	vm, err := decodeVMetrics(pdf.NewCursor(g), a)
	if err != nil {
		t.Fatal(err)
	}
	if len(vm) != 1000 || vm[999] != (dict.VMetrics{DeltaY: 2997, OffsX: 2998, OffsY: 2999}) {
		t.Errorf("got %d vertical metrics, want 1000", len(vm))
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
	got, err := decodeCompositeWidths(pdf.NewCursor(g), a)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(widths) || got[2999] != 2999 {
		t.Errorf("got %d widths, want %d", len(got), len(widths))
	}
	if g.count > numObjects {
		t.Errorf("W: %d objects read, want at most %d", g.count, numObjects)
	}

	g.count = 0
	vm, err := decodeVMetrics(pdf.NewCursor(g), a)
	if err != nil {
		t.Fatal(err)
	}
	if len(vm) != 1000 || vm[999] != (dict.VMetrics{DeltaY: 2997, OffsX: 2998, OffsY: 2999}) {
		t.Errorf("got %d vertical metrics, want 1000", len(vm))
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
	got, err := decodeCompositeWidths(pdf.NewCursor(w), a)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(map[cid.CID]float64{1: 500}, got); diff != "" {
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
		ranges := make([]cidRange, rng.IntN(20))
		for k := range ranges {
			a, b := rng.IntN(100), rng.IntN(100)
			ranges[k] = cidRange{first: min(a, b), last: max(a, b)}
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

	wantW := map[cid.CID]float64{2: 500, 3: 500}
	for name, w := range map[string]pdf.Array{
		"range": {pdf.Real(1.6), pdf.Real(3.2), pdf.Integer(500)},
		"indiv": {pdf.Real(1.6), pdf.Array{pdf.Integer(500), pdf.Integer(500)}},
	} {
		got, err := decodeCompositeWidths(c, w)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(wantW, got); diff != "" {
			t.Errorf("%s: widths mismatch (-want +got):\n%s", name, diff)
		}
	}

	v := dict.VMetrics{DeltaY: -900, OffsX: 250, OffsY: 800}
	triple := pdf.Array{pdf.Integer(-900), pdf.Integer(250), pdf.Integer(800)}
	wantV := map[cid.CID]dict.VMetrics{2: v, 3: v}
	for name, a := range map[string]pdf.Array{
		"range": append(pdf.Array{pdf.Real(1.6), pdf.Real(3.2)}, triple...),
		"indiv": {pdf.Real(1.6), append(slices.Clone(triple), triple...)},
	} {
		got, err := decodeVMetrics(c, a)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(wantV, got); diff != "" {
			t.Errorf("%s: vertical metrics mismatch (-want +got):\n%s", name, diff)
		}
	}
}
