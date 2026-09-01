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

package extract

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/pattern"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestPatternBBoxFallback checks that a tiling pattern with no usable BBox
// falls back to a cell matching the tiling step, instead of failing to read.
// The step may be negative, so the fallback uses its magnitude.
func TestPatternBBoxFallback(t *testing.T) {
	for _, tc := range []struct {
		name string
		bbox pdf.Object
	}{
		{"missing", nil},
		{"null element", pdf.Array{pdf.Integer(0), pdf.Integer(0), nil, pdf.Integer(5)}},
		{"zero area", pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(0), pdf.Integer(0)}},
		{"wrong length", pdf.Array{pdf.Integer(0), pdf.Integer(0)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

			dict := pdf.Dict{
				"PatternType": pdf.Integer(1),
				"PaintType":   pdf.Integer(1),
				"TilingType":  pdf.Integer(1),
				"XStep":       pdf.Number(-8),
				"YStep":       pdf.Number(12),
				"Resources":   pdf.Dict{},
			}
			if tc.bbox != nil {
				dict["BBox"] = tc.bbox
			}

			ref := w.Alloc()
			stm, err := w.OpenStream(ref, dict)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := stm.Write([]byte("0 0 1 1 re f\n")); err != nil {
				t.Fatal(err)
			}
			if err := stm.Close(); err != nil {
				t.Fatal(err)
			}

			p, err := Pattern(pdf.NewCursor(w), ref, false)
			if err != nil {
				t.Fatal(err)
			}
			tiling, ok := p.(*pattern.Type1)
			if !ok {
				t.Fatalf("expected a tiling pattern, got %T", p)
			}

			want := pdf.Rectangle{URx: 8, URy: 12}
			if tiling.BBox != want {
				t.Errorf("BBox = %v, want %v", tiling.BBox, want)
			}
		})
	}
}

// TestPatternStepFallback checks that a tiling pattern with no usable XStep
// or YStep falls back to the corresponding side of the BBox, instead of
// failing to read.  This is the mirror image of the BBox fallback above.
func TestPatternStepFallback(t *testing.T) {
	bbox := pdf.Array{
		pdf.Integer(10), pdf.Integer(20), pdf.Integer(18), pdf.Integer(50),
	}
	for _, tc := range []struct {
		name         string
		xStep, yStep pdf.Object
		wantX, wantY float64
	}{
		{"both missing", nil, nil, 8, 30},
		{"x missing", nil, pdf.Number(4), 8, 4},
		{"y missing", pdf.Number(4), nil, 4, 30},
		{"both zero", pdf.Number(0), pdf.Number(0), 8, 30},
		{"not a number", pdf.Name("x"), pdf.Name("x"), 8, 30},
		{"both given", pdf.Number(4), pdf.Number(5), 4, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

			dict := pdf.Dict{
				"PatternType": pdf.Integer(1),
				"PaintType":   pdf.Integer(1),
				"TilingType":  pdf.Integer(1),
				"BBox":        bbox,
				"Resources":   pdf.Dict{},
			}
			if tc.xStep != nil {
				dict["XStep"] = tc.xStep
			}
			if tc.yStep != nil {
				dict["YStep"] = tc.yStep
			}

			tiling := writeTilingPattern(t, w, dict)
			if tiling.XStep != tc.wantX || tiling.YStep != tc.wantY {
				t.Errorf("step = (%v, %v), want (%v, %v)",
					tiling.XStep, tiling.YStep, tc.wantX, tc.wantY)
			}
			// the box is usable, so it must be kept as given
			want := pdf.Rectangle{LLx: 10, LLy: 20, URx: 18, URy: 50}
			if tiling.BBox != want {
				t.Errorf("BBox = %v, want %v", tiling.BBox, want)
			}
		})
	}
}

// TestPatternNoCellSize checks that a tiling pattern which gives neither a
// usable BBox nor a usable tiling step is refused: without a cell size it
// would tile forever.
func TestPatternNoCellSize(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	ref := writePatternStream(t, w, pdf.Dict{
		"PatternType": pdf.Integer(1),
		"PaintType":   pdf.Integer(1),
		"TilingType":  pdf.Integer(1),
		"BBox":        pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(0), pdf.Integer(0)},
		"Resources":   pdf.Dict{},
	})

	if _, err := Pattern(pdf.NewCursor(w), ref, false); err == nil {
		t.Error("expected an error, got none")
	}
}

// writeTilingPattern writes dict as a pattern stream and reads it back.
func writeTilingPattern(t *testing.T, w *pdf.Writer, dict pdf.Dict) *pattern.Type1 {
	t.Helper()

	ref := writePatternStream(t, w, dict)
	p, err := Pattern(pdf.NewCursor(w), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	tiling, ok := p.(*pattern.Type1)
	if !ok {
		t.Fatalf("expected a tiling pattern, got %T", p)
	}
	return tiling
}

func writePatternStream(t *testing.T, w *pdf.Writer, dict pdf.Dict) pdf.Reference {
	t.Helper()

	ref := w.Alloc()
	stm, err := w.OpenStream(ref, dict)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stm.Write([]byte("0 0 1 1 re f\n")); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	return ref
}
