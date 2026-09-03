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

package pdf_test

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestTwoManagersShareObjects(t *testing.T) {
	w, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm1 := pdf.NewResourceManager(w)
	rm2 := pdf.NewResourceManager(w)

	info := &pdf.Info{Title: "shared"}
	ref1, err := rm1.Embed(info)
	if err != nil {
		t.Fatal(err)
	}
	ref2, err := rm2.Embed(info)
	if err != nil {
		t.Fatal(err)
	}
	if ref1 != ref2 {
		t.Errorf("same value embedded as %v and %v", ref1, ref2)
	}
	if err := rm1.Close(); err != nil {
		t.Fatal(err)
	}
	if err := rm2.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(f.Data, []byte("(shared)")); n != 1 {
		t.Errorf("title written %d times, want 1", n)
	}
}
