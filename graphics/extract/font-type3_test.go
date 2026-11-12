// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestType3ResourcesReadError checks that I/O errors when reading the font
// or glyph resources are propagated.
func TestType3ResourcesReadError(t *testing.T) {
	for _, glyphLevel := range []bool{false, true} {
		w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
		bad := w.Alloc()
		if err := w.Put(bad, pdf.Dict{}); err != nil {
			t.Fatal(err)
		}

		var stmDict pdf.Dict
		if glyphLevel {
			stmDict = pdf.Dict{"Resources": bad}
		}
		procRef := w.Alloc()
		stm, err := w.OpenStream(procRef, stmDict)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := stm.Write([]byte("500 0 d0")); err != nil {
			t.Fatal(err)
		}
		if err := stm.Close(); err != nil {
			t.Fatal(err)
		}

		fontDict := pdf.Dict{
			"Type":       pdf.Name("Font"),
			"Subtype":    pdf.Name("Type3"),
			"FontBBox":   &pdf.Rectangle{URx: 1000, URy: 1000},
			"FontMatrix": pdf.Array{pdf.Real(0.001), pdf.Integer(0), pdf.Integer(0), pdf.Real(0.001), pdf.Integer(0), pdf.Integer(0)},
			"CharProcs":  pdf.Dict{"g0": procRef},
			"Encoding":   pdf.Dict{"Differences": pdf.Array{pdf.Integer(65), pdf.Name("g0")}},
			"FirstChar":  pdf.Integer(65),
			"LastChar":   pdf.Integer(65),
			"Widths":     pdf.Array{pdf.Integer(500)},
		}
		if !glyphLevel {
			fontDict["Resources"] = bad
		}

		c := pdf.CursorAt(pdf.NewExtractor(failingGetter{Getter: w, bad: bad}), nil)
		if _, err := extractFontType3(c, fontDict); !pdf.IsReadError(err) {
			t.Errorf("glyph level %t: got %v, want read error", glyphLevel, err)
		}
	}
}
