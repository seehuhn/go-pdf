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

package form

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestRead verifies that a form XObject read from one PDF file can be written
// to another PDF file.
func TestRead(t *testing.T) {
	writer1, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm1 := pdf.NewResourceManager(writer1)

	form0 := &Form{
		Draw: func(w *graphics.Writer) error {
			w.SetFillColor(color.DeviceGray(0.5))
			w.Rectangle(0, 0, 100, 100)
			w.Fill()
			return nil
		},
		BBox: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
	}
	ref, _, err := pdf.ResourceManagerEmbed(rm1, form0)
	if err != nil {
		t.Fatal(err)
	}
	err = rm1.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = writer1.Close()
	if err != nil {
		t.Fatal(err)
	}

	form1, err := Extract(writer1, ref)
	if err != nil {
		t.Fatal(err)
	}

	writer2, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm2 := pdf.NewResourceManager(writer2)
	ref2, _, err := pdf.ResourceManagerEmbed(rm2, form1)
	if err != nil {
		t.Fatal(err)
	}
	err = rm2.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = writer2.Close()
	if err != nil {
		t.Fatal(err)
	}

	form2, err := Extract(writer2, ref2)
	if err != nil {
		t.Fatal(err)
	}

	// check that form1 and form2 are the same (excluding Draw function)
	if diff := cmp.Diff(form1, form2); diff != "" {
		t.Errorf("Round trip failed (-got +want):\n%s", diff)
	}
}
