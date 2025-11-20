package operator

import (
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/resource"
)

// TestTextOperators_RenderingModeDependencies verifies that text showing
// operators track dependencies based on rendering mode
func TestTextOperators_RenderingModeDependencies(t *testing.T) {
	tests := []struct {
		name            string
		renderingMode   graphics.TextRenderingMode
		expectFill      bool
		expectStroke    bool
		expectLineWidth bool
	}{
		{
			name:            "Fill mode",
			renderingMode:   graphics.TextRenderingModeFill,
			expectFill:      true,
			expectStroke:    false,
			expectLineWidth: false,
		},
		{
			name:            "Stroke mode",
			renderingMode:   graphics.TextRenderingModeStroke,
			expectFill:      false,
			expectStroke:    true,
			expectLineWidth: true,
		},
		{
			name:            "FillStroke mode",
			renderingMode:   graphics.TextRenderingModeFillStroke,
			expectFill:      true,
			expectStroke:    true,
			expectLineWidth: true,
		},
		{
			name:            "Invisible mode",
			renderingMode:   graphics.TextRenderingModeInvisible,
			expectFill:      false,
			expectStroke:    false,
			expectLineWidth: false,
		},
		{
			name:            "FillClip mode",
			renderingMode:   graphics.TextRenderingModeFillClip,
			expectFill:      true,
			expectStroke:    false,
			expectLineWidth: false,
		},
		{
			name:            "StrokeClip mode",
			renderingMode:   graphics.TextRenderingModeStrokeClip,
			expectFill:      false,
			expectStroke:    true,
			expectLineWidth: true,
		},
		{
			name:            "FillStrokeClip mode",
			renderingMode:   graphics.TextRenderingModeFillStrokeClip,
			expectFill:      true,
			expectStroke:    true,
			expectLineWidth: true,
		},
		{
			name:            "Clip mode",
			renderingMode:   graphics.TextRenderingModeClip,
			expectFill:      false,
			expectStroke:    false,
			expectLineWidth: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &State{CurrentObject: objText}
			state.Param.TextRenderingMode = tt.renderingMode
			res := &resource.Resource{}

			op := Operator{Name: "Tj", Args: []pdf.Native{pdf.String("test")}}
			if err := ApplyOperator(state, op, res); err != nil {
				t.Fatalf("Tj operator failed: %v", err)
			}

			// Check fill color dependency
			hasFill := state.In&graphics.StateFillColor != 0
			if hasFill != tt.expectFill {
				t.Errorf("FillColor dependency = %v, want %v", hasFill, tt.expectFill)
			}

			// Check stroke color dependency
			hasStroke := state.In&graphics.StateStrokeColor != 0
			if hasStroke != tt.expectStroke {
				t.Errorf("StrokeColor dependency = %v, want %v", hasStroke, tt.expectStroke)
			}

			// Check line width dependency
			hasLineWidth := state.In&graphics.StateLineWidth != 0
			if hasLineWidth != tt.expectLineWidth {
				t.Errorf("LineWidth dependency = %v, want %v", hasLineWidth, tt.expectLineWidth)
			}

			// All modes should require font and text matrix
			if state.In&graphics.StateTextFont == 0 {
				t.Error("TextFont dependency not marked")
			}
			if state.In&graphics.StateTextMatrix == 0 {
				t.Error("TextMatrix dependency not marked")
			}

			// TextMatrix should be marked as output
			if state.Out&graphics.StateTextMatrix == 0 {
				t.Error("TextMatrix not marked in Out")
			}
		})
	}
}

// TestTextOperators_TextPositioning verifies that text positioning operators
// modify TextMatrix correctly
func TestTextOperators_TextPositioning(t *testing.T) {
	t.Run("Td operator", func(t *testing.T) {
		state := &State{CurrentObject: objText}
		state.Param.TextLineMatrix = matrix.Identity
		state.Param.TextMatrix = matrix.Identity
		res := &resource.Resource{}

		op := Operator{Name: "Td", Args: []pdf.Native{pdf.Real(10.0), pdf.Real(20.0)}}
		if err := ApplyOperator(state, op, res); err != nil {
			t.Fatalf("Td operator failed: %v", err)
		}

		expected := matrix.Matrix{1, 0, 0, 1, 10, 20}
		if state.Param.TextMatrix != expected {
			t.Errorf("TextMatrix = %v, want %v", state.Param.TextMatrix, expected)
		}
		if state.Param.TextLineMatrix != expected {
			t.Errorf("TextLineMatrix = %v, want %v", state.Param.TextLineMatrix, expected)
		}
		if state.Out&graphics.StateTextMatrix == 0 {
			t.Error("StateTextMatrix not marked in Out")
		}
	})

	t.Run("TD operator", func(t *testing.T) {
		state := &State{CurrentObject: objText}
		state.Param.TextLineMatrix = matrix.Identity
		state.Param.TextMatrix = matrix.Identity
		res := &resource.Resource{}

		op := Operator{Name: "TD", Args: []pdf.Native{pdf.Real(10.0), pdf.Real(-5.0)}}
		if err := ApplyOperator(state, op, res); err != nil {
			t.Fatalf("TD operator failed: %v", err)
		}

		// TD should set leading to -ty (5.0)
		if state.Param.TextLeading != 5.0 {
			t.Errorf("TextLeading = %v, want 5.0", state.Param.TextLeading)
		}
		if state.Out&graphics.StateTextLeading == 0 {
			t.Error("StateTextLeading not marked in Out")
		}
	})

	t.Run("Tm operator", func(t *testing.T) {
		state := &State{CurrentObject: objText}
		res := &resource.Resource{}

		op := Operator{
			Name: "Tm",
			Args: []pdf.Native{
				pdf.Real(1.0), pdf.Real(0.0),
				pdf.Real(0.0), pdf.Real(1.0),
				pdf.Real(100.0), pdf.Real(200.0),
			},
		}
		if err := ApplyOperator(state, op, res); err != nil {
			t.Fatalf("Tm operator failed: %v", err)
		}

		expected := matrix.Matrix{1, 0, 0, 1, 100, 200}
		if state.Param.TextMatrix != expected {
			t.Errorf("TextMatrix = %v, want %v", state.Param.TextMatrix, expected)
		}
		if state.Param.TextLineMatrix != expected {
			t.Errorf("TextLineMatrix = %v, want %v", state.Param.TextLineMatrix, expected)
		}
	})

	t.Run("T* operator", func(t *testing.T) {
		state := &State{CurrentObject: objText}
		state.Param.TextLineMatrix = matrix.Identity
		state.Param.TextMatrix = matrix.Identity
		state.Param.TextLeading = 12.0
		res := &resource.Resource{}

		op := Operator{Name: "T*", Args: nil}
		if err := ApplyOperator(state, op, res); err != nil {
			t.Fatalf("T* operator failed: %v", err)
		}

		// Should mark dependency on TextLeading
		if state.In&graphics.StateTextLeading == 0 {
			t.Error("TextLeading dependency not marked")
		}

		// Should move down by leading amount
		expected := matrix.Matrix{1, 0, 0, 1, 0, -12}
		if state.Param.TextMatrix != expected {
			t.Errorf("TextMatrix = %v, want %v", state.Param.TextMatrix, expected)
		}
	})
}

// TestTextOperators_CompositeOperators verifies composite text showing operators
func TestTextOperators_CompositeOperators(t *testing.T) {
	t.Run("' operator", func(t *testing.T) {
		state := &State{CurrentObject: objText}
		state.Param.TextLineMatrix = matrix.Identity
		state.Param.TextMatrix = matrix.Identity
		state.Param.TextLeading = 10.0
		res := &resource.Resource{}

		op := Operator{Name: "'", Args: []pdf.Native{pdf.String("test")}}
		if err := ApplyOperator(state, op, res); err != nil {
			t.Fatalf("' operator failed: %v", err)
		}

		// Should have dependencies from T* (TextLeading) and Tj (font, matrix)
		if state.In&graphics.StateTextLeading == 0 {
			t.Error("TextLeading dependency not marked")
		}
		if state.In&graphics.StateTextFont == 0 {
			t.Error("TextFont dependency not marked")
		}
	})

	t.Run("\" operator", func(t *testing.T) {
		state := &State{CurrentObject: objText}
		state.Param.TextLineMatrix = matrix.Identity
		state.Param.TextMatrix = matrix.Identity
		state.Param.TextLeading = 10.0
		res := &resource.Resource{}

		op := Operator{
			Name: `"`,
			Args: []pdf.Native{pdf.Real(1.0), pdf.Real(2.0), pdf.String("test")},
		}
		if err := ApplyOperator(state, op, res); err != nil {
			t.Fatalf("\" operator failed: %v", err)
		}

		// Should set word and character spacing
		if state.Param.TextWordSpacing != 1.0 {
			t.Errorf("TextWordSpacing = %v, want 1.0", state.Param.TextWordSpacing)
		}
		if state.Param.TextCharacterSpacing != 2.0 {
			t.Errorf("TextCharacterSpacing = %v, want 2.0", state.Param.TextCharacterSpacing)
		}
		if state.Out&graphics.StateTextWordSpacing == 0 {
			t.Error("StateTextWordSpacing not marked in Out")
		}
		if state.Out&graphics.StateTextCharacterSpacing == 0 {
			t.Error("StateTextCharacterSpacing not marked in Out")
		}
	})
}

// TestTextOperators_FontResolution verifies font resource resolution
func TestTextOperators_FontResolution(t *testing.T) {
	t.Run("valid font", func(t *testing.T) {
		state := &State{}
		mockFont := &mockFontInstance{}
		res := &resource.Resource{
			Font: map[pdf.Name]font.Instance{
				"F1": mockFont,
			},
		}

		op := Operator{Name: "Tf", Args: []pdf.Native{pdf.Name("F1"), pdf.Real(12.0)}}
		if err := ApplyOperator(state, op, res); err != nil {
			t.Fatalf("Tf operator failed: %v", err)
		}

		if state.Param.TextFont != mockFont {
			t.Error("TextFont not set correctly")
		}
		if state.Param.TextFontSize != 12.0 {
			t.Errorf("TextFontSize = %v, want 12.0", state.Param.TextFontSize)
		}
		if state.Out&graphics.StateTextFont == 0 {
			t.Error("StateTextFont not marked in Out")
		}
	})

	t.Run("missing font", func(t *testing.T) {
		state := &State{}
		res := &resource.Resource{
			Font: map[pdf.Name]font.Instance{},
		}

		op := Operator{Name: "Tf", Args: []pdf.Native{pdf.Name("F1"), pdf.Real(12.0)}}
		err := ApplyOperator(state, op, res)
		if err == nil {
			t.Error("expected error for missing font")
		}
	})

	t.Run("no font resources", func(t *testing.T) {
		state := &State{}
		res := &resource.Resource{}

		op := Operator{Name: "Tf", Args: []pdf.Native{pdf.Name("F1"), pdf.Real(12.0)}}
		err := ApplyOperator(state, op, res)
		if err == nil {
			t.Error("expected error when no font resources available")
		}
	})
}

// TestTextOperators_ContextValidation verifies proper context checking
func TestTextOperators_ContextValidation(t *testing.T) {
	tests := []struct {
		name     string
		operator string
		args     []pdf.Native
		object   graphics.ObjectType
		wantErr  bool
	}{
		{"BT in page context", "BT", nil, objPage, false},
		{"BT in text context", "BT", nil, objText, true},
		{"ET in text context", "ET", nil, objText, false},
		{"ET in page context", "ET", nil, objPage, true},
		{"Td in text context", "Td", []pdf.Native{pdf.Real(0), pdf.Real(0)}, objText, false},
		{"Td in page context", "Td", []pdf.Native{pdf.Real(0), pdf.Real(0)}, objPage, true},
		{"Tj in text context", "Tj", []pdf.Native{pdf.String("test")}, objText, false},
		{"Tj in page context", "Tj", []pdf.Native{pdf.String("test")}, objPage, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &State{CurrentObject: tt.object}
			res := &resource.Resource{}

			op := Operator{Name: pdf.Name(tt.operator), Args: tt.args}
			err := ApplyOperator(state, op, res)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyOperator() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
