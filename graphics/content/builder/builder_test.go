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

package builder

import (
	"errors"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
)

func TestBuilder_NewForContent(t *testing.T) {
	b := New(content.Page, nil)

	// Page: line width is Known (can elide)
	if !b.State.IsSet(graphics.StateLineWidth) {
		t.Error("Page: line width should be Known")
	}

	// Page: font is NOT Known
	if b.State.IsSet(graphics.StateTextFont) {
		t.Error("Page: font should not be Known")
	}
}

func TestBuilder_FormNoElision(t *testing.T) {
	b := New(content.Form, nil)

	// Form: line width is Set but not Known (cannot elide)
	if !b.State.IsUsable(graphics.StateLineWidth) {
		t.Error("Form: line width should be Set")
	}
	if b.State.IsSet(graphics.StateLineWidth) {
		t.Error("Form: line width should not be Known")
	}
}

func TestBuilder_Harvest(t *testing.T) {
	b := New(content.Page, nil)

	b.SetLineWidth(5.0)
	b.MoveTo(0, 0)
	b.LineTo(100, 100)
	b.Stroke()

	stream, err := b.Harvest()
	if err != nil {
		t.Fatalf("Harvest failed: %v", err)
	}

	if len(stream) != 4 {
		t.Errorf("Harvest returned %d ops, want 4", len(stream))
	}

	// Stream should be cleared
	if len(b.Stream) != 0 {
		t.Errorf("Stream not cleared after Harvest")
	}
}

func TestBuilder_HarvestError(t *testing.T) {
	b := New(content.Page, nil)
	b.Err = errors.New("test error")

	_, err := b.Harvest()
	if err == nil {
		t.Error("Harvest should return error when Err is set")
	}

	// Err should still be set (sticky)
	if b.Err == nil {
		t.Error("Err should remain set after Harvest")
	}
}

func TestBuilder_Validate(t *testing.T) {
	b := New(content.Page, nil)

	// Valid state
	if err := b.Close(); err != nil {
		t.Errorf("Valid state should pass: %v", err)
	}

	// Unclosed q
	b.PushGraphicsState()
	if err := b.Close(); err == nil {
		t.Error("Unclosed q should fail validation")
	}

	// Close it
	b.PopGraphicsState()
	if err := b.Close(); err != nil {
		t.Errorf("After closing q should pass: %v", err)
	}
}

func TestBuilder_Reset(t *testing.T) {
	res := &content.Resources{}
	b := New(content.Page, res)

	// build some content
	b.SetLineWidth(5.0)
	b.MoveTo(0, 0)
	b.LineTo(100, 100)
	b.Stroke()

	if len(b.Stream) == 0 {
		t.Fatal("expected non-empty stream")
	}

	// reset and verify state is cleared
	b.Reset()

	if len(b.Stream) != 0 {
		t.Errorf("stream not cleared after Reset, got %d ops", len(b.Stream))
	}
	if b.Err != nil {
		t.Errorf("Err not cleared after Reset: %v", b.Err)
	}
	if b.Resources != res {
		t.Error("resources should be preserved after Reset")
	}

	// verify we can build new content
	b.SetLineWidth(10.0)
	b.MoveTo(50, 50)
	b.LineTo(150, 150)
	b.Stroke()

	if len(b.Stream) != 4 {
		t.Errorf("expected 4 ops after second build, got %d", len(b.Stream))
	}
	if err := b.Close(); err != nil {
		t.Errorf("Close failed after Reset: %v", err)
	}
}

func TestBuilder_ResetClearsError(t *testing.T) {
	b := New(content.Page, nil)
	b.Err = errors.New("test error")

	b.Reset()

	if b.Err != nil {
		t.Error("Reset should clear Err")
	}
}

// TestBuilder_FailingOperatorRecordedInStream verifies that when an
// operator fails validation in [Builder.emit] the failing operator is
// still appended to [Builder.Stream].  This lets a downstream consumer
// such as [content.Writer] replay the stream, re-trigger the same error,
// and report the root cause rather than a cascading "unclosed operators"
// failure caused by suppressed matching closers.
func TestBuilder_FailingOperatorRecordedInStream(t *testing.T) {
	b := New(content.Form, nil)

	b.PushGraphicsState() // q  (valid, pushes pairQ)
	b.LineTo(1, 2)        // l  (invalid: outside Path context — sets Err)
	b.PopGraphicsState()  // Q  (no-op, Err is sticky)

	if b.Err == nil {
		t.Fatal("expected Err to be set by LineTo without MoveTo")
	}

	// The failing operator must be preserved in the stream.
	if len(b.Stream) != 2 {
		t.Fatalf("expected 2 ops in stream (q, l), got %d", len(b.Stream))
	}
	if b.Stream[1].Name != content.OpLineTo {
		t.Errorf("expected last op to be %q, got %q", content.OpLineTo, b.Stream[1].Name)
	}

	// When the stream is replayed through the writer, it should surface
	// the root-cause error — not a balance-check failure.
	err := content.NewWriter(pdf.V2_0, content.Form, b.Resources).Validate(b.Stream)
	if !errors.Is(err, content.ErrInvalidContext) {
		t.Errorf("expected ErrInvalidContext, got %v", err)
	}
}

// makeType3 builds a minimal Type 3 font for tests.
func makeType3(t *testing.T, name pdf.Name) font.Layouter {
	t.Helper()
	f := &type3.Font{
		Glyphs:     []*type3.Glyph{{}},
		FontMatrix: [6]float64{0.001, 0, 0, 0.001, 0, 0},
		Name:       name,
	}
	inst, err := f.New()
	if err != nil {
		t.Fatal(err)
	}
	return inst
}

// TestBuilder_FontNameUsesResourceName verifies that FontName picks up the
// font's configured ResourceName instead of auto-allocating.
func TestBuilder_FontNameUsesResourceName(t *testing.T) {
	b := New(content.Page, nil)
	f := makeType3(t, "MyFont")
	if got := b.FontName(f); got != "MyFont" {
		t.Errorf("FontName = %q, want %q", got, "MyFont")
	}
	if _, ok := b.Resources.Font["MyFont"]; !ok {
		t.Error("font not registered under user-supplied name")
	}
}

// TestBuilder_FontNameCollision tests that registering two different fonts
// under the same name surfaces an error.
func TestBuilder_FontNameCollision(t *testing.T) {
	b := New(content.Page, nil)
	f1 := makeType3(t, "F")
	f2 := makeType3(t, "F")
	b.FontName(f1)
	if b.Err != nil {
		t.Fatalf("unexpected Err: %v", b.Err)
	}
	b.FontName(f2)
	if b.Err == nil {
		t.Error("expected Err for name collision")
	}
}

// TestBuilder_SetFontNameInternalMismatch tests that SetFontNameInternal
// refuses a name that disagrees with the font's own ResourceName.
func TestBuilder_SetFontNameInternalMismatch(t *testing.T) {
	b := New(content.Page, nil)
	f := makeType3(t, "F1")
	if err := b.SetFontNameInternal(f, "F2"); err == nil {
		t.Error("expected error for mismatch between dict Name and requested key")
	}
}

// TestBuilder_PrefillMismatch tests that builder.New with inconsistent
// Resources.Font (dict-key != font.ResourceName()) sets b.Err.
func TestBuilder_PrefillMismatch(t *testing.T) {
	f := makeType3(t, "MyFont")
	res := &content.Resources{
		Font: map[pdf.Name]font.Instance{"F1": f},
	}
	b := New(content.Page, res)
	if b.Err == nil {
		t.Error("expected Err for dict-key vs Name mismatch in Resources.Font")
	}
}

// TestBuilder_PrefillXObjectMismatch verifies the same check on XObjects.
func TestBuilder_PrefillXObjectMismatch(t *testing.T) {
	f := &form.Form{
		BBox: pdf.Rectangle{URx: 10, URy: 10},
		Res:  &content.Resources{},
		Name: "MyForm",
	}
	res := &content.Resources{
		XObject: map[pdf.Name]graphics.XObject{"X1": f},
	}
	b := New(content.Page, res)
	if b.Err == nil {
		t.Error("expected Err for dict-key vs Name mismatch in Resources.XObject")
	}
}
