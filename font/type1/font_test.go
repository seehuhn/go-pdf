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

package type1

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/internal/makefont"
	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/type1"
)

// TestEmbed checks that the font can be embedded into a PDF file.
func TestEmbed(t *testing.T) {
	psFont := makefont.Type1()
	metrics := makefont.AFM()

	for i := 1; i <= 3; i++ {
		t.Run(fmt.Sprintf("%02b", i), func(t *testing.T) {
			var psf *type1.Font
			var metr *afm.Metrics

			// try all allowed combinations of psfont and metrics
			if includeFont := i&1 != 0; includeFont {
				psf = psFont
			}
			if useMetrics := i&2 != 0; useMetrics {
				metr = metrics
			}
			F, err := New(psf, metr)
			if err != nil {
				t.Fatal(err)
			}

			// encode some characters and embed the font
			data := pdf.NewData(pdf.V1_7)
			E, err := F.Embed(data, nil)
			if err != nil {
				t.Fatal(err)
			}
			testString := "Hello, World!"
			gg := E.Layout(nil, 1, testString)
			var codes pdf.String
			for _, g := range gg.Seq {
				codes, _, _ = E.CodeAndWidth(codes, g.GID, g.Text)
			}
			err = E.Close()
			if err != nil {
				t.Fatal(err)
			}

			// read back the font and check the result
			fontDicts, err := font.ExtractDicts(data, E.PDFObject())
			if err != nil {
				t.Fatal(err)
			}
			if fontDicts.Type != font.Type1 {
				t.Errorf("wrong font type %s (instead of Type1)", fontDicts.Type)
			}
			if fontDicts.PostScriptName != pdf.Name(psFont.FontName) {
				t.Errorf("wrong font name: %q != %q",
					fontDicts.PostScriptName, psf.FontName)
			}

			info, err := Extract(data, fontDicts)
			if err != nil {
				t.Fatal(err)
			}
			if (info.Font != nil) != (psf != nil) {
				t.Errorf("font are included: %t, font should be included: %t",
					info.Font != nil, psf != nil)
			}
			if info.Metrics == nil {
				t.Error("metrics are missing")
			}
		})
	}
}

// TestToUnicode verifies that the ToUnicode cmap is only generated if
// necessary, and that in this case it is works.
func TestToUnicode(t *testing.T) {
	F := TimesRoman
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for _, X := range []string{"A", "B"} {
			t.Run(v.String()+X, func(t *testing.T) {
				data := pdf.NewData(v)

				E, err := F.Embed(data, nil)
				if err != nil {
					t.Fatal(err)
				}

				l := E.Layout(nil, 10, "AB")
				gg := l.Seq
				if len(gg) != 2 {
					panic("test is broken")
				}

				var codes pdf.String
				codes, _, _ = E.CodeAndWidth(codes, gg[0].GID, []rune("A"))
				codes, _, _ = E.CodeAndWidth(codes, gg[0].GID, []rune(X))
				codes, _, _ = E.CodeAndWidth(codes, gg[1].GID, []rune("B"))
				if len(codes) != 3 {
					panic("test is broken")
				}
				err = E.Close()
				if err != nil {
					t.Fatal(err)
				}

				fontDicts, err := font.ExtractDicts(data, E.PDFObject())
				if err != nil {
					t.Fatal(err)
				}
				info, err := Extract(data, fontDicts)
				if err != nil {
					t.Fatal(err)
				}

				needToUni := X != "A"
				if needToUni {
					if info.ToUnicode == nil {
						t.Fatal("ToUnicode cmap is missing")
					}
					m := info.ToUnicode.GetMappingNew()
					if !slices.Equal(m[string(codes[0:1])], []rune("A")) {
						t.Errorf("m[%d] != A: %q", codes[0], m[string(codes[0:1])])
					}
					if !slices.Equal(m[string(codes[1:2])], []rune(X)) {
						t.Errorf("m[%d] != %s: %q", codes[1], X, m[string(codes[1:2])])
					}
					if !slices.Equal(m[string(codes[2:3])], []rune("B")) {
						t.Errorf("m[%d] != B: %q", codes[2], m[string(codes[2:3])])
					}
				} else if info.ToUnicode != nil {
					t.Error("ToUnicode cmap is present")
				}
			})
		}
	}
}

// TestNotdefGlyph verifies that the ".notdef" glyph can be generated.
// This requires to allocate a code which is mapped to a non-existing glyph
// name.
func TestNotdefGlyph(t *testing.T) {
	F := TimesRoman

	// Try both the built-in version (PDF-1.7) and the embedded version (PDF-2.0)
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		t.Run(v.String(), func(t *testing.T) {
			data := pdf.NewData(v)
			E, err := F.Embed(data, nil)
			if err != nil {
				t.Fatal(err)
			}
			// Allocate codes for GID 0 and 2:
			var s pdf.String
			s, _, _ = E.CodeAndWidth(s, 0, nil)
			s, _, _ = E.CodeAndWidth(s, 5, []rune("test"))
			if len(s) != 2 {
				panic("test is broken")
			}
			code0 := s[0]
			code1 := s[1]
			err = E.Close()
			if err != nil {
				t.Fatal(err)
			}

			fontDicts, err := font.ExtractDicts(data, E.PDFObject())
			if err != nil {
				t.Fatal(err)
			}
			info, err := Extract(data, fontDicts)
			if err != nil {
				t.Fatal(err)
			}
			name0 := info.Encoding[code0]
			name1 := info.Encoding[code1]

			if info.Font != nil {
				if _, exists := info.Font.Glyphs[name0]; exists {
					t.Errorf("existing name %q used for code %d",
						name0, code0)
				}
				if _, exists := info.Font.Glyphs[name1]; !exists {
					t.Errorf("glyph %q (code %d) does not exist",
						name1, code1)
				}
			} else {
				if _, exists := info.Metrics.Glyphs[name0]; exists {
					t.Errorf("existing name %q used for code %d",
						name0, code0)
				}
				if _, exists := info.Metrics.Glyphs[name1]; !exists {
					t.Errorf("glyph %q (code %d) does not exist",
						name1, code1)
				}
			}
		})
	}
}

func TestDefaultFontRoundTrip(t *testing.T) {
	t1, err := TimesItalic.psFont()
	if err != nil {
		t.Fatal(err)
	}

	encoding := make([]string, 256)
	for i := range encoding {
		encoding[i] = ".notdef"
	}
	encoding[65] = "A"
	encoding[66] = "C"

	m := map[charcode.CharCode][]rune{
		65: {'A'},
		66: {'C'},
	}
	toUnicode := cmap.NewToUnicode(charcode.Simple, m)

	info1 := &FontDict{
		Font:      t1,
		Encoding:  encoding,
		ToUnicode: toUnicode,
	}

	rw := pdf.NewData(pdf.V1_7)
	ref := rw.Alloc()
	err = info1.Embed(rw, ref)
	if err != nil {
		t.Fatal(err)
	}

	dicts, err := font.ExtractDicts(rw, ref)
	if err != nil {
		t.Fatal(err)
	}
	info2, err := Extract(rw, dicts)
	if err != nil {
		t.Fatal(err)
	}

	// Compare encodings:
	if len(info1.Encoding) != len(info2.Encoding) {
		t.Fatalf("len(info1.Encoding) != len(info2.Encoding): %d != %d", len(info1.Encoding), len(info2.Encoding))
	}
	for i := range info1.Encoding {
		if info1.Encoding[i] != ".notdef" && info1.Encoding[i] != info2.Encoding[i] {
			t.Fatalf("info1.Encoding[%d] != info2.Encoding[%d]: %q != %q", i, i, info1.Encoding[i], info2.Encoding[i])
		}
	}

	for _, info := range []*FontDict{info1, info2} {
		info.Encoding = nil // already compared above
		info.Metrics = nil  // TODO(voss): re-enable this once it works
	}

	cmpFloat := cmp.Comparer(func(x, y float64) bool {
		return math.Abs(x-y) < 1/65536.
	})
	if d := cmp.Diff(info1, info2, cmpFloat); d != "" {
		t.Errorf("info mismatch (-want +got):\n%s", d)
	}
}

// TestEncoding checks that the encoding of a Type 1 font is the standard
// encoding, if the set of included characters is in the standard encoding.
func TestEncoding(t *testing.T) {
	t1 := makefont.Type1()
	metrics := makefont.AFM()
	F, err := New(t1, metrics)
	if err != nil {
		t.Fatal(err)
	}

	// Embed the font
	data := pdf.NewData(pdf.V1_7)
	E, err := F.Embed(data, nil)
	if err != nil {
		t.Fatal(err)
	}
	gg := E.Layout(nil, 10, ".MiAbc")
	for _, g := range gg.Seq {
		E.CodeAndWidth(nil, g.GID, g.Text) // allocate codes
	}
	err = E.Close()
	if err != nil {
		t.Fatal(err)
	}

	dicts, err := font.ExtractDicts(data, E.PDFObject())
	if err != nil {
		t.Fatal(err)
	}
	info, err := Extract(data, dicts)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 256; i++ {
		if info.Encoding[i] != ".notdef" && info.Encoding[i] != pdfenc.StandardEncoding[i] {
			t.Error(i, info.Encoding[i])
		}
	}
}

func TestRoundTrip(t *testing.T) {
	t1 := makefont.Type1()

	encoding := make([]string, 256)
	for i := range encoding {
		encoding[i] = ".notdef"
	}
	encoding[65] = "A"
	encoding[66] = "B"

	m := map[charcode.CharCode][]rune{
		65: {'A'},
		66: {'B'},
	}
	toUnicode := cmap.NewToUnicode(charcode.Simple, m)

	info1 := &FontDict{
		Font:      t1,
		SubsetTag: "UVWXYZ",
		Encoding:  encoding,
		ToUnicode: toUnicode,
	}

	rw := pdf.NewData(pdf.V1_7)
	ref := rw.Alloc()
	err := info1.Embed(rw, ref)
	if err != nil {
		t.Fatal(err)
	}

	dicts, err := font.ExtractDicts(rw, ref)
	if err != nil {
		t.Fatal(err)
	}
	info2, err := Extract(rw, dicts)
	if err != nil {
		t.Fatal(err)
	}

	// Compare encodings:
	if len(info1.Encoding) != len(info2.Encoding) {
		t.Fatalf("len(info1.Encoding) != len(info2.Encoding): %d != %d", len(info1.Encoding), len(info2.Encoding))
	}
	for i := range info1.Encoding {
		if info1.Encoding[i] != ".notdef" && info1.Encoding[i] != info2.Encoding[i] {
			t.Fatalf("info1.Encoding[%d] != info2.Encoding[%d]: %q != %q", i, i, info1.Encoding[i], info2.Encoding[i])
		}
	}

	for _, info := range []*FontDict{info1, info2} {
		info.Encoding = nil // already compared above
		info.Metrics = nil  // TODO(voss): enable this once it works
	}

	cmpFloat := cmp.Comparer(func(x, y float64) bool {
		return math.Abs(x-y) < 1/65536.
	})
	if d := cmp.Diff(info1, info2, cmpFloat); d != "" {
		t.Errorf("info mismatch (-want +got):\n%s", d)
	}
}
