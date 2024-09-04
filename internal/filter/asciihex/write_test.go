// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package asciihex

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"testing"
)

func TestEncoder(t *testing.T) {
	type testCase struct {
		in  []byte
		out string
	}
	cases := []testCase{
		{[]byte("ABC"), "414243>"},
		{[]byte(" "), "20>"},
		{[]byte(""), ">"},
		{[]byte{0x00, 0x0F, 0xF0, 0xFF}, "000ff0ff>"},
	}
	for i, test := range cases {
		t.Run(fmt.Sprintf("%d", i+1), func(t *testing.T) {
			buf := &bytes.Buffer{}
			enc := Encode(withDummyClose{buf}, 79)

			n, err := enc.Write(test.in)
			if err != nil {
				t.Fatalf("Write: %v", err)
			}
			if n != len(test.in) {
				t.Fatalf("Write: n=%d, want %d", n, len(test.in))
			}
			err = enc.Close()
			if err != nil {
				t.Fatalf("Close: %v", err)
			}

			if got := buf.String(); got != test.out {
				t.Fatalf("buf=%q, want %q", got, test.out)
			}
		})
	}
}

func TestLineWidths(t *testing.T) {
	for _, w := range []int{2, 39, 40, 79, 80} {
		for l := 2*w - 3; l <= 2*w+3; l++ {
			buf := &bytes.Buffer{}
			enc := Encode(withDummyClose{buf}, w)

			_, err := enc.Write(bytes.Repeat([]byte{0x1E}, l))
			if err != nil {
				t.Fatalf("Write: %v", err)
			}
			err = enc.Close()
			if err != nil {
				t.Fatalf("Close: %v", err)
			}

			scanner := bufio.NewScanner(buf)
			for scanner.Scan() {
				line := scanner.Text()
				if len(line) > w {
					t.Fatalf("width=%d, len=%d: %q", w, l, line)
				}
			}
		}
	}
}

// withDummyClose turns and io.Writer into an io.WriteCloser.
type withDummyClose struct {
	io.Writer
}

func (w withDummyClose) Close() error {
	return nil
}
