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
	"testing"

	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
)

func TestBuilder_Type3SetWidthOnly(t *testing.T) {
	b := New(content.Glyph, nil)

	b.Type3SetWidthOnly(500, 0)
	if b.Err != nil {
		t.Fatalf("Type3SetWidthOnly failed: %v", b.Err)
	}

	if b.State.ColorOpsForbidden {
		t.Error("ColorOpsForbidden should be false after d0")
	}
}

func TestBuilder_Type3SetWidthAndBBox(t *testing.T) {
	b := New(content.Glyph, nil)

	b.Type3SetWidthAndBoundingBox(600, 0, 0, 0, 500, 700)
	if b.Err != nil {
		t.Fatalf("Type3SetWidthAndBoundingBox failed: %v", b.Err)
	}

	if !b.State.ColorOpsForbidden {
		t.Error("ColorOpsForbidden should be true after d1")
	}
}

func TestBuilder_Type3ColorRestriction(t *testing.T) {
	b := New(content.Glyph, nil)

	// d1 mode
	b.Type3SetWidthAndBoundingBox(600, 0, 0, 0, 500, 700)
	if b.Err != nil {
		t.Fatalf("Type3SetWidthAndBoundingBox failed: %v", b.Err)
	}

	// Color should fail in d1 mode
	b.SetFillColor(color.DeviceGray(0.5))
	if b.Err == nil {
		t.Error("SetFillColor should fail in d1 mode")
	}
}

func TestBuilder_Type3NotFirstOp(t *testing.T) {
	b := New(content.Glyph, nil)

	// Some other operator first
	b.SetLineWidth(1.0)

	// d0/d1 must be first
	b.Type3SetWidthOnly(500, 0)
	if b.Err == nil {
		t.Error("d0 should fail if not first operator")
	}
}

func TestBuilder_Type3D0AllowsColor(t *testing.T) {
	b := New(content.Glyph, nil)

	// d0 mode
	b.Type3SetWidthOnly(500, 0)
	if b.Err != nil {
		t.Fatalf("Type3SetWidthOnly failed: %v", b.Err)
	}

	// Color should work in d0 mode
	b.SetFillColor(color.DeviceGray(0.5))
	if b.Err != nil {
		t.Errorf("SetFillColor should work in d0 mode: %v", b.Err)
	}
}
