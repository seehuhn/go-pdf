// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package printprep

import (
	"bytes"
	"io"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// FuzzWrite checks that Write never panics or hangs on malformed input.  The
// source is untrusted data; simplification must fail gracefully or succeed,
// but never crash.
func FuzzWrite(f *testing.F) {
	// seed: a page with text and a filled rectangle
	buf := memfile.New()
	if doc, err := document.WriteMultiPage(buf, document.A4, pdf.V1_7, nil); err == nil {
		if F, err := gofont.Regular.NewSimple(nil); err == nil {
			p := doc.AddPage()
			p.SetFillColor(color.DeviceGray(0.5))
			p.Rectangle(72, 72, 100, 100)
			p.Fill()
			p.TextBegin()
			p.TextSetFont(F, 12)
			p.TextFirstLine(72, 700)
			p.TextShow("Hello, fuzz!")
			p.TextEnd()
			p.Close()
			doc.Close()
			f.Add(buf.Data)
		}
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// the whole pipeline (open + simplify) must not panic or hang on
		// untrusted input; errors are acceptable
		r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)), nil)
		if err != nil {
			t.Skip()
		}
		_ = Write(io.Discard, r, nil)
		_ = Write(io.Discard, r, &Options{Pages: []int{0}})
	})
}
