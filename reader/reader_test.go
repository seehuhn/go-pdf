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

	"seehuhn.de/go/sfnt/cff"

	"seehuhn.de/go/pdf"
	pdffont "seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/matrix"
	"seehuhn.de/go/pdf/internal/dummyfont"
	"seehuhn.de/go/pdf/reader/scanner"
)

func TestParameters(t *testing.T) {
	data := pdf.NewData(pdf.V1_7)
	rm := pdf.NewResourceManager(data)

	buf := &bytes.Buffer{}
	w := graphics.NewWriter(buf, rm)
	w.Set = 0

	font := dummyfont.Embed(data)

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
	w.TextSetFont(font, 14)
	w.TextSetRenderingMode(graphics.TextRenderingModeFillStrokeClip)
	w.TextSetRise(15)

	r := New(data, nil)
	r.Resources = w.Resources
	r.State = graphics.NewState()
	r.Set = 0
	s := scanner.NewScanner()
	s.SetInput(bytes.NewReader(buf.Bytes()))
	for s.Scan() {
		op := s.Operator()
		err := r.do(op)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Error(); err != nil {
		t.Fatal(err)
	}

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
	if !resEqual(r.State.TextFont, font) || r.State.TextFontSize != 14 { // TODO(voss)
		t.Errorf("Font: got %v, %v, want %v, 14", r.State.TextFont, r.State.TextFontSize, font)
	}
	if r.State.TextRenderingMode != graphics.TextRenderingModeFillStrokeClip {
		t.Errorf("TextRenderingMode: got %v, want %v", r.State.TextRenderingMode, graphics.TextRenderingModeFillStrokeClip)
	}
	if r.State.TextRise != 15 {
		t.Errorf("Tr: got %v, want 15", r.State.TextRise)
	}

	cmpFDSelectFn := cmp.Comparer(func(fn1, fn2 cff.FDSelectFn) bool {
		return true
	})
	cmpFont := cmp.Comparer(func(f1, f2 pdffont.Embedded) bool {
		if f1.PDFObject() != f2.PDFObject() {
			return false
		}
		if f1.WritingMode() != f2.WritingMode() {
			return false
		}
		// TODO(voss): add more checks?
		return true
	})

	if d := cmp.Diff(w.State, r.State, cmpFDSelectFn, cmpFont); d != "" {
		t.Errorf("State: %s", d)
	}
}

func resEqual(a, b pdf.Resource) bool {
	return a.PDFObject() == b.PDFObject()
}
