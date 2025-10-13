package action

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestTargetParentEncode(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	target := &TargetParent{}

	obj, err := target.Encode(rm)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("expected Dict, got %T", obj)
	}

	if dict["R"] != pdf.Name("P") {
		t.Errorf("R = %v, want P", dict["R"])
	}
}

func TestTargetNamedChildEncode(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	target := &TargetNamedChild{
		Name: pdf.String("embedded.pdf"),
	}

	obj, err := target.Encode(rm)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("expected Dict, got %T", obj)
	}

	if dict["R"] != pdf.Name("C") {
		t.Errorf("R = %v, want C", dict["R"])
	}

	name, ok := dict["N"].(pdf.String)
	if !ok {
		t.Fatalf("N is not a pdf.String, got %T", dict["N"])
	}
	if string(name) != "embedded.pdf" {
		t.Errorf("N = %v, want embedded.pdf", name)
	}
}

func TestTargetNamedChildEmptyName(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	target := &TargetNamedChild{
		Name: pdf.String(""),
	}

	_, err := target.Encode(rm)
	if err == nil {
		t.Error("expected error for empty Name, got nil")
	}
}

func TestTargetAnnotationChildEncode(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	target := &TargetAnnotationChild{
		Page:       pdf.Integer(5),
		Annotation: pdf.String("attach1"),
	}

	obj, err := target.Encode(rm)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("expected Dict, got %T", obj)
	}

	if dict["R"] != pdf.Name("C") {
		t.Errorf("R = %v, want C", dict["R"])
	}

	if dict["P"] != pdf.Integer(5) {
		t.Errorf("P = %v, want 5", dict["P"])
	}

	annot, ok := dict["A"].(pdf.String)
	if !ok {
		t.Fatalf("A is not a pdf.String, got %T", dict["A"])
	}
	if string(annot) != "attach1" {
		t.Errorf("A = %v, want attach1", annot)
	}
}

func TestTargetAnnotationChildMissingFields(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	tests := []struct {
		name   string
		target *TargetAnnotationChild
	}{
		{"missing page", &TargetAnnotationChild{Annotation: pdf.Integer(0)}},
		{"missing annotation", &TargetAnnotationChild{Page: pdf.Integer(0)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.target.Encode(rm)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
