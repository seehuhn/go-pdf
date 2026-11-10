// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
