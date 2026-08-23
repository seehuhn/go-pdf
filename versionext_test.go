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
	"io"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestReadFutureVersion checks that a file whose header announces a
// version newer than MaxVersion is read, with the version preserved in
// the metadata.
func TestReadFutureVersion(t *testing.T) {
	w, buf := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	data := bytes.Replace(buf.Data, []byte("%PDF-2.0"), []byte("%PDF-2.3"), 1)
	if bytes.Equal(data, buf.Data) {
		t.Fatal("header not found")
	}

	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.GetMeta().Version; got != pdf.Version(203) {
		t.Errorf("version: got %v, want 2.3", got)
	}
	if r.GetMeta().Version <= pdf.V2_0 {
		t.Error("future version does not sort after V2_0")
	}
}

// TestWriteFutureVersion checks that the writer refuses versions beyond
// MaxVersion.
func TestWriteFutureVersion(t *testing.T) {
	if _, err := pdf.NewWriter(io.Discard, pdf.Version(203), nil); err == nil {
		t.Error("writer accepted a version beyond MaxVersion")
	}
}
