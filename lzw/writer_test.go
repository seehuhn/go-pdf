// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package lzw

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"
)

func TestLZWSimple(t *testing.T) {
	// This is example 1 from section 7.4.4.2 of PDF 32000-1:2008
	in := []byte{45, 45, 45, 45, 45, 65, 45, 45, 45, 66}

	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, false)
	if err != nil {
		t.Fatal(err)
	}

	_, err = w.Write(in)
	if err != nil {
		t.Fatal(err)
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
	out := buf.Bytes()

	expected := []byte{0x80, 0x0B, 0x60, 0x50, 0x22, 0x0C, 0x0C, 0x85, 0x01}

	if !bytes.Equal(out, expected) {
		fmt.Printf("% 0X\n", out)
		fmt.Printf("% 0X\n", expected)
		t.Fatal("wrong result")
	}
}

func TestRoundtrip(t *testing.T) {
	for _, ecw := range []bool{false, true} {
		for _, ecr := range []bool{false, true} {
			fmt.Println(ecw, ecr)
			good := ecw == ecr

			body, err := os.ReadFile("reader.go")
			if err != nil {
				t.Fatal(err)
			}

			buf := &bytes.Buffer{}
			w, err := NewWriter(buf, ecw)
			if err != nil {
				t.Fatal(err)
			}
			var in []byte
			for len(in) < 1e5 {
				in = append(in, body...)
				_, err = w.Write(body)
				if err != nil {
					t.Fatal(err)
				}
			}
			err = w.Close()
			if err != nil {
				t.Fatal(err)
			}

			r := NewReader(buf, ecr)
			out, err := io.ReadAll(r)
			if err != nil {
				if good {
					t.Error(ecw, ecr, err)
				}
				continue
			}
			err = r.Close()
			if err != nil {
				if good {
					t.Error(ecw, ecr, err)
				}
				continue
			}

			if good && !bytes.Equal(in, out) {
				t.Error(ecw, ecr, "roundtrip failed")
			} else if !good && bytes.Equal(in, out) {
				t.Error(ecw, ecr, "roundtrip unexpectedly succeeded")
			}
		}
	}
}
