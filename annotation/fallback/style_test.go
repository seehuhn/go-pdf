// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package fallback

import (
	"errors"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// Appearances are built for every PDF version the library supports.  The reset
// state uses the line cap, join, miter limit and dash operators, which are
// available throughout, so no version is left without a fallback appearance.
func TestAddAppearanceAnyVersion(t *testing.T) {
	rect := pdf.Rectangle{LLx: 0, LLy: 0, URx: 20, URy: 20}

	build := map[string]func() annotation.Annotation{
		"text": func() annotation.Annotation {
			return &annotation.Text{Common: annotation.Common{Rect: rect}}
		},
		"widget": func() annotation.Annotation {
			return combWidget("AB", 6, pdf.TextAlignLeft)
		},
	}

	for name, mk := range build {
		t.Run(name, func(t *testing.T) {
			for _, v := range []pdf.Version{pdf.V1_0, pdf.V1_1, pdf.V1_2, pdf.V1_3, pdf.V1_4, pdf.V1_7, pdf.V2_0} {
				if err := newGen(t, v).AddAppearance(mk()); err != nil {
					t.Errorf("unexpected error for PDF %s: %v", v, err)
				}
			}
		})
	}
}

// The reset state carries text knockout and stroke adjustment in a graphics
// state dictionary, which cannot hold either before PDF 1.4.  Writing one into
// an older file would make the file invalid, so the appearance streams must
// leave the dictionary out below that version and include it from 1.4 on.
func TestResetGraphicsStateVersion(t *testing.T) {
	for _, tc := range []struct {
		version pdf.Version
		wantGS  bool
	}{
		{pdf.V1_0, false},
		{pdf.V1_2, false},
		{pdf.V1_3, false},
		{pdf.V1_4, true},
		{pdf.V2_0, true},
	} {
		t.Run(tc.version.String(), func(t *testing.T) {
			a := &annotation.Text{
				Common: annotation.Common{
					Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 20, URy: 20},
				},
			}
			if err := newGen(t, tc.version).AddAppearance(a); err != nil {
				t.Fatal(err)
			}

			normal := a.Appearance.Normal
			if normal == nil {
				t.Fatal("no normal appearance")
			}
			gotGS := len(normal.Res.ExtGState) > 0
			if gotGS != tc.wantGS {
				t.Errorf("graphics state dictionary present = %t, want %t", gotGS, tc.wantGS)
			}
		})
	}
}

// TestErrNoFallback checks that a type without a fallback implementation is
// reported as such, and that a type which has one is not.  Callers use this
// distinction to tell an annotation they are content to skip from one whose
// fallback could not be built.
func TestErrNoFallback(t *testing.T) {
	s := newGen(t, pdf.V2_0)
	rect := pdf.Rectangle{URx: 100, URy: 100}

	for _, a := range []annotation.Annotation{
		&annotation.PrinterMark{Common: annotation.Common{Rect: rect}},
		&annotation.TrapNet{Common: annotation.Common{Rect: rect}},
		&annotation.Projection{Common: annotation.Common{Rect: rect}},
	} {
		if err := s.AddAppearance(a); !errors.Is(err, ErrNoFallback) {
			t.Errorf("%T: expected ErrNoFallback, got %v", a, err)
		}
	}

	square := &annotation.Square{Common: annotation.Common{Rect: rect}}
	if err := s.AddAppearance(square); errors.Is(err, ErrNoFallback) {
		t.Error("Square: expected a fallback to be available")
	}
}

// TestAddAppearanceKeepsRollover checks that generating a normal appearance
// leaves the rollover and down appearances alone.  Those are content the
// caller supplied, and a reader which discarded them would hide a hover
// effect the file asks for.
func TestAddAppearanceKeepsRollover(t *testing.T) {
	rect := pdf.Rectangle{URx: 100, URy: 100}
	rollover := &form.Form{
		Content: &content.Operators{},
		BBox:    pdf.Rectangle{URx: 1, URy: 1},
	}
	down := &form.Form{
		Content: &content.Operators{},
		BBox:    pdf.Rectangle{URx: 2, URy: 2},
	}

	build := map[string]func() annotation.Annotation{
		"text": func() annotation.Annotation {
			return &annotation.Text{Common: annotation.Common{
				Rect:       rect,
				Appearance: &appearance.Dict{RollOver: rollover, Down: down},
			}}
		},
		"widget": func() annotation.Annotation {
			w := combWidget("AB", 6, pdf.TextAlignLeft)
			w.Appearance = &appearance.Dict{RollOver: rollover, Down: down}
			return w
		},
	}

	for name, mk := range build {
		t.Run(name, func(t *testing.T) {
			a := mk()
			if err := newGen(t, pdf.V2_0).AddAppearance(a); err != nil {
				t.Fatal(err)
			}
			ap := a.GetCommon().Appearance
			if ap.RollOver != rollover {
				t.Errorf("rollover appearance = %v, want it kept", ap.RollOver)
			}
			if ap.Down != down {
				t.Errorf("down appearance = %v, want it kept", ap.Down)
			}
			if !annotation.HasAppearance(a) {
				t.Error("no normal appearance was generated")
			}
		})
	}
}

// TestAddAppearanceKeepsRepeats checks the other side of
// [TestAddAppearanceKeepsRollover]: a rollover or down appearance which only
// repeats the normal one keeps repeating the generated appearance.  Reading an
// annotation whose appearance the file leaves out yields three entries sharing
// one form, and leaving them behind would turn the discarded appearance into a
// hover effect the file never asked for.
func TestAddAppearanceKeepsRepeats(t *testing.T) {
	rect := pdf.Rectangle{URx: 100, URy: 100}
	empty := &form.Form{BBox: rect}

	build := map[string]func() annotation.Annotation{
		"text": func() annotation.Annotation {
			return &annotation.Text{Common: annotation.Common{
				Rect: rect,
				Appearance: &appearance.Dict{
					Normal: empty, RollOver: empty, Down: empty,
				},
			}}
		},
		"widget": func() annotation.Annotation {
			w := combWidget("AB", 6, pdf.TextAlignLeft)
			w.Appearance = &appearance.Dict{
				Normal: empty, RollOver: empty, Down: empty,
			}
			return w
		},
	}

	for name, mk := range build {
		t.Run(name, func(t *testing.T) {
			a := mk()
			if err := newGen(t, pdf.V2_0).AddAppearance(a); err != nil {
				t.Fatal(err)
			}

			c := a.GetCommon()
			ap := c.Appearance
			normal := ap.Resolve(c.AppearanceState, appearance.Normal)
			if normal == nil || normal == empty {
				t.Fatal("no normal appearance was generated")
			}
			if got := ap.Resolve(c.AppearanceState, appearance.RollOver); got != normal {
				t.Errorf("rollover appearance = %v, want the generated one", got)
			}
			if got := ap.Resolve(c.AppearanceState, appearance.Down); got != normal {
				t.Errorf("down appearance = %v, want the generated one", got)
			}
			if annotation.HasRolloverAppearance(a) {
				t.Error("the annotation gained a rollover appearance")
			}
		})
	}
}

// TestAddAppearanceKeepsStateForRollover checks that an annotation whose
// rollover or down appearance selects by state keeps an appearance state to
// select with, even though the generated normal appearance is a single stream.
// Without one the appearance dictionary could not be written back.
func TestAddAppearanceKeepsStateForRollover(t *testing.T) {
	rollover := &form.Form{
		Content: &content.Operators{},
		BBox:    pdf.Rectangle{URx: 1, URy: 1},
	}

	cases := []struct {
		name  string
		state pdf.Name // the state the file names, if any
		want  pdf.Name
	}{
		{name: "keptState", state: "Off", want: "Off"},
		// the file names no state, so one has to be supplied
		{name: "suppliedState", state: "", want: "Off"},
	}

	build := map[string]func() *appearance.Dict{
		"rollover": func() *appearance.Dict {
			return &appearance.Dict{
				RollOverMap: map[pdf.Name]*form.Form{"Off": rollover},
			}
		},
		"down": func() *appearance.Dict {
			return &appearance.Dict{
				DownMap: map[pdf.Name]*form.Form{"Off": rollover},
			}
		},
	}

	for entry, mkDict := range build {
		for _, tc := range cases {
			t.Run(entry+"/"+tc.name, func(t *testing.T) {
				w := combWidget("AB", 6, pdf.TextAlignLeft)
				w.Appearance = mkDict()
				w.AppearanceState = tc.state

				if err := newGen(t, pdf.V2_0).AddAppearance(w); err != nil {
					t.Fatal(err)
				}

				if got := w.AppearanceState; got != tc.want {
					t.Errorf("appearance state = %q, want %q", got, tc.want)
				}
				if !annotation.HasAppearance(w) {
					t.Error("no normal appearance was generated")
				}

				buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
				rm := pdf.NewResourceManager(buf)
				if _, err := w.Encode(rm); err != nil {
					t.Errorf("encoding the annotation: %v", err)
				}
			})
		}
	}
}

// TestAddAppearanceDropsUnusedState checks the other direction: an appearance
// dictionary which no longer selects by state has no use for an appearance
// state, and keeping one would name a state nothing defines.
func TestAddAppearanceDropsUnusedState(t *testing.T) {
	w := combWidget("AB", 6, pdf.TextAlignLeft)
	w.AppearanceState = "Off"

	if err := newGen(t, pdf.V2_0).AddAppearance(w); err != nil {
		t.Fatal(err)
	}

	if got := w.AppearanceState; got != "" {
		t.Errorf("appearance state = %q, want it dropped", got)
	}
}

// TestAddAppearanceLeavesSharedDictAlone checks that generating an appearance
// does not write through to other annotations.  Reading a file shares one
// appearance dictionary between every annotation whose AP entry points at it,
// so a repair applied in place would change annotations nobody asked about.
func TestAddAppearanceLeavesSharedDictAlone(t *testing.T) {
	rect := pdf.Rectangle{URx: 100, URy: 100}
	original := &form.Form{
		Content: &content.Operators{},
		BBox:    pdf.Rectangle{URx: 1, URy: 1},
	}

	build := map[string]func(*appearance.Dict) annotation.Annotation{
		"text": func(ap *appearance.Dict) annotation.Annotation {
			return &annotation.Text{Common: annotation.Common{Rect: rect, Appearance: ap}}
		},
		"widget": func(ap *appearance.Dict) annotation.Annotation {
			w := combWidget("AB", 6, pdf.TextAlignLeft)
			w.Appearance = ap
			return w
		},
	}

	for name, mk := range build {
		t.Run(name, func(t *testing.T) {
			// the shape reading an indirect AP entry produces: the rollover
			// and down entries hold the normal appearance, and one dictionary
			// serves both annotations
			shared := &appearance.Dict{
				NormalMap:   map[pdf.Name]*form.Form{"On": original},
				RollOverMap: map[pdf.Name]*form.Form{"On": original},
				DownMap:     map[pdf.Name]*form.Form{"On": original},
			}
			a, other := mk(shared), mk(shared)
			a.GetCommon().AppearanceState = "On"
			other.GetCommon().AppearanceState = "On"

			if err := newGen(t, pdf.V2_0).AddAppearance(a); err != nil {
				t.Fatal(err)
			}

			if other.GetCommon().Appearance != shared {
				t.Fatal("the other annotation lost its appearance dictionary")
			}
			if shared.NormalMap["On"] != original {
				t.Error("the shared normal appearance was replaced")
			}
			if shared.RollOverMap["On"] != original || shared.DownMap["On"] != original {
				t.Error("the shared rollover or down appearance was replaced")
			}
			if a.GetCommon().Appearance.NormalMap["On"] == original {
				t.Error("no normal appearance was generated")
			}
		})
	}
}

// The fonts a Generator draws with are made once and then reused, so that a
// file holds a single copy of each however many appearances are built.  The
// icon font is made on first use rather than with the Generator, which is where
// a second copy could otherwise creep in.
func TestGeneratorReusesFonts(t *testing.T) {
	g := newGen(t, pdf.V2_0)

	iconFontOf := func() font.Instance {
		t.Helper()
		a := &annotation.Text{
			Common: annotation.Common{
				Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 20, URy: 20},
			},
			Icon: annotation.TextIconHelp,
		}
		if err := g.AddAppearance(a); err != nil {
			t.Fatal(err)
		}
		for _, F := range a.Appearance.Normal.Res.Font {
			return F
		}
		t.Fatal("appearance references no font")
		return nil
	}

	if first, second := iconFontOf(), iconFontOf(); first != second {
		t.Error("the icon font was made twice")
	}
}

// newGen returns a Generator for the default style, for use in tests.
func newGen(t *testing.T, version pdf.Version) *Generator {
	t.Helper()
	gen, err := NewStyle().New(version)
	if err != nil {
		t.Fatal(err)
	}
	return gen
}
