// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package extract_test

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestType3Roundtrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for i, d := range type3Dicts {
			t.Run(fmt.Sprintf("D%dv%s-%s", i, v, d.Name), func(t *testing.T) {
				w, _ := memfile.NewPDFWriter(v, nil)
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
				d2Any, err := extract.Dict(x, fontDictRef)
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

				if d := cmp.Diff(d1, d2); d != "" {
					t.Fatal(d)
				}
			})
		}
	}
}

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

	f.Fuzz(func(t *testing.T, fileData []byte) {
		// Get "random" Type3Dict from PDF.
		// Make sure we don't panic on random input.
		opt := &pdf.ReaderOptions{
			ErrorHandling: pdf.ErrorHandlingReport,
		}
		r, err := pdf.NewReader(bytes.NewReader(fileData), opt)
		if err != nil {
			t.Skip("broken PDF: " + err.Error())
		}
		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			pdf.Format(os.Stdout, pdf.OptPretty, r.GetMeta().Trailer)
			t.Skip("broken reference")
		}
		x := pdf.NewExtractor(r)
		d1Any, err := extract.Dict(x, obj)
		if err != nil {
			t.Skip("broken Type3Dict")
		}
		d1, ok := d1Any.(*dict.Type3)
		if !ok {
			t.Skip("not a Type3 font")
		}

		// Write the Type3Dict back to a new PDF file.
		// Make sure we can write arbitrary Type3Dicts.
		w, _ := memfile.NewPDFWriter(pdf.GetVersion(r), nil)
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
		d2Any, err := extract.Dict(x2, fontDictRef)
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

		if d := cmp.Diff(d1, d2); d != "" {
			t.Fatal(d)
		}
	})
}

// testCharProcs contains various CharProc content streams for testing.
var testCharProcs = map[string]*dict.CharProc{
	// d0 with simple filled rectangle
	"filledRect": {
		Content: content.Stream{
			{Name: content.OpType3ColoredGlyph, Args: []pdf.Object{pdf.Number(500), pdf.Integer(0)}},
			{Name: content.OpRectangle, Args: []pdf.Object{pdf.Integer(50), pdf.Integer(0), pdf.Integer(400), pdf.Integer(700)}},
			{Name: content.OpFill},
		},
	},
	// d0 with stroked path and color
	"strokedPath": {
		Content: content.Stream{
			{Name: content.OpType3ColoredGlyph, Args: []pdf.Object{pdf.Number(600), pdf.Integer(0)}},
			{Name: content.OpSetStrokeGray, Args: []pdf.Object{pdf.Number(0.5)}},
			{Name: content.OpSetLineWidth, Args: []pdf.Object{pdf.Number(10)}},
			{Name: content.OpMoveTo, Args: []pdf.Object{pdf.Integer(0), pdf.Integer(0)}},
			{Name: content.OpLineTo, Args: []pdf.Object{pdf.Integer(500), pdf.Integer(700)}},
			{Name: content.OpStroke},
		},
	},
	// d1 (uncolored) with bounding box and filled shape
	"uncolored": {
		Content: content.Stream{
			{Name: content.OpType3UncoloredGlyph, Args: []pdf.Object{
				pdf.Number(500), pdf.Integer(0), // wx, wy
				pdf.Integer(0), pdf.Integer(0), pdf.Integer(500), pdf.Integer(700), // bbox
			}},
			{Name: content.OpRectangle, Args: []pdf.Object{pdf.Integer(0), pdf.Integer(0), pdf.Integer(500), pdf.Integer(700)}},
			{Name: content.OpFill},
		},
	},
	// d0 with nested q/Q (save/restore graphics state)
	"nested": {
		Content: content.Stream{
			{Name: content.OpType3ColoredGlyph, Args: []pdf.Object{pdf.Number(500), pdf.Integer(0)}},
			{Name: content.OpPushGraphicsState},
			{Name: content.OpSetFillGray, Args: []pdf.Object{pdf.Number(0.8)}},
			{Name: content.OpRectangle, Args: []pdf.Object{pdf.Integer(0), pdf.Integer(0), pdf.Integer(250), pdf.Integer(700)}},
			{Name: content.OpFill},
			{Name: content.OpPopGraphicsState},
			{Name: content.OpRectangle, Args: []pdf.Object{pdf.Integer(250), pdf.Integer(0), pdf.Integer(250), pdf.Integer(700)}},
			{Name: content.OpFill},
		},
	},
	// d0 with bezier curve
	"curve": {
		Content: content.Stream{
			{Name: content.OpType3ColoredGlyph, Args: []pdf.Object{pdf.Number(500), pdf.Integer(0)}},
			{Name: content.OpMoveTo, Args: []pdf.Object{pdf.Integer(0), pdf.Integer(0)}},
			{Name: content.OpCurveTo, Args: []pdf.Object{
				pdf.Integer(100), pdf.Integer(400), // control point 1
				pdf.Integer(400), pdf.Integer(400), // control point 2
				pdf.Integer(500), pdf.Integer(0), // end point
			}},
			{Name: content.OpClosePath},
			{Name: content.OpFill},
		},
	},
	// d0 with fill and stroke
	"fillAndStroke": {
		Content: content.Stream{
			{Name: content.OpType3ColoredGlyph, Args: []pdf.Object{pdf.Number(500), pdf.Integer(0)}},
			{Name: content.OpSetFillRGB, Args: []pdf.Object{pdf.Number(1), pdf.Number(0), pdf.Number(0)}},
			{Name: content.OpSetStrokeRGB, Args: []pdf.Object{pdf.Number(0), pdf.Number(0), pdf.Number(1)}},
			{Name: content.OpSetLineWidth, Args: []pdf.Object{pdf.Number(5)}},
			{Name: content.OpRectangle, Args: []pdf.Object{pdf.Integer(10), pdf.Integer(10), pdf.Integer(480), pdf.Integer(680)}},
			{Name: content.OpFillAndStroke},
		},
	},
	// minimal d0 (just width, no drawing)
	"minimal": {
		Content: content.Stream{
			{Name: content.OpType3ColoredGlyph, Args: []pdf.Object{pdf.Number(250), pdf.Integer(0)}},
		},
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
		Width:     makeTestWidth(65, 500.0, 66, 600.0, 67, 500.0, 68, 500.0),
		CharProcs: map[pdf.Name]*dict.CharProc{"A": testCharProcs["filledRect"], "B": testCharProcs["strokedPath"], "C": testCharProcs["curve"], "D": testCharProcs["fillAndStroke"]},
		FontBBox:  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 600, URy: 700},
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
			FontName:    "NestedTest",
			IsSymbolic:  true,
			Ascent:      700,
			Descent:     0,
			CapHeight:   700,
			StemV:       80,
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
