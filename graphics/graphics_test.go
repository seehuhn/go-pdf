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
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/scanner"
)

func TestParameters(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf, pdf.V1_7)
	w.SetLineWidth(12.3)
	w.SetLineCap(LineCapRound)
	w.SetLineJoin(LineJoinBevel)
	w.SetMiterLimit(4)
	w.SetDashPattern([]float64{5, 6, 7}, 8)
	w.SetRenderingIntent(RenderingIntentPerceptual)
	w.SetFlatness(10)
	m := Matrix{1, 2, 3, 4, 5, 6}
	w.Transform(m)
	w.SetCharSpacing(9)

	r := &Reader{
		R:         nil,
		Resources: w.Resources,
		State:     NewState(),
	}
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
	if r.State.Tc != 9 {
		t.Errorf("Tc: got %v, want 9", r.State.Tc)
	}

	if d := cmp.Diff(w.State, r.State); d != "" {
		t.Errorf("State: %s", d)
	}
}
