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

	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
)

func TestBuilder_NewForContent(t *testing.T) {
	b := New(content.Page, nil)

	// Page: line width is Known (can elide)
	if !b.State.IsKnown(graphics.StateLineWidth) {
		t.Error("Page: line width should be Known")
	}

	// Page: font is NOT Known
	if b.State.IsKnown(graphics.StateTextFont) {
		t.Error("Page: font should not be Known")
	}
}

func TestBuilder_FormNoElision(t *testing.T) {
	b := New(content.Form, nil)

	// Form: line width is Set but not Known (cannot elide)
	if !b.State.IsSet(graphics.StateLineWidth) {
		t.Error("Form: line width should be Set")
	}
	if b.State.IsKnown(graphics.StateLineWidth) {
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
