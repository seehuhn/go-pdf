// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

func TestFreeTextRejectsInvalidAlign(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	a := &FreeText{
		Common: Common{
			Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50},
		},
		DefaultAppearance: "/Helv 12 Tf 0 g",
		Align:             pdf.TextAlign(99),
	}

	if _, err := a.Encode(rm); err == nil {
		t.Error("expected error for out-of-range Align, got nil")
	}
}

func TestRedactRejectsInvalidAlign(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	a := &Redact{
		Common: Common{
			Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50},
		},
		Align: pdf.TextAlign(99),
	}

	if _, err := a.Encode(rm); err == nil {
		t.Error("expected error for out-of-range Align, got nil")
	}
}
