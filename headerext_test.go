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

// TestReaderLongPreamble checks that a file opens however many bytes
// precede the %PDF- header, both via NewReader and via SequentialScan.
func TestReaderLongPreamble(t *testing.T) {
	w, buf := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	ref := w.Alloc()
	if err := w.Put(ref, pdf.String("payload")); err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Catalog.Pages = w.Alloc()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	preambles := map[string][]byte{
		"none":         nil,
		"1000 bytes":   bytes.Repeat([]byte{'x'}, 1000),
		"1024 bytes":   bytes.Repeat([]byte{'x'}, 1024),
		"5000 bytes":   bytes.Repeat([]byte{'x'}, 5000),
		"200000 bytes": bytes.Repeat([]byte{'x'}, 200_000),
		"stray marker": []byte("mentions %PDF- in passing\n"),
	}
	for name, preamble := range preambles {
		t.Run(name, func(t *testing.T) {
			data := append(preamble, buf.Data...)

			r1, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)), nil)
			if err != nil {
				t.Fatal(err)
			}
			info, err := pdf.SequentialScan(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatal(err)
			}
			if info.PDFStart != int64(len(preamble)) {
				t.Errorf("wrong PDFStart: got %d, want %d", info.PDFStart, len(preamble))
			}
			r2, err := info.MakeReader(nil)
			if err != nil {
				t.Fatal(err)
			}

			for _, r := range []*pdf.Reader{r1, r2} {
				if v := r.GetMeta().Version; v != pdf.V1_7 {
					t.Errorf("wrong version %s", v)
				}
				obj, err := r.Get(ref, true)
				if err != nil {
					t.Fatal(err)
				}
				if s, ok := obj.(pdf.String); !ok || string(s) != "payload" {
					t.Errorf("wrong object: %v", obj)
				}
			}
		})
	}
}
