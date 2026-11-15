// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package extract_test

import (
	"maps"
	"testing"
	"unsafe"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// cidMetrics holds the metrics fields shared by CIDFontType0 and
// CIDFontType2 dictionaries.
type cidMetrics struct {
	Width           map[cid.CID]float64
	DefaultWidth    float64
	VMetrics        map[cid.CID]dict.VMetrics
	DefaultVMetrics dict.DefaultVMetrics
}

// malformedMetricsCase is a CIDFont dictionary with malformed metrics
// entries, together with the metrics a reader must report.
type malformedMetricsCase struct {
	name    string
	entries pdf.Dict
	want    cidMetrics
	cycles  []pdf.Name // entries which are replaced by a reference cycle
}

// addCycles replaces the entries named by cycles by references to a pair of
// objects which refer to each other.
func addCycles(w *pdf.Writer, entries pdf.Dict, cycles []pdf.Name) (pdf.Dict, error) {
	if len(cycles) == 0 {
		return entries, nil
	}
	res := maps.Clone(entries)
	if res == nil {
		res = pdf.Dict{}
	}
	for _, key := range cycles {
		r1, r2 := w.Alloc(), w.Alloc()
		if err := w.Put(r1, r2); err != nil {
			return nil, err
		}
		if err := w.Put(r2, r1); err != nil {
			return nil, err
		}
		res[key] = r1
	}
	return res, nil
}

var vm1 = dict.VMetrics{DeltaY: -900, OffsX: 250, OffsY: 800}

var malformedMetricsCases = []malformedMetricsCase{
	{
		name:    "W2 one element",
		entries: pdf.Dict{"W2": pdf.Array{pdf.Integer(1)}},
		want: cidMetrics{
			DefaultWidth:    dict.DefaultWidthDefault,
			DefaultVMetrics: dict.DefaultVMetricsDefault,
		},
	},
	{
		name: "W2 bad second group",
		entries: pdf.Dict{"W2": pdf.Array{
			pdf.Integer(1), pdf.Array{pdf.Integer(-900), pdf.Integer(250), pdf.Integer(800)},
			pdf.Integer(5), pdf.Name("x"),
		}},
		want: cidMetrics{
			DefaultWidth:    dict.DefaultWidthDefault,
			VMetrics:        map[cid.CID]dict.VMetrics{1: vm1},
			DefaultVMetrics: dict.DefaultVMetricsDefault,
		},
	},
	{
		name:    "W2 not an array",
		entries: pdf.Dict{"W2": pdf.Integer(1)},
		want: cidMetrics{
			DefaultWidth:    dict.DefaultWidthDefault,
			DefaultVMetrics: dict.DefaultVMetricsDefault,
		},
	},
	{
		name:    "DW2 not an array",
		entries: pdf.Dict{"DW2": pdf.Integer(5)},
		want: cidMetrics{
			DefaultWidth:    dict.DefaultWidthDefault,
			DefaultVMetrics: dict.DefaultVMetricsDefault,
		},
	},
	{
		name:    "DW2 bad element",
		entries: pdf.Dict{"DW2": pdf.Array{pdf.Integer(880), pdf.Name("x")}},
		want: cidMetrics{
			DefaultWidth:    dict.DefaultWidthDefault,
			DefaultVMetrics: dict.DefaultVMetricsDefault,
		},
	},
	{
		name:    "DW2 too short",
		entries: pdf.Dict{"DW2": pdf.Array{pdf.Integer(700)}},
		want: cidMetrics{
			DefaultWidth:    dict.DefaultWidthDefault,
			DefaultVMetrics: dict.DefaultVMetricsDefault,
		},
	},
	{
		name:    "W one element",
		entries: pdf.Dict{"W": pdf.Array{pdf.Integer(1)}},
		want: cidMetrics{
			DefaultWidth:    dict.DefaultWidthDefault,
			DefaultVMetrics: dict.DefaultVMetricsDefault,
		},
	},
	{
		name: "W bad second group",
		entries: pdf.Dict{"W": pdf.Array{
			pdf.Integer(1), pdf.Array{pdf.Integer(500), pdf.Integer(600)},
			pdf.Integer(5), pdf.Integer(7), pdf.Name("x"),
		}},
		want: cidMetrics{
			Width:           map[cid.CID]float64{1: 500, 2: 600},
			DefaultWidth:    dict.DefaultWidthDefault,
			DefaultVMetrics: dict.DefaultVMetricsDefault,
		},
	},
	{
		name:    "W not an array",
		entries: pdf.Dict{"W": pdf.Name("x")},
		want: cidMetrics{
			DefaultWidth:    dict.DefaultWidthDefault,
			DefaultVMetrics: dict.DefaultVMetricsDefault,
		},
	},
	{
		name:   "W is a reference cycle",
		cycles: []pdf.Name{"W"},
		want: cidMetrics{
			DefaultWidth:    dict.DefaultWidthDefault,
			DefaultVMetrics: dict.DefaultVMetricsDefault,
		},
	},
	{
		name:   "W2 is a reference cycle",
		cycles: []pdf.Name{"W2"},
		want: cidMetrics{
			DefaultWidth:    dict.DefaultWidthDefault,
			DefaultVMetrics: dict.DefaultVMetricsDefault,
		},
	},
	{
		name:    "DW not a number",
		entries: pdf.Dict{"DW": pdf.Array{pdf.Integer(500)}},
		want: cidMetrics{
			DefaultWidth:    dict.DefaultWidthDefault,
			DefaultVMetrics: dict.DefaultVMetricsDefault,
		},
	},
	{
		name: "all malformed",
		entries: pdf.Dict{
			"W":   pdf.Array{pdf.Integer(1), pdf.Array{pdf.Integer(500)}, pdf.Integer(3)},
			"DW":  pdf.Name("x"),
			"W2":  pdf.Array{pdf.Integer(1), pdf.Integer(1), pdf.Integer(-900), pdf.Integer(250), pdf.Integer(800), pdf.Integer(7)},
			"DW2": pdf.Array{pdf.Name("x"), pdf.Integer(-500)},
		},
		want: cidMetrics{
			Width:           map[cid.CID]float64{1: 500},
			DefaultWidth:    dict.DefaultWidthDefault,
			VMetrics:        map[cid.CID]dict.VMetrics{1: vm1},
			DefaultVMetrics: dict.DefaultVMetricsDefault,
		},
	},
}

// malformedMetricsFont returns a Type0 font dictionary whose descendant font
// has the given entries.
func malformedMetricsFont(cidType pdf.Name, entries pdf.Dict) pdf.Dict {
	cidFont := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  cidType,
		"BaseFont": pdf.Name("Test"),
		"CIDSystemInfo": pdf.Dict{
			"Registry":   pdf.String("Adobe"),
			"Ordering":   pdf.String("Identity"),
			"Supplement": pdf.Integer(0),
		},
	}
	maps.Copy(cidFont, entries)
	return pdf.Dict{
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        pdf.Name("Test"),
		"Encoding":        pdf.Name("Identity-V"),
		"DescendantFonts": pdf.Array{cidFont},
	}
}

// malformedMetricsFiles returns PDF files whose trailer entry "Quir:E" refers
// to a font from malformedMetricsCases, for use as fuzzer seeds.
func malformedMetricsFiles(f *testing.F, cidType pdf.Name) [][]byte {
	var res [][]byte
	for _, tc := range malformedMetricsCases {
		out := memfile.New()
		w, err := pdf.NewWriter(out, pdf.V1_7, &pdf.WriterOptions{HumanReadable: true})
		if err != nil {
			f.Fatal(err)
		}
		if err := memfile.AddBlankPage(w); err != nil {
			f.Fatal(err)
		}
		entries, err := addCycles(w, tc.entries, tc.cycles)
		if err != nil {
			f.Fatal(err)
		}
		ref := w.Alloc()
		if err := w.Put(ref, malformedMetricsFont(cidType, entries)); err != nil {
			f.Fatal(err)
		}
		w.GetMeta().Trailer["Quir:E"] = ref
		if err := w.Close(); err != nil {
			f.Fatal(err)
		}
		res = append(res, out.Data)
	}
	return res
}

// TestCIDMalformedMetrics checks that malformed optional W, DW, W2 and DW2
// entries do not make a CIDFont unreadable, that they read as described in
// malformedMetricsCases, and that the result survives a write-read cycle.
func TestCIDMalformedMetrics(t *testing.T) {
	for _, cidType := range []pdf.Name{"CIDFontType0", "CIDFontType2"} {
		for _, tc := range malformedMetricsCases {
			t.Run(string(cidType)+"-"+tc.name, func(t *testing.T) {
				w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
				entries, err := addCycles(w, tc.entries, tc.cycles)
				if err != nil {
					t.Fatal(err)
				}
				fontDict := malformedMetricsFont(cidType, entries)

				x := pdf.NewExtractor(w)
				d, err := extract.Dict(pdf.CursorAt(x, nil), fontDict, false)
				if err != nil {
					t.Fatal(err)
				}

				var got cidMetrics
				switch d := d.(type) {
				case *dict.CIDFontType0:
					got = cidMetrics{collectMetrics(d.Width), d.DefaultWidth, collectMetrics(d.VMetrics), d.DefaultVMetrics}
					checkRoundTripT0(t, d, pdf.V1_7)
				case *dict.CIDFontType2:
					got = cidMetrics{collectMetrics(d.Width), d.DefaultWidth, collectMetrics(d.VMetrics), d.DefaultVMetrics}
					checkRoundTripT2(t, d, pdf.V1_7)
				default:
					t.Fatalf("unexpected font type %T", d)
				}
				if diff := cmp.Diff(tc.want, got); diff != "" {
					t.Errorf("metrics mismatch (-want +got):\n%s", diff)
				}
			})
		}
	}
}

// collectMetrics expands m to a map, which is nil if m has no entries.
func collectMetrics[T any](m dict.CIDMetrics[T]) map[cid.CID]T {
	var res map[cid.CID]T
	for c, v := range m.All() {
		if res == nil {
			res = make(map[cid.CID]T)
		}
		res[c] = v
	}
	return res
}

// TestCIDMetricsSharedWidths checks that font dictionaries sharing an indirect
// W or W2 array, decoded through one extractor, share the decoded runs.
func TestCIDMetricsSharedWidths(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	wRef, w2Ref := w.Alloc(), w.Alloc()
	if err := w.Put(wRef, pdf.Array{pdf.Integer(1), pdf.Array{pdf.Integer(500), pdf.Integer(600)}}); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(w2Ref, pdf.Array{
		pdf.Integer(1), pdf.Array{pdf.Integer(-900), pdf.Integer(250), pdf.Integer(800)},
	}); err != nil {
		t.Fatal(err)
	}
	entries := pdf.Dict{"W": wRef, "W2": w2Ref}

	x := pdf.NewExtractor(w)
	var d [2]*dict.CIDFontType2
	for i := range d {
		fd, err := extract.Dict(pdf.CursorAt(x, nil), malformedMetricsFont("CIDFontType2", entries), false)
		if err != nil {
			t.Fatal(err)
		}
		d[i] = fd.(*dict.CIDFontType2)
	}
	if len(d[0].Width) != 1 || len(d[0].VMetrics) != 1 {
		t.Fatalf("unexpected metrics %v, %v", d[0].Width, d[0].VMetrics)
	}
	if unsafe.SliceData(d[0].Width) != unsafe.SliceData(d[1].Width) {
		t.Error("Width slices are not shared")
	}
	if unsafe.SliceData(d[0].VMetrics) != unsafe.SliceData(d[1].VMetrics) {
		t.Error("VMetrics slices are not shared")
	}
}
