// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package type3

import (
	"errors"
	"io"
	"math"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
)

// TestGeometryCapHeight checks that the cap height and x-height reported by
// the font geometry are in text space units, i.e. that the font matrix is
// applied to the values given in Type 3 glyph space.
func TestGeometryCapHeight(t *testing.T) {
	const unitsPerEm = 2048

	f := &Font{
		Glyphs:     []*Glyph{{}},
		FontMatrix: [6]float64{1. / unitsPerEm, 0, 0, 1. / unitsPerEm, 0, 0},
		Ascent:     1600,
		Descent:    -400,
		CapHeight:  1400,
		XHeight:    1000,
	}
	inst, err := f.New()
	if err != nil {
		t.Fatal(err)
	}

	geom := inst.GetGeometry()
	cases := []struct {
		name string
		got  float64
		want float64
	}{
		{"CapHeight", geom.CapHeight, 1400. / unitsPerEm},
		{"XHeight", geom.XHeight, 1000. / unitsPerEm},
	}
	for _, c := range cases {
		if math.Abs(c.got-c.want) > 1e-9 {
			t.Errorf("%s = %g, want %g", c.name, c.got, c.want)
		}
	}
}

// TestGeometryMeasuredHeights checks that a Type 3 font which leaves CapHeight
// and XHeight unset has them measured from the "H" and "x" glyphs.
func TestGeometryMeasuredHeights(t *testing.T) {
	const unitsPerEm = 1000

	f := &Font{
		Glyphs: []*Glyph{
			{},
			{Name: "H", Content: box(700)},
			{Name: "x", Content: box(450)},
		},
		FontMatrix: [6]float64{1. / unitsPerEm, 0, 0, 1. / unitsPerEm, 0, 0},
		Ascent:     800,
		Descent:    -200,
	}
	inst, err := f.New()
	if err != nil {
		t.Fatal(err)
	}

	geom := inst.GetGeometry()
	if math.Abs(geom.CapHeight-700./unitsPerEm) > 1e-9 {
		t.Errorf("CapHeight = %g, want %g", geom.CapHeight, 700./unitsPerEm)
	}
	if math.Abs(geom.XHeight-450./unitsPerEm) > 1e-9 {
		t.Errorf("XHeight = %g, want %g", geom.XHeight, 450./unitsPerEm)
	}
}

// TestGeometryEstimatedHeights checks the estimates used for a Type 3 font
// which reports no heights and has no "H" or "x" glyph to measure.
func TestGeometryEstimatedHeights(t *testing.T) {
	f := &Font{
		Glyphs:     []*Glyph{{}, {Name: "square", Content: box(700)}},
		FontMatrix: [6]float64{0.001, 0, 0, 0.001, 0, 0},
	}
	inst, err := f.New()
	if err != nil {
		t.Fatal(err)
	}

	geom := inst.GetGeometry()
	if !(0 < geom.XHeight && geom.XHeight <= geom.CapHeight && geom.CapHeight <= geom.Leading) {
		t.Errorf("estimates out of order: XHeight %g, CapHeight %g, Leading %g",
			geom.XHeight, geom.CapHeight, geom.Leading)
	}
}

// box returns a glyph drawing a filled rectangle of the given height, in
// Type 3 glyph space units.
func box(height float64) content.Stream {
	b := builder.New(content.Glyph, nil, pdf.V2_0)
	b.Type3UncoloredGlyph(500, 0, 0, 0, 400, height)
	b.Rectangle(0, 0, 400, height)
	b.Fill()
	return builder.Must(b.Harvest())
}

// TestGeometryFallbackGlyphs checks that the heights are measured from a later
// candidate when the font has no "H" or "x" glyph, as a subset may not.
func TestGeometryFallbackGlyphs(t *testing.T) {
	const unitsPerEm = 1000

	f := &Font{
		Glyphs: []*Glyph{
			{},
			{Name: "L", Content: box(680)},
			{Name: "v", Content: box(430)},
		},
		FontMatrix: [6]float64{1. / unitsPerEm, 0, 0, 1. / unitsPerEm, 0, 0},
	}
	inst, err := f.New()
	if err != nil {
		t.Fatal(err)
	}

	geom := inst.GetGeometry()
	if math.Abs(geom.CapHeight-680./unitsPerEm) > 1e-9 {
		t.Errorf("CapHeight = %g, want %g", geom.CapHeight, 680./unitsPerEm)
	}
	if math.Abs(geom.XHeight-430./unitsPerEm) > 1e-9 {
		t.Errorf("XHeight = %g, want %g", geom.XHeight, 430./unitsPerEm)
	}
}

// TestNewInvalidGlyphs checks that Font.New reports glyph procedures which
// cannot be written.
func TestNewInvalidGlyphs(t *testing.T) {
	noMetrics := &content.Operators{Ops: []content.Operator{
		{Name: content.OpFill},
	}}

	unreadable := content.NewScanner(func() (io.ReadCloser, error) {
		return nil, errors.New("I/O error")
	})

	b := builder.New(content.Glyph, nil, pdf.V2_0)
	b.Type3ColoredGlyph(500, 0)
	valid := builder.Must(b.Harvest())

	cases := []struct {
		name   string
		glyphs []*Glyph
		ok     bool
	}{
		{"valid", []*Glyph{{}, {Name: "A", Content: valid}}, true},
		{"empty notdef", []*Glyph{{}}, true},
		{"nil content", []*Glyph{{}, {Name: "A"}}, false},
		{"empty content", []*Glyph{{}, {Name: "A", Content: &content.Operators{}}}, false},
		{"no metrics", []*Glyph{{}, {Name: "A", Content: noMetrics}}, false},
		{"read error", []*Glyph{{}, {Name: "A", Content: unreadable}}, false},
		{"no glyph 0", nil, false},
		{"named glyph 0", []*Glyph{{Name: "A", Content: valid}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &Font{
				Glyphs:     c.glyphs,
				FontMatrix: [6]float64{0.001, 0, 0, 0.001, 0, 0},
			}
			_, err := f.New()
			if (err == nil) != c.ok {
				t.Errorf("err = %v, want success = %t", err, c.ok)
			}
		})
	}
}
