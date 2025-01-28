// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package simple_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/simple"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestType1DictRoundtrip(t *testing.T) {
	data, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(data)

	F1 := makefont.Type1()
	M1 := makefont.AFM()
	fd := &font.Descriptor{
		FontName:     F1.PostScriptName(),
		FontFamily:   F1.FamilyName,
		IsFixedPitch: F1.IsFixedPitch,
		IsSymbolic:   true,
		FontBBox:     F1.FontBBoxPDF(),
		Ascent:       M1.Ascent,
		Descent:      M1.Descent,
		CapHeight:    M1.CapHeight,
		XHeight:      M1.XHeight,
		StemV:        F1.Private.StdVW * (F1.FontMatrix[0] * 1000),
		StemH:        F1.Private.StdHW * (F1.FontMatrix[3] * 1000),
		MissingWidth: F1.GlyphWidthPDF(".notdef"),
	}
	dicts1 := &simple.Type1Dict{
		Ref:            data.Alloc(),
		PostScriptName: F1.PostScriptName(),
		Descriptor:     fd,
		Encoding:       encoding.Standard,
		Width:          [256]float64{}, // TODO(voss): fill in
		Text:           [256]string{},  // TODO(voss): fill in
		GetFont: func() (any, error) {
			return F1, nil
		},
	}
	err := dicts1.WriteToPDF(rm)
	if err != nil {
		t.Fatal(err)
	}

	dicts2, err := simple.ExtractType1Dict(data, dicts1.Ref)
	if err != nil {
		t.Fatal(err)
	}

	compareType1Dicts(t, dicts1, dicts2)

	F2Interface, err := dicts2.GetFont()
	if err != nil {
		t.Fatal(err)
	}
	F2, ok := F2Interface.(*type1.Font)
	if !ok {
		t.Fatalf("got %T, want *type1.Font", F2Interface)
	}
	compareType1Fonts(t, F1, F2)
}

// compareType1Dicts compares two Type1Dicts.
// d1 must be the original, d2 the one that was read back from the PDF file.
func compareType1Dicts(t *testing.T, d1, d2 *simple.Type1Dict) {
	t.Helper()

	if d1.Ref != d2.Ref {
		t.Errorf("Ref: got %s, want %s", d2.Ref, d1.Ref)
	}
	if d1.PostScriptName != d2.PostScriptName {
		t.Errorf("PostScriptName: got %s, want %s", d2.PostScriptName, d1.PostScriptName)
	}
	if d1.SubsetTag != d2.SubsetTag {
		t.Errorf("SubsetTag: got %s, want %s", d2.SubsetTag, d1.SubsetTag)
	}
	if d1.Name != d2.Name {
		t.Errorf("Name: got %s, want %s", d2.Name, d1.Name)
	}
	if d := cmp.Diff(d1.Descriptor, d2.Descriptor); d != "" {
		t.Errorf("Descriptor: %s", d)
	}

	// compare encoding, widths and text
	for code := range 256 {
		if d1.Encoding(byte(code)) == "" {
			// unused codes are not guaranteed to have correct values
			continue
		}

		name1 := d1.Encoding(byte(code))
		name2 := d2.Encoding(byte(code))
		if name1 != name2 {
			t.Errorf("Encoding(%d): got %s, want %s", code, name2, name1)
		}

		w1 := d1.Width[code]
		w2 := d2.Width[code]
		if w1 != w2 {
			t.Errorf("Widths[%d]: got %f, want %f", code, w2, w1)
		}

		text1 := d1.Text[code]
		text2 := d2.Text[code]
		if text1 != text2 {
			t.Errorf("Text[%d]: got %s, want %s", code, text2, text1)
		}
	}
}

// compareType1Fonts compares two *type1.Font objects.
func compareType1Fonts(t *testing.T, f1, f2 *type1.Font) {
	if d := cmp.Diff(f1.FontInfo, f2.FontInfo); d != "" {
		t.Errorf("FontInfo: %s", d)
	}

	glyphs1 := f1.GlyphList()
	glyphs2 := f2.GlyphList()
	if d := cmp.Diff(glyphs1, glyphs2); d != "" {
		t.Errorf("GlyphList: %s", d)
	}

	// TODO(voss): why are the actually glyphs slightly different?
	// (Apparently this is caused by discretisation errors.)

	if d := cmp.Diff(f1.Private, f2.Private); d != "" {
		t.Errorf("Private: %s", d)
	}
	if d := cmp.Diff(f1.Encoding, f2.Encoding); d != "" {
		t.Errorf("Encoding: %s", d)
	}
	if f1.CreationDate != f2.CreationDate {
		t.Errorf("CreationDate: got %s, want %s", f2.CreationDate, f1.CreationDate)
	}
}
