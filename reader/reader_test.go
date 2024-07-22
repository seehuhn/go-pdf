// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/matrix"
	"seehuhn.de/go/pdf/internal/dummyfont"
)

// TestParameters verifies that the graphics state is correctly updated by the
// reader.
func TestParameters(t *testing.T) {
	data := pdf.NewData(pdf.V1_7)
	rm := pdf.NewResourceManager(data)

	// We start by creating a content stream where we set various graphics
	// parameters.
	buf := &bytes.Buffer{}
	w := graphics.NewWriter(buf, rm)
	w.Set = 0

	testFont := dummyfont.Must()

	w.SetLineWidth(12.3)
	w.SetLineCap(graphics.LineCapRound)
	w.SetLineJoin(graphics.LineJoinBevel)
	w.SetMiterLimit(4)
	w.SetLineDash([]float64{5, 6, 7}, 8)
	w.SetRenderingIntent(graphics.Perceptual)
	w.SetFlatnessTolerance(10)
	m := matrix.Matrix{1, 2, 3, 4, 5, 6}
	w.Transform(m)
	w.TextSetCharacterSpacing(9)
	w.TextSetWordSpacing(10)
	w.TextSetHorizontalScaling(11)
	w.TextSetLeading(12)
	w.TextSetFont(testFont, 14)
	w.TextSetRenderingMode(graphics.TextRenderingModeFillStrokeClip)
	w.TextSetRise(15)

	err := rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Now we read back the content stream and check that the final graphics
	// state matches the expected values.
	r := New(data, nil)
	r.Reset()
	r.Resources = w.Resources
	r.State.Set = 0 // TODO(voss): why do we need this?
	err = r.ParseContentStream(buf)
	if err != nil {
		t.Fatal(err)
	}

	fontsEqual := func(a, b font.Font) bool {
		if a == nil || b == nil {
			return a == b
		}
		// TODO(voss): update this once we have a way of comparing a loaded
		// font to an original font.  Maybe we can use the font name?
		return true
	}

	// First check: the individual parameters are as we set them.
	if r.State.LineWidth != 12.3 {
		t.Errorf("LineWidth: got %v, want 12.3", r.State.LineWidth)
	}
	if r.State.LineCap != graphics.LineCapRound {
		t.Errorf("LineCap: got %v, want %v", r.State.LineCap, graphics.LineCapRound)
	}
	if r.State.LineJoin != graphics.LineJoinBevel {
		t.Errorf("LineJoin: got %v, want %v", r.State.LineJoin, graphics.LineJoinBevel)
	}
	if r.State.MiterLimit != 4 {
		t.Errorf("MiterLimit: got %v, want 4", r.State.MiterLimit)
	}
	if d := cmp.Diff(r.State.DashPattern, []float64{5, 6, 7}); d != "" {
		t.Errorf("DashPattern: %s", d)
	}
	if r.State.DashPhase != 8 {
		t.Errorf("DashPhase: got %v, want 8", r.State.DashPhase)
	}
	if r.State.RenderingIntent != graphics.Perceptual {
		t.Errorf("RenderingIntent: got %v, want %v", r.State.RenderingIntent, graphics.Perceptual)
	}
	if r.State.FlatnessTolerance != 10 {
		t.Errorf("Flatness: got %v, want 10", r.State.FlatnessTolerance)
	}
	if r.State.CTM != m {
		t.Errorf("CTM: got %v, want %v", r.State.CTM, m)
	}
	if r.State.TextCharacterSpacing != 9 {
		t.Errorf("Tc: got %v, want 9", r.State.TextCharacterSpacing)
	}
	if r.State.TextWordSpacing != 10 {
		t.Errorf("Tw: got %v, want 10", r.State.TextWordSpacing)
	}
	if r.State.TextHorizontalScaling != 11 {
		t.Errorf("Th: got %v, want 11", r.State.TextHorizontalScaling)
	}
	if r.State.TextLeading != 12 {
		t.Errorf("Tl: got %v, want 12", r.State.TextLeading)
	}
	if !fontsEqual(r.State.TextFont, testFont) || r.State.TextFontSize != 14 {
		t.Errorf("Font: got %v, %v, want %v, 14", r.State.TextFont, r.State.TextFontSize, testFont)
	}
	if r.State.TextRenderingMode != graphics.TextRenderingModeFillStrokeClip {
		t.Errorf("TextRenderingMode: got %v, want %v", r.State.TextRenderingMode, graphics.TextRenderingModeFillStrokeClip)
	}
	if r.State.TextRise != 15 {
		t.Errorf("Tr: got %v, want 15", r.State.TextRise)
	}

	for b := graphics.StateBits(1); b != 0; b <<= 1 {
		if w.State.Set&b != r.State.Set&b {
			if w.State.Set&b != 0 {
				t.Errorf("State bit %s only set in writer", b.Names())
			} else {
				t.Errorf("State bit %s only set in reader", b.Names())
			}
		}
	}

	// Second check: the final graphics states are the same.
	// This checks that no parameters different from the ones we explicitly used
	// were changed.
	cmpFont := cmp.Comparer(fontsEqual)
	if d := cmp.Diff(w.State, r.State, cmpFont); d != "" {
		t.Errorf("State: %s", d)
	}
}

func resEqual(a, b pdf.Resource) bool {
	return a.PDFObject() == b.PDFObject()
}
