// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package extract_test

import (
	"maps"
	"testing"

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
		ref := w.Alloc()
		if err := w.Put(ref, malformedMetricsFont(cidType, tc.entries)); err != nil {
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
				fontDict := malformedMetricsFont(cidType, tc.entries)

				x := pdf.NewExtractor(w)
				d, err := extract.Dict(pdf.CursorAt(x, nil), fontDict, false)
				if err != nil {
					t.Fatal(err)
				}

				var got cidMetrics
				switch d := d.(type) {
				case *dict.CIDFontType0:
					got = cidMetrics{d.Width, d.DefaultWidth, d.VMetrics, d.DefaultVMetrics}
					checkRoundTripT0(t, d, pdf.V1_7)
				case *dict.CIDFontType2:
					got = cidMetrics{d.Width, d.DefaultWidth, d.VMetrics, d.DefaultVMetrics}
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
