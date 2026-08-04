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
	"maps"
	"slices"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
)

// toggleWidget returns a check box widget for a field whose value is value,
// with the on-state named by on.  Setting radio makes it a radio button.
func toggleWidget(t *testing.T, value, on pdf.Name, radio bool) *annotation.Widget {
	t.Helper()
	f := acroform.NewButtonField("b")
	f.V = value
	if radio {
		f.Flags = acroform.FieldRadio
		f.Opt = []string{string(on)}
	} else if on != "" {
		// a lone check box names its on-state through the field value
		f.Opt = []string{string(on)}
	}
	return annotation.AddWidget(f, pdf.Rectangle{LLx: 0, LLy: 0, URx: 16, URy: 16})
}

// shownText returns the strings drawn by the text-showing operators of f, in
// order.  This is what a viewer would put on the page, as opposed to how the
// drawing code arrived at it.
func shownText(t *testing.T, f *form.Form) []string {
	t.Helper()

	var out []string
	add := func(o pdf.Object) {
		if s, ok := o.(pdf.String); ok {
			out = append(out, string(s))
		}
	}

	iter := f.Content.NewIter()
	for op, args := range iter.All() {
		switch op {
		case content.OpTextShow, content.OpTextShowMoveNextLine,
			content.OpTextShowMoveNextLineSetSpacing:
			if len(args) > 0 {
				add(args[len(args)-1])
			}
		case content.OpTextShowArray:
			if len(args) > 0 {
				if a, ok := args[len(args)-1].(pdf.Array); ok {
					for _, o := range a {
						add(o)
					}
				}
			}
		}
	}
	if err := iter.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// A check box needs an appearance for each of its two states, so a viewer can
// show the box ticked or empty without asking for a new one.  The states are
// named by the field's on-state and "Off", the names a check box's value
// selects between.
func TestCheckBoxHasBothStates(t *testing.T) {
	w := toggleWidget(t, "Yes", "Yes", false)
	if err := newGen(t, pdf.V2_0).AddAppearance(w); err != nil {
		t.Fatal(err)
	}

	ap := w.Appearance
	if ap.Normal != nil {
		t.Error("a check box must not have a single stateless appearance")
	}
	got := slices.Sorted(maps.Keys(ap.NormalMap))
	want := []pdf.Name{"Off", "Yes"}
	if !slices.Equal(got, want) {
		t.Errorf("appearance states = %v, want %v", got, want)
	}
}

// The appearance a viewer shows first follows the field's value: a box whose
// value is its on-state starts ticked, and any other value starts empty.
func TestCheckBoxStateFollowsValue(t *testing.T) {
	for _, tc := range []struct {
		value pdf.Name
		want  pdf.Name
	}{
		{"Yes", "Yes"},
		{"Off", "Off"},
		{"", "Off"},
	} {
		t.Run(string(tc.value), func(t *testing.T) {
			w := toggleWidget(t, tc.value, "Yes", false)
			if err := newGen(t, pdf.V2_0).AddAppearance(w); err != nil {
				t.Fatal(err)
			}
			if got := w.AppearanceState; got != tc.want {
				t.Errorf("appearance state = %q, want %q", got, tc.want)
			}
		})
	}
}

// The on state shows a marker glyph and the off state shows an empty box, which
// is what tells a reader of the page whether the box is ticked.
func TestToggleOnShowsMarker(t *testing.T) {
	for _, radio := range []bool{false, true} {
		name := "checkbox"
		if radio {
			name = "radio"
		}
		t.Run(name, func(t *testing.T) {
			w := toggleWidget(t, "Yes", "Yes", radio)
			if err := newGen(t, pdf.V2_0).AddAppearance(w); err != nil {
				t.Fatal(err)
			}

			onState := w.AppearanceState
			if onState == "Off" {
				t.Fatalf("expected the widget to start in its on state")
			}
			if got := shownText(t, w.Appearance.NormalMap[onState]); len(got) != 1 {
				t.Errorf("on state shows %v, want exactly one marker", got)
			}
			if got := shownText(t, w.Appearance.NormalMap["Off"]); len(got) != 0 {
				t.Errorf("off state shows %v, want nothing", got)
			}
		})
	}
}

// The MK.CA characteristic names the marker in the ZapfDingbats encoding, so
// different characteristics must put different glyphs on the page and equal
// ones the same glyph.  A check box and a radio button which name no marker
// fall back on different defaults, a tick and a dot.
func TestToggleMarkerReachesThePage(t *testing.T) {
	marker := func(t *testing.T, caption pdf.Name, radio bool) string {
		t.Helper()
		w := toggleWidget(t, "Yes", "Yes", radio)
		if caption != "" {
			w.Style = &appearance.Characteristics{Caption: string(caption)}
		}
		if err := newGen(t, pdf.V2_0).AddAppearance(w); err != nil {
			t.Fatal(err)
		}
		shown := shownText(t, w.Appearance.NormalMap[w.AppearanceState])
		if len(shown) != 1 {
			t.Fatalf("on state shows %v, want exactly one marker", shown)
		}
		return shown[0]
	}

	tick := marker(t, "4", false)
	cross := marker(t, "8", false)
	star := marker(t, "H", false)
	if tick == cross || tick == star || cross == star {
		t.Errorf("distinct markers drew the same glyph: %q %q %q", tick, cross, star)
	}
	if again := marker(t, "4", false); again != tick {
		t.Errorf("the same marker drew %q and %q", tick, again)
	}

	// the defaults: a check box ticks, a radio button fills its dot
	if got := marker(t, "", false); got != tick {
		t.Errorf("a check box with no MK.CA drew %q, want the tick %q", got, tick)
	}
	if got := marker(t, "", true); got == tick {
		t.Error("a radio button with no MK.CA drew the check box's tick")
	}
}

// dingbatText carries the MK.CA convention, under which a marker is given as
// codes in the ZapfDingbats encoding rather than as the characters themselves.
func TestDingbatText(t *testing.T) {
	for _, tc := range []struct {
		marker string
		want   string
	}{
		{"4", "✔"}, // check mark
		{"5", "✕"}, // multiplication x
		{"8", "✘"}, // heavy ballot x
		{"H", "★"}, // black star
		{"l", "●"}, // black circle
		{"n", "■"}, // black square
		{"48", "✔✘"},
		{"", ""},
		{"\x00", ""}, // unencoded, so nothing to draw
	} {
		if got := dingbatText(tc.marker); got != tc.want {
			t.Errorf("dingbatText(%q) = %q, want %q", tc.marker, got, tc.want)
		}
	}
}

// A marker naming no glyph leaves the box empty rather than failing: the
// characteristic comes from the file and may say anything.
func TestToggleUnknownMarkerDrawsNothing(t *testing.T) {
	w := toggleWidget(t, "Yes", "Yes", false)
	w.Style = &appearance.Characteristics{Caption: "\x00"}
	if err := newGen(t, pdf.V2_0).AddAppearance(w); err != nil {
		t.Fatal(err)
	}

	if got := len(w.Appearance.NormalMap); got != 2 {
		t.Fatalf("appearance states = %d, want 2", got)
	}
	if got := shownText(t, w.Appearance.NormalMap[w.AppearanceState]); len(got) != 0 {
		t.Errorf("on state shows %v, want nothing", got)
	}
}

// A radio button is round where a check box is square, so the two draw
// different chrome even when everything else about them agrees.
func TestRadioChromeDiffersFromCheckBox(t *testing.T) {
	style := func() *appearance.Characteristics {
		return &appearance.Characteristics{
			BackgroundColor: color.DeviceGray(0.9),
			BorderColor:     color.DeviceGray(0.2),
		}
	}

	off := func(radio bool) *form.Form {
		w := toggleWidget(t, "Off", "Yes", radio)
		w.Style = style()
		w.BorderStyle = &annotation.BorderStyle{Width: 1}
		if err := newGen(t, pdf.V2_0).AddAppearance(w); err != nil {
			t.Fatal(err)
		}
		return w.Appearance.NormalMap["Off"]
	}

	// the off state carries chrome only, so any difference is the chrome
	if off(false).Content.(*content.Operators).Equal(off(true).Content.(*content.Operators)) {
		t.Error("a radio button and a check box drew the same chrome")
	}
}
