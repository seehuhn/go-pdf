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

package simple

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/stdmtx"
	"seehuhn.de/go/sfnt/os2"
)

func TestType1Roundtrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for i, d := range testDicts {
			t.Run(fmt.Sprintf("D%dv%s-%s", i, v, d.PostScriptName), func(t *testing.T) {
				w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

				// == Write ==

				d := clone(d)
				d.Ref = w.Alloc()

				rm := pdf.NewResourceManager(w)
				err := d.WriteToPDF(rm)
				if err != nil {
					t.Fatal(err)
				}
				err = rm.Close()
				if err != nil {
					t.Fatal(err)
				}

				// == Read ==

				e, err := ExtractType1Dict(w, d.Ref)
				if err != nil {
					t.Fatal(err)
				}

				// Text and glyph for unused codes are arbitrary after roundtrip.
				// We compare these manually here, and zero the values for the comparison
				// below.
				for code := range 256 {
					if d.Encoding(byte(code)) != "" {
						if d.Encoding(byte(code)) != e.Encoding(byte(code)) {
							t.Errorf("glyphName[%d]: %q != %q", code, d.Encoding(byte(code)), e.Encoding(byte(code)))
						}
						if d.Text[code] != e.Text[code] {
							t.Errorf("text[%d]: %q != %q", code, d.Text[code], e.Text[code])
						}
						if d.Width[code] != e.Width[code] {
							t.Errorf("width[%d]: %f != %f", code, d.Width[code], e.Width[code])
						}
					}

					d.Text[code] = ""
					e.Text[code] = ""
					d.Width[code] = 0
					e.Width[code] = 0
				}

				d.Encoding = nil
				e.Encoding = nil

				if d := cmp.Diff(d, e); d != "" {
					t.Fatal(d)
				}
			})
		}
	}
}

var testDicts = []*Type1Dict{
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
		Width: makeTestWidth(65, 100.0, 66, 100.0),
		Text:  makeTestText(65, "A", "B"),
	},
	makeTestDictStandard("Courier"),
	makeTestDictStandard("Times-Roman"),
	makeTestDictStandard("Symbol"),
}

func makeTestWidth(args ...any) (ww [256]float64) {
	for i := 0; i+1 < len(args); i += 2 {
		code := args[i].(int)
		width := args[i+1].(float64)
		ww[code] = width
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

func makeTestDictStandard(fontName string) *Type1Dict {
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
	if stdInfo.FontFamily == "Symbol" || stdInfo.FontFamily == "ZapfDingbats" {
		fd.IsSymbolic = true
	}
	d := &Type1Dict{
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
