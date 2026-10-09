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

package dict

import (
	"testing"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// A resource dictionary shared between glyphs and the font is written to the
// file only once.
func TestType3SharedResources(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	res := &content.Resources{}
	glyph := func() *CharProc {
		return &CharProc{
			Content: &content.Operators{Ops: []content.Operator{
				{Name: content.OpType3ColoredGlyph, Args: []pdf.Object{pdf.Integer(500), pdf.Integer(0)}},
			}},
			Resources: res,
		}
	}
	d := &Type3{
		Descriptor: &font.Descriptor{},
		Encoding:   encodeAt(0x41, 0x42),
		CharProcs:  map[pdf.Name]*CharProc{"A": glyph(), "B": glyph()},
		FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
		Resources:  res,
	}
	d.Width[0x41] = 500
	d.Width[0x42] = 500

	fontObj, err := rm.Embed(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	resolve := func(obj pdf.Object) pdf.Native {
		t.Helper()
		native, err := pdf.Resolve(w, obj)
		if err != nil {
			t.Fatal(err)
		}
		return native
	}
	fontDict := resolve(fontObj).(pdf.Dict)
	charProcs := resolve(fontDict["CharProcs"]).(pdf.Dict)
	want := fontDict["Resources"]
	if _, isRef := want.(pdf.Reference); !isRef {
		t.Fatalf("font Resources = %v, want an indirect reference", want)
	}
	for _, name := range []pdf.Name{"A", "B"} {
		stm := resolve(charProcs[name]).(*pdf.Stream)
		if got := stm.Dict["Resources"]; got != want {
			t.Errorf("glyph %s Resources = %v, want %v", name, got, want)
		}
	}
}

// Font-level resources need PDF 1.2, per-glyph resources need PDF 2.0.
func TestType3ResourcesVersion(t *testing.T) {
	for _, tc := range []struct {
		name     string
		version  pdf.Version
		fontRes  bool
		glyphRes bool
		wantErr  bool
	}{
		{"font-1.1", pdf.V1_1, true, false, true},
		{"font-1.2", pdf.V1_2, true, false, false},
		{"glyph-1.7", pdf.V1_7, false, true, true},
		{"glyph-2.0", pdf.V2_0, false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, tc.version, nil)
			rm := pdf.NewResourceManager(w)

			cp := &CharProc{
				Content: &content.Operators{Ops: []content.Operator{
					{Name: content.OpType3ColoredGlyph, Args: []pdf.Object{pdf.Integer(500), pdf.Integer(0)}},
				}},
			}
			d := &Type3{
				Name:       "F",
				Descriptor: &font.Descriptor{},
				Encoding:   encodeAt(0x41),
				CharProcs:  map[pdf.Name]*CharProc{"A": cp},
				FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
			}
			d.Width[0x41] = 500
			if tc.fontRes {
				d.Resources = &content.Resources{}
			}
			if tc.glyphRes {
				cp.Resources = &content.Resources{}
			}

			_, err := rm.Embed(d)
			if gotErr := err != nil; gotErr != tc.wantErr {
				t.Errorf("error = %v, want error: %t", err, tc.wantErr)
			}
		})
	}
}
