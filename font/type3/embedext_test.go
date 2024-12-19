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

package type3_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestRoundTrip(t *testing.T) {
	rw, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	glyphs := make(map[string]pdf.Reference)
	glyphs["A"] = rw.Alloc()
	glyphs["C"] = rw.Alloc()

	encoding := make([]string, 256)
	encoding[65] = "A"
	encoding[66] = "C"

	widths := make([]float64, 256)
	widths[65] = 100
	widths[66] = 200

	resources := &pdf.Resources{
		Font: map[pdf.Name]pdf.Object{
			"F": rw.Alloc(),
		},
	}

	m := map[charcode.CharCode][]rune{
		65: {'A'},
		66: {'C'},
	}
	toUnicode := cmap.NewToUnicode(charcode.Simple, m)

	info1 := &type3.EmbedInfo{
		Glyphs:       glyphs,
		FontMatrix:   [6]float64{0.001, 0, 0, 0.001, 0, 0},
		Encoding:     encoding,
		Widths:       widths,
		Resources:    resources,
		ItalicAngle:  10,
		IsFixedPitch: true,
		IsSerif:      false,
		IsScript:     true,
		ForceBold:    false,
		IsAllCap:     true,
		IsSmallCap:   false,
		ToUnicode:    toUnicode,
	}

	ref := rw.Alloc()
	err := info1.Embed(rw, ref)
	if err != nil {
		t.Fatal(err)
	}

	dicts, err := font.ExtractDicts(rw, ref)
	if err != nil {
		t.Fatal(err)
	}
	info2, err := type3.Extract(rw, dicts)
	if err != nil {
		t.Fatal(err)
	}

	if d := cmp.Diff(info1, info2); d != "" {
		t.Errorf("info mismatch (-want +got):\n%s", d)
	}
}
