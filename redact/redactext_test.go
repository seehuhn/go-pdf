// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/redact"
)

// versionDoc returns a one-page document with the given header version.  A
// non-empty catVersion is the /Version entry of the catalog.
func versionDoc(t *testing.T, header, catVersion string) *pdf.Reader {
	t.Helper()
	cat := "<</Type/Catalog/Pages 2 0 R>>"
	if catVersion != "" {
		cat = "<</Type/Catalog/Pages 2 0 R/Version/" + catVersion + ">>"
	}
	data := []byte("%PDF-" + header + "\n" +
		"1 0 obj" + cat + "endobj\n" +
		"2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n" +
		"3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 100 100]>>endobj\n")
	return openPDF(t, data)
}

// openPDF opens the PDF file data.
func openPDF(t testing.TB, data []byte) *pdf.Reader {
	t.Helper()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestWriteUnsupportedVersion(t *testing.T) {
	for _, c := range []struct{ header, catVersion string }{
		{"1.8", ""},
		{"1.7", "1.9"},
		{"2.3", ""},
	} {
		r := versionDoc(t, c.header, c.catVersion)
		err := redact.Write(io.Discard, r, nil)
		if !errors.Is(err, redact.ErrUnsupportedVersion) {
			t.Errorf("header %s, catalog %q: got %v, want ErrUnsupportedVersion",
				c.header, c.catVersion, err)
		}
	}
}

func TestWriteSupportedVersion(t *testing.T) {
	for _, header := range []string{"1.7", "2.0"} {
		r := versionDoc(t, header, "")
		var buf bytes.Buffer
		if err := redact.Write(&buf, r, nil); err != nil {
			t.Errorf("header %s: %v", header, err)
			continue
		}
		out := openPDF(t, buf.Bytes())
		if got := out.GetMeta().Version.String(); got != header {
			t.Errorf("output has version %s, want %s", got, header)
		}
	}
}
