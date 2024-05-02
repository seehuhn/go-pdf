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

package ghostscript

import (
	"math"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
)

func TestTextPos(t *testing.T) {
	xTest := 100.0
	yTest := 100.0
	x, y, err := FindTextPos(pdf.V1_7, document.A5r, func(page *document.Page) error {
		page.TextFirstLine(xTest, yTest)
		return nil
	})
	if err == ErrNoGhostscript {
		t.Skip("ghostscript not found")
	} else if err != nil {
		t.Fatal(err)
	}

	// fmt.Printf("got (%g,%g) instead of (%g,%g)\n", x, y, xTest, yTest)
	if math.Abs(x-xTest) > 0.2 || math.Abs(y-yTest) > 0.2 {
		t.Fatalf("expected x=%f, y=%f, got x=%f, y=%f", xTest, yTest, x, y)
	}
}
