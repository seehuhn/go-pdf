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

package type1_test

import (
	"strings"
	"testing"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/fonttypes"
	"seehuhn.de/go/pdf/reader"
)

func TestEmbed(t *testing.T) {
	// step 1: embed a font instance into a simple PDF file
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	fontData := makefont.Type1()
	fontMetrics := makefont.AFM()
	fontInstance, err := type1.New(fontData, fontMetrics, nil)
	if err != nil {
		t.Fatal(err)
	}

	ref, _, err := pdf.ResourceManagerEmbed(rm, fontInstance)
	if err != nil {
		t.Fatal(err)
	}

	// make sure a few glyphs are included and encoded
	fontInstance.Layout(nil, 12, "Hello")

	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	// step 2: read back the font and verify that everything is as expected
	dict, err := dict.ExtractType1(w, ref)
	if err != nil {
		t.Fatal(err)
	}

	if dict.Ref != ref {
		t.Errorf("wrong object reference: expected %v, got %v", ref, dict.Ref)
	}
	if dict.PostScriptName != fontData.PostScriptName() {
		t.Errorf("wrong PostScript name: expected %v, got %v",
			fontData.PostScriptName(), dict.PostScriptName)
	}
	if len(dict.SubsetTag) != 6 {
		t.Errorf("wrong subset tag: %q", dict.SubsetTag)
	}

	// TODO(voss): more tests
}

func TestTextContent(t *testing.T) {
	text := `“Hello World!”`

	// step 1: embed a Type 1 font into a simple PDF document
	// and make sure all the characters from the text are included.
	mem := memfile.New()
	page, err := document.WriteSinglePage(mem, document.A5, pdf.V2_0, nil)
	if err != nil {
		t.Fatal(err)
	}
	pageRef := page.Out.Alloc() // fix the reference for the page dictionary
	page.Ref = pageRef

	F := fonttypes.Type1WithoutMetrics(page.RM)
	page.TextBegin()
	page.TextSetFont(F, 12)
	page.TextFirstLine(100, 100)
	page.TextShow(text)
	page.TextEnd()

	// keep a reference to the font
	ref, _, _ := pdf.ResourceManagerEmbed(page.RM, F)

	err = page.Close()
	if err != nil {
		t.Fatal(err)
	}

	// step 2: extract the encoded string from the content stream
	var textString pdf.String
	r := reader.New(page.Out, nil)
	r.EveryOp = func(op string, args []pdf.Object) error {
		if op == "Tj" {
			textString = args[0].(pdf.String)
		}
		return nil
	}
	r.ParsePage(pageRef, matrix.Identity)

	// step 3: read back the font dictionary to inspect it.
	dict, err := dict.ExtractType1(page.Out, ref)
	if err != nil {
		t.Fatal(err)
	}

	s := &strings.Builder{}
	for _, code := range textString {
		s.WriteString(dict.Text[code])
	}
	if s.String() != text {
		t.Fatalf("expected %q, got %q", text, s.String())
	}
}
