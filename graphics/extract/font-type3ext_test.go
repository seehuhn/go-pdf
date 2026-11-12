// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package extract_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestType3RoundTrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for i, d := range type3Dicts {
			t.Run(fmt.Sprintf("D%dv%s-%s", i, v, d.Name), func(t *testing.T) {
				w, _ := memfile.NewPDFWriter(t, v, nil)
				rm := pdf.NewResourceManager(w)

				// == Write ==

				d1 := clone(d)

				fontDictRef, err := rm.Embed(d1)
				if err != nil {
					t.Fatal(err)
				}
				err = rm.Close()
				if err != nil {
					t.Fatal(err)
				}

				// == Read ==

				x := pdf.NewExtractor(w)
				d2Any, err := extract.Dict(pdf.CursorAt(x, nil), fontDictRef, false)
				if err != nil {
					t.Fatal(err)
				}
				d2 := d2Any.(*dict.Type3)

				// Text, glyph and width for unused codes are arbitrary after roundtrip.
				// We compare these manually here, and zero the values before the comparison
				// below.
				text1 := dict.SimpleTextMap("", d1.Encoding, d1.ToUnicode)
				text2 := dict.SimpleTextMap("", d2.Encoding, d2.ToUnicode)
				for i := range 256 {
					code := byte(i)
					if d1.Encoding(code) != "" {
						if d1.Encoding(code) != d2.Encoding(code) {
							t.Errorf("glyphName[%d]: %q != %q", code, d1.Encoding(code), d2.Encoding(code))
						}
						if text1[code] != text2[code] {
							t.Errorf("text[%d]: %q != %q", code, text1[code], text2[code])
						}
						if d1.Width[code] != d2.Width[code] {
							t.Errorf("width[%d]: %f != %f", code, d1.Width[code], d2.Width[code])
						}
					}

					d1.Width[code] = 0
					d2.Width[code] = 0
				}
				d1.Encoding = nil
				d2.Encoding = nil

				if d := cmp.Diff(d1, d2, contentStreamOpt); d != "" {
					t.Fatal(d)
				}
			})
		}
	}
}

// contentStreamOpt compares content.Stream interface values semantically
// (by operator sequence) rather than by concrete type, so a materialised
// *content.Operators in test data round-trips against the lazy
// *streamFactory produced by extract.
var contentStreamOpt = cmp.Comparer(content.StreamsEqual)

func FuzzType3Dict(f *testing.F) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for _, d := range type3Dicts {
			out := memfile.New()
			opt := &pdf.WriterOptions{
				HumanReadable: true,
			}
			w, err := pdf.NewWriter(out, v, opt)
			if err != nil {
				f.Fatal(err)
			}
			if err := memfile.AddBlankPage(w); err != nil {
				f.Fatal(err)
			}

			d := clone(d)
			rm := pdf.NewResourceManager(w)
			fontDictRef, err := rm.Embed(d)
			if err != nil {
				f.Fatal(err)
			}
			err = rm.Close()
			if err != nil {
				f.Fatal(err)
			}

			w.GetMeta().Trailer["Quir:E"] = fontDictRef

			err = w.Close()
			if err != nil {
				f.Fatal(err)
			}

			f.Add(out.Data)
		}
	}

	// glyph procedures which the writer rejects, written as raw objects
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for _, proc := range invalidGlyphProcs {
			f.Add(rawType3File(f, v, proc))
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		// Get "random" Type3Dict from PDF.
		// Make sure we don't panic on random input.
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("broken PDF: " + err.Error())
		}
		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			pdf.Format(os.Stdout, pdf.OptPretty, r.GetMeta().Trailer)
			t.Skip("broken reference")
		}
		x := pdf.NewExtractor(r)
		d1Any, err := extract.Dict(pdf.CursorAt(x, nil), obj, false)
		if err != nil {
			t.Skip("broken Type3Dict")
		}
		d1, ok := d1Any.(*dict.Type3)
		if !ok {
			t.Skip("not a Type3 font")
		}

		// reading accepts versions the writer cannot reproduce
		version := pdf.GetVersion(r)
		if !version.IsSupported() {
			t.Skip("version cannot be written")
		}

		// Write the Type3Dict back to a new PDF file.
		// Make sure we can write arbitrary Type3Dicts.
		w, _ := memfile.NewPDFWriter(t, version, nil)
		rm := pdf.NewResourceManager(w)

		fontDictRef, err := rm.Embed(d1)
		if err != nil {
			t.Fatal(err)
		}
		err = rm.Close()
		if err != nil {
			t.Fatal(err)
		}

		// Read back the data.
		// Make sure we get the same Type3Dict back.
		x2 := pdf.NewExtractor(w)
		d2Any, err := extract.Dict(pdf.CursorAt(x2, nil), fontDictRef, false)
		if err != nil {
			t.Fatal(err)
		}
		d2 := d2Any.(*dict.Type3)

		// Text, glyph and width for unused codes are arbitrary after roundtrip.
		// We compare these manually here, and zero the values before the comparison
		// below.
		text1 := dict.SimpleTextMap("", d1.Encoding, d1.ToUnicode)
		text2 := dict.SimpleTextMap("", d2.Encoding, d2.ToUnicode)
		for i := range 256 {
			code := byte(i)
			if d1.Encoding(code) != "" {
				if d1.Encoding(code) != d2.Encoding(code) {
					t.Errorf("glyphName[%d]: %q != %q", code, d1.Encoding(code), d2.Encoding(code))
				}
				if text1[code] != text2[code] {
					t.Errorf("text[%d]: %q != %q", code, text1[code], text2[code])
				}
				if d1.Width[code] != d2.Width[code] {
					t.Errorf("width[%d]: %f != %f", code, d1.Width[code], d2.Width[code])
				}
			}

			d1.Width[code] = 0
			d2.Width[code] = 0
		}
		d1.Encoding = nil
		d2.Encoding = nil

		if d := cmp.Diff(d1, d2, contentStreamOpt); d != "" {
			t.Fatal(d)
		}
	})
}

// testCharProcs contains various CharProc content streams for testing.
var testCharProcs = map[string]*dict.CharProc{
	// d0 with simple filled rectangle
	"filledRect": {
		Content: &content.Operators{Ops: []content.Operator{
			{Name: content.OpType3ColoredGlyph, Args: []pdf.Object{pdf.Number(500), pdf.Integer(0)}},
			{Name: content.OpRectangle, Args: []pdf.Object{pdf.Integer(50), pdf.Integer(0), pdf.Integer(400), pdf.Integer(700)}},
			{Name: content.OpFill},
		}},
	},
	// d0 with stroked path and color
	"strokedPath": {
		Content: &content.Operators{Ops: []content.Operator{
			{Name: content.OpType3ColoredGlyph, Args: []pdf.Object{pdf.Number(600), pdf.Integer(0)}},
			{Name: content.OpSetStrokeGray, Args: []pdf.Object{pdf.Number(0.5)}},
			{Name: content.OpSetLineWidth, Args: []pdf.Object{pdf.Number(10)}},
			{Name: content.OpMoveTo, Args: []pdf.Object{pdf.Integer(0), pdf.Integer(0)}},
			{Name: content.OpLineTo, Args: []pdf.Object{pdf.Integer(500), pdf.Integer(700)}},
			{Name: content.OpStroke},
		}},
	},
	// d1 (uncolored) with bounding box and filled shape
	"uncolored": {
		Content: &content.Operators{Ops: []content.Operator{
			{Name: content.OpType3UncoloredGlyph, Args: []pdf.Object{
				pdf.Number(500), pdf.Integer(0), // wx, wy
				pdf.Integer(0), pdf.Integer(0), pdf.Integer(500), pdf.Integer(700), // bbox
			}},
			{Name: content.OpRectangle, Args: []pdf.Object{pdf.Integer(0), pdf.Integer(0), pdf.Integer(500), pdf.Integer(700)}},
			{Name: content.OpFill},
		}},
	},
	// d0 with nested q/Q (save/restore graphics state)
	"nested": {
		Content: &content.Operators{Ops: []content.Operator{
			{Name: content.OpType3ColoredGlyph, Args: []pdf.Object{pdf.Number(500), pdf.Integer(0)}},
			{Name: content.OpPushGraphicsState},
			{Name: content.OpSetFillGray, Args: []pdf.Object{pdf.Number(0.8)}},
			{Name: content.OpRectangle, Args: []pdf.Object{pdf.Integer(0), pdf.Integer(0), pdf.Integer(250), pdf.Integer(700)}},
			{Name: content.OpFill},
			{Name: content.OpPopGraphicsState},
			{Name: content.OpRectangle, Args: []pdf.Object{pdf.Integer(250), pdf.Integer(0), pdf.Integer(250), pdf.Integer(700)}},
			{Name: content.OpFill},
		}},
	},
	// d0 with bezier curve
	"curve": {
		Content: &content.Operators{Ops: []content.Operator{
			{Name: content.OpType3ColoredGlyph, Args: []pdf.Object{pdf.Number(500), pdf.Integer(0)}},
			{Name: content.OpMoveTo, Args: []pdf.Object{pdf.Integer(0), pdf.Integer(0)}},
			{Name: content.OpCurveTo, Args: []pdf.Object{
				pdf.Integer(100), pdf.Integer(400), // control point 1
				pdf.Integer(400), pdf.Integer(400), // control point 2
				pdf.Integer(500), pdf.Integer(0), // end point
			}},
			{Name: content.OpClosePath},
			{Name: content.OpFill},
		}},
	},
	// d0 with fill and stroke
	"fillAndStroke": {
		Content: &content.Operators{Ops: []content.Operator{
			{Name: content.OpType3ColoredGlyph, Args: []pdf.Object{pdf.Number(500), pdf.Integer(0)}},
			{Name: content.OpSetFillRGB, Args: []pdf.Object{pdf.Number(1), pdf.Number(0), pdf.Number(0)}},
			{Name: content.OpSetStrokeRGB, Args: []pdf.Object{pdf.Number(0), pdf.Number(0), pdf.Number(1)}},
			{Name: content.OpSetLineWidth, Args: []pdf.Object{pdf.Number(5)}},
			{Name: content.OpRectangle, Args: []pdf.Object{pdf.Integer(10), pdf.Integer(10), pdf.Integer(480), pdf.Integer(680)}},
			{Name: content.OpFillAndStroke},
		}},
	},
	// minimal d0 (just width, no drawing)
	"minimal": {
		Content: &content.Operators{Ops: []content.Operator{
			{Name: content.OpType3ColoredGlyph, Args: []pdf.Object{pdf.Number(250), pdf.Integer(0)}},
		}},
	},
}

var type3Dicts = []*dict.Type3{
	// Basic font with minimal glyphs
	{
		Name: "Minimal",
		Encoding: func(code byte) string {
			switch code {
			case 65:
				return "A"
			case 66:
				return "B"
			default:
				return ""
			}
		},
		Width:      makeTestWidth(65, 500.0, 66, 250.0),
		CharProcs:  map[pdf.Name]*dict.CharProc{"A": testCharProcs["filledRect"], "B": testCharProcs["minimal"]},
		FontMatrix: matrix.Scale(0.001, 0.001),
	},
	// Font with various drawing operations
	{
		Name: "Drawing",
		Encoding: func(code byte) string {
			switch code {
			case 65:
				return "A"
			case 66:
				return "B"
			case 67:
				return "C"
			case 68:
				return "D"
			default:
				return ""
			}
		},
		Width:      makeTestWidth(65, 500.0, 66, 600.0, 67, 500.0, 68, 500.0),
		CharProcs:  map[pdf.Name]*dict.CharProc{"A": testCharProcs["filledRect"], "B": testCharProcs["strokedPath"], "C": testCharProcs["curve"], "D": testCharProcs["fillAndStroke"]},
		FontBBox:   &pdf.Rectangle{LLx: 0, LLy: 0, URx: 600, URy: 700},
		FontMatrix: matrix.Scale(0.001, 0.001),
	},
	// Font with d1 (uncolored) glyphs
	{
		Name: "Uncolored",
		Encoding: func(code byte) string {
			switch code {
			case 65:
				return "A"
			default:
				return ""
			}
		},
		Width:      makeTestWidth(65, 500.0),
		CharProcs:  map[pdf.Name]*dict.CharProc{"A": testCharProcs["uncolored"]},
		FontBBox:   &pdf.Rectangle{LLx: 0, LLy: 0, URx: 500, URy: 700},
		FontMatrix: matrix.Scale(0.001, 0.001),
	},
	// Font with nested graphics state
	{
		Name: "Nested",
		Descriptor: &font.Descriptor{
			FontName:   "NestedTest",
			IsSymbolic: true,
			Ascent:     700,
			Descent:    0,
			CapHeight:  700,
			StemV:      80,
		},
		Encoding: func(code byte) string {
			switch code {
			case 65:
				return "A"
			default:
				return ""
			}
		},
		Width:      makeTestWidth(65, 500.0),
		CharProcs:  map[pdf.Name]*dict.CharProc{"A": testCharProcs["nested"]},
		FontBBox:   &pdf.Rectangle{LLx: 0, LLy: 0, URx: 500, URy: 700},
		FontMatrix: matrix.Scale(0.001, 0.001),
	},
}

// Entries which cannot be written at the version of the file are dropped on
// read: font Resources before PDF 1.2, glyph Resources before PDF 2.0.
func TestType3VersionRepair(t *testing.T) {
	res := &content.Resources{
		Font: map[pdf.Name]font.Instance{
			"F1": font.Must(standard.Helvetica.New()),
		},
	}
	d1 := &dict.Type3{
		Name:       "F",
		Descriptor: &font.Descriptor{},
		CharProcs: map[pdf.Name]*dict.CharProc{
			"A": {Content: testCharProcs["filledRect"].Content, Resources: res},
		},
		FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
		Resources:  res,
	}
	d1.Encoding = func(code byte) string {
		if code == 'A' {
			return "A"
		}
		return ""
	}
	d1.Width['A'] = 500

	// cycle writes d at version wv and reads it back as if from a file of
	// version rv
	cycle := func(t *testing.T, d *dict.Type3, wv, rv pdf.Version) *dict.Type3 {
		t.Helper()
		w, _ := memfile.NewPDFWriter(t, wv, nil)
		rm := pdf.NewResourceManager(w)
		ref, err := rm.Embed(d)
		if err != nil {
			t.Fatal(err)
		}
		if err := rm.Close(); err != nil {
			t.Fatal(err)
		}
		w.GetMeta().Version = rv
		obj, err := extract.Dict(pdf.CursorAt(pdf.NewExtractor(w), nil), ref, false)
		if err != nil {
			t.Fatal(err)
		}
		return obj.(*dict.Type3)
	}

	for _, tc := range []struct {
		version  pdf.Version
		fontRes  bool
		glyphRes bool
	}{
		{pdf.V1_1, false, false},
		{pdf.V1_2, true, false},
		{pdf.V1_7, true, false},
		{pdf.V2_0, true, true},
	} {
		t.Run(tc.version.String(), func(t *testing.T) {
			d2 := cycle(t, d1, pdf.V2_0, tc.version)
			if got := d2.Resources != nil; got != tc.fontRes {
				t.Errorf("font Resources present = %t, want %t", got, tc.fontRes)
			}
			if got := d2.CharProcs["A"].Resources != nil; got != tc.glyphRes {
				t.Errorf("glyph Resources present = %t, want %t", got, tc.glyphRes)
			}

			d3 := cycle(t, d2, tc.version, tc.version)
			d2.Encoding = nil
			d3.Encoding = nil
			if diff := cmp.Diff(d2, d3, contentStreamOpt); diff != "" {
				t.Errorf("round trip failed (-want +got):\n%s", diff)
			}
		})
	}
}

// invalidGlyphProcs holds glyph procedures which break the rules for Type 3
// glyphs in different ways.
var invalidGlyphProcs = map[string]string{
	"operators before d0": "0 0 m 100 100 l f 500 0 d0 0 0 m",
	"second d0":           "500 0 d0 0 0 m 600 0 d0 1 2 3 4 5 6 d1 f",
	"short d1":            "1 0 5 d1 0 0 m 700 0 d0",
	"no metrics":          "0 0 m 100 100 l S",
	"empty":               "",
	"comment first":       "% comment\n500 0 d0 0 0 m",
	"comment only":        "% comment\n",
	"wy":                  "500 3 d0 0 0 m",
	"d1 wy":               "500 3 1 2 3 4 d1 0 0 m",
	"non-numeric":         "/a /b d0 0 0 m",
	"inline image":        "0 0 m BI /W 1 /H 1 /BPC 8 /CS /G ID \x80\nEI f",
}

// rawType3File returns a PDF file containing a Type 3 font with a single
// glyph g0 at code 65, whose glyph procedure is proc.  The font dictionary
// is stored in the trailer under Quir:E.
func rawType3File(tb testing.TB, v pdf.Version, proc string) []byte {
	tb.Helper()

	out := memfile.New()
	w, err := pdf.NewWriter(out, v, &pdf.WriterOptions{HumanReadable: true})
	if err != nil {
		tb.Fatal(err)
	}
	if err := memfile.AddBlankPage(w); err != nil {
		tb.Fatal(err)
	}

	ref := w.Alloc()
	stm, err := w.OpenStream(ref, nil)
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := stm.Write([]byte(proc)); err != nil {
		tb.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		tb.Fatal(err)
	}

	fontDict := pdf.Dict{
		"Type":       pdf.Name("Font"),
		"Subtype":    pdf.Name("Type3"),
		"FontBBox":   &pdf.Rectangle{URx: 1000, URy: 1000},
		"FontMatrix": pdf.Array{pdf.Real(0.001), pdf.Integer(0), pdf.Integer(0), pdf.Real(0.001), pdf.Integer(0), pdf.Integer(0)},
		"CharProcs":  pdf.Dict{"g0": ref},
		"Encoding": pdf.Dict{
			"Type":        pdf.Name("Encoding"),
			"Differences": pdf.Array{pdf.Integer(65), pdf.Name("g0")},
		},
		"FirstChar": pdf.Integer(65),
		"LastChar":  pdf.Integer(65),
		"Widths":    pdf.Array{pdf.Integer(250)},
	}
	fontRef := w.Alloc()
	if err := w.Put(fontRef, fontDict); err != nil {
		tb.Fatal(err)
	}
	w.GetMeta().Trailer["Quir:E"] = fontRef
	if err := w.Close(); err != nil {
		tb.Fatal(err)
	}
	return out.Data
}

// Glyph procedures which break the rules are repaired on read, so that every
// glyph is kept and the font can be written again.
func TestType3RepairGlyphs(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for key, proc := range invalidGlyphProcs {
			t.Run(fmt.Sprintf("v%s-%s", v, key), func(t *testing.T) {
				data := rawType3File(t, v, proc)
				r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)), nil)
				if err != nil {
					t.Fatal(err)
				}
				obj := r.GetMeta().Trailer["Quir:E"]
				d1Any, err := extract.Dict(pdf.CursorAt(pdf.NewExtractor(r), nil), obj, false)
				if err != nil {
					t.Fatal(err)
				}
				d1 := d1Any.(*dict.Type3)

				cp := d1.CharProcs["g0"]
				if cp == nil {
					t.Fatal("glyph was dropped")
				}
				m1, ok := content.ReadGlyphMetrics(cp.Content)
				if !ok {
					t.Fatal("glyph procedure is not valid")
				}
				if !strings.Contains(proc, "d0") && !strings.Contains(proc, "d1") && m1.Width != 0 {
					t.Errorf("synthesised width = %g, want 0", m1.Width)
				}
				nMetrics := 0
				for name := range cp.Content.NewIter().All() {
					if name == content.OpType3ColoredGlyph || name == content.OpType3UncoloredGlyph {
						nMetrics++
					}
				}
				if nMetrics != 1 {
					t.Errorf("glyph procedure has %d d0/d1 operators, want 1", nMetrics)
				}

				w, _ := memfile.NewPDFWriter(t, v, nil)
				rm := pdf.NewResourceManager(w)
				ref, err := rm.Embed(d1)
				if err != nil {
					t.Fatal(err)
				}
				if err := rm.Close(); err != nil {
					t.Fatal(err)
				}
				d2Any, err := extract.Dict(pdf.CursorAt(pdf.NewExtractor(w), nil), ref, false)
				if err != nil {
					t.Fatal(err)
				}
				d2 := d2Any.(*dict.Type3)
				if !content.StreamsEqual(cp.Content, d2.CharProcs["g0"].Content) {
					t.Error("second read differs from first read")
				}
				d1.Encoding, d2.Encoding = nil, nil
				if diff := cmp.Diff(d1, d2, contentStreamOpt); diff != "" {
					t.Errorf("round trip failed (-want +got):\n%s", diff)
				}
			})
		}
	}
}

// TestType3SharedCharProcs checks that glyph names which refer to one
// stream, in one font or in several, share one CharProc, and that the
// sharing is kept when the fonts are written and read again.
func TestType3SharedCharProcs(t *testing.T) {
	procs := map[string]string{
		"valid":      "500 0 d0 0 0 m 100 100 l f",
		"no metrics": "0 0 m 100 100 l f",
	}
	for key, proc := range procs {
		t.Run(key, func(t *testing.T) {
			out := memfile.New()
			w, err := pdf.NewWriter(out, pdf.V2_0, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := memfile.AddBlankPage(w); err != nil {
				t.Fatal(err)
			}
			stmRef := w.Alloc()
			stm, err := w.OpenStream(stmRef, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := stm.Write([]byte(proc)); err != nil {
				t.Fatal(err)
			}
			if err := stm.Close(); err != nil {
				t.Fatal(err)
			}
			fontDict := func(charProcs pdf.Dict, diff pdf.Array) pdf.Dict {
				return pdf.Dict{
					"Type":       pdf.Name("Font"),
					"Subtype":    pdf.Name("Type3"),
					"FontBBox":   &pdf.Rectangle{URx: 1000, URy: 1000},
					"FontMatrix": pdf.Array{pdf.Real(0.001), pdf.Integer(0), pdf.Integer(0), pdf.Real(0.001), pdf.Integer(0), pdf.Integer(0)},
					"CharProcs":  charProcs,
					"Encoding":   pdf.Dict{"Differences": diff},
					"FirstChar":  pdf.Integer(65),
					"LastChar":   pdf.Integer(66),
					"Widths":     pdf.Array{pdf.Integer(500), pdf.Integer(500)},
				}
			}
			f1, f2 := w.Alloc(), w.Alloc()
			err = w.Put(f1, fontDict(pdf.Dict{"g0": stmRef, "g1": stmRef},
				pdf.Array{pdf.Integer(65), pdf.Name("g0"), pdf.Name("g1")}))
			if err != nil {
				t.Fatal(err)
			}
			err = w.Put(f2, fontDict(pdf.Dict{"g0": stmRef},
				pdf.Array{pdf.Integer(65), pdf.Name("g0")}))
			if err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			// read both fonts through one extractor
			read := func(g pdf.Getter, refs ...pdf.Reference) []*dict.Type3 {
				t.Helper()
				x := pdf.NewExtractor(g)
				var res []*dict.Type3
				for _, ref := range refs {
					d, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
					if err != nil {
						t.Fatal(err)
					}
					res = append(res, d.(*dict.Type3))
				}
				return res
			}
			r, err := pdf.NewReader(bytes.NewReader(out.Data), int64(len(out.Data)), nil)
			if err != nil {
				t.Fatal(err)
			}
			fonts := read(r, f1, f2)
			checkShared := func(fonts []*dict.Type3) {
				t.Helper()
				cp := fonts[0].CharProcs["g0"]
				if cp == nil {
					t.Fatal("glyph was dropped")
				}
				if fonts[0].CharProcs["g1"] != cp || fonts[1].CharProcs["g0"] != cp {
					t.Error("glyph procedure is not shared")
				}
			}
			checkShared(fonts)

			// write both fonts to a new file
			w2, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w2)
			var refs []pdf.Reference
			for _, d := range fonts {
				obj, err := rm.Embed(d)
				if err != nil {
					t.Fatal(err)
				}
				refs = append(refs, obj.(pdf.Reference))
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}

			c := pdf.CursorAt(pdf.NewExtractor(w2), nil)
			var stmRefs []pdf.Object
			for i, ref := range refs {
				fd, err := c.Dict(ref)
				if err != nil {
					t.Fatal(err)
				}
				charProcs, err := c.Dict(fd["CharProcs"])
				if err != nil {
					t.Fatal(err)
				}
				stmRefs = append(stmRefs, charProcs["g0"])
				if i == 0 {
					stmRefs = append(stmRefs, charProcs["g1"])
				}
			}
			for _, obj := range stmRefs[1:] {
				if obj != stmRefs[0] {
					t.Errorf("glyph procedure written more than once: %v", stmRefs)
					break
				}
			}

			checkShared(read(w2, refs...))
		})
	}
}
