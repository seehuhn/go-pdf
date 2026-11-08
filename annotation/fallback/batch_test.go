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

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/form"
)

// batchRadio returns a radio button field with n widgets, every third of
// which names its on-state through an appearance; the others need made-up
// names.
func batchRadio(n int) *acroform.ButtonField {
	f := acroform.NewButtonField("r", acroform.ButtonRadio)
	for i := range n {
		w := annotation.AddWidget(f, pdf.Rectangle{URx: 10, URy: 10})
		if i%3 == 0 {
			name := pdf.Name([]string{"0", "2", "x"}[i/3%3])
			w.Appearance = nil
			setNormalAppearance(w, nil, map[pdf.Name]*form.Form{name: nil, "Off": nil})
		}
	}
	return f
}

// widgetStates returns, for each widget of f, the names in its normal
// appearance map and its appearance state.
func widgetStates(f *acroform.ButtonField) [][]pdf.Name {
	var res [][]pdf.Name
	for _, w := range f.Widgets {
		w := w.(*annotation.Widget)
		names := slices.Sorted(maps.Keys(w.Appearance.NormalMap))
		res = append(res, append(names, w.AppearanceState))
	}
	return res
}

// TestBatchMatchesGenerator checks that a batch gives the widgets of a radio
// button field the same appearance states as the generator does one widget
// at a time.
func TestBatchMatchesGenerator(t *testing.T) {
	gen, err := NewStyle().New(pdf.V2_0)
	if err != nil {
		t.Fatal(err)
	}

	single := batchRadio(12)
	want := single.OnStates()
	for _, w := range single.Widgets {
		if err := gen.AddAppearance(w.(*annotation.Widget)); err != nil {
			t.Fatal(err)
		}
	}

	batched := batchRadio(12)
	b := gen.NewBatch()
	for _, w := range batched.Widgets {
		if err := b.AddAppearance(w.(*annotation.Widget)); err != nil {
			t.Fatal(err)
		}
	}

	if diff := cmp.Diff(widgetStates(single), widgetStates(batched)); diff != "" {
		t.Errorf("batch differs from generator (-single +batch):\n%s", diff)
	}
	// generating the appearances leaves the on-state names unchanged
	if diff := cmp.Diff(want, batched.OnStates()); diff != "" {
		t.Errorf("on-states changed (-before +after):\n%s", diff)
	}
}
