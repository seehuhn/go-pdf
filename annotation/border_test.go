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

package annotation

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestBorderDefaults(t *testing.T) {
	// Test that default border values are not written to PDF
	annotation := &Text{
		Common: Common{
			Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50},
			Border: &Border{
				HCornerRadius: 0,
				VCornerRadius: 0,
				Width:         1, // PDF default
				DashArray:     nil,
			},
		},
	}

	buf, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(buf)

	embedded, err := annotation.AsDict(rm)
	if err != nil {
		t.Fatal(err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	dict, err := pdf.GetDict(buf, embedded)
	if err != nil {
		t.Fatal(err)
	}

	// Border should not be present since it's the default value
	if _, exists := dict["Border"]; exists {
		t.Error("default border should not be written to PDF")
	}
}
