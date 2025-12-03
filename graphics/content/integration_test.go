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
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
)

func TestMultiSegmentPage(t *testing.T) {
	// Build two segments
	b := builder.New(content.PageContent, nil)

	// First segment
	b.SetLineWidth(2)
	b.MoveTo(0, 0)
	b.LineTo(100, 100)
	b.Stroke()
	stream1, err := b.Harvest()
	if err != nil {
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
	for _, op := range stream2 {
		if op.Name == content.OpSetLineWidth {
			t.Error("SetLineWidth should have been elided in stream2")
		}
	}

	// Validate
	if err := b.Close(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Write both segments
	var buf1, buf2 bytes.Buffer
	w := content.NewWriter(content.PageContent, b.Resources, pdf.V1_7)
	if err := w.Write(&buf1, stream1); err != nil {
		t.Fatalf("Write stream1: %v", err)
	}
	if err := w.Write(&buf2, stream2); err != nil {
		t.Fatalf("Write stream2: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Writer.Validate: %v", err)
	}
}

func TestType3FontGlyphs(t *testing.T) {
	sharedRes := &content.Resources{}

	// Glyph 'A' with d0 (colored)
	bA := builder.New(content.Type3Content, sharedRes)
	bA.Type3SetWidthOnly(500, 0)
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
	bB := builder.New(content.Type3Content, sharedRes)
	bB.Type3SetWidthAndBoundingBox(600, 0, 0, 0, 600, 700)
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

	// Write both
	var bufA, bufB bytes.Buffer
	wA := content.NewWriter(content.Type3Content, sharedRes, pdf.V1_7)
	if err := wA.Write(&bufA, streamA); err != nil {
		t.Fatalf("Write glyph A: %v", err)
	}
	wB := content.NewWriter(content.Type3Content, sharedRes, pdf.V1_7)
	if err := wB.Write(&bufB, streamB); err != nil {
		t.Fatalf("Write glyph B: %v", err)
	}

	if bufA.Len() == 0 || bufB.Len() == 0 {
		t.Error("Glyph streams should not be empty")
	}
}

func TestFormContentInheritedState(t *testing.T) {
	// FormContent: all params are Set-Unknown, no elision
	b := builder.New(content.FormContent, nil)

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
	if len(stream) != 2 {
		t.Errorf("Expected 2 ops in stream, got %d", len(stream))
	}
}

func TestWriterVersionValidation(t *testing.T) {
	b := builder.New(content.PageContent, nil)
	b.SetRenderingIntent("Perceptual")
	stream, _ := b.Harvest()

	// PDF 1.0 doesn't support ri
	var buf bytes.Buffer
	w := content.NewWriter(content.PageContent, b.Resources, pdf.V1_0)
	err := w.Write(&buf, stream)
	if err == nil {
		t.Error("ri should fail for PDF 1.0")
	}

	// PDF 1.1+ supports ri
	buf.Reset()
	w2 := content.NewWriter(content.PageContent, b.Resources, pdf.V1_1)
	err = w2.Write(&buf, stream)
	if err != nil {
		t.Errorf("ri should work for PDF 1.1: %v", err)
	}
}
