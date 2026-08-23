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

package builder

import (
	"io"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestCleanNumbers exercises builder paths which compute their operand
// values, writing the resulting operators as an uncompressed content stream.
// There are no explicit assertions on the emitted numbers here: the check
// registered by memfile.NewPDFWriter fails this test if an unrounded number
// reaches the output.
func TestCleanNumbers(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)

	font, err := standard.Helvetica.New()
	if err != nil {
		t.Fatal(err)
	}

	b := New(content.Page, nil, pdf.V1_7)
	b.TextBegin()
	b.TextSetFont(font, 10)
	b.TextSetHorizontalScaling(0.07) // 0.07*100 = 7.000000000000001
	b.TextShow("x")
	b.TextEnd()
	if b.Err != nil {
		t.Fatal(b.Err)
	}

	ops := Must(b.Harvest())
	raw, err := ops.RawBytes()
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()

	stm, err := w.OpenStream(w.Alloc(), pdf.Dict{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(stm, raw); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}
