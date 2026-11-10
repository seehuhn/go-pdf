// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
