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

package memfile

import (
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestCheckNumbersPlain(t *testing.T) {
	data := []byte("<< /X 137.63799999999998 /Y 42 >>")
	hits := CheckNumbers(data)
	if len(hits) != 1 {
		t.Fatalf("got %d hits, want 1: %q", len(hits), hits)
	}
	if !strings.Contains(hits[0], "137.637") {
		t.Errorf("hit %q does not mention the offending number", hits[0])
	}
}

func TestCheckNumbersClean(t *testing.T) {
	for _, data := range [][]byte{
		[]byte("<< /X 137.638 /Y 42 /Z 0.123456 >>"),
		[]byte("<< /WhitePoint[0.9504559 1 1.0890578]>>"), // deliberate constants
		[]byte("<< /CYX 0.000189394 >>"),                  // leading zeros are not precision
		[]byte("1 0 obj\n<< /FontMatrix [0.001 0 0 0.001 0 0] >>\nendobj"),
		nil,
	} {
		if hits := CheckNumbers(data); len(hits) != 0 {
			t.Errorf("%q: got %d hits, want 0: %q", data, len(hits), hits)
		}
	}
}

func TestCheckNumbersThreshold(t *testing.T) {
	for _, data := range [][]byte{
		[]byte("/A 137.63799999"),       // eight significant fractional digits
		[]byte("/Tz 7.000000000000001"), // dust behind leading zeros (0.07*100)
		[]byte("/M 28.999999999999996"), // dust behind trailing nines (0.29*100)
	} {
		if hits := CheckNumbers(data); len(hits) != 1 {
			t.Errorf("%q: got %d hits, want 1", data, len(hits))
		}
	}
	for _, data := range [][]byte{
		[]byte("/A 137.6379999"),   // seven significant fractional digits
		[]byte("/CYX 0.000189394"), // leading zeros are not precision
		[]byte("/S 1.0000001"),     // short zero runs are ordinary decimals
	} {
		if hits := CheckNumbers(data); len(hits) != 0 {
			t.Errorf("%q: got %d hits, want 0: %q", data, len(hits), hits)
		}
	}
}

func TestCheckNumbersSkipBinaryStream(t *testing.T) {
	binary := append([]byte{}, make([]byte, 8)...)
	copy(binary, "\x00\x01\x02.1234567890\xff\xfe")
	data := append([]byte("<< /Length 18 >>\nstream\n"), binary...)
	data = append(data, []byte("\nendstream\ntrailer")...)
	if hits := CheckNumbers(data); len(hits) != 0 {
		t.Errorf("binary stream contents leaked into check: %q", hits)
	}
}

func TestCheckNumbersScanContentStream(t *testing.T) {
	data := []byte("<< /Length 36 >>\nstream\nq 137.63799999999998 0 0 1 0 0 cm Q\nendstream")
	hits := CheckNumbers(data)
	if len(hits) != 1 {
		t.Fatalf("got %d hits, want 1: %q", len(hits), hits)
	}
	if !strings.Contains(hits[0], "137.637") {
		t.Errorf("hit %q does not mention the offending number", hits[0])
	}
}

func TestCheckNumbersEndstreamIsNotAStreamStart(t *testing.T) {
	plain, streams := splitRegions([]byte("\nendstream\n<< /X 137.63799999999998 >>"))
	if len(streams) != 0 {
		t.Errorf("'endstream' misdetected as stream start (%d streams)", len(streams))
	}
	if len(plain) == 0 {
		t.Fatal("no plain regions found")
	}
	last := plain[len(plain)-1]
	if !strings.Contains(string(last), "137.637") {
		t.Errorf("dict region lost: %q", last)
	}
}

func TestUglyNumberIsDetectable(t *testing.T) {
	// build the file without NewPDFWriter, so the automatic check does not
	// fail this test for the very value we want to detect here
	buf := New()
	w, err := pdf.NewWriter(buf, pdf.V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := AddBlankPage(w); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(w.Alloc(), pdf.Dict{"Ugly": pdf.Real(137.63799999999998)}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if hits := CheckNumbers(buf.Data); len(hits) == 0 {
		t.Error("generated file contains no detectable long decimal tail")
	}
}

func TestNewPDFWriterCleanFile(t *testing.T) {
	w, _ := NewPDFWriter(t, pdf.V1_7, nil)
	err := w.Put(w.Alloc(), pdf.Dict{"Fine": pdf.Real(13.75)})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCheckNumbersSkipStrings(t *testing.T) {
	for _, data := range [][]byte{
		[]byte("<< /Type /OCG /Name (0000000.00000000000) >>"),     // a layer name
		[]byte("<< /T (a\\) 137.63799999999998) >>"),               // escaped delimiter
		[]byte("<< /T ((137.63799999999998)) >>"),                  // nested parentheses
		[]byte("stream\nBT (137.63799999999998) Tj ET\nendstream"), // shown text
	} {
		if hits := CheckNumbers(data); len(hits) != 0 {
			t.Errorf("%q: got %d hits, want 0: %q", data, len(hits), hits)
		}
	}
}

func TestCheckNumbersAfterString(t *testing.T) {
	data := []byte("<< /T (text) /X 137.63799999999998 >>")
	hits := CheckNumbers(data)
	if len(hits) != 1 {
		t.Fatalf("got %d hits, want 1: %q", len(hits), hits)
	}
	if !strings.Contains(hits[0], "137.637") {
		t.Errorf("hit %q does not mention the offending number", hits[0])
	}
}

func TestCheckNumbersUnterminatedString(t *testing.T) {
	// a lone '(' in the prose of a metadata stream is not a string, and must
	// not hide the numbers which follow it
	data := []byte("stream\n<x>a lone ( paren</x> 137.63799999999998\nendstream")
	hits := CheckNumbers(data)
	if len(hits) != 1 {
		t.Fatalf("got %d hits, want 1: %q", len(hits), hits)
	}
	if !strings.Contains(hits[0], "137.637") {
		t.Errorf("hit %q does not mention the offending number", hits[0])
	}
}
