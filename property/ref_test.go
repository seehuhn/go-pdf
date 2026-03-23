// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package property

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestRefIndirect(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	// create an indirect OCG-like dict
	dict := pdf.Dict{
		"Type": pdf.Name("OCG"),
		"Name": pdf.TextString("Layer 1"),
	}
	ref := w.Alloc()
	err := w.Put(ref, dict)
	if err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)

	// extract through ExtractorGet (simulates resource extraction)
	props, err := pdf.ExtractorGet(x, nil, ref, ExtractList)
	if err != nil {
		t.Fatal(err)
	}

	got := props.Ref()
	if got != ref {
		t.Errorf("Ref() = %v, want %v", got, ref)
	}
}

func TestRefDirect(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	// direct (inline) extraction
	dict := pdf.Dict{
		"Type": pdf.Name("OCG"),
		"Name": pdf.TextString("Layer 1"),
	}
	props, err := ExtractList(x, nil, dict, true)
	if err != nil {
		t.Fatal(err)
	}

	got := props.Ref()
	if got != 0 {
		t.Errorf("Ref() = %v, want zero for direct property list", got)
	}
}

func TestRefDirectCalledWithoutExtractorGet(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	// calling ExtractList directly with isDirect=false but path=nil
	// (defensive check)
	dict := pdf.Dict{"X": pdf.Integer(1)}
	ref := w.Alloc()
	err := w.Put(ref, dict)
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := x.Resolve(nil, ref)
	if err != nil {
		t.Fatal(err)
	}

	props, err := ExtractList(x, nil, resolved, false)
	if err != nil {
		t.Fatal(err)
	}

	got := props.Ref()
	if got != 0 {
		t.Errorf("Ref() = %v, want zero when path is nil", got)
	}
}

func TestRefActualText(t *testing.T) {
	a := &ActualText{Text: "hello"}
	if got := a.Ref(); got != 0 {
		t.Errorf("ActualText.Ref() = %v, want zero", got)
	}
}

func TestRefAF(t *testing.T) {
	a := &AF{}
	if got := a.Ref(); got != 0 {
		t.Errorf("AF.Ref() = %v, want zero", got)
	}
}
