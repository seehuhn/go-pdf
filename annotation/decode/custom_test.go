// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestCustomAnnotation(t *testing.T) {
	// Create an unknown annotation type
	customDict := pdf.Dict{
		"Subtype":     pdf.Name("CustomType"),
		"Rect":        pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(100), pdf.Number(50)},
		"CustomField": pdf.Text("custom value"),
	}

	buf, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)

	// Add the dictionary directly to the PDF
	ref := buf.Alloc()
	buf.Put(ref, customDict)

	err := buf.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Extract should return an Unknown annotation
	x := pdf.NewExtractor(buf)
	annot, err := Annotation(pdf.CursorAt(x, nil), ref, false)
	if err != nil {
		t.Fatal(err)
	}

	custom, ok := annot.(*annotation.Custom)
	if !ok {
		t.Fatalf("expected *Unknown, got %T", annot)
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
	buf2, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm2 := pdf.NewResourceManager(buf2)

	_, err = custom.Encode(rm2)
	if err != nil {
		t.Errorf("failed to embed unknown annotation: %v", err)
	}
}
