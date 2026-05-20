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

package opentype_test

import (
	"math"
	"testing"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestEmbedSimple(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		t.Run(v.String(), func(t *testing.T) {
			// step 1: embed a font instance into a simple PDF file
			w, _ := memfile.NewPDFWriter(v, nil)
			rm := pdf.NewResourceManager(w)

			fontData := makefont.OpenType()
			fontInstance, err := opentype.NewSimple(fontData, nil)
			if err != nil {
				t.Fatal(err)
			}

			ref, err := rm.Embed(fontInstance)
			if err != nil {
				t.Fatal(err)
			}

			// make sure a few glyphs are included and encoded
			gg := fontInstance.Layout(nil, 12, "Hello")
			for _, g := range gg.Seq {
				_, _ = fontInstance.Encode(g.GID, string(g.Text))
			}

			err = rm.Close()
			if err != nil {
				t.Fatal(err)
			}

			// step 2: read back the font and verify that everything is as expected
			x := pdf.NewExtractor(w)
			dictObj, err := extract.Dict(x, nil, ref, false)
			if err != nil {
				t.Fatal(err)
			}
			dict, ok := dictObj.(*dict.Type1)
			if !ok {
				t.Fatalf("wrong font dictionary type: %T", dictObj)
			}

			if dict.PostScriptName != fontData.PostScriptName() {
				t.Errorf("wrong PostScript name: expected %v, got %v",
					fontData.PostScriptName(), dict.PostScriptName)
			}
			if len(dict.SubsetTag) != 6 {
				t.Errorf("wrong subset tag: %q", dict.SubsetTag)
			}

			// TODO(voss): more tests
		})
	}
}

// TestSimpleCFFDescriptor verifies that the PDF font descriptor of an
// OpenType+CFF simple font reports metrics in PDF glyph space (1/1000 em),
// follows the Ascent-Descent+LineGap convention for Leading, and surfaces
// the CFF Private dict's StdVW as StemV.  The Descriptor API doc declares
// "StemV: 0 = unknown", so the descriptor must not report 0 when the source
// has a known StdVW.
func TestSimpleCFFDescriptor(t *testing.T) {
	fontData := makefont.OpenType()

	// The toCFF conversion leaves StdVW at 0, which would mask a regression
	// of the "always emit StemV = 0" form.  Inject a known value.
	const stdVW = 80.0
	fontData.AsCFF().Private[0].StdVW = stdVW

	fd := embedSimpleForDescriptor(t, fontData).Descriptor
	checkDescriptorMetrics(t, fd, fontData)

	if fd.StemV == 0 {
		t.Errorf("StemV: got 0, want non-zero (source StdVW = %v)", stdVW)
	}
}

// embedSimpleForDescriptor embeds fontData as an OpenType+CFF simple font
// and returns the round-tripped font dictionary.
func embedSimpleForDescriptor(t *testing.T, fontData *sfnt.Font) *dict.Type1 {
	t.Helper()
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	F, err := opentype.NewSimple(fontData, nil)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := rm.Embed(F)
	if err != nil {
		t.Fatal(err)
	}
	gg := F.Layout(nil, 12, "Hello")
	for _, g := range gg.Seq {
		_, _ = F.Encode(g.GID, string(g.Text))
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	dictObj, err := extract.Dict(x, nil, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := dictObj.(*dict.Type1)
	if !ok {
		t.Fatalf("wrong font dictionary type: %T", dictObj)
	}
	return d
}

// checkDescriptorMetrics verifies that the descriptor reports Ascent,
// Descent, CapHeight, XHeight, and Leading in PDF glyph space (1/1000 em).
// Old code paths emitted some of these in raw font units or left Leading
// at 0; this check catches both regressions.
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
