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

package oc

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/property"
)

// TestOCGPointerIdentity verifies that extracting the same OCG via
// OCProperties and via a content stream's Properties resource returns
// the same *Group pointer, thanks to the extractor cache.
func TestOCGPointerIdentity(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	// create an OCG dictionary as an indirect object
	ocgDict := pdf.Dict{
		"Type": pdf.Name("OCG"),
		"Name": pdf.TextString("TestLayer"),
	}
	ocgRef := w.Alloc()
	err := w.Put(ocgRef, ocgDict)
	if err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)

	// path 1: extract as *Group (simulates OCProperties extraction)
	group1, err := pdf.ExtractorGet(x, nil, ocgRef, ExtractGroup)
	if err != nil {
		t.Fatalf("extract Group: %v", err)
	}

	// path 2: extract as property.List (simulates resource Properties extraction)
	props, err := pdf.ExtractorGet(x, nil, ocgRef, property.ExtractList)
	if err != nil {
		t.Fatalf("extract List: %v", err)
	}

	// recover the reference from the property list
	ref := props.Ref()
	if ref == 0 {
		t.Fatal("Ref() returned zero for indirect property list")
	}

	// re-extract as *Group using the reference
	group2, err := pdf.ExtractorGet(x, nil, ref, ExtractGroup)
	if err != nil {
		t.Fatalf("re-extract Group: %v", err)
	}

	// the two *Group pointers must be identical
	if group1 != group2 {
		t.Error("OCG pointers differ: extraction from OCProperties and from Properties resource returned different *Group objects")
	}
}
