// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package reader

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/state"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestParameters verifies that the graphics state is correctly updated by the
// reader.
func TestParameters(t *testing.T) {
	data, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	testFont := standard.Helvetica.New()
	m := matrix.Matrix{1, 2, 3, 4, 5, 6}

	// Build a content stream where we set various graphics parameters.
	b := builder.New(content.Page, nil)
	b.SetLineWidth(12.3)
	b.SetLineCap(graphics.LineCapRound)
	b.SetLineJoin(graphics.LineJoinBevel)
	b.SetMiterLimit(4)
	b.SetLineDash([]float64{5, 6, 7}, 8)
	b.SetRenderingIntent(graphics.Perceptual)
	b.SetFlatnessTolerance(10)
	b.Transform(m)
	b.TextSetCharacterSpacing(9)
	b.TextSetWordSpacing(10)
	b.TextSetHorizontalScaling(0.11)
	b.TextSetLeading(12)
	b.TextSetFont(testFont, 14)
	b.TextSetRenderingMode(graphics.TextRenderingModeFillStrokeClip)
	b.TextSetRise(15)

	if b.Err != nil {
		t.Fatal(b.Err)
	}

	// Write the content stream to a buffer
	buf := &bytes.Buffer{}
	err := content.Write(buf, b.Stream, pdf.V1_7, content.Page, b.Resources)
	if err != nil {
		t.Fatal(err)
	}

	// Now we read back the content stream and check that the final graphics
	// state matches the expected values.
	r := New(data)
	r.Reset()
	r.Resources = b.Resources
	r.State.Set = 0
	err = r.ParseContentStream(buf)
	if err != nil {
		t.Fatal(err)
	}

	// First check: the individual parameters are as we set them.
	if r.State.LineWidth != 12.3 {
		t.Errorf("LineWidth: got %v, want 12.3", r.State.LineWidth)
	}
	if r.State.LineCap != graphics.LineCapRound {
		t.Errorf("LineCap: got %v, want %v", r.State.LineCap, graphics.LineCapRound)
	}
	if r.State.LineJoin != graphics.LineJoinBevel {
		t.Errorf("LineJoin: got %v, want %v", r.State.LineJoin, graphics.LineJoinBevel)
	}
	if r.State.MiterLimit != 4 {
		t.Errorf("MiterLimit: got %v, want 4", r.State.MiterLimit)
	}
	if d := cmp.Diff(r.State.DashPattern, []float64{5, 6, 7}); d != "" {
		t.Errorf("DashPattern: %s", d)
	}
	if r.State.DashPhase != 8 {
		t.Errorf("DashPhase: got %v, want 8", r.State.DashPhase)
	}
	if r.State.RenderingIntent != graphics.Perceptual {
		t.Errorf("RenderingIntent: got %v, want %v", r.State.RenderingIntent, graphics.Perceptual)
	}
	if r.State.FlatnessTolerance != 10 {
		t.Errorf("Flatness: got %v, want 10", r.State.FlatnessTolerance)
	}
	if r.State.CTM != m {
		t.Errorf("CTM: got %v, want %v", r.State.CTM, m)
	}
	if r.State.TextCharacterSpacing != 9 {
		t.Errorf("Tc: got %v, want 9", r.State.TextCharacterSpacing)
	}
	if r.State.TextWordSpacing != 10 {
		t.Errorf("Tw: got %v, want 10", r.State.TextWordSpacing)
	}
	if r.State.TextHorizontalScaling != 0.11 {
		t.Errorf("Th: got %v, want 0.11", r.State.TextHorizontalScaling)
	}
	if r.State.TextLeading != 12 {
		t.Errorf("Tl: got %v, want 12", r.State.TextLeading)
	}
	// TODO(voss): compare the fonts
	if r.State.TextFontSize != 14 {
		t.Errorf("Font: got %v, %v, want %v, 14", r.State.TextFont, r.State.TextFontSize, testFont)
	}
	if r.State.TextRenderingMode != graphics.TextRenderingModeFillStrokeClip {
		t.Errorf("TextRenderingMode: got %v, want %v", r.State.TextRenderingMode, graphics.TextRenderingModeFillStrokeClip)
	}
	if r.State.TextRise != 15 {
		t.Errorf("Tr: got %v, want 15", r.State.TextRise)
	}

	// Check that the expected bits are set
	expectedBits := state.LineWidth | state.LineCap | state.LineJoin |
		state.MiterLimit | state.LineDash | state.RenderingIntent |
		state.FlatnessTolerance | state.TextCharacterSpacing |
		state.TextWordSpacing | state.TextHorizontalScaling |
		state.TextLeading | state.TextFont | state.TextRenderingMode | state.TextRise

	for b := state.Bits(1); b != 0; b <<= 1 {
		if expectedBits&b != 0 {
			if r.State.Set&b == 0 {
				t.Errorf("State bit %s not set in reader but expected", b.Names())
			}
		}
	}
}

// TestParsePage verifies that Reader.ParsePage correctly handles split content streams.
func TestParsePage(t *testing.T) {
	pdfData, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	stream1Ref := pdfData.Alloc()
	stream1, err := pdfData.OpenStream(stream1Ref, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = stream1.Write([]byte("q\n1 0 0 1 100 200 cm\n"))
	if err != nil {
		t.Fatal(err)
	}
	err = stream1.Close()
	if err != nil {
		t.Fatal(err)
	}

	stream2Ref := pdfData.Alloc()
	stream2, err := pdfData.OpenStream(stream2Ref, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = stream2.Write([]byte("5 w\nQ\n"))
	if err != nil {
		t.Fatal(err)
	}
	err = stream2.Close()
	if err != nil {
		t.Fatal(err)
	}

	pageRef := pdfData.Alloc()
	pageDict := pdf.Dict{
		"Type":      pdf.Name("Page"),
		"Contents":  pdf.Array{stream1Ref, stream2Ref},
		"Resources": pdf.Dict{},
	}
	err = pdfData.Put(pageRef, pageDict)
	if err != nil {
		t.Fatal(err)
	}

	err = pdfData.Close()
	if err != nil {
		t.Fatal(err)
	}

	reader := New(pdfData)

	type operation struct {
		Op   string
		Args []pdf.Object
	}

	var operations []operation
	reader.EveryOp = func(op string, args []pdf.Object) error {
		// clone arguments since scanner reuses the slice
		var clonedArgs []pdf.Object
		if len(args) > 0 {
			clonedArgs = make([]pdf.Object, len(args))
			copy(clonedArgs, args)
		}
		operations = append(operations, operation{Op: op, Args: clonedArgs})
		return nil
	}

	err = reader.ParsePage(pageRef, matrix.Identity)
	if err != nil {
		t.Fatal(err)
	}

	expected := []operation{
		{Op: "q", Args: nil},
		{Op: "cm", Args: []pdf.Object{pdf.Integer(1), pdf.Integer(0), pdf.Integer(0), pdf.Integer(1), pdf.Integer(100), pdf.Integer(200)}},
		{Op: "w", Args: []pdf.Object{pdf.Integer(5)}},
		{Op: "Q", Args: nil},
	}

	// Compare what we got to what we expected
	if diff := cmp.Diff(expected, operations); diff != "" {
		t.Errorf("operations mismatch (-want +got):\n%s", diff)
	}
}
