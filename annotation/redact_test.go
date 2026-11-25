// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package annotation

import (
	"testing"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation/colorenc"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestRedactEncodeErrors(t *testing.T) {
	rect := pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50}
	tests := []struct {
		name string
		a    *Redact
	}{
		{"overlay text without DA", &Redact{
			Rect:        rect,
			OverlayText: pdf.Text("x"),
		}},
		{"incomplete quadrilateral", &Redact{
			Rect:       rect,
			QuadPoints: make([]vec.Vec2, 6),
		}},
		{"gray fill", &Redact{
			Rect:      rect,
			FillColor: color.DeviceGray(0.5),
		}},
		{"transparent fill", &Redact{
			Rect:      rect,
			FillColor: colorenc.Transparent,
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := tc.a.Encode(rm); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
