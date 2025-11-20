package operator

import (
	"iter"
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/resource"
)

func TestTextOperators_BeginEnd(t *testing.T) {
	state := &State{CurrentObject: objPage}
	res := &resource.Resource{}

	// Begin text
	opBT := Operator{Name: "BT", Args: nil}
	if err := ApplyOperator(state, opBT, res); err != nil {
		t.Fatalf("BT operator failed: %v", err)
	}

	if state.CurrentObject != objText {
		t.Errorf("CurrentObject = %v, want objText", state.CurrentObject)
	}
	if state.Param.TextMatrix != matrix.Identity {
		t.Error("TextMatrix not reset to identity")
	}
	if state.Out&graphics.StateTextMatrix == 0 {
		t.Error("StateTextMatrix not marked in Out")
	}

	// End text
	opET := Operator{Name: "ET", Args: nil}
	if err := ApplyOperator(state, opET, res); err != nil {
		t.Fatalf("ET operator failed: %v", err)
	}

	if state.CurrentObject != objPage {
		t.Errorf("CurrentObject = %v, want objPage", state.CurrentObject)
	}
	if state.Out&graphics.StateTextMatrix != 0 {
		t.Error("StateTextMatrix still marked after ET")
	}
}

func TestTextOperators_SetFont(t *testing.T) {
	state := &State{}
	mockFont := &mockFontInstance{}
	res := &resource.Resource{
		Font: map[pdf.Name]font.Instance{
			"F1": mockFont,
		},
	}

	op := Operator{
		Name: "Tf",
		Args: []pdf.Native{pdf.Name("F1"), pdf.Real(12.0)},
	}

	if err := ApplyOperator(state, op, res); err != nil {
		t.Fatalf("Tf operator failed: %v", err)
	}

	if state.Param.TextFont != mockFont {
		t.Error("TextFont not set")
	}
	if state.Param.TextFontSize != 12.0 {
		t.Errorf("TextFontSize = %v, want 12.0", state.Param.TextFontSize)
	}
	if state.Out&graphics.StateTextFont == 0 {
		t.Error("StateTextFont not marked in Out")
	}
}

// mockFontInstance for testing
type mockFontInstance struct{}

func (m *mockFontInstance) PostScriptName() string                     { return "MockFont" }
func (m *mockFontInstance) WritingMode() font.WritingMode              { return font.Horizontal }
func (m *mockFontInstance) Codec() *charcode.Codec                     { return nil }
func (m *mockFontInstance) Codes(s pdf.String) iter.Seq[*font.Code]    { return nil }
func (m *mockFontInstance) FontInfo() any                              { return nil }
func (m *mockFontInstance) Embed(*pdf.EmbedHelper) (pdf.Native, error) { return nil, nil }
