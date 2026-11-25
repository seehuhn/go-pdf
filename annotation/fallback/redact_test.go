// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fallback

import (
	"maps"
	"testing"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
)

var redactRect = pdf.Rectangle{LLx: 50, LLy: 100, URx: 250, URy: 140}

func redactQuads() []vec.Vec2 {
	return []vec.Vec2{
		{X: 50, Y: 140}, {X: 150, Y: 140}, {X: 50, Y: 120}, {X: 150, Y: 120},
		{X: 100, Y: 120}, {X: 250, Y: 120}, {X: 100, Y: 100}, {X: 250, Y: 100},
	}
}

// TestRedactAppearances checks the appearance dictionary generated for a
// redaction annotation.
func TestRedactAppearances(t *testing.T) {
	for _, quads := range [][]vec.Vec2{nil, redactQuads()} {
		a := &annotation.Redact{
			Rect:       redactRect,
			QuadPoints: quads,
		}
		if err := newGen(t, pdf.V1_7).AddAppearance(a); err != nil {
			t.Fatal(err)
		}

		ap := a.Appearance
		if ap == nil || ap.Normal == nil || ap.RollOver == nil || ap.Down == nil {
			t.Fatal("normal, rollover or down appearance missing")
		}
		if ap.Down != ap.RollOver {
			t.Error("down differs from rollover")
		}
		if ap.RollOver == ap.Normal {
			t.Error("rollover repeats the normal appearance")
		}
		if !annotation.HasRolloverAppearance(a) {
			t.Error("rollover not recognised")
		}
		for _, f := range []*form.Form{ap.Normal, ap.RollOver} {
			if f.BBox != redactRect {
				t.Errorf("bbox = %v, want %v", f.BBox, redactRect)
			}
		}
		if a.Rect != redactRect {
			t.Errorf("rect changed to %v", a.Rect)
		}
	}
}

func TestRedactOverlayIsDrawn(t *testing.T) {
	overlay := &form.Form{
		Res:  &content.Resources{},
		BBox: pdf.Rectangle{URx: 200, URy: 40},
	}
	a := &annotation.Redact{
		Rect:    redactRect,
		Overlay: overlay,
	}
	if err := newGen(t, pdf.V1_7).AddAppearance(a); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, x := range a.Appearance.RollOver.Res.XObject {
		if x == overlay {
			found = true
		}
	}
	if !found {
		t.Error("rollover does not draw the overlay")
	}
	for _, x := range a.Appearance.Normal.Res.XObject {
		if x == overlay {
			t.Error("normal appearance draws the overlay")
		}
	}
}

func TestRedactOverlayText(t *testing.T) {
	for _, quads := range [][]vec.Vec2{nil, redactQuads()} {
		for _, repeat := range []bool{false, true} {
			for _, align := range []pdf.TextAlign{pdf.TextAlignLeft, pdf.TextAlignCenter, pdf.TextAlignRight} {
				for _, da := range []string{"/Helv 12 Tf 0 g", "/Helv 0 Tf 1 0 0 rg", "/Helv 400 Tf"} {
					a := &annotation.Redact{
						Rect:              redactRect,
						QuadPoints:        quads,
						FillColor:         color.DeviceRGB{0, 0, 0},
						OverlayText:       pdf.Text("REDACTED"),
						DefaultAppearance: da,
						Repeat:            repeat,
						Align:             align,
					}
					if err := newGen(t, pdf.V1_7).AddAppearance(a); err != nil {
						t.Fatal(err)
					}
					rollover := a.Appearance.RollOver
					if rollover == nil || len(rollover.Res.Font) == 0 {
						t.Errorf("repeat=%t align=%d da=%q: rollover draws no text", repeat, align, da)
					}
				}
			}
		}
	}
}

// TestRedactAppearanceState checks that an annotation which selects its
// appearance by state gets the preview as rollover and down appearance for
// its current state.
func TestRedactAppearanceState(t *testing.T) {
	old := &form.Form{Res: &content.Resources{}, BBox: redactRect}
	states := map[pdf.Name]*form.Form{"A": old, "B": old}
	a := &annotation.Redact{
		Rect:            redactRect,
		AppearanceState: "A",
		Appearance: &appearance.Dict{
			NormalMap:   maps.Clone(states),
			RollOverMap: maps.Clone(states),
			DownMap:     maps.Clone(states),
		},
	}
	if err := newGen(t, pdf.V1_7).AddAppearance(a); err != nil {
		t.Fatal(err)
	}

	normal := annotation.Resolve(&a.Common, appearance.Normal)
	rollover := annotation.Resolve(&a.Common, appearance.RollOver)
	down := annotation.Resolve(&a.Common, appearance.Down)
	if normal == nil || normal == old {
		t.Error("normal appearance not replaced")
	}
	if rollover == nil || rollover == old || rollover == normal {
		t.Error("rollover does not show the preview")
	}
	if down != rollover {
		t.Error("down differs from rollover")
	}
}
