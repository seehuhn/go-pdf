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
	"io"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

func TestType1RoundTrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for i, d := range t1Dicts {
			if v >= pdf.V2_0 && d.Name != "" {
				continue
			}

			t.Run(fmt.Sprintf("D%dv%s-%s", i, v, d.PostScriptName), func(t *testing.T) {
				checkRoundTripT1(t, d, v)
			})
		}
	}
}

// TestMMType1Extract checks that a hand-built font dictionary with Subtype
// /MMType1 is extracted as a *dict.Type1 with MultipleMaster set.
func TestMMType1Extract(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	ref := w.Alloc()
	fontDict := pdf.Dict{
		"Type":      pdf.Name("Font"),
		"Subtype":   pdf.Name("MMType1"),
		"BaseFont":  pdf.Name("MinionMM_366_465_11_"),
		"FirstChar": pdf.Integer(65),
		"LastChar":  pdf.Integer(65),
		"Widths":    pdf.Array{pdf.Integer(600)},
	}
	if err := w.Put(ref, fontDict); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	dictAny, err := extract.Dict(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := dictAny.(*dict.Type1)
	if !ok {
		t.Fatalf("expected *dict.Type1, got %T", dictAny)
	}
	if !d.MultipleMaster {
		t.Error("expected MultipleMaster to be true")
	}
	if d.PostScriptName != "MinionMM_366_465_11_" {
		t.Errorf("unexpected PostScriptName: %q", d.PostScriptName)
	}
}

func FuzzType1Dict(f *testing.F) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for _, d := range t1Dicts {
			if v >= pdf.V2_0 && d.Name != "" {
				continue
			}

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
			rm := pdf.NewResourceManager(w)

			d := clone(d)
			if d.FontFile != nil {
				// create a fake font data stream for testing
				fontRef := w.Alloc()
				var subtype pdf.Object
				switch d.FontFile.Type {
				case glyphdata.CFFSimple:
					subtype = pdf.Name("Type1C")
				case glyphdata.OpenTypeCFFSimple:
					subtype = pdf.Name("OpenType")
				}
				stm, err := w.OpenStream(fontRef, pdf.Dict{"Subtype": subtype})
				if err != nil {
					f.Fatal(err)
				}
				err = stm.Close()
				if err != nil {
					f.Fatal(err)
				}

				// Keep FontFile but simplify WriteTo for test
				d.FontFile = &glyphdata.Stream{
					Type: d.FontFile.Type,
					WriteTo: func(w io.Writer, length *glyphdata.Lengths) error {
						return nil // test stub
					},
				}
			}
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
		// Get a "random" Type1Dict from the PDF file.

		// Make sure we don't panic on random input.
		opt := &pdf.ReaderOptions{
			ErrorHandling: pdf.ErrorHandlingReport,
		}
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), opt)
		if err != nil {
			t.Skip("broken PDF: " + err.Error())
		}
		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			pdf.Format(os.Stdout, pdf.OptPretty, r.GetMeta().Trailer)
			t.Skip("broken reference")
		}
		x := pdf.NewExtractor(r)
		dictAny, err := extract.Dict(pdf.CursorAt(x, nil), obj, false)
		if err != nil {
			t.Skip("no valid Type1Dict")
		}
		d, ok := dictAny.(*dict.Type1)
		if !ok {
			t.Skip("not a Type1 font")
		}

		// Make sure we can write the dict, and read it back.
		checkRoundTripT1(t, d, pdf.GetVersion(r))
	})
}

// TestMMType1RoundTripSubtype checks that a Type1 dict with MultipleMaster
// set writes Subtype /MMType1, and that extracting it again yields
// MultipleMaster == true.
func TestMMType1RoundTripSubtype(t *testing.T) {
	d1 := &dict.Type1{
		PostScriptName: "MinionMM_366_465_11_",
		MultipleMaster: true,
		Descriptor: &font.Descriptor{
			FontName:  "MinionMM_366_465_11_",
			FontBBox:  rect.Rect{LLx: 0, LLy: -100, URx: 200, URy: 300},
			Ascent:    250,
			Descent:   -50,
			CapHeight: 150,
		},
		Encoding: func(code byte) string {
			if code == 65 {
				return "A"
			}
			return ""
		},
		Width: makeTestWidth(65, 600.0),
	}

	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	fontDictRef, err := rm.Embed(d1)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	c := pdf.NewCursor(w)
	fontDict, err := c.DictTyped(fontDictRef, "Font")
	if err != nil {
		t.Fatal(err)
	}
	subtype, err := c.Name(fontDict["Subtype"])
	if err != nil {
		t.Fatal(err)
	}
	if subtype != "MMType1" {
		t.Errorf("expected Subtype MMType1, got %q", subtype)
	}

	x := pdf.NewExtractor(w)
	dictAny, err := extract.Dict(pdf.CursorAt(x, nil), fontDictRef, false)
	if err != nil {
		t.Fatal(err)
	}
	d2, ok := dictAny.(*dict.Type1)
	if !ok {
		t.Fatalf("expected *dict.Type1, got %T", dictAny)
	}
	if !d2.MultipleMaster {
		t.Error("expected MultipleMaster to be true after round trip")
	}
}

func checkRoundTripT1(t *testing.T, d1 *dict.Type1, v pdf.Version) {
	d1 = clone(d1)

	w, _ := memfile.NewPDFWriter(v, nil)
	rm := pdf.NewResourceManager(w)

	// == Write ==

	if d1.FontFile != nil {
		// create a fake font data stream for testing
		fontRef := w.Alloc()
		var subtype pdf.Object
		switch d1.FontFile.Type {
		case glyphdata.CFFSimple:
			subtype = pdf.Name("Type1C")
		case glyphdata.OpenTypeCFFSimple:
			subtype = pdf.Name("OpenType")
		}
		stm, err := w.OpenStream(fontRef, pdf.Dict{"Subtype": subtype})
		if err != nil {
			t.Fatal(err)
		}
		err = stm.Close()
		if err != nil {
			t.Fatal(err)
		}

		// Keep FontFile but simplify WriteTo for test
		d1.FontFile = &glyphdata.Stream{
			Type: d1.FontFile.Type,
			WriteTo: func(w io.Writer, length *glyphdata.Lengths) error {
				return nil // test stub
			},
		}
	}
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
	dictAny, err := extract.Dict(pdf.CursorAt(x, nil), fontDictRef, false)
	if err != nil {
		t.Fatal(err)
	}
	d2 := dictAny.(*dict.Type1)

	// == Compare ==

	// Text, glyph and width for unused codes are arbitrary after roundtrip.
	// We compare these manually here, and zero the values before the comparison
	// below.
	text1 := dict.SimpleTextMap(d1.PostScriptName, d1.Encoding, d1.ToUnicode)
	text2 := dict.SimpleTextMap(d2.PostScriptName, d2.Encoding, d2.ToUnicode)
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

	// Compare FontFile types but exclude WriteTo functions
	if (d1.FontFile == nil) != (d2.FontFile == nil) {
		t.Errorf("FontFile presence mismatch: d1=%v, d2=%v", d1.FontFile != nil, d2.FontFile != nil)
	}
	if d1.FontFile != nil && d2.FontFile != nil {
		if d1.FontFile.Type != d2.FontFile.Type {
			t.Errorf("FontFile type mismatch: %v != %v", d1.FontFile.Type, d2.FontFile.Type)
		}
	}
	d1.FontFile = nil
	d2.FontFile = nil

	if d := cmp.Diff(d1, d2); d != "" {
		t.Fatal(d)
	}
}

var t1Dicts = []*dict.Type1{
	{
		PostScriptName: "Test",
		Descriptor: &font.Descriptor{
			FontName:     "Test",
			IsFixedPitch: true,
			IsSerif:      false,
			IsSymbolic:   true,
			IsScript:     false,
			IsItalic:     true,
			IsAllCap:     false,
			IsSmallCap:   true,
			ForceBold:    false,
			FontBBox: rect.Rect{
				LLx: 0,
				LLy: -100,
				URx: 200,
				URy: 300,
			},
			ItalicAngle: 10,
			Ascent:      250,
			Descent:     -50,
			Leading:     450,
			CapHeight:   150,
			XHeight:     50,
			StemV:       75,
			StemH:       25,
		},
		Encoding: func(code byte) string {
			switch code {
			case 65:
				return "A"
			default:
				return ""
			}
		},
		Width: makeTestWidth(65, 100.0),
	},
	{
		PostScriptName: "MinionMM_366_465_11_",
		MultipleMaster: true,
		Descriptor: &font.Descriptor{
			FontName:     "MinionMM_366_465_11_",
			IsFixedPitch: false,
			IsSerif:      true,
			IsSymbolic:   false,
			FontBBox: rect.Rect{
				LLx: 0,
				LLy: -100,
				URx: 200,
				URy: 300,
			},
			Ascent:    250,
			Descent:   -50,
			CapHeight: 150,
		},
		Encoding: func(code byte) string {
			switch code {
			case 65:
				return "A"
			default:
				return ""
			}
		},
		Width: makeTestWidth(65, 600.0),
	},
	makeTestDictStandard("Courier"),
	makeTestDictStandard("Times-Roman"),
	makeTestDictStandard("Symbol"),
	{
		PostScriptName: "Toaster",
		SubsetTag:      "XXXXXX",
		Descriptor: &font.Descriptor{
			FontName:     "XXXXXX+Toaster",
			IsFixedPitch: true,
			FontBBox: rect.Rect{
				LLx: 0,
				LLy: -100,
				URx: 200,
				URy: 300,
			},
			Ascent:       250,
			Descent:      -50,
			CapHeight:    150,
			MissingWidth: 199,
		},
		Encoding: func(c byte) string { return pdfenc.Standard.Encoding[c] },
		Width:    makeConstWidth(199),
		FontFile: &glyphdata.Stream{
			Type: glyphdata.Type1,
			WriteTo: func(w io.Writer, length *glyphdata.Lengths) error {
				return nil // test stub
			},
		},
	},
	{
		PostScriptName: "Toaster",
		SubsetTag:      "XXXXXX",
		Descriptor: &font.Descriptor{
			FontName:     "XXXXXX+Toaster",
			IsFixedPitch: true,
			FontBBox: rect.Rect{
				LLx: 0,
				LLy: -100,
				URx: 200,
				URy: 300,
			},
			Ascent:       250,
			Descent:      -50,
			CapHeight:    150,
			MissingWidth: 199,
		},
		Encoding: func(c byte) string { return pdfenc.Standard.Encoding[c] },
		Width:    makeConstWidth(199),
		FontFile: &glyphdata.Stream{
			Type: glyphdata.CFFSimple,
			WriteTo: func(w io.Writer, length *glyphdata.Lengths) error {
				return nil // test stub
			},
		},
	},
	{
		PostScriptName: "Trickster",
		SubsetTag:      "XXXXXX",
		Descriptor: &font.Descriptor{
			FontName:     "XXXXXX+Trickster",
			IsFixedPitch: true,
			FontBBox: rect.Rect{
				LLx: 0,
				LLy: -100,
				URx: 200,
				URy: 300,
			},
			Ascent:       250,
			Descent:      -50,
			CapHeight:    150,
			MissingWidth: 199,
		},
		Encoding: func(c byte) string { return pdfenc.Standard.Encoding[c] },
		Width:    makeConstWidth(199),
		FontFile: &glyphdata.Stream{
			Type: glyphdata.OpenTypeCFFSimple,
			WriteTo: func(w io.Writer, length *glyphdata.Lengths) error {
				return nil // test stub
			},
		},
	},
}

func makeTestWidth(args ...any) (ww [256]float64) {
	for i := 0; i+1 < len(args); i += 2 {
		code := args[i].(int)
		width := args[i+1].(float64)
		ww[code] = width
	}
	return
}

func makeConstWidth(dw float64) (ww [256]float64) {
	for i := range ww {
		ww[i] = dw
	}
	return
}

func makeTestDictStandard(fontName string) *dict.Type1 {
	stdInfo := stdmtx.Metrics[fontName]

	type g struct {
		code  byte
		name  string
		width float64
	}
	var gg []g
	for code, name := range stdInfo.Encoding {
		if name == "" || name == ".notdef" {
			continue
		}
		width := stdInfo.Width[name]
		gg = append(gg, g{byte(code), name, width})
		if len(gg) > 5 {
			break
		}
	}
	// use a non-trivial encoding
	gg[0].code, gg[1].code = gg[1].code, gg[0].code

	enc := make(map[byte]string)
	for _, g := range gg {
		enc[g.code] = g.name
	}

	fd := &font.Descriptor{
		FontName:     fontName,
		FontFamily:   stdInfo.FontFamily,
		FontStretch:  os2.WidthNormal,
		FontWeight:   stdInfo.FontWeight,
		IsFixedPitch: stdInfo.IsFixedPitch,
		IsSerif:      stdInfo.IsSerif,
		IsItalic:     stdInfo.ItalicAngle != 0,
		IsSymbolic:   stdInfo.IsSymbolic,
		FontBBox:     stdInfo.FontBBox,
		ItalicAngle:  stdInfo.ItalicAngle,
		Ascent:       stdInfo.Ascent,
		Descent:      stdInfo.Descent,
		CapHeight:    stdInfo.CapHeight,
		XHeight:      stdInfo.XHeight,
		StemV:        stdInfo.StemV,
		StemH:        stdInfo.StemH,
		MissingWidth: stdInfo.Width[".notdef"],
	}
	d := &dict.Type1{
		PostScriptName: fontName,
		Descriptor:     fd,
		Encoding: func(code byte) string {
			return enc[code]
		},
	}
	for _, g := range gg {
		d.Width[g.code] = g.width
	}

	return d
}

func clone[T any](v *T) *T {
	copy := *v
	return &copy
}
