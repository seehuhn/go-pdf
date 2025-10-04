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

package runlength

import (
	"bytes"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRoundTrip(t *testing.T) {
	testCases := [][]byte{
		{},
		{0},
		{0, 0},
		{0, 0, 0},
		{1, 2, 3, 4, 5},
		{1, 1, 1, 1, 1},
		{0, 1, 2, 3, 0, 0, 0, 0, 4, 5, 6},
		bytes.Repeat([]byte{7}, 128),
		bytes.Repeat([]byte{8}, 127),
		bytes.Repeat([]byte{9}, 2),
	}

	for i, data := range testCases {
		buf := &bytes.Buffer{}
		enc := Encode(withDummyClose{buf})
		_, err := enc.Write(data)
		if err != nil {
			t.Fatalf("case %d: encode write: %v", i, err)
		}
		err = enc.Close()
		if err != nil {
			t.Fatalf("case %d: encode close: %v", i, err)
		}

		dec := Decode(bytes.NewReader(buf.Bytes()))
		out, err := io.ReadAll(dec)
		if err != nil {
			t.Fatalf("case %d: decode: %v", i, err)
		}

		if diff := cmp.Diff(data, out); diff != "" {
			t.Errorf("case %d: round trip failed (-want +got):\n%s", i, diff)
		}
	}
}

func FuzzRoundTrip(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("Hello, World!"))
	f.Add([]byte{0, 0, 0, 0})
	f.Add([]byte{1, 2, 3, 4, 5})

	f.Fuzz(func(t *testing.T, data []byte) {
		buf := &bytes.Buffer{}
		enc := Encode(withDummyClose{buf})
		_, err := enc.Write(data)
		if err != nil {
			t.Fatal(err)
		}
		err = enc.Close()
		if err != nil {
			t.Fatal(err)
		}

		dec := Decode(bytes.NewReader(buf.Bytes()))
		out, err := io.ReadAll(dec)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(data, out); diff != "" {
			t.Errorf("round trip failed (-want +got):\n%s", diff)
		}
	})
}

func TestDecodeExamples(t *testing.T) {
	testCases := []struct {
		name     string
		encoded  []byte
		expected []byte
	}{
		{
			name:     "empty",
			encoded:  []byte{128},
			expected: []byte{},
		},
		{
			name:     "literal run",
			encoded:  []byte{4, 1, 2, 3, 4, 5, 128},
			expected: []byte{1, 2, 3, 4, 5},
		},
		{
			name:     "replicated run",
			encoded:  []byte{255, 7, 128},
			expected: bytes.Repeat([]byte{7}, 2),
		},
		{
			name:     "max replicated run",
			encoded:  []byte{129, 7, 128},
			expected: bytes.Repeat([]byte{7}, 128),
		},
		{
			name:     "mixed runs",
			encoded:  []byte{2, 1, 2, 3, 253, 4, 1, 5, 6, 128},
			expected: []byte{1, 2, 3, 4, 4, 4, 4, 5, 6},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dec := Decode(bytes.NewReader(tc.encoded))
			out, err := io.ReadAll(dec)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			if diff := cmp.Diff(tc.expected, out); diff != "" {
				t.Errorf("decode failed (-want +got):\n%s", diff)
			}
		})
	}
}

type withDummyClose struct {
	io.Writer
}

func (w withDummyClose) Close() error {
	return nil
}
