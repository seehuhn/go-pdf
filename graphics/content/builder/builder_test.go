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
	b := New(content.PageContent, nil)

	// PageContent: line width is Known (can elide)
	if !b.State.IsKnown(graphics.StateLineWidth) {
		t.Error("PageContent: line width should be Known")
	}

	// PageContent: font is NOT Known
	if b.State.IsKnown(graphics.StateTextFont) {
		t.Error("PageContent: font should not be Known")
	}
}

func TestBuilder_FormContentNoElision(t *testing.T) {
	b := New(content.FormContent, nil)

	// FormContent: line width is Set but not Known (cannot elide)
	if !b.State.IsSet(graphics.StateLineWidth) {
		t.Error("FormContent: line width should be Set")
	}
	if b.State.IsKnown(graphics.StateLineWidth) {
		t.Error("FormContent: line width should not be Known")
	}
}

func TestBuilder_Harvest(t *testing.T) {
	b := New(content.PageContent, nil)

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
	b := New(content.PageContent, nil)
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
	b := New(content.PageContent, nil)

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
