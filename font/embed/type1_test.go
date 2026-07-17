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

package embed_test

import (
	"bytes"
	"strings"
	"testing"

	"seehuhn.de/go/postscript/afm"
	pst1 "seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/embed"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// afmFromInstance builds AFM metrics that describe exactly the given
// (already instantiated) font, so that the font/metrics consistency check
// passes regardless of the instance's blended widths.
func afmFromInstance(inst *pst1.Font) *afm.Metrics {
	glyphs := make(map[string]*afm.GlyphInfo, len(inst.Glyphs))
	for name, g := range inst.Glyphs {
		glyphs[name] = &afm.GlyphInfo{WidthX: g.WidthX * inst.FontMatrix[0] * 1000}
	}
	return &afm.Metrics{
		FontName: inst.FontName,
		Glyphs:   glyphs,
	}
}

// TestType1FontVariationsMatrix covers the (MM?, Variations?, AFM?)
// behavior matrix for [Type1Font].
func TestType1FontVariationsMatrix(t *testing.T) {
	plainFont := makefont.Type1()
	plainMetrics := makefont.AFM()

	mmFont := makefont.MMType1()
	defaultInstance, err := mmFont.Instantiate(nil)
	if err != nil {
		t.Fatal(err)
	}
	defaultMetrics := afmFromInstance(defaultInstance)

	someVariations := map[string]float64{"Weight": 900}

	tests := []struct {
		name      string
		psFont    *pst1.Font
		metrics   *afm.Metrics
		opt       *embed.Type1Options
		wantError bool
	}{
		{
			name:   "plain, no variations, no metrics",
			psFont: plainFont,
		},
		{
			name:    "plain, no variations, with metrics",
			psFont:  plainFont,
			metrics: plainMetrics,
		},
		{
			name:      "plain, with variations, no metrics",
			psFont:    plainFont,
			opt:       &embed.Type1Options{Variations: someVariations},
			wantError: true,
		},
		{
			name:      "plain, with variations, with metrics",
			psFont:    plainFont,
			metrics:   plainMetrics,
			opt:       &embed.Type1Options{Variations: someVariations},
			wantError: true,
		},
		{
			name:   "MM, no variations, no metrics",
			psFont: mmFont,
		},
		{
			name:    "MM, no variations, with metrics",
			psFont:  mmFont,
			metrics: defaultMetrics,
		},
		{
			name:   "MM, with variations, no metrics",
			psFont: mmFont,
			opt:    &embed.Type1Options{Variations: someVariations},
		},
		{
			name:      "MM, with variations, with metrics",
			psFont:    mmFont,
			metrics:   defaultMetrics,
			opt:       &embed.Type1Options{Variations: someVariations},
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := embed.Type1Font(test.psFont, test.metrics, test.opt)
			if test.wantError && err == nil {
				t.Fatal("expected an error")
			} else if !test.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestType1FontMMInstanceEmbed checks that embedding a multiple master font
// produces an ordinary /Type1 font dict for the requested instance: no
// MultipleMaster flag, a BaseFont carrying the instance name, and a
// FontFile stream without any /WeightVector entry.
func TestType1FontMMInstanceEmbed(t *testing.T) {
	mmFont := makefont.MMType1()

	fontInstance, err := embed.Type1Font(mmFont, nil, &embed.Type1Options{
		Variations: map[string]float64{"Weight": 900},
	})
	if err != nil {
		t.Fatal(err)
	}

	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	ref, err := rm.Embed(fontInstance)
	if err != nil {
		t.Fatal(err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	c := pdf.NewCursor(w)

	fontDict, err := c.DictTyped(ref, "Font")
	if err != nil {
		t.Fatal(err)
	}
	if subtype, err := c.Name(fontDict["Subtype"]); err != nil || subtype != "Type1" {
		t.Errorf("wrong Subtype: %v, %v", subtype, err)
	}
	baseFont, err := c.Name(fontDict["BaseFont"])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(baseFont), "_900_100") {
		t.Errorf("BaseFont does not carry instance name: %q", baseFont)
	}
	if !strings.Contains(string(baseFont), "+") {
		t.Errorf("BaseFont does not carry a subset tag: %q", baseFont)
	}

	fdDict, err := c.Dict(fontDict["FontDescriptor"])
	if err != nil {
		t.Fatal(err)
	}

	fontFileData, err := c.ReadAll(fdDict["FontFile"], 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(fontFileData, []byte("/WeightVector")) {
		t.Error("embedded font file still contains /WeightVector")
	}

	// [M6] dict.Type1.MultipleMaster must stay false on this path: the
	// embedded font is an ordinary single-master instance.
	x := pdf.NewExtractor(w)
	dictObj, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := dictObj.(*dict.Type1)
	if !ok {
		t.Fatalf("wrong font dictionary type: %T", dictObj)
	}
	if d.MultipleMaster {
		t.Error("MultipleMaster must be false for an instanced MM font")
	}
}
