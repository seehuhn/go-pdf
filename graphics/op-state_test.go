// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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
	"seehuhn.de/go/pdf/graphics/matrix"
	"seehuhn.de/go/pdf/internal/debug/tempfile"
)

func TestPushPop(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		t.Run(v.String(), func(t *testing.T) {
			data, _ := tempfile.NewTempWriter(v, nil)
			rm := pdf.NewResourceManager(data)

			buf := &bytes.Buffer{}
			w := NewWriter(buf, rm)

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
		})
	}
}

func TestPushPopErr1(t *testing.T) {
	data, _ := tempfile.NewTempWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(data)

	buf := &bytes.Buffer{}
	w := NewWriter(buf, rm)

	w.PushGraphicsState()
	w.PopGraphicsState()
	w.PopGraphicsState()

	if w.Err == nil {
		t.Fatal("expected error, but got nil")
	}
}

func TestPushPopErr2(t *testing.T) {
	data, _ := tempfile.NewTempWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(data)

	buf := &bytes.Buffer{}
	w := NewWriter(buf, rm)

	w.TextBegin()
	w.PushGraphicsState()
	w.TextEnd()
	w.PopGraphicsState()

	if w.Err == nil {
		t.Fatal("expected error, but got nil")
	}
}

func TestPushPopInText(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		t.Run(v.String(), func(t *testing.T) {
			data, _ := tempfile.NewTempWriter(v, nil)
			rm := pdf.NewResourceManager(data)

			buf := &bytes.Buffer{}
			w := NewWriter(buf, rm)

			w.TextBegin()
			w.PushGraphicsState()
			w.PopGraphicsState()
			w.TextEnd()

			if v <= pdf.V1_7 {
				if w.Err == nil {
					t.Fatal("expected error, but got nil")
				}
			} else {
				if w.Err != nil {
					t.Fatal(w.Err)
				}
			}
		})
	}
}

func TestWriterCTM(t *testing.T) {
	data, _ := tempfile.NewTempWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(data)

	buf := &bytes.Buffer{}
	w := NewWriter(buf, rm)

	w.Transform(matrix.Rotate(math.Pi / 2)) // rotate 90 degrees counter-clockwise
	w.Transform(matrix.Translate(10, 20))
	w.Transform(matrix.Rotate(math.Pi / 7))

	x, y := w.CTM[4], w.CTM[5]
	if !nearlyEqual(x, -20) || !nearlyEqual(y, 10) {
		t.Errorf("CurrentGraphicsPosition: got %v, %v, want -20, 10", x, y)
	}
}
