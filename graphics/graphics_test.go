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

package graphics_test

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/scanner"
	"seehuhn.de/go/pdf/internal/dummyfont"
)

func TestParameters(t *testing.T) {
	buf := &bytes.Buffer{}
	w := graphics.NewWriter(buf, pdf.V1_7)
	w.Set = 0

	data := pdf.NewData(pdf.V1_7)
	font := dummyfont.Embed(data)

	w.SetLineWidth(12.3)
	w.SetLineCap(graphics.LineCapRound)
	w.SetLineJoin(graphics.LineJoinBevel)
	w.SetMiterLimit(4)
	w.SetDashPattern([]float64{5, 6, 7}, 8)
	w.SetRenderingIntent(graphics.RenderingIntentPerceptual)
	w.SetFlatnessTolerance(10)
	m := graphics.Matrix{1, 2, 3, 4, 5, 6}
	w.Transform(m)
	w.TextSetCharacterSpacing(9)
	w.TextSetWordSpacing(10)
	w.TextSetHorizontalScaling(11)
	w.TextSetLeading(12)
	w.TextSetFont(font, 14)
	w.TextSetRenderingMode(graphics.TextRenderingModeFillStrokeClip)
	w.TextSetRise(15)

	r := &graphics.Reader{
		R:         data,
		Resources: w.Resources,
		State:     graphics.NewState(),
	}
	r.Set = 0
	s := scanner.NewScanner()
	iter := s.Scan(bytes.NewReader(buf.Bytes()))
	iter(func(op string, args []pdf.Object) bool {
		err := r.UpdateState(op, args)
		if err != nil {
			t.Fatal(err)
		}
		return true
	})

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
	if r.State.RenderingIntent != graphics.RenderingIntentPerceptual {
		t.Errorf("RenderingIntent: got %v, want %v", r.State.RenderingIntent, graphics.RenderingIntentPerceptual)
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
	if r.State.TextHorizonalScaling != 11 {
		t.Errorf("Th: got %v, want 11", r.State.TextHorizonalScaling)
	}
	if r.State.TextLeading != 12 {
		t.Errorf("Tl: got %v, want 12", r.State.TextLeading)
	}
	if !resEqual(r.State.TextFont, font) || r.State.TextFontSize != 14 { // TODO(voss)
		t.Errorf("Font: got %v, %v, want %v, 14", r.State.TextFont, r.State.TextFontSize, font)
	}
	if r.State.TextRenderingMode != graphics.TextRenderingModeFillStrokeClip {
		t.Errorf("TextRenderingMode: got %v, want %v", r.State.TextRenderingMode, graphics.TextRenderingModeFillStrokeClip)
	}
	if r.State.TextRise != 15 {
		t.Errorf("Tr: got %v, want 15", r.State.TextRise)
	}

	if d := cmp.Diff(w.State, r.State); d != "" {
		t.Errorf("State: %s", d)
	}
}

func resEqual(a, b graphics.Resource) bool {
	return a.DefaultName() == b.DefaultName() && a.PDFObject() == b.PDFObject()
}
