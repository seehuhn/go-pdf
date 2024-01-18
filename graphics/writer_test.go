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

package graphics

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/scanner"
	"seehuhn.de/go/pdf/internal/dummyfont"
)

func TestPushPop(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf, pdf.V1_7)
	w.SetLineWidth(2)
	w.PushGraphicsState()
	w.SetLineWidth(3)
	w.PopGraphicsState()
	if w.Err != nil {
		t.Fatal(w.Err)
	}
	if w.LineWidth != 2 {
		t.Errorf("LineWidth: got %v, want 2", w.LineWidth)
	}
	commands := strings.Fields(buf.String())
	expected := []string{"2", "w", "q", "3", "w", "Q"}
	if d := cmp.Diff(commands, expected); d != "" {
		t.Errorf("commands: %s", d)
	}
}

func TestPushPopErr(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf, pdf.V1_7)
	w.PushGraphicsState()
	w.PopGraphicsState()
	w.PopGraphicsState()
	if w.Err == nil {
		t.Fatal("expected error")
	}
}

func TestWriterCTM(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf, pdf.V1_7)
	w.Transform(Rotate(math.Pi / 2))
	w.Transform(Translate(10, 20))
	w.Transform(Rotate(math.Pi / 7))
	x, y := w.CTM[4], w.CTM[5]
	if !nearlyEqual(x, -20) || !nearlyEqual(y, 10) {
		t.Errorf("CurrentGraphicsPosition: got %v, %v, want -20, 10", x, y)
	}
}

func TestParameters(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf, pdf.V1_7)
	w.Set = 0

	data := pdf.NewData(pdf.V1_7)
	font := dummyfont.Embed(data, "dummy")

	w.SetLineWidth(12.3)
	w.SetLineCap(LineCapRound)
	w.SetLineJoin(LineJoinBevel)
	w.SetMiterLimit(4)
	w.SetDashPattern([]float64{5, 6, 7}, 8)
	w.SetRenderingIntent(RenderingIntentPerceptual)
	w.SetFlatnessTolerance(10)
	m := Matrix{1, 2, 3, 4, 5, 6}
	w.Transform(m)
	w.TextSetCharacterSpacing(9)
	w.TextSetWordSpacing(10)
	w.TextSetHorizontalScaling(1100)
	w.TextSetLeading(12)
	w.TextSetFont(font, 14)
	w.TextSetRenderingMode(TextRenderingModeFillStrokeClip)
	w.TextSetRise(15)

	r := &Reader{
		R:         data,
		Resources: w.Resources,
		State:     NewState(),
	}
	r.Set = 0
	s := scanner.NewScanner()
	iter := s.Scan(bytes.NewReader(buf.Bytes()))
	err := iter(func(op string, args []pdf.Object) error {
		err := r.do(op, args)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if r.State.LineWidth != 12.3 {
		t.Errorf("LineWidth: got %v, want 12.3", r.State.LineWidth)
	}
	if r.State.LineCap != LineCapRound {
		t.Errorf("LineCap: got %v, want %v", r.State.LineCap, LineCapRound)
	}
	if r.State.LineJoin != LineJoinBevel {
		t.Errorf("LineJoin: got %v, want %v", r.State.LineJoin, LineJoinBevel)
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
	if r.State.RenderingIntent != RenderingIntentPerceptual {
		t.Errorf("RenderingIntent: got %v, want %v", r.State.RenderingIntent, RenderingIntentPerceptual)
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
	if r.State.TextRenderingMode != TextRenderingModeFillStrokeClip {
		t.Errorf("TextRenderingMode: got %v, want %v", r.State.TextRenderingMode, TextRenderingModeFillStrokeClip)
	}
	if r.State.TextRise != 15 {
		t.Errorf("Tr: got %v, want 15", r.State.TextRise)
	}

	if d := cmp.Diff(w.State, r.State); d != "" {
		t.Errorf("State: %s", d)
	}
}

func resEqual(a, b Resource) bool {
	return a.DefaultName() == b.DefaultName() && a.PDFObject() == b.PDFObject()
}
