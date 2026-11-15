// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package dict

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/internal/debug/memfile"

	"seehuhn.de/go/postscript/cid"
)

func TestCompositeWidthsEncode(t *testing.T) {
	for _, tc := range compositeWidthTests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := encodeCompositeWidths(CIDMetricsFromMap(tc.widthMap))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(res, tc.expected) {
				t.Errorf("unexpected result\nGot: %#v\nExpected: %#v", res, tc.expected)
			}
		})
	}
}

var compositeWidthTests = []struct {
	name     string
	widthMap map[cid.CID]float64
	expected pdf.Array
}{
	{
		name:     "single width",
		widthMap: map[cid.CID]float64{5: 105},
		expected: pdf.Array{
			pdf.Integer(5),
			pdf.Array{pdf.Number(105)},
		},
	},
	{
		name:     "contiguous equal widths",
		widthMap: map[cid.CID]float64{1: 101, 2: 101, 3: 101},
		expected: pdf.Array{
			pdf.Integer(1),
			pdf.Integer(3),
			pdf.Number(101),
		},
	},
	{
		name:     "non contiguous equal widths",
		widthMap: map[cid.CID]float64{1: 101, 3: 101},
		expected: pdf.Array{
			pdf.Integer(1),
			pdf.Array{pdf.Number(101)},
			pdf.Integer(3),
			pdf.Array{pdf.Number(101)},
		},
	},
	{
		name:     "contiguous unequal widths",
		widthMap: map[cid.CID]float64{1: 101, 2: 102, 3: 103},
		expected: pdf.Array{
			pdf.Integer(1),
			pdf.Array{
				pdf.Number(101),
				pdf.Number(102),
				pdf.Number(103),
			},
		},
	},
	{
		name: "mixed equal and unequal widths",
		widthMap: map[cid.CID]float64{
			1: 101,
			2: 101,
			3: 101,
			4: 102,
			5: 103,
		},
		expected: pdf.Array{
			pdf.Integer(1),
			pdf.Integer(3),
			pdf.Number(101),
			pdf.Integer(4),
			pdf.Array{
				pdf.Number(102),
				pdf.Number(103),
			},
		},
	},
}

func TestEncodeCompositeWidthsVerbatim(t *testing.T) {
	m := CIDMetrics[float64]{
		{First: 1, Last: 3, Value: 500},
		{First: 4, Last: 4, Values: []float64{600}},
	}
	got, err := encodeCompositeWidths(m)
	if err != nil {
		t.Fatal(err)
	}
	want := pdf.Array{
		pdf.Integer(1), pdf.Integer(3), pdf.Number(500),
		pdf.Integer(4), pdf.Array{pdf.Number(600)},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected result (-want +got):\n%s", diff)
	}

	if got, err := encodeCompositeWidths(nil); err != nil || got != nil {
		t.Errorf("empty input: got %v, %v", got, err)
	}

	bad := CIDMetrics[float64]{{First: 1, Last: 3, Value: 1}, {First: 3, Last: 4, Value: 2}}
	if _, err := encodeCompositeWidths(bad); err == nil {
		t.Error("overlapping runs not rejected")
	}
}

func TestEncodeVMetricsVerbatim(t *testing.T) {
	v1 := VMetrics{DeltaY: -1000, OffsX: 500, OffsY: 880}
	v2 := VMetrics{DeltaY: -900, OffsX: 250, OffsY: 700}
	m := CIDMetrics[VMetrics]{
		{First: 1, Last: 3, Value: v1},
		{First: 4, Last: 4, Values: []VMetrics{v2}},
	}
	got, err := encodeVMetrics(m)
	if err != nil {
		t.Fatal(err)
	}
	want := pdf.Array{
		pdf.Integer(1), pdf.Integer(3),
		pdf.Number(-1000), pdf.Number(500), pdf.Number(880),
		pdf.Integer(4), pdf.Array{pdf.Number(-900), pdf.Number(250), pdf.Number(700)},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected result (-want +got):\n%s", diff)
	}

	if got, err := encodeVMetrics(nil); err != nil || got != nil {
		t.Errorf("empty input: got %v, %v", got, err)
	}

	bad := CIDMetrics[VMetrics]{{First: 1, Last: 65536, Value: v1}}
	if _, err := encodeVMetrics(bad); err == nil {
		t.Error("out-of-range CID not rejected")
	}
}

func TestCIDFontInvalidMetricsEncode(t *testing.T) {
	ros := &cid.SystemInfo{Registry: "Adobe", Ordering: "Identity", Supplement: 0}
	overlap := CIDMetrics[float64]{
		{First: 1, Last: 5, Value: 500},
		{First: 5, Last: 6, Value: 600},
	}
	tooBig := CIDMetrics[VMetrics]{
		{First: 65530, Last: 65536, Value: VMetrics{DeltaY: -1000}},
	}

	tests := map[string]func(w CIDMetrics[float64], v CIDMetrics[VMetrics]) pdf.Embedder{
		"type0": func(w CIDMetrics[float64], v CIDMetrics[VMetrics]) pdf.Embedder {
			return &CIDFontType0{
				PostScriptName: "Test", ROS: ros, CMap: cmap.IdentityH,
				Descriptor:   &font.Descriptor{FontName: "Test"},
				Width:        w,
				DefaultWidth: DefaultWidthDefault, DefaultVMetrics: DefaultVMetricsDefault,
				VMetrics: v,
			}
		},
		"type2": func(w CIDMetrics[float64], v CIDMetrics[VMetrics]) pdf.Embedder {
			return &CIDFontType2{
				PostScriptName: "Test", ROS: ros, CMap: cmap.IdentityH,
				Descriptor:   &font.Descriptor{FontName: "Test"},
				Width:        w,
				DefaultWidth: DefaultWidthDefault, DefaultVMetrics: DefaultVMetricsDefault,
				VMetrics: v,
			}
		},
	}
	for name, mk := range tests {
		for what, d := range map[string]pdf.Embedder{
			"valid":   mk(CIDMetrics[float64]{{First: 1, Last: 5, Value: 500}}, nil),
			"overlap": mk(overlap, nil),
			"too big": mk(nil, tooBig),
		} {
			t.Run(name+"/"+what, func(t *testing.T) {
				w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
				rm := pdf.NewResourceManager(w)
				_, err := rm.Embed(d)
				if (err != nil) != (what != "valid") {
					t.Errorf("unexpected error state: %v", err)
				}
			})
		}
	}
}
