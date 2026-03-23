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

	// path 2: extract as property.List, then re-extract as *Group via ListGet
	props, err := pdf.ExtractorGet(x, nil, ocgRef, property.ExtractList)
	if err != nil {
		t.Fatalf("extract List: %v", err)
	}

	group2, err := property.ListGet(props, ExtractGroup)
	if err != nil {
		t.Fatalf("ListGet Group: %v", err)
	}

	// the two *Group pointers must be identical
	if group1 != group2 {
		t.Error("OCG pointers differ: extraction from OCProperties and from Properties resource returned different *Group objects")
	}
}

// TestVEGroupPointerIdentity verifies that groups extracted from a
// visibility expression have the same pointer identity as groups
// extracted directly.  This is essential for OCG state evaluation.
func TestVEGroupPointerIdentity(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	// create an OCG
	ocgDict := pdf.Dict{
		"Type": pdf.Name("OCG"),
		"Name": pdf.TextString("Layer"),
	}
	ocgRef := w.Alloc()
	if err := w.Put(ocgRef, ocgDict); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)

	// extract as *Group (simulates OCProperties)
	group1, err := pdf.ExtractorGet(x, nil, ocgRef, ExtractGroup)
	if err != nil {
		t.Fatal(err)
	}

	// create an OCMD with a VE referencing the same OCG
	veArray := pdf.Array{
		pdf.Name("Not"),
		ocgRef,
	}
	ocmdDict := pdf.Dict{
		"Type": pdf.Name("OCMD"),
		"VE":   veArray,
	}

	// extract the OCMD
	md, err := ExtractMembership(x, nil, ocmdDict, true)
	if err != nil {
		t.Fatal(err)
	}

	// the VE should contain the same *Group pointer
	notExpr, ok := md.VE.(*VisibilityExpressionNot)
	if !ok {
		t.Fatalf("VE is %T, want *VisibilityExpressionNot", md.VE)
	}
	groupExpr, ok := notExpr.Arg.(*VisibilityExpressionGroup)
	if !ok {
		t.Fatalf("VE arg is %T, want *VisibilityExpressionGroup", notExpr.Arg)
	}

	if groupExpr.Group != group1 {
		t.Error("VE group pointer differs from directly extracted group")
	}
}
