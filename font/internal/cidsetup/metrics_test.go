// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cidsetup

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding/cidenc"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

func TestGlyphVMetrics(t *testing.T) {
	info := makefont.TrueType()
	for gid := range glyph.ID(info.NumGlyphs()) {
		advance, originY := info.GlyphVerticalPDF(gid)
		want := cidenc.VMetrics{OffsY: math.Round(originY), DeltaY: -math.Round(advance)}
		if got := GlyphVMetrics(info, gid); got != want {
			t.Fatalf("glyph %d: got %v, want %v", gid, got, want)
		}
	}
}

// glyphData is the metrics of one CID, as given to the encoder
type glyphData struct {
	cid   cid.CID
	width float64
	v     cidenc.VMetrics
}

func metricsFor(t *testing.T, wMode font.WritingMode, glyphs []glyphData, notdef cidenc.VMetrics) (map[cid.CID]float64, float64, dict.DefaultVMetrics, map[cid.CID]dict.VMetrics) {
	t.Helper()
	enc := cidenc.NewCompositeUtf8(nil, 600, notdef, wMode)
	for i, g := range glyphs {
		if _, err := enc.Encode(g.cid, string(rune('a'+i)), g.width, g.v); err != nil {
			t.Fatal(err)
		}
	}
	return Metrics(enc)
}

func TestMetrics(t *testing.T) {
	short := cidenc.VMetrics{OffsY: 640, DeltaY: -750}
	tall := cidenc.VMetrics{OffsY: 771, DeltaY: -1000}
	notdef := cidenc.VMetrics{OffsY: 880, DeltaY: -1000}
	glyphs := []glyphData{
		{1, 500, short},
		{2, 501, tall},
		{3, 400, short},
		{1, 500, short}, // a second code for CID 1
		{4, 300, tall},
		{5, 200, short},
	}

	t.Run("horizontal", func(t *testing.T) {
		ww, dw, dw2, w2 := metricsFor(t, font.Horizontal, glyphs, notdef)
		wantWW := map[cid.CID]float64{1: 500, 2: 501, 3: 400, 4: 300, 5: 200}
		if d := cmp.Diff(wantWW, ww); d != "" {
			t.Errorf("widths (-want +got):\n%s", d)
		}
		if dw != 600 {
			t.Errorf("DW is %g, want 600", dw)
		}
		if dw2 != dict.DefaultVMetricsDefault || w2 != nil {
			t.Errorf("got %v, %v, want the default and no W2", dw2, w2)
		}
	})

	t.Run("vertical", func(t *testing.T) {
		_, _, dw2, w2 := metricsFor(t, font.Vertical, glyphs, notdef)
		if want := (dict.DefaultVMetrics{OffsY: 880, DeltaY: -1000}); dw2 != want {
			t.Errorf("DW2 is %v, want %v", dw2, want)
		}
		want := map[cid.CID]dict.VMetrics{
			1: {OffsX: 250, OffsY: 640, DeltaY: -750},
			2: {OffsX: 250.5, OffsY: 771, DeltaY: -1000},
			3: {OffsX: 200, OffsY: 640, DeltaY: -750},
			4: {OffsX: 150, OffsY: 771, DeltaY: -1000},
			5: {OffsX: 100, OffsY: 640, DeltaY: -750},
		}
		if d := cmp.Diff(want, w2); d != "" {
			t.Errorf("W2 (-want +got):\n%s", d)
		}
	})

	// DW2 is the metrics of CID 0, also where most CIDs share other metrics
	t.Run("notdef default", func(t *testing.T) {
		_, _, dw2, w2 := metricsFor(t, font.Vertical, glyphs, short)
		if want := (dict.DefaultVMetrics{OffsY: 640, DeltaY: -750}); dw2 != want {
			t.Errorf("DW2 is %v, want %v", dw2, want)
		}
		want := map[cid.CID]dict.VMetrics{
			2: {OffsX: 250.5, OffsY: 771, DeltaY: -1000},
			4: {OffsX: 150, OffsY: 771, DeltaY: -1000},
		}
		if d := cmp.Diff(want, w2); d != "" {
			t.Errorf("W2 (-want +got):\n%s", d)
		}
	})

	// CID 0 in use is treated like any other CID
	t.Run("notdef used", func(t *testing.T) {
		gg := append([]glyphData{{0, 600, notdef}}, glyphs...)
		ww, dw, _, w2 := metricsFor(t, font.Vertical, gg, notdef)
		if _, ok := w2[0]; ok {
			t.Errorf("CID 0 has W2 entry %v", w2[0])
		}
		if ww[0] != 600 || dw != 600 {
			t.Errorf("CID 0 has width %g and DW %g, want 600", ww[0], dw)
		}
	})
}

func TestMetricsEmpty(t *testing.T) {
	for _, notdef := range []cidenc.VMetrics{
		{OffsY: 880, DeltaY: -1000},
		{OffsY: 771, DeltaY: -1000},
	} {
		_, _, dw2, w2 := metricsFor(t, font.Vertical, nil, notdef)
		if want := (dict.DefaultVMetrics{OffsY: notdef.OffsY, DeltaY: notdef.DeltaY}); dw2 != want {
			t.Errorf("DW2 is %v, want %v", dw2, want)
		}
		if w2 != nil {
			t.Errorf("W2 is %v, want none", w2)
		}
	}
}
