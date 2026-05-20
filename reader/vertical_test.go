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

package reader

import (
	"iter"
	"math"
	"testing"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/graphics/content"
)

// fakeFont is a minimal font.Instance for testing.  Each byte of input is
// treated as one character; widths are taken from the maps below.  When the
// vertical flag is true, the font reports vertical writing mode and yields
// the configured VerticalAdvance.
type fakeFont struct {
	vertical bool
	hWidth   float64 // horizontal width in 1000 units per em
	vAdvance float64 // vertical advance in 1000 units per em (signed)
}

func (f *fakeFont) PostScriptName() string                     { return "Test" }
func (f *fakeFont) Codec() *charcode.Codec                     { return nil }
func (f *fakeFont) FontInfo() any                              { return nil }
func (f *fakeFont) ResourceName() pdf.Name                     { return "" }
func (f *fakeFont) Embed(*pdf.EmbedHelper) (pdf.Native, error) { return nil, nil }

func (f *fakeFont) WritingMode() font.WritingMode {
	if f.vertical {
		return font.Vertical
	}
	return font.Horizontal
}

func (f *fakeFont) Codes(s pdf.String) iter.Seq[font.Code] {
	return func(yield func(font.Code) bool) {
		for range s {
			c := font.Code{Width: f.hWidth / 1000, Text: "x"}
			if f.vertical {
				c.VerticalAdvance = f.vAdvance / 1000
			}
			if !yield(c) {
				return
			}
		}
	}
}

// withTextState configures r.State.GState as if an enclosing BT/Tf had run
// with the given font and font size.  Sets TextMatrix to identity at the
// given (x, y) origin.
func withTextState(r *Reader, f font.Instance, fontSize, x, y float64) {
	r.State = content.NewState(content.Page, &content.Resources{})
	r.State.GState.TextFont = f
	r.State.GState.TextFontSize = fontSize
	r.State.GState.TextMatrix = matrix.Translate(x, y)
	r.State.GState.TextHorizontalScaling = 1
	r.State.GState.CTM = matrix.Identity
	r.prevEndValid = false
}

// TestVerticalAdvanceDirection verifies that the text matrix advances
// downward (negative y) for vertical writing mode, using the per-glyph
// VerticalAdvance from font.Code.
func TestVerticalAdvanceDirection(t *testing.T) {
	r := New(nil)
	f := &fakeFont{vertical: true, hWidth: 500, vAdvance: -1000}
	withTextState(r, f, 10, 100, 700)

	if err := r.processText(pdf.String{0x01}); err != nil {
		t.Fatal(err)
	}

	// translate(0, advance) where advance = -1 * 10 = -10
	wantTM := matrix.Translate(0, -10).Mul(matrix.Translate(100, 700))
	if r.State.GState.TextMatrix != wantTM {
		t.Errorf("TextMatrix after one vertical glyph = %v, want %v",
			r.State.GState.TextMatrix, wantTM)
	}
}

// TestVerticalAdvanceFallback verifies that a font with no VerticalAdvance
// (zero value) falls back to the spec default of one em downward.
func TestVerticalAdvanceFallback(t *testing.T) {
	r := New(nil)
	f := &fakeFont{vertical: true, hWidth: 500, vAdvance: 0}
	withTextState(r, f, 12, 0, 0)

	if err := r.processText(pdf.String{0x01}); err != nil {
		t.Fatal(err)
	}

	wantTM := matrix.Translate(0, -12)
	if r.State.GState.TextMatrix != wantTM {
		t.Errorf("TextMatrix = %v, want %v (1 em downward)",
			r.State.GState.TextMatrix, wantTM)
	}
}

// TestHorizontalAdvanceDirection is a control case: verifies horizontal
// advance is along +x using info.Width.
func TestHorizontalAdvanceDirection(t *testing.T) {
	r := New(nil)
	f := &fakeFont{vertical: false, hWidth: 500}
	withTextState(r, f, 10, 100, 700)

	if err := r.processText(pdf.String{0x01}); err != nil {
		t.Fatal(err)
	}

	wantTM := matrix.Translate(0.5*10, 0).Mul(matrix.Translate(100, 700))
	if r.State.GState.TextMatrix != wantTM {
		t.Errorf("TextMatrix after one horizontal glyph = %v, want %v",
			r.State.GState.TextMatrix, wantTM)
	}
}

// TestVerticalGapClassifier verifies that the gap classifier swaps axes
// for vertical writing mode.
func TestVerticalGapClassifier(t *testing.T) {
	type event struct {
		kind TextEvent
		arg  float64
	}

	cases := []struct {
		name string
		// jumpTo translates the text matrix to the given absolute
		// position before painting the second glyph (simulating a Tm
		// or BT/ET reset between two text-shows).
		jumpToX, jumpToY float64
		want             []TextEvent
	}{
		{
			name:    "continuous (no jump)",
			jumpToX: 100, jumpToY: 700 - 12, // exactly where prev ended
			want: nil, // dx=0, dy=0 → no event
		},
		{
			name:    "extra gap along writing direction",
			jumpToX: 100, jumpToY: 700 - 12 - 6, // 6 pt extra below
			want: []TextEvent{TextEventSpace},
		},
		{
			name:    "different column",
			jumpToX: 130, jumpToY: 700 - 12, // x change, y continues
			want: []TextEvent{TextEventNL},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := New(nil)
			f := &fakeFont{vertical: true, hWidth: 1000, vAdvance: -1000}
			withTextState(r, f, 12, 100, 700)

			var got []event
			r.TextEvent = func(e TextEvent, arg float64) {
				got = append(got, event{e, arg})
			}

			// First glyph at (100, 700); afterwards prevEnd = (100, 688).
			if err := r.processText(pdf.String{0x01}); err != nil {
				t.Fatal(err)
			}

			// Jump to the configured position and paint the second glyph.
			r.State.GState.TextMatrix = matrix.Translate(tc.jumpToX, tc.jumpToY)
			if err := r.processText(pdf.String{0x01}); err != nil {
				t.Fatal(err)
			}

			if len(got) != len(tc.want) {
				t.Fatalf("got %d events, want %d (%v)", len(got), len(tc.want), got)
			}
			for i, want := range tc.want {
				if got[i].kind != want {
					t.Errorf("event %d: got %v, want %v", i, got[i].kind, want)
				}
			}
		})
	}
}

// TestHorizontalGapClassifier is a control case mirroring
// TestVerticalGapClassifier for horizontal writing mode.
func TestHorizontalGapClassifier(t *testing.T) {
	type event struct {
		kind TextEvent
		arg  float64
	}

	// Horizontal advance: width 1000/1000 * 12 = 12 pt to the right.
	cases := []struct {
		name             string
		jumpToX, jumpToY float64
		want             []TextEvent
	}{
		{
			name:    "continuous",
			jumpToX: 100 + 12, jumpToY: 700,
			want: nil,
		},
		{
			name:    "horizontal gap",
			jumpToX: 100 + 12 + 8, jumpToY: 700,
			want: []TextEvent{TextEventSpace},
		},
		{
			name:    "different line",
			jumpToX: 100, jumpToY: 700 - 14,
			want: []TextEvent{TextEventNL},
		},
		{
			// A small backward step (well under one em) is in the
			// range of pair kerning and must not trigger an event.
			name:    "backward kerning",
			jumpToX: 100 + 12 - 3, jumpToY: 700,
			want: nil,
		},
		{
			// A backward jump of more than one em on the same
			// baseline indicates a new region.
			name:    "backward jump",
			jumpToX: 50, jumpToY: 700,
			want: []TextEvent{TextEventNL},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := New(nil)
			f := &fakeFont{vertical: false, hWidth: 1000}
			withTextState(r, f, 12, 100, 700)

			var got []event
			r.TextEvent = func(e TextEvent, arg float64) {
				got = append(got, event{e, arg})
			}

			if err := r.processText(pdf.String{0x01}); err != nil {
				t.Fatal(err)
			}
			r.State.GState.TextMatrix = matrix.Translate(tc.jumpToX, tc.jumpToY)
			if err := r.processText(pdf.String{0x01}); err != nil {
				t.Fatal(err)
			}

			if len(got) != len(tc.want) {
				t.Fatalf("got %d events, want %d (%v)", len(got), len(tc.want), got)
			}
			for i, want := range tc.want {
				if got[i].kind != want {
					t.Errorf("event %d: got %v, want %v", i, got[i].kind, want)
				}
			}
		})
	}
}

// TestVerticalCharacterSpacing verifies that TextCharacterSpacing is added
// to the signed vertical displacement, per PDF 2.0 §9.4.4.  In vertical mode
// w1 is typically negative; positive Tc therefore reduces the magnitude of
// the downward advance.
func TestVerticalCharacterSpacing(t *testing.T) {
	r := New(nil)
	f := &fakeFont{vertical: true, hWidth: 500, vAdvance: -1000}
	withTextState(r, f, 10, 0, 0)
	r.State.GState.TextCharacterSpacing = 2

	if err := r.processText(pdf.String{0x01}); err != nil {
		t.Fatal(err)
	}

	// expected advance: vAdv*fontSize + charSpacing = -10 + 2 = -8
	wantTM := matrix.Translate(0, -8)
	if !approxEqual(r.State.GState.TextMatrix, wantTM, 1e-9) {
		t.Errorf("TextMatrix = %v, want %v", r.State.GState.TextMatrix, wantTM)
	}
}

func approxEqual(a, b matrix.Matrix, eps float64) bool {
	for i := range a {
		if math.Abs(a[i]-b[i]) > eps {
			return false
		}
	}
	return true
}
