// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package dict

import (
	"slices"
	"testing"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
)

// makeVerticalCMap returns a CMap that maps single-byte codes 1..255 to the
// CIDs with the same numeric value, in vertical writing mode.
func makeVerticalCMap() *cmap.File {
	return &cmap.File{
		Name:           "Test-V",
		ROS:            &cid.SystemInfo{Registry: "Adobe", Ordering: "Identity", Supplement: 0},
		WMode:          font.Vertical,
		CodeSpaceRange: charcode.CodeSpaceRange{{Low: []byte{0x00}, High: []byte{0xFF}}},
		CIDRanges: []cmap.Range{
			{First: []byte{0x01}, Last: []byte{0xFF}, Value: 1},
		},
	}
}

// TestVerticalAdvancePopulationDefault verifies that a vertical font without
// per-glyph VMetrics yields the default DeltaY in font.Code.VerticalAdvance.
func TestVerticalAdvancePopulationDefault(t *testing.T) {
	d := &CIDFontType2{
		PostScriptName:  "Test",
		ROS:             &cid.SystemInfo{Registry: "Adobe", Ordering: "Identity", Supplement: 0},
		CMap:            makeVerticalCMap(),
		Width:           map[cid.CID]float64{1: 500},
		DefaultWidth:    1000,
		DefaultVMetrics: DefaultVMetrics{OffsY: 880, DeltaY: -1000},
	}

	f := d.MakeFont()
	for c := range f.Codes(pdf.String{0x01}) {
		if c.Width != 0.5 {
			t.Errorf("Width = %v, want 0.5", c.Width)
		}
		if c.VerticalAdvance != -1 {
			t.Errorf("VerticalAdvance = %v, want -1", c.VerticalAdvance)
		}
	}
}

// TestVerticalAdvancePopulationPerGlyph verifies that VMetrics overrides
// DefaultVMetrics for matching CIDs.
func TestVerticalAdvancePopulationPerGlyph(t *testing.T) {
	d := &CIDFontType2{
		PostScriptName:  "Test",
		ROS:             &cid.SystemInfo{Registry: "Adobe", Ordering: "Identity", Supplement: 0},
		CMap:            makeVerticalCMap(),
		Width:           map[cid.CID]float64{1: 500, 2: 500},
		DefaultWidth:    1000,
		DefaultVMetrics: DefaultVMetricsDefault,
		VMetrics: map[cid.CID]VMetrics{
			2: {DeltaY: -1200, OffsX: 0, OffsY: 880},
		},
	}

	f := d.MakeFont()
	got := make(map[cid.CID]float64)
	for c := range f.Codes(pdf.String{0x01, 0x02}) {
		got[c.CID] = c.VerticalAdvance
	}

	if got[1] != -1 {
		t.Errorf("CID 1 (default) VerticalAdvance = %v, want -1", got[1])
	}
	if got[2] != -1.2 {
		t.Errorf("CID 2 (override) VerticalAdvance = %v, want -1.2", got[2])
	}
}

// TestHorizontalCodeNoVerticalAdvance verifies that horizontal-mode fonts
// leave VerticalAdvance unset (zero) even when VMetrics is configured.
func TestHorizontalCodeNoVerticalAdvance(t *testing.T) {
	cm := makeVerticalCMap()
	cm.WMode = font.Horizontal

	d := &CIDFontType2{
		PostScriptName: "Test",
		ROS:            &cid.SystemInfo{Registry: "Adobe", Ordering: "Identity", Supplement: 0},
		CMap:           cm,
		Width:          map[cid.CID]float64{1: 500},
		DefaultWidth:   1000,
		VMetrics: map[cid.CID]VMetrics{
			1: {DeltaY: -1200, OffsX: 0, OffsY: 880},
		},
	}

	f := d.MakeFont()
	for c := range f.Codes(pdf.String{0x01}) {
		if c.VerticalAdvance != 0 {
			t.Errorf("horizontal-mode VerticalAdvance = %v, want 0", c.VerticalAdvance)
		}
	}
}

// TestVerticalCodeMetrics checks the vertical advance and the position
// vector the composite font readers report: from W2, or from DW2 with half
// the glyph width.  A displacement of zero is reported as zero.
func TestVerticalCodeMetrics(t *testing.T) {
	ros := &cid.SystemInfo{Registry: "Adobe", Ordering: "Identity", Supplement: 0}
	width := map[cid.CID]float64{1: 600, 2: 500}
	dw2 := DefaultVMetrics{OffsY: 880, DeltaY: 0}
	w2 := map[cid.CID]VMetrics{2: {OffsX: 250, OffsY: 772, DeltaY: 0}, 3: {OffsX: 100, OffsY: 700, DeltaY: -900}}
	want := [][3]float64{
		{0, 0.3, 0.88},   // CID 1: DW2, half of W
		{0, 0.25, 0.772}, // CID 2: W2
		{-0.9, 0.1, 0.7}, // CID 3: W2
		{0, 0.5, 0.88},   // CID 4: DW2, half of DW
	}
	for name, F := range map[string]font.Instance{
		"CIDFontType0": (&CIDFontType0{
			PostScriptName: "Test", ROS: ros, CMap: makeVerticalCMap(),
			Width: width, DefaultWidth: 1000, DefaultVMetrics: dw2, VMetrics: w2,
		}).MakeFont(),
		"CIDFontType2": (&CIDFontType2{
			PostScriptName: "Test", ROS: ros, CMap: makeVerticalCMap(),
			Width: width, DefaultWidth: 1000, DefaultVMetrics: dw2, VMetrics: w2,
		}).MakeFont(),
	} {
		var got [][3]float64
		for c := range F.Codes(pdf.String{1, 2, 3, 4}) {
			got = append(got, [3]float64{c.VerticalAdvance, c.VerticalOriginX, c.VerticalOriginY})
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
}
