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

package charcode

import (
	"bytes"
	"fmt"
	"testing"
)

type testCase struct {
	name        string
	input       []byte
	wantCode    Code
	wantConsume int
	wantValid   bool
}

type testRanges struct {
	name   string
	ranges CodeSpaceRange
	cases  []testCase
}

var testCases = []testRanges{
	{
		name: "simple range",
		ranges: CodeSpaceRange{
			{Low: []byte{0x00}, High: []byte{0xFF}},
		},
		cases: []testCase{
			{
				name:        "valid input",
				input:       []byte{0x20},
				wantCode:    0x20,
				wantConsume: 1,
				wantValid:   true,
			},
			{
				name:        "empty input",
				input:       []byte{},
				wantConsume: 0,
			},
		},
	},

	{
		name: "one-byte range",
		ranges: CodeSpaceRange{
			{Low: []byte{0x20}, High: []byte{0x7F}},
		},
		cases: []testCase{
			{
				name:        "lowest valid input",
				input:       []byte{0x20},
				wantCode:    0x20,
				wantConsume: 1,
				wantValid:   true,
			},
			{
				name:        "valid input",
				input:       []byte{0x40},
				wantCode:    0x40,
				wantConsume: 1,
				wantValid:   true,
			},
			{
				name:        "highest valid input",
				input:       []byte{0x7F},
				wantCode:    0x7F,
				wantConsume: 1,
				wantValid:   true,
			},
			{
				name:        "low invalid input",
				input:       []byte{0x1F, 0x01, 0x02},
				wantCode:    0x1F,
				wantConsume: 1,
			},
			{
				name:        "high invalid input",
				input:       []byte{0x80, 0x01, 0x02},
				wantCode:    0x80,
				wantConsume: 1,
			},
			{
				name:        "empty input",
				input:       []byte{},
				wantConsume: 0,
			},
		},
	},

	{
		name: "2-byte codes",
		ranges: CodeSpaceRange{
			{Low: []byte{0x00, 0x00}, High: []byte{0x7F, 0x7F}},
		},
		cases: []testCase{
			{
				name:        "valid input",
				input:       []byte{0x20, 0x30},
				wantCode:    0x3020,
				wantConsume: 2,
				wantValid:   true,
			},
			{
				name:        "byte 1 invalid",
				input:       []byte{0x80, 0x20, 0x30},
				wantCode:    0x2080,
				wantConsume: 2,
			},
			{
				name:        "byte 2 invalid",
				input:       []byte{0x20, 0x80},
				wantCode:    0x8020,
				wantConsume: 2,
			},
			{
				name:        "empty input",
				input:       []byte{},
				wantConsume: 0,
			},
			{
				name:        "short input",
				input:       []byte{0x20},
				wantCode:    0x0020,
				wantConsume: 1,
			},
		},
	},

	{
		name: "3-byte codes",
		ranges: CodeSpaceRange{
			{Low: []byte{0x00, 0x00, 0x00}, High: []byte{0x7F, 0x7F, 0x7F}},
		},
		cases: []testCase{
			{
				name:        "valid input",
				input:       []byte{0x20, 0x30, 0x40},
				wantCode:    0x403020,
				wantConsume: 3,
				wantValid:   true,
			},
			{
				name:        "byte 1 invalid",
				input:       []byte{0x80, 0x20, 0x20},
				wantCode:    0x202080,
				wantConsume: 3,
			},
			{
				name:        "byte 2 invalid",
				input:       []byte{0x20, 0x80, 0x20},
				wantCode:    0x208020,
				wantConsume: 3,
			},
			{
				name:        "byte 3 invalid",
				input:       []byte{0x20, 0x20, 0x80},
				wantCode:    0x802020,
				wantConsume: 3,
			},
			{
				name:        "empty input",
				input:       []byte{},
				wantConsume: 0,
			},
			{
				name:        "short input 1",
				input:       []byte{0x20},
				wantCode:    0x000020,
				wantConsume: 1,
			},
			{
				name:        "short input 2",
				input:       []byte{0x20, 0x30},
				wantCode:    0x003020,
				wantConsume: 2,
			},
		},
	},

	{
		name: "4-byte codes",
		ranges: CodeSpaceRange{
			{Low: []byte{0x00, 0x00, 0x00, 0x00}, High: []byte{0x7F, 0x7F, 0x7F, 0x7F}},
		},
		cases: []testCase{
			{
				name:        "valid input",
				input:       []byte{0x20, 0x30, 0x40, 0x50},
				wantCode:    0x50403020,
				wantConsume: 4,
				wantValid:   true,
			},
			{
				name:        "byte 1 invalid",
				input:       []byte{0x80, 0x20, 0x20, 0x20},
				wantCode:    0x20202080,
				wantConsume: 4,
			},
			{
				name:        "byte 2 invalid",
				input:       []byte{0x20, 0x80, 0x20, 0x20},
				wantCode:    0x20208020,
				wantConsume: 4,
			},
			{
				name:        "byte 3 invalid",
				input:       []byte{0x20, 0x20, 0x80, 0x20},
				wantCode:    0x20802020,
				wantConsume: 4,
			},
			{
				name:        "byte 4 invalid",
				input:       []byte{0x20, 0x20, 0x20, 0x80},
				wantCode:    0x80202020,
				wantConsume: 4,
			},
			{
				name:        "empty input",
				input:       []byte{},
				wantConsume: 0,
			},
			{
				name:        "short input 1",
				input:       []byte{0x20},
				wantCode:    0x00000020,
				wantConsume: 1,
			},
			{
				name:        "short input 2",
				input:       []byte{0x20, 0x30},
				wantCode:    0x00003020,
				wantConsume: 2,
			},
			{
				name:        "short input 3",
				input:       []byte{0x20, 0x30, 0x40},
				wantCode:    0x00403020,
				wantConsume: 3,
			},
		},
	},

	{
		name: "mixed ranges",
		ranges: CodeSpaceRange{
			{Low: []byte{0x00}, High: []byte{0x7F}},
			{Low: []byte{0x81, 0x40}, High: []byte{0xFE, 0xFE}},
		},
		cases: []testCase{
			{
				name:        "valid single-byte",
				input:       []byte{0x7F},
				wantCode:    0x7F,
				wantConsume: 1,
				wantValid:   true,
			},
			{
				name:        "valid double-byte",
				input:       []byte{0x90, 0x41},
				wantCode:    0x4190,
				wantConsume: 2,
				wantValid:   true,
			},
			{
				name:        "invalid single-byte",
				input:       []byte{0x80},
				wantCode:    0x80,
				wantConsume: 1,
			},
			{
				name:        "invalid second byte",
				input:       []byte{0xFE, 0x3F},
				wantCode:    0x3FFE,
				wantConsume: 2,
			},
		},
	},

	{
		name: "complicated ranges",
		ranges: CodeSpaceRange{
			{Low: []byte{0x00, 0x80, 0x00}, High: []byte{0x20, 0x80, 0x80}},
			{Low: []byte{0x10, 0x80, 0x81, 0x01}, High: []byte{0x30, 0x80, 0x81, 0xFF}},
		},
		cases: []testCase{
			{
				name:        "valid 3 bytes",
				input:       []byte{0x10, 0x80, 0x80},
				wantCode:    0x808010,
				wantConsume: 3,
				wantValid:   true,
			},
			{
				name:        "valid 4 bytes",
				input:       []byte{0x10, 0x80, 0x81, 0xFF},
				wantCode:    0xFF818010,
				wantConsume: 4,
				wantValid:   true,
			},
			{
				name:        "invalid byte 1",
				input:       []byte{0x40, 0x80, 0x81, 0xFF},
				wantCode:    0x818040,
				wantConsume: 3,
			},
			{
				name:        "invalid byte 2",
				input:       []byte{0x18, 0x81, 0x81, 0xFF},
				wantCode:    0x818118,
				wantConsume: 3,
			},
			{
				name:        "invalid byte 3",
				input:       []byte{0x18, 0x80, 0x82, 0xFF},
				wantCode:    0x828018,
				wantConsume: 3,
			},
			{
				name:        "invalid byte 4",
				input:       []byte{0x18, 0x80, 0x81, 0x00},
				wantCode:    0x00818018,
				wantConsume: 4,
			},
		},
	},

	{
		name: "many lengths",
		ranges: CodeSpaceRange{
			{Low: []byte{0x00}, High: []byte{0x7F}},
			{Low: []byte{0x80, 0x01}, High: []byte{0xBF, 0xFF}},
			{Low: []byte{0xC0, 0x01, 0x01}, High: []byte{0xDF, 0xFF, 0xFF}},
			{Low: []byte{0xE0, 0x01, 0x01, 0x01}, High: []byte{0xEF, 0xFF, 0xFF, 0xFF}},
		},
		cases: []testCase{
			{
				name:        "valid code",
				input:       []byte{0xC0, 0x80, 0xA0},
				wantCode:    0xA080C0,
				wantConsume: 3,
				wantValid:   true,
			},
			{
				name:        "invalid code",
				input:       []byte{0xD0, 0x00, 0x80},
				wantCode:    0x8000D0,
				wantConsume: 3,
			},
		},
	},

	{
		name:   "no codes",
		ranges: CodeSpaceRange{},
		cases: []testCase{
			{
				name:        "single byte",
				input:       []byte{0x01},
				wantCode:    0x01,
				wantConsume: 1,
			},
		},
	},

	{
		name: "repeated ranges",
		ranges: CodeSpaceRange{
			{Low: []byte{0x00, 0x80}, High: []byte{0x7F, 0xFF}},
			{Low: []byte{0x00, 0x80}, High: []byte{0x7F, 0xFF}},
		},
		cases: []testCase{
			{
				name:        "valid input",
				input:       []byte{0x20, 0xA0},
				wantCode:    0xA020,
				wantConsume: 2,
				wantValid:   true,
			},
			{
				name:        "byte 1 invalid",
				input:       []byte{0x80, 0x20, 0x30},
				wantCode:    0x2080,
				wantConsume: 2,
			},
			{
				name:        "byte 2 invalid",
				input:       []byte{0x20, 0x20},
				wantCode:    0x2020,
				wantConsume: 2,
			},
			{
				name:        "empty input",
				input:       []byte{},
				wantConsume: 0,
			},
			{
				name:        "short input",
				input:       []byte{0x20},
				wantCode:    0x0020,
				wantConsume: 1,
			},
		},
	},

	{
		name: "gap",
		ranges: []Range{
			{Low: []byte{0x00}, High: []byte{0x1f}},
			{Low: []byte{0x30}, High: []byte{0xff}},
		},
		cases: []testCase{
			{
				name:        "start of first range",
				input:       []byte{0x00},
				wantCode:    0x00,
				wantConsume: 1,
				wantValid:   true,
			},
			{
				name:        "end of first range",
				input:       []byte{0x1f},
				wantCode:    0x1f,
				wantConsume: 1,
				wantValid:   true,
			},
			{
				name:        "start of gap",
				input:       []byte{0x20},
				wantCode:    0x20,
				wantConsume: 1,
				wantValid:   false,
			},
			{
				name:        "end of gap",
				input:       []byte{0x2f},
				wantCode:    0x2f,
				wantConsume: 1,
				wantValid:   false,
			},
			{
				name:        "start of second range",
				input:       []byte{0x30},
				wantCode:    0x30,
				wantConsume: 1,
				wantValid:   true,
			},
			{
				name:        "end of second range",
				input:       []byte{0xff},
				wantCode:    0xff,
				wantConsume: 1,
				wantValid:   true,
			},
		},
	},
}

func TestCodecDecode(t *testing.T) {
	for i, r := range testCases {
		t.Run(fmt.Sprintf("%02d-%s", i+1, r.name), func(t *testing.T) {
			c, err := NewCodec(r.ranges)
			if err != nil {
				t.Fatal(err)
			}
			for j, testCase := range r.cases {
				t.Run(fmt.Sprintf("%02d-%s", j+1, testCase.name), func(t *testing.T) {
					gotCode, gotConsume, gotValid := c.Decode(testCase.input)

					if gotConsume > len(testCase.input) {
						t.Errorf("Decode() consumed more bytes than input: got %d, want %d", gotConsume, len(testCase.input))
					}

					if gotValid != testCase.wantValid {
						t.Errorf("Decode() valid = %v, want %v", gotValid, testCase.wantValid)
						return
					}

					if gotCode != testCase.wantCode {
						t.Errorf("Decode() code = %08x, want %08x", gotCode, testCase.wantCode)
					}
					if gotConsume != testCase.wantConsume {
						t.Errorf("Decode() consume = %v, want %v", gotConsume, testCase.wantConsume)
					}
				})
			}
		})
	}
}

func TestCodecError(t *testing.T) {
	testCases := []struct {
		name   string
		ranges CodeSpaceRange
	}{
		{
			name: "1 byte prefix",
			ranges: CodeSpaceRange{
				{Low: []byte{0x00}, High: []byte{0xFF}},
				{Low: []byte{0x00, 0x80}, High: []byte{0x00, 0xFF}},
			},
		},
		{
			name: "3 byte prefix",
			ranges: CodeSpaceRange{
				{Low: []byte{0x00, 0x00, 0x00, 0x00}, High: []byte{0x00, 0x00, 0x00, 0x00}},
				{Low: []byte{0x00, 0x00, 0x00}, High: []byte{0x00, 0x00, 0x00}},
			},
		},
		{
			name: "inconsistent length",
			ranges: CodeSpaceRange{
				{Low: []byte{0x00}, High: []byte{0x00, 0xFF}},
			},
		},
		{
			name: "empty range",
			ranges: CodeSpaceRange{
				{Low: []byte{}, High: []byte{}},
			},
		},
		{
			name: "too long",
			ranges: CodeSpaceRange{
				{Low: []byte{0x00, 0x00, 0x00, 0x00, 0x00}, High: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%02d-%s", i, tc.name), func(t *testing.T) {
			_, err := NewCodec(tc.ranges)
			if err != errInvalidCodeSpaceRange {
				t.Errorf("NewCodec() returned unexpected error: got %v, want %v", err, errInvalidCodeSpaceRange)
			}
		})
	}
}

func TestCodecDeduplication(t *testing.T) {
	// In the following code space range, one sub-tree can be shared
	// to parse the second byte of all three ranges.
	csr := CodeSpaceRange{
		{Low: []byte{1, 10}, High: []byte{1, 20}},
		{Low: []byte{3, 10}, High: []byte{3, 20}},
		{Low: []byte{5, 10}, High: []byte{5, 20}},
	}
	c, err := NewCodec(csr)
	if err != nil {
		t.Fatal(err)
	}

	var cc []uint16
	for _, node := range c.nodes {
		cc = append(cc, node.child)
		if node.bound == 0xFF {
			break
		}
	}

	// now cc should alternate between invalidConsume1 and the shared sub-tree
	if len(cc) != 7 {
		t.Fatalf("expected 7 nodes, got %d", len(cc))
	}
	for i := 0; i < 7; i += 2 {
		if cc[i] != invalidConsume1 {
			t.Fatalf("expected invalidConsume1, got %d", cc[i])
		}
	}
	for i := 3; i < 7; i += 2 {
		if cc[i] != cc[1] {
			t.Fatalf("expected shared sub-tree %d, got %d", cc[1], cc[i])
		}
	}
}

func TestCodecCodeSpaceRange(t *testing.T) {
	for i, r := range testCases {
		t.Run(fmt.Sprintf("%02d-%s", i+1, r.name), func(t *testing.T) {
			csr1 := r.ranges
			c, err := NewCodec(csr1)
			if err != nil {
				t.Fatal(err)
			}
			csr2 := c.CodeSpaceRange()
			if !csr1.isEquivalent(csr2) {
				t.Errorf("CodeSpaceRange() = %v, want %v", csr2, csr1)
			}
		})
	}
}

func FuzzRoundTrip(f *testing.F) {
	for sel, tc := range testCases {
		for _, c := range tc.cases {
			f.Add(uint(sel), c.input)
		}
	}

	f.Fuzz(func(t *testing.T, sel uint, input []byte) {
		sel = sel % uint(len(testCases))
		c, err := NewCodec(testCases[sel].ranges)
		if err != nil {
			t.Fatal(err)
		}

		orig := input
		output := make([]byte, 0, len(input))
		for len(input) > 0 {
			code, consumed, _ := c.Decode(input)
			if consumed == 0 {
				break
			}
			input = input[consumed:]

			output = c.AppendCode(output, code)
		}

		nIn := len(orig)
		nOut := len(output)
		if nIn > nOut || nIn < nOut-3 || !bytes.Equal(orig, output[:nIn]) || hasNonZero(output[nIn:]) {
			t.Fatalf("sel: %d, input: %x, output: %x", sel, orig, output)
		}
	})
}

func hasNonZero(slice []byte) bool {
	for _, b := range slice {
		if b != 0 {
			return true
		}
	}
	return false
}

// BenchmarkCodecSingleByte benchmarks the Decode method for single-byte codes
func BenchmarkCodecSingleByte(b *testing.B) {
	csr := CodeSpaceRange{
		{Low: []byte{0x00}, High: []byte{0xFF}},
	}
	c, err := NewCodec(csr)
	if err != nil {
		b.Fatal(err)
	}

	input := []byte("ABCDE")

	b.ResetTimer()
	for range b.N {
		j := 0
		for j < len(input) {
			_, consumed, _ := c.Decode(input[j:])
			j += consumed
		}
	}
}

// BenchmarkCodecTwoByte benchmarks the Decode method for two-byte codes
func BenchmarkCodecTwoByte(b *testing.B) {
	csr := CodeSpaceRange{
		{Low: []byte{0x81, 0x40}, High: []byte{0xFE, 0xFE}},
	}
	c, err := NewCodec(csr)
	if err != nil {
		b.Fatal(err)
	}

	input := []byte{0x81, 0x40, 0x90, 0x80, 0xA0, 0xC0, 0xB0, 0xFF, 0xFE, 0xFE}

	b.ResetTimer()
	for range b.N {
		j := 0
		for j < len(input) {
			_, consumed, _ := c.Decode(input[j:])
			j += consumed
		}
	}
}

func BenchmarkCodecUTF8(b *testing.B) {
	csr := CodeSpaceRange{
		// 1-byte sequences (ASCII): 0xxxxxxx
		{Low: []byte{0x00}, High: []byte{0x7F}},

		// 2-byte sequences: 110xxxxx 10xxxxxx
		{Low: []byte{0xC0, 0x80}, High: []byte{0xDF, 0xBF}},

		// 3-byte sequences: 1110xxxx 10xxxxxx 10xxxxxx
		{Low: []byte{0xE0, 0x80, 0x80}, High: []byte{0xEF, 0xBF, 0xBF}},

		// 4-byte sequences: 11110xxx 10xxxxxx 10xxxxxx 10xxxxxx
		{Low: []byte{0xF0, 0x80, 0x80, 0x80}, High: []byte{0xF7, 0x8F, 0xBF, 0xBF}},
	}
	c, err := NewCodec(csr)
	if err != nil {
		b.Fatal(err)
	}

	input := []byte("Hello, 世界")

	b.ResetTimer()
	for range b.N {
		j := 0
		for j < len(input) {
			_, consumed, _ := c.Decode(input[j:])
			j += consumed
		}
	}
}
