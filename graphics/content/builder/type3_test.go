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
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/extgstate"
)

func TestBuilder_Type3SetWidthOnly(t *testing.T) {
	b := New(content.Glyph, nil, pdf.V2_0)

	b.Type3ColoredGlyph(500, 0)
	if b.Err != nil {
		t.Fatalf("Type3SetWidthOnly failed: %v", b.Err)
	}

	if b.State.ColorOpsForbidden {
		t.Error("ColorOpsForbidden should be false after d0")
	}
}

func TestBuilder_Type3SetWidthAndBBox(t *testing.T) {
	b := New(content.Glyph, nil, pdf.V2_0)

	b.Type3UncoloredGlyph(600, 0, 0, 0, 500, 700)
	if b.Err != nil {
		t.Fatalf("Type3SetWidthAndBoundingBox failed: %v", b.Err)
	}

	if !b.State.ColorOpsForbidden {
		t.Error("ColorOpsForbidden should be true after d1")
	}
}

func TestBuilder_Type3ColorRestriction(t *testing.T) {
	b := New(content.Glyph, nil, pdf.V2_0)

	// d1 mode
	b.Type3UncoloredGlyph(600, 0, 0, 0, 500, 700)
	if b.Err != nil {
		t.Fatalf("Type3SetWidthAndBoundingBox failed: %v", b.Err)
	}

	// Color should fail in d1 mode
	b.SetFillColor(color.DeviceGray(0.5))
	if !errors.Is(b.Err, ErrColorForbidden) {
		t.Error("SetFillColor should fail in d1 mode")
	}
}

func TestBuilder_Type3NotFirstOp(t *testing.T) {
	b := New(content.Glyph, nil, pdf.V2_0)

	// Some other operator first
	b.SetLineWidth(1.0)

	// d0/d1 must be first
	b.Type3ColoredGlyph(500, 0)
	if b.Err == nil {
		t.Error("d0 should fail if not first operator")
	}
}

func TestBuilder_Type3D0AllowsColor(t *testing.T) {
	b := New(content.Glyph, nil, pdf.V2_0)

	// d0 mode
	b.Type3ColoredGlyph(500, 0)
	if b.Err != nil {
		t.Fatalf("Type3SetWidthOnly failed: %v", b.Err)
	}

	// Color should work in d0 mode
	b.SetFillColor(color.DeviceGray(0.5))
	if b.Err != nil {
		t.Errorf("SetFillColor should work in d0 mode: %v", b.Err)
	}
}

// TestBuilder_Type3ExtGStateRestriction checks that colour-related ExtGState
// entries are rejected after d1, while other entries are accepted.
func TestBuilder_Type3ExtGStateRestriction(t *testing.T) {
	b := New(content.Glyph, nil, pdf.V2_0)
	b.Type3UncoloredGlyph(600, 0, 0, 0, 500, 700)
	b.SetExtGState(&extgstate.ExtGState{
		Set:       graphics.StateLineWidth,
		LineWidth: 2,
	})
	if b.Err != nil {
		t.Fatalf("unexpected error: %v", b.Err)
	}

	b.SetExtGState(&extgstate.ExtGState{
		Set:                    graphics.StateBlackPointCompensation,
		BlackPointCompensation: "ON",
	})
	if !errors.Is(b.Err, ErrColorForbidden) {
		t.Errorf("got error %v, want ErrColorForbidden", b.Err)
	}
}

// TestBuilder_Type3InlineImageMask checks that inline image masks are
// accepted after d1, while other inline images are rejected.
func TestBuilder_Type3InlineImageMask(t *testing.T) {
	b := New(content.Glyph, nil, pdf.V2_0)
	b.Type3UncoloredGlyph(600, 0, 0, 0, 500, 700)
	b.DrawInlineImageRaw(pdf.Dict{
		"W": pdf.Integer(1), "H": pdf.Integer(1), "IM": pdf.Boolean(true),
	}, []byte{0})
	if b.Err != nil {
		t.Fatalf("unexpected error: %v", b.Err)
	}

	b.DrawInlineImageRaw(pdf.Dict{
		"W": pdf.Integer(1), "H": pdf.Integer(1),
		"CS": pdf.Name("G"), "BPC": pdf.Integer(8),
	}, []byte{0})
	if !errors.Is(b.Err, ErrColorForbidden) {
		t.Errorf("got error %v, want ErrColorForbidden", b.Err)
	}
}
