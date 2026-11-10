// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
