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
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// AddAppearance must report an error, not panic, when the target PDF version
// is too old for the operators the appearance needs (e.g. `gs`, PDF 1.2+).
// Readers synthesize appearances at the document's version, where the input
// is untrusted, so a malformed low-version file must not crash the caller.
func TestAddAppearanceLowVersion(t *testing.T) {
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
			// pre-1.2: gs is unavailable, so building must fail with an error
			if err := NewStyle(pdf.V1_1).AddAppearance(mk()); err == nil {
				t.Error("expected an error for PDF 1.1, got nil")
			}
			// 1.2 and later: building succeeds
			if err := NewStyle(pdf.V1_2).AddAppearance(mk()); err != nil {
				t.Errorf("unexpected error for PDF 1.2: %v", err)
			}
		})
	}
}

// TestErrNoFallback checks that a type without a fallback implementation is
// reported as such, and that a type which has one is not.  Callers use this
// distinction to tell an annotation they are content to skip from one whose
// fallback could not be built.
func TestErrNoFallback(t *testing.T) {
	s := NewStyle(pdf.V2_0)
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
			if err := NewStyle(pdf.V2_0).AddAppearance(a); err != nil {
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
			if err := NewStyle(pdf.V2_0).AddAppearance(a); err != nil {
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

				if err := NewStyle(pdf.V2_0).AddAppearance(w); err != nil {
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

	if err := NewStyle(pdf.V2_0).AddAppearance(w); err != nil {
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

			if err := NewStyle(pdf.V2_0).AddAppearance(a); err != nil {
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
