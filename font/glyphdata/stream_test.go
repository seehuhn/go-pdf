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

package glyphdata

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/limits"
)

// TestExtractStreamOversize verifies that a font program stream larger
// than limits.MaxFontProgramBytes is rejected during WriteTo,
// blocking a decompression-bomb attack on font embedding.
func TestExtractStreamOversize(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	ref := w.Alloc()
	stm, err := w.OpenStream(ref, pdf.Dict{})
	if err != nil {
		t.Fatal(err)
	}
	body := make([]byte, limits.MaxFontProgramBytes+1)
	if _, err := stm.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := ExtractStream(pdf.NewCursor(w), ref, "Type1", "FontFile")
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("ExtractStream returned nil stream")
	}
	if err := s.WriteTo(&bytes.Buffer{}, nil); err == nil {
		t.Fatal("expected error for oversize font program, got nil")
	}
}
