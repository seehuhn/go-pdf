package action

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/destination"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestActionListEncode_Empty(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	var al ActionList
	obj, err := al.Encode(rm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj != nil {
		t.Errorf("expected nil for empty ActionList, got %v", obj)
	}
}

func TestGoToAction(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	// create a simple XYZ destination
	dest := &destination.XYZ{
		Page: pdf.Reference(5),
		Left: 100,
		Top:  200,
		Zoom: 1.5,
	}

	action := &GoTo{
		Dest: dest,
	}

	// encode
	obj, err := action.Encode(rm)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("expected Dict, got %T", obj)
	}

	// verify S field
	if dict["S"] != pdf.Name("GoTo") {
		t.Errorf("S = %v, want GoTo", dict["S"])
	}

	// decode
	x := pdf.NewExtractor(w)
	decoded, err := Decode(x, dict)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	goToAction, ok := decoded.(*GoTo)
	if !ok {
		t.Fatalf("expected *GoTo, got %T", decoded)
	}

	if goToAction.ActionType() != TypeGoTo {
		t.Errorf("ActionType = %v, want %v", goToAction.ActionType(), TypeGoTo)
	}
}

func TestURIAction(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	action := &URI{
		URI:   "https://example.com",
		IsMap: true,
	}

	// encode
	obj, err := action.Encode(rm)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("expected Dict, got %T", obj)
	}

	if dict["S"] != pdf.Name("URI") {
		t.Errorf("S = %v, want URI", dict["S"])
	}
	if string(dict["URI"].(pdf.String)) != "https://example.com" {
		t.Errorf("URI = %v, want https://example.com", dict["URI"])
	}

	// decode
	x := pdf.NewExtractor(w)
	decoded, err := Decode(x, dict)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	uriAction, ok := decoded.(*URI)
	if !ok {
		t.Fatalf("expected *URI, got %T", decoded)
	}

	if uriAction.URI != "https://example.com" {
		t.Errorf("URI = %v, want https://example.com", uriAction.URI)
	}
	if !uriAction.IsMap {
		t.Errorf("IsMap = false, want true")
	}
}

func TestNamedAction(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	action := &Named{
		N: "NextPage",
	}

	obj, err := action.Encode(rm)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("expected Dict, got %T", obj)
	}

	if dict["N"] != pdf.Name("NextPage") {
		t.Errorf("N = %v, want NextPage", dict["N"])
	}

	x := pdf.NewExtractor(w)
	decoded, err := Decode(x, dict)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	namedAction := decoded.(*Named)
	if namedAction.N != "NextPage" {
		t.Errorf("N = %v, want NextPage", namedAction.N)
	}
}

func TestGoToRAction(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	action := &GoToR{
		F: pdf.String("other.pdf"),
		D: pdf.Array{pdf.Integer(0), pdf.Name("Fit")},
	}

	obj, err := action.Encode(rm)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("expected Dict, got %T", obj)
	}

	if dict["S"] != pdf.Name("GoToR") {
		t.Errorf("S = %v, want GoToR", dict["S"])
	}

	x := pdf.NewExtractor(w)
	decoded, err := Decode(x, dict)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	goToRAction := decoded.(*GoToR)
	if goToRAction.F == nil {
		t.Error("F is nil")
	}
}
