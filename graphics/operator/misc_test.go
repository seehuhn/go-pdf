package operator

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/resource"
)

func TestMiscOperators_XObject(t *testing.T) {
	state := &State{}
	mockXObj := &mockXObject{}
	res := &resource.Resource{
		XObject: map[pdf.Name]graphics.XObject{
			"Im1": mockXObj,
		},
	}

	op := Operator{Name: "Do", Args: []pdf.Native{pdf.Name("Im1")}}
	if err := state.Apply(res, op); err != nil {
		t.Fatalf("Do operator failed: %v", err)
	}
}

func TestMiscOperators_MarkedContent(t *testing.T) {
	state := &State{}
	res := &resource.Resource{}

	// BMC
	opBMC := Operator{Name: "BMC", Args: []pdf.Native{pdf.Name("Tag1")}}
	if err := state.Apply(res, opBMC); err != nil {
		t.Fatalf("BMC operator failed: %v", err)
	}

	// EMC
	opEMC := Operator{Name: "EMC", Args: nil}
	if err := state.Apply(res, opEMC); err != nil {
		t.Fatalf("EMC operator failed: %v", err)
	}
}

func TestMiscOperators_SpecialOperators(t *testing.T) {
	state := &State{}
	res := &resource.Resource{}

	// %raw%
	opRaw := Operator{Name: "%raw%", Args: []pdf.Native{pdf.String("  % comment\n")}}
	if err := state.Apply(res, opRaw); err != nil {
		t.Fatalf("%%raw%% operator failed: %v", err)
	}

	// %image%
	opImage := Operator{
		Name: "%image%",
		Args: []pdf.Native{
			pdf.Dict{"W": pdf.Integer(10), "H": pdf.Integer(10)},
			pdf.String("imagedata"),
		},
	}
	if err := state.Apply(res, opImage); err != nil {
		t.Fatalf("%%image%% operator failed: %v", err)
	}
}

// mockXObject for testing
type mockXObject struct{}

func (m *mockXObject) Embed(*pdf.EmbedHelper) (pdf.Native, error) { return nil, nil }
func (m *mockXObject) Subtype() pdf.Name                          { return "Form" }
