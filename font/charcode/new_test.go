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
	"fmt"
	"testing"
)

func TestDecoder_Decode(t *testing.T) {
	type testCase struct {
		name        string
		input       []byte
		wantCode    uint32
		wantConsume int
		wantValid   bool
	}
	type testCSR struct {
		name   string
		ranges CodeSpaceRange
		cases  []testCase
	}

	tests := []testCSR{
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
					input:       []byte{0x1f},
					wantConsume: 1,
				},
				{
					name:        "high invalid input",
					input:       []byte{0x80},
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
					input:       []byte{0x80, 0x20},
					wantConsume: 2,
				},
				{
					name:        "byte 2 invalid",
					input:       []byte{0x20, 0x80},
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
					wantConsume: 3,
				},
				{
					name:        "byte 2 invalid",
					input:       []byte{0x20, 0x80, 0x20},
					wantConsume: 3,
				},
				{
					name:        "byte 3 invalid",
					input:       []byte{0x20, 0x20, 0x80},
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
					wantConsume: 1,
				},
				{
					name:        "short input 2",
					input:       []byte{0x20, 0x30},
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
					wantConsume: 4,
				},
				{
					name:        "byte 2 invalid",
					input:       []byte{0x20, 0x80, 0x20, 0x20},
					wantConsume: 4,
				},
				{
					name:        "byte 3 invalid",
					input:       []byte{0x20, 0x20, 0x80, 0x20},
					wantConsume: 4,
				},
				{
					name:        "byte 4 invalid",
					input:       []byte{0x20, 0x20, 0x20, 0x80},
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
					wantConsume: 1,
				},
				{
					name:        "short input 2",
					input:       []byte{0x20, 0x30},
					wantConsume: 2,
				},
				{
					name:        "short input 3",
					input:       []byte{0x20, 0x30, 0x40},
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
					wantConsume: 1,
				},
				{
					name:        "invalid second byte",
					input:       []byte{0xFE, 0x3F},
					wantCode:    0x00,
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
					wantCode:    0,
					wantConsume: 3,
				},
				{
					name:        "invalid byte 2",
					input:       []byte{0x18, 0x81, 0x81, 0xFF},
					wantCode:    0,
					wantConsume: 3,
				},
				{
					name:        "invalid byte 3",
					input:       []byte{0x18, 0x80, 0x82, 0xFF},
					wantCode:    0,
					wantConsume: 3,
				},
				{
					name:        "invalid byte 4",
					input:       []byte{0x18, 0x80, 0x81, 0x00},
					wantCode:    0,
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
					wantConsume: 3,
				},
			},
		},
	}

	for i, r := range tests {
		t.Run(fmt.Sprintf("%02d-%s", i+1, r.name), func(t *testing.T) {
			d := NewDecoder(r.ranges)
			for j, c := range r.cases {
				t.Run(fmt.Sprintf("%02d-%s", j+1, c.name), func(t *testing.T) {
					gotCode, gotConsume, gotValid := d.Decode(c.input)
					if gotValid != c.wantValid {
						t.Errorf("Decode() valid = %v, want %v", gotValid, c.wantValid)
						return
					}

					if gotCode != c.wantCode {
						t.Errorf("Decode() code = %08x, want %08x", gotCode, c.wantCode)
					}
					if gotConsume != c.wantConsume {
						t.Errorf("Decode() consume = %v, want %v", gotConsume, c.wantConsume)
					}
				})
			}
		})
	}
}

func TestDeduplication(t *testing.T) {
	// In the following code space range, one sub-tree can be shared
	// to parse the second byte of all three ranges.
	csr := CodeSpaceRange{
		{Low: []byte{1, 10}, High: []byte{1, 20}},
		{Low: []byte{3, 10}, High: []byte{3, 20}},
		{Low: []byte{5, 10}, High: []byte{5, 20}},
	}
	decoder := NewDecoder(csr)

	var cc []uint16
	for _, node := range decoder.tree {
		cc = append(cc, node.child)
		if node.high == 0xFF {
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

// BenchmarkDecoderSingleByte benchmarks the Decode method for single-byte codes
func BenchmarkDecoderSingleByte(b *testing.B) {
	csr := CodeSpaceRange{
		{Low: []byte{0x00}, High: []byte{0xFF}},
	}
	decoder := NewDecoder(csr)

	input := []byte("ABCDE")

	b.ResetTimer()
	for range b.N {
		j := 0
		for j < len(input) {
			_, consumed, _ := decoder.Decode(input[j:])
			j += consumed
		}
	}
}

// BenchmarkDecoderTwoByte benchmarks the Decode method for two-byte codes
func BenchmarkDecoderTwoByte(b *testing.B) {
	csr := CodeSpaceRange{
		{Low: []byte{0x81, 0x40}, High: []byte{0xFE, 0xFE}},
	}
	decoder := NewDecoder(csr)

	input := []byte{0x81, 0x40, 0x90, 0x80, 0xA0, 0xC0, 0xB0, 0xFF, 0xFE, 0xFE}

	b.ResetTimer()
	for range b.N {
		j := 0
		for j < len(input) {
			_, consumed, _ := decoder.Decode(input[j:])
			j += consumed
		}
	}
}

func BenchmarkDecoderUTF8(b *testing.B) {
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
	decoder := NewDecoder(csr)

	input := []byte("Hello, 世界")

	b.ResetTimer()
	for range b.N {
		j := 0
		for j < len(input) {
			_, consumed, _ := decoder.Decode(input[j:])
			j += consumed
		}
	}
}
