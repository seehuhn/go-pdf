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
