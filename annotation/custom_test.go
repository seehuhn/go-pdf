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

package annotation

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestCustomAnnotation(t *testing.T) {
	// Create an unknown annotation type
	customDict := pdf.Dict{
		"Subtype":     pdf.Name("CustomType"),
		"Rect":        pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(100), pdf.Number(50)},
		"CustomField": pdf.TextString("custom value"),
	}

	buf, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	// Add the dictionary directly to the PDF
	ref := buf.Alloc()
	buf.Put(ref, customDict)

	err := buf.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Extract should return an Unknown annotation
	x := pdf.NewExtractor(buf)
	annotation, err := Decode(x, ref)
	if err != nil {
		t.Fatal(err)
	}

	custom, ok := annotation.(*Custom)
	if !ok {
		t.Fatalf("expected *Unknown, got %T", annotation)
	}

	// Check that the annotation type is correct
	if custom.AnnotationType() != "CustomType" {
		t.Errorf("expected annotation type 'CustomType', got '%s'", custom.AnnotationType())
	}

	// Check that the custom field is preserved
	if customField := custom.Data["CustomField"]; customField == nil {
		t.Error("custom field not preserved")
	}

	// For unknown annotations, we don't expect perfect round-trip
	// because the embedding process may add common annotation fields
	// Just verify it doesn't crash and basic fields are preserved
	buf2, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm2 := pdf.NewResourceManager(buf2)

	_, err = custom.Encode(rm2)
	if err != nil {
		t.Errorf("failed to embed unknown annotation: %v", err)
	}
}
