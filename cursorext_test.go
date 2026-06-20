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
	"errors"
	"io"
	"os"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// writeStream writes body to a new stream object and returns its reference.
func writeStream(t *testing.T, w *pdf.Writer, body []byte, filters ...pdf.Filter) pdf.Reference {
	t.Helper()
	ref := w.Alloc()
	stm, err := w.OpenStream(ref, nil, filters...)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stm.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	return ref
}

func TestCursorStreamReader(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	body := []byte("hello, cursor stream")
	ref := writeStream(t, w, body, pdf.FilterCompress{})

	c := pdf.NewCursor(w)

	// round trip via a reference
	r, err := c.StreamReader(ref)
	if err != nil {
		t.Fatalf("StreamReader: %v", err)
	}
	got, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("round trip: got %q, want %q", got, body)
	}

	// a nil object yields os.ErrNotExist
	if _, err := c.StreamReader(nil); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("nil obj: got %v, want os.ErrNotExist", err)
	}

	// a non-stream object yields an error
	if _, err := c.StreamReader(pdf.Integer(3)); err == nil {
		t.Error("non-stream obj: expected an error")
	}
}

func TestCursorReadAll(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	body := []byte("0123456789")
	ref := writeStream(t, w, body)

	c := pdf.NewCursor(w)

	got, err := c.ReadAll(ref, 100)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("ReadAll: got %q, want %q", got, body)
	}

	// decoded content larger than the limit is an error
	if _, err := c.ReadAll(ref, 4); err == nil {
		t.Error("over limit: expected an error")
	}

	// a nil object yields a nil result
	if got, err := c.ReadAll(nil, 100); err != nil || got != nil {
		t.Errorf("nil obj: got (%q, %v), want (nil, nil)", got, err)
	}
}

func TestCursorFilters(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	c := pdf.NewCursor(w)

	// a dictionary without /Filter has an empty filter chain
	if filters, err := c.Filters(pdf.Dict{}); err != nil {
		t.Fatalf("Filters(no /Filter): %v", err)
	} else if len(filters) != 0 {
		t.Errorf("no /Filter: got %d filters, want 0", len(filters))
	}

	// a single named filter resolves to one entry
	dict := pdf.Dict{"Filter": pdf.Name("FlateDecode")}
	if filters, err := c.Filters(dict); err != nil {
		t.Fatalf("Filters(FlateDecode): %v", err)
	} else if len(filters) != 1 {
		t.Errorf("FlateDecode: got %d filters, want 1", len(filters))
	}
}

func TestCursorVersion(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	if v := pdf.NewCursor(w).Version(); v != pdf.V1_7 {
		t.Errorf("Version: got %v, want %v", v, pdf.V1_7)
	}
}
