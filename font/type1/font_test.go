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
	"math"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/gofont"
)

// TestToUnicode verifies that the ToUnicode cmap is only generated if
// necessary, and that in this case it is works.
func TestToUnicode(t *testing.T) {
	F := TimesRoman
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for _, X := range []string{"A", "B"} {
			t.Run(v.String()+X, func(t *testing.T) {
				data := pdf.NewData(v)

				E, err := F.Embed(data, "")
				if err != nil {
					t.Fatal(err)
				}

				l := E.Layout(10, "AB")
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
			E, err := F.Embed(data, "")
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

func TestRoundTrip(t *testing.T) {
	t1, err := gofont.Type1(gofont.GoRegular)
	if err != nil {
		t.Fatal(err)
	}

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

	info1 := &EmbedInfo{
		Font:      t1,
		SubsetTag: "UVWXYZ",
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

	for _, info := range []*EmbedInfo{info1, info2} {
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

	info1 := &EmbedInfo{
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

	for _, info := range []*EmbedInfo{info1, info2} {
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

var _ font.Embedded = (*embeddedSimple)(nil)
