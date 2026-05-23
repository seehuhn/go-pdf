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

package content_test

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
)

func TestMultiSegmentPage(t *testing.T) {
	// Build two segments
	b := builder.New(content.Page, nil, pdf.V2_0)

	// First segment
	b.SetLineWidth(2)
	b.MoveTo(0, 0)
	b.LineTo(100, 100)
	b.Stroke()
	if _, err := b.Harvest(); err != nil {
		t.Fatalf("Harvest stream1: %v", err)
	}

	// Second segment (line width Known from first)
	b.SetLineWidth(2) // should elide
	b.MoveTo(100, 100)
	b.LineTo(200, 200)
	b.Stroke()
	stream2, err := b.Harvest()
	if err != nil {
		t.Fatalf("Harvest stream2: %v", err)
	}

	// stream2 should not have SetLineWidth (elided)
	for _, op := range stream2.Ops {
		if op.Name == content.OpSetLineWidth {
			t.Error("SetLineWidth should have been elided in stream2")
		}
	}

	// Validate
	if err := b.Close(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestType3FontGlyphs(t *testing.T) {
	sharedRes := &content.Resources{}

	// Glyph 'A' with d0 (colored)
	bA := builder.New(content.Glyph, sharedRes, pdf.V2_0)
	bA.Type3ColoredGlyph(500, 0)
	bA.MoveTo(0, 0)
	bA.LineTo(250, 700)
	bA.LineTo(500, 0)
	bA.Stroke()
	streamA, err := bA.Harvest()
	if err != nil {
		t.Fatalf("Harvest glyph A: %v", err)
	}
	if err := bA.Close(); err != nil {
		t.Fatalf("Validate glyph A: %v", err)
	}

	// Glyph 'B' with d1 (inherits color)
	bB := builder.New(content.Glyph, sharedRes, pdf.V2_0)
	bB.Type3UncoloredGlyph(600, 0, 0, 0, 600, 700)
	// No color operators allowed
	bB.MoveTo(0, 0)
	bB.LineTo(300, 700)
	bB.LineTo(600, 0)
	bB.ClosePath()
	bB.Fill()
	streamB, err := bB.Harvest()
	if err != nil {
		t.Fatalf("Harvest glyph B: %v", err)
	}
	if err := bB.Close(); err != nil {
		t.Fatalf("Validate glyph B: %v", err)
	}

	if len(streamA.Ops) == 0 || len(streamB.Ops) == 0 {
		t.Error("Glyph streams should not be empty")
	}
}

func TestFormInheritedState(t *testing.T) {
	// Form: all params are Set-Unknown, no elision
	b := builder.New(content.Form, nil, pdf.V2_0)

	// First set: should emit (not Known)
	b.SetLineWidth(5.0)
	if len(b.Stream) != 1 {
		t.Errorf("First SetLineWidth should emit, got %d ops", len(b.Stream))
	}

	// Second set with different value: should emit
	b.SetLineWidth(10.0)
	if len(b.Stream) != 2 {
		t.Errorf("Different value should emit, got %d ops", len(b.Stream))
	}

	// Third set with same value: should elide (now Known)
	b.SetLineWidth(10.0)
	if len(b.Stream) != 2 {
		t.Errorf("Same value should elide, got %d ops", len(b.Stream))
	}

	stream, err := b.Harvest()
	if err != nil {
		t.Fatalf("Harvest: %v", err)
	}
	if len(stream.Ops) != 2 {
		t.Errorf("Expected 2 ops in stream, got %d", len(stream.Ops))
	}
}

// (Version validation lives at construction time in [builder.Builder] —
// see TestBuilder_VersionRejectsTooNewOperator.  The Writer no longer
// validates, so a former TestWriterVersionValidation test was removed.)
