// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package graphics_test

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/internal/ghostscript"
)

// TestLineWidth checks that a vertical line of width 6 colours the correct
// pixels.
func TestLineWidth(t *testing.T) {
	img := ghostscript.Render(t, 20, 5, pdf.V1_7, func(r *document.Page) error {
		r.SetLineWidth(6.0)
		r.MoveTo(10, 0)
		r.LineTo(10, 5)
		r.Stroke()
		return nil
	})

	rect := img.Bounds()
	for i := rect.Min.X; i < rect.Max.X; i++ {
		for j := rect.Min.Y; j < rect.Max.Y; j++ {
			r, g, b, a := img.At(i, j).RGBA()
			if i >= 4*7 && i < 4*13 {
				// should be black
				if r != 0 || g != 0 || b != 0 || a != 0xffff {
					t.Errorf("pixel (%d,%d) should be black, but is %d,%d,%d,%d", i, j, r, g, b, a)
				}
			} else {
				// should be white
				if r != 0xffff || g != 0xffff || b != 0xffff || a != 0xffff {
					t.Errorf("pixel (%d,%d) should be white, but is %d,%d,%d,%d", i, j, r, g, b, a)
				}
			}
		}
	}
}
