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
