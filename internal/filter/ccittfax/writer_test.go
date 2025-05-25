// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package ccittfax

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSimpleWriteWithoutEOL(t *testing.T) {
	p := &Params{
		Columns:   8,
		K:         0,
		EndOfLine: false,
	}

	buf := &bytes.Buffer{}
	w := NewWriter(buf, p)
	n, err := w.Write([]byte{0xFF}) // one row, eight columns, all white
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	} else if n != 1 {
		t.Fatalf("unexpected number of bytes written: %d", n)
	}
	err = w.Close()
	if err != nil {
		t.Fatalf("unexpected error on close: %v", err)
	}

	expected := []byte{0b00000000, 0b0001_1001, 0b1_0000000}
	if d := cmp.Diff(expected, buf.Bytes()); d != "" {
		t.Fatalf("unexpected output: %s", d)
	}
}

func TestSimepleWriteWithEOL(t *testing.T) {
	p := &Params{
		Columns:   8,
		K:         0,
		EndOfLine: true,
	}

	buf := &bytes.Buffer{}
	w := NewWriter(buf, p)
	n, err := w.Write([]byte{0xFF}) // one row, eight columns, all white
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	} else if n != 1 {
		t.Fatalf("unexpected number of bytes written: %d", n)
	}
	err = w.Close()
	if err != nil {
		t.Fatalf("unexpected error on close: %v", err)
	}

	expected := []byte{0b00000000, 0b0001_1001, 0b1_0000000, 0b00001_000}
	if d := cmp.Diff(expected, buf.Bytes()); d != "" {
		t.Fatalf("unexpected output: %s", d)
	}
}
