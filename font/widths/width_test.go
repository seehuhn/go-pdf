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

package widths_test

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/internal/debug/tempfile"
	"seehuhn.de/go/pdf/internal/makefont"
	"seehuhn.de/go/pdf/reader"
)

func TestWidthsFull(t *testing.T) {
	data, _ := tempfile.NewTempWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(data)

	// TODO(voss): iterate over all font types

	otf := makefont.OpenType()

	F, err := cff.New(otf, nil)
	if err != nil {
		t.Fatal(err)
	}
	ref, E, err := pdf.ResourceManagerEmbed(rm, F)
	if err != nil {
		t.Fatal(err)
	}

	sampleText := "Hello World!"

	// Layout and encode a string to make sure the corresponding glyphs are
	// included in the embedded font.
	gg := F.Layout(nil, 10, sampleText)
	var s pdf.String
	var ww []float64
	for _, g := range gg.Seq {
		ww = append(ww, otf.GlyphWidthPDF(g.GID))
		s, _ = E.(font.EmbeddedLayouter).AppendEncoded(s, g.GID, g.Text)
	}
	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	qqq := reader.New(data, nil)
	D, err := qqq.ReadFont(ref)
	if err != nil {
		t.Fatal(err)
	}

	i := 0
	for len(s) > 0 {
		info, k := D.Decode(s)

		wFromFont := ww[i]
		wFromPDF := info.W
		if wFromPDF != wFromFont {
			t.Errorf("widths differ for code 0x% 02x: %f vs %f", s[:k], wFromPDF, wFromFont)
		}

		s = s[k:]
		i++
	}
}
