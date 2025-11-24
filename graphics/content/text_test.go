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

package content

import (
	"iter"
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/graphics"
)

func TestTextOperators_BeginEnd(t *testing.T) {
	state := &GraphicsState{CurrentObject: ObjPage}
	res := &Resources{}

	// Begin text
	opBT := Operator{Name: "BT", Args: nil}
	if err := state.Apply(res, opBT); err != nil {
		t.Fatalf("BT operator failed: %v", err)
	}

	if state.CurrentObject != ObjText {
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
	if err := state.Apply(res, opET); err != nil {
		t.Fatalf("ET operator failed: %v", err)
	}

	if state.CurrentObject != ObjPage {
		t.Errorf("CurrentObject = %v, want objPage", state.CurrentObject)
	}
	if state.Out&graphics.StateTextMatrix != 0 {
		t.Error("StateTextMatrix still marked after ET")
	}
}

func TestTextOperators_SetFont(t *testing.T) {
	state := &GraphicsState{}
	mockFont := &mockFontInstance{}
	res := &Resources{
		Font: map[pdf.Name]font.Instance{
			"F1": mockFont,
		},
	}

	op := Operator{
		Name: "Tf",
		Args: []pdf.Native{pdf.Name("F1"), pdf.Real(12.0)},
	}

	if err := state.Apply(res, op); err != nil {
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
			state := &GraphicsState{CurrentObject: ObjText}
			state.Param.TextRenderingMode = tt.renderingMode
			res := &Resources{}

			op := Operator{Name: "Tj", Args: []pdf.Native{pdf.String("test")}}
			if err := state.Apply(res, op); err != nil {
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
		state := &GraphicsState{CurrentObject: ObjText}
		state.Param.TextLineMatrix = matrix.Identity
		state.Param.TextMatrix = matrix.Identity
		res := &Resources{}

		op := Operator{Name: "Td", Args: []pdf.Native{pdf.Real(10.0), pdf.Real(20.0)}}
		if err := state.Apply(res, op); err != nil {
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
		state := &GraphicsState{CurrentObject: ObjText}
		state.Param.TextLineMatrix = matrix.Identity
		state.Param.TextMatrix = matrix.Identity
		res := &Resources{}

		op := Operator{Name: "TD", Args: []pdf.Native{pdf.Real(10.0), pdf.Real(-5.0)}}
		if err := state.Apply(res, op); err != nil {
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
		state := &GraphicsState{CurrentObject: ObjText}
		res := &Resources{}

		op := Operator{
			Name: "Tm",
			Args: []pdf.Native{
				pdf.Real(1.0), pdf.Real(0.0),
				pdf.Real(0.0), pdf.Real(1.0),
				pdf.Real(100.0), pdf.Real(200.0),
			},
		}
		if err := state.Apply(res, op); err != nil {
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
		state := &GraphicsState{CurrentObject: ObjText}
		state.Param.TextLineMatrix = matrix.Identity
		state.Param.TextMatrix = matrix.Identity
		state.Param.TextLeading = 12.0
		res := &Resources{}

		op := Operator{Name: "T*", Args: nil}
		if err := state.Apply(res, op); err != nil {
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
		state := &GraphicsState{CurrentObject: ObjText}
		state.Param.TextLineMatrix = matrix.Identity
		state.Param.TextMatrix = matrix.Identity
		state.Param.TextLeading = 10.0
		res := &Resources{}

		op := Operator{Name: "'", Args: []pdf.Native{pdf.String("test")}}
		if err := state.Apply(res, op); err != nil {
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
		state := &GraphicsState{CurrentObject: ObjText}
		state.Param.TextLineMatrix = matrix.Identity
		state.Param.TextMatrix = matrix.Identity
		state.Param.TextLeading = 10.0
		res := &Resources{}

		op := Operator{
			Name: `"`,
			Args: []pdf.Native{pdf.Real(1.0), pdf.Real(2.0), pdf.String("test")},
		}
		if err := state.Apply(res, op); err != nil {
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
		state := &GraphicsState{}
		mockFont := &mockFontInstance{}
		res := &Resources{
			Font: map[pdf.Name]font.Instance{
				"F1": mockFont,
			},
		}

		op := Operator{Name: "Tf", Args: []pdf.Native{pdf.Name("F1"), pdf.Real(12.0)}}
		if err := state.Apply(res, op); err != nil {
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
		state := &GraphicsState{}
		res := &Resources{
			Font: map[pdf.Name]font.Instance{},
		}

		op := Operator{Name: "Tf", Args: []pdf.Native{pdf.Name("F1"), pdf.Real(12.0)}}
		err := state.Apply(res, op)
		if err == nil {
			t.Error("expected error for missing font")
		}
	})

	t.Run("no font resources", func(t *testing.T) {
		state := &GraphicsState{}
		res := &Resources{}

		op := Operator{Name: "Tf", Args: []pdf.Native{pdf.Name("F1"), pdf.Real(12.0)}}
		err := state.Apply(res, op)
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
		object   Object
		wantErr  bool
	}{
		{"BT in page context", "BT", nil, ObjPage, false},
		{"BT in text context", "BT", nil, ObjText, true},
		{"ET in text context", "ET", nil, ObjText, false},
		{"ET in page context", "ET", nil, ObjPage, true},
		{"Td in text context", "Td", []pdf.Native{pdf.Real(0), pdf.Real(0)}, ObjText, false},
		{"Td in page context", "Td", []pdf.Native{pdf.Real(0), pdf.Real(0)}, ObjPage, true},
		{"Tj in text context", "Tj", []pdf.Native{pdf.String("test")}, ObjText, false},
		{"Tj in page context", "Tj", []pdf.Native{pdf.String("test")}, ObjPage, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &GraphicsState{CurrentObject: tt.object}
			res := &Resources{}

			op := Operator{Name: pdf.Name(tt.operator), Args: tt.args}
			err := state.Apply(res, op)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyOperator() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
