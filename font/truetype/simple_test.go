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

package truetype_test

import (
	"math"
	"testing"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestSimpleRoundTrip embeds a TrueType simple font and reads it back from
// the resulting PDF, checking the identifying fields.
func TestSimpleRoundTrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		t.Run(v.String(), func(t *testing.T) {
			fontData := makefont.TrueType()
			d := embedTrueTypeSimple(t, v, fontData)

			if d.PostScriptName != fontData.PostScriptName() {
				t.Errorf("PostScriptName: got %q, want %q",
					d.PostScriptName, fontData.PostScriptName())
			}
			if len(d.SubsetTag) != 6 {
				t.Errorf("SubsetTag: got %q, want 6 uppercase letters", d.SubsetTag)
			}
		})
	}
}

// TestSimpleDescriptor verifies that the PDF font descriptor reports metrics
// in PDF glyph space (1/1000 em) and follows the Ascent-Descent+LineGap
// convention for Leading.  These are the values PDF consumers rely on for
// line spacing and glyph positioning.
func TestSimpleDescriptor(t *testing.T) {
	fontData := makefont.TrueType()
	d := embedTrueTypeSimple(t, pdf.V2_0, fontData)
	checkDescriptorMetrics(t, d.Descriptor, fontData)
}

// TestSimpleLayout verifies that Layout returns positive advances and that
// the advances scale linearly with ptSize.  This pins the contract that the
// returned glyph sequence is in text space (1 em equals 1 unit at ptSize=1).
func TestSimpleLayout(t *testing.T) {
	fontData := makefont.TrueType()
	F, err := truetype.NewSimple(fontData, nil)
	if err != nil {
		t.Fatal(err)
	}

	const ptSize = 12.0
	seq := F.Layout(nil, ptSize, "Hello")
	if len(seq.Seq) == 0 {
		t.Fatal("Layout returned empty glyph sequence")
	}
	for i, g := range seq.Seq {
		if g.Advance <= 0 {
			t.Errorf("glyph %d (GID %d): advance %v, want > 0", i, g.GID, g.Advance)
		}
	}

	seq2 := F.Layout(nil, 2*ptSize, "Hello")
	if got, want := totalAdvance(seq2), 2*totalAdvance(seq); math.Abs(got-want) > 1e-9 {
		t.Errorf("total advance at 2x ptSize: got %v, want %v", got, want)
	}
}

func embedTrueTypeSimple(t *testing.T, v pdf.Version, fontData *sfnt.Font) *dict.TrueType {
	t.Helper()
	w, _ := memfile.NewPDFWriter(v, nil)
	rm := pdf.NewResourceManager(w)

	F, err := truetype.NewSimple(fontData, nil)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	gg := F.Layout(nil, 12, "Hello")
	for _, g := range gg.Seq {
		_, _ = F.Encode(g.GID, g.Text)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	dictObj, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := dictObj.(*dict.TrueType)
	if !ok {
		t.Fatalf("wrong font dictionary type: %T", dictObj)
	}
	return d
}

func checkDescriptorMetrics(t *testing.T, fd *font.Descriptor, src *sfnt.Font) {
	t.Helper()
	q := 1000.0 / float64(src.UnitsPerEm)
	checks := []struct {
		name string
		got  float64
		want float64
	}{
		{"Ascent", fd.Ascent, math.Round(float64(src.Ascent) * q)},
		{"Descent", fd.Descent, math.Round(float64(src.Descent) * q)},
		{"CapHeight", fd.CapHeight, math.Round(float64(src.CapHeight) * q)},
		{"XHeight", fd.XHeight, math.Round(float64(src.XHeight) * q)},
		{"Leading", fd.Leading, math.Round(float64(src.Ascent-src.Descent+src.LineGap) * q)},
	}
	for _, tc := range checks {
		if tc.got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

func totalAdvance(seq *font.GlyphSeq) float64 {
	a := seq.Skip
	for _, g := range seq.Seq {
		a += g.Advance
	}
	return a
}
