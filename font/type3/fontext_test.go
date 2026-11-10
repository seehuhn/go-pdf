// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package type3_test

import (
	"math"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/squarefont"
)

// TestDescriptorGlyphSpace checks that the descriptor of a Type 3 font is
// written in the font's own glyph space, as the Widths array is, whatever
// the font matrix.
func TestDescriptorGlyphSpace(t *testing.T) {
	for _, s := range squarefont.All {
		if !strings.HasPrefix(s.Label, "Type3") {
			continue
		}
		t.Run(s.Label, func(t *testing.T) {
			F := s.MakeFont()

			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)
			ref, err := rm.Embed(F)
			if err != nil {
				t.Fatal(err)
			}
			for _, g := range F.Layout(nil, 10, "A").Seq {
				F.Encode(g.GID, g.Text)
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}

			obj, err := extract.Dict(pdf.CursorAt(pdf.NewExtractor(w), nil), ref, false)
			if err != nil {
				t.Fatal(err)
			}
			d := obj.(*dict.Type3)
			fd := d.Descriptor

			// squarefont's values are in PDF glyph space, 1/1000 text space
			qh := 1 / (1000 * d.FontMatrix[0])
			qv := 1 / (1000 * d.FontMatrix[3])
			cases := []struct {
				name      string
				got, want float64
			}{
				{"MissingWidth", fd.MissingWidth, squarefont.NotdefWidth * qh},
				{"Ascent", fd.Ascent, squarefont.Ascent * qv},
				{"Descent", fd.Descent, squarefont.Descent * qv},
				{"Leading", fd.Leading, squarefont.Leading * qv},
				{"CapHeight", fd.CapHeight, squarefont.CapHeight * qv},
				{"XHeight", fd.XHeight, squarefont.XHeight * qv},
			}
			for _, c := range cases {
				if math.Abs(c.got-c.want) > 1e-6 {
					t.Errorf("%s = %g, want %g", c.name, c.got, c.want)
				}
			}
		})
	}
}
