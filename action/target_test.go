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
