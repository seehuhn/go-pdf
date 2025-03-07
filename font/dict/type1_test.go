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

package dict

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

func TestType1Roundtrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for i, d := range t1Dicts {
			if v >= pdf.V2_0 && d.Name != "" {
				continue
			}

			t.Run(fmt.Sprintf("D%dv%s-%s", i, v, d.PostScriptName), func(t *testing.T) {
				checkRoundtripT1(t, d, v)
			})
		}
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
			rm := pdf.NewResourceManager(w)
			fontDictRef := w.Alloc()

			d := clone(d)
			if d.FontRef != 0 {
				d.FontRef = w.Alloc()
				// write a fake font data stream
				var subtype pdf.Object
				switch d.FontType {
				case glyphdata.CFFSimple:
					subtype = pdf.Name("Type1C")
				case glyphdata.OpenTypeCFFSimple:
					subtype = pdf.Name("OpenType")
				}
				stm, err := w.OpenStream(d.FontRef, pdf.Dict{"Subtype": subtype})
				if err != nil {
					f.Fatal(err)
				}
				err = stm.Close()
				if err != nil {
					f.Fatal(err)
				}
			}
			err = d.WriteToPDF(rm, fontDictRef)
			if err != nil {
				f.Fatal(err)
			}
			err = rm.Close()
			if err != nil {
				f.Fatal(err)
			}

			w.GetMeta().Trailer["Seeh:X"] = fontDictRef

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
		r, err := pdf.NewReader(bytes.NewReader(fileData), opt)
		if err != nil {
			t.Skip("broken PDF: " + err.Error())
		}
		obj := r.GetMeta().Trailer["Seeh:X"]
		if obj == nil {
			pdf.Format(os.Stdout, pdf.OptPretty, r.GetMeta().Trailer)
			t.Skip("broken reference")
		}
		d, err := ExtractType1(r, obj)
		if err != nil {
			t.Skip("no valid Type1Dict")
		}

		// Make sure we can write the dict, and read it back.
		checkRoundtripT1(t, d, pdf.GetVersion(r))
	})
}

func checkRoundtripT1(t *testing.T, d1 *Type1, v pdf.Version) {
	d1 = clone(d1)

	w, _ := memfile.NewPDFWriter(v, nil)
	rm := pdf.NewResourceManager(w)
	fontDictRef := w.Alloc()

	// == Write ==

	if d1.FontRef != 0 {
		d1.FontRef = w.Alloc()
		// write a fake font data stream
		var subtype pdf.Object
		switch d1.FontType {
		case glyphdata.CFFSimple:
			subtype = pdf.Name("Type1C")
		case glyphdata.OpenTypeCFFSimple:
			subtype = pdf.Name("OpenType")
		}
		stm, err := w.OpenStream(d1.FontRef, pdf.Dict{"Subtype": subtype})
		if err != nil {
			t.Fatal(err)
		}
		err = stm.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	err := d1.WriteToPDF(rm, fontDictRef)
	if err != nil {
		t.Fatal(err)
	}
	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	// == Read ==

	d2, err := ExtractType1(w, fontDictRef)
	if err != nil {
		t.Fatal(err)
	}

	// == Compare ==

	// Text and glyph for unused codes are arbitrary after roundtrip.
	// We compare these manually here, and zero the values for the comparison
	// below.
	for code := range 256 {
		if d1.Encoding(byte(code)) != "" {
			if d1.Encoding(byte(code)) != d2.Encoding(byte(code)) {
				t.Errorf("glyphName[%d]: %q != %q", code, d1.Encoding(byte(code)), d2.Encoding(byte(code)))
			}
			if d1.Text[code] != "" && d1.Text[code] != d2.Text[code] {
				t.Errorf("text[%d]: %q != %q", code, d1.Text[code], d2.Text[code])
			}
			if d1.Width[code] != d2.Width[code] {
				t.Errorf("width[%d]: %f != %f", code, d1.Width[code], d2.Width[code])
			}
		}

		d1.Text[code] = ""
		d2.Text[code] = ""
		d1.Width[code] = 0
		d2.Width[code] = 0
	}
	d1.Encoding = nil
	d2.Encoding = nil

	if d := cmp.Diff(d1, d2); d != "" {
		t.Fatal(d)
	}
}

var t1Dicts = []*Type1{
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
		Text:  makeTestText(65, "A"),
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
		Text:     makeTestText(65, "A"),
		FontType: glyphdata.Type1,
		FontRef:  pdf.NewReference(999, 0),
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
		Text:     makeTestText(65, "A"),
		FontType: glyphdata.CFFSimple,
		FontRef:  pdf.NewReference(999, 0),
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
		Text:     makeTestText(65, "A"),
		FontType: glyphdata.OpenTypeCFFSimple,
		FontRef:  pdf.NewReference(999, 0),
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

func makeTestText(args ...any) (tt [256]string) {
	for i := 0; i+1 < len(args); i += 2 {
		code := args[i].(int)
		text := args[i+1].(string)
		tt[code] = text
	}
	return
}

func makeTestDictStandard(fontName string) *Type1 {
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
	d := &Type1{
		PostScriptName: fontName,
		Descriptor:     fd,
		Encoding: func(code byte) string {
			return enc[code]
		},
	}
	for _, g := range gg {
		d.Width[g.code] = g.width
		d.Text[g.code] = g.name
	}

	return d
}

func clone[T any](v *T) *T {
	copy := *v
	return &copy
}
