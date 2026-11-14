// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package charcode

import (
	"bytes"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
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

	{
		name: "different cuts",
		ranges: []Range{
			{Low: []byte{0x00, 0x00, 0x00}, High: []byte{0xFF, 0xFF, 0x00}},
			{Low: []byte{0x00, 0x00, 0x01}, High: []byte{0x7F, 0xFF, 0x01}},
			{Low: []byte{0x00, 0x00, 0x02}, High: []byte{0xFF, 0x7F, 0x02}},
		},
		cases: []testCase{},
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

	var cc []int
	for i, bound := range c.bound {
		cc = append(cc, c.child[i])
		if bound == 0xFF {
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
			if !csr1.Equivalent(csr2) {
				t.Errorf("CodeSpaceRanges differ for % x", csr1.getDifference(csr2))
				for _, r := range csr1 {
					t.Logf("before: % x - % x", r.Low, r.High)
				}
				for _, r := range csr2 {
					t.Logf("after: % x - % x", r.Low, r.High)
				}
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

// refDecode is a slow reference implementation of [Codec.Decode], which
// matches the input directly against the list of ranges.
//
// A code is valid if it matches one of the ranges.  Otherwise, the
// input is consumed up to the length of the shortest range which matches
// the longest possible prefix of the input, as far as the input allows.
func refDecode(csr CodeSpaceRange, s []byte) (Code, int, bool) {
	pack := func(n int) Code {
		var c Code
		for i := range n {
			c |= Code(s[i]) << (8 * i)
		}
		return c
	}

	cur := csr // ranges which match the first d bytes of s
	for d := 0; ; d++ {
		if d >= len(s) {
			return pack(d), d, false
		}
		var next CodeSpaceRange
		for _, r := range cur {
			if len(r.Low) > d && r.Low[d] <= s[d] && s[d] <= r.High[d] {
				if len(r.Low) == d+1 {
					return pack(d + 1), d + 1, true
				}
				next = append(next, r)
			}
		}
		if len(next) == 0 {
			n := d + 1
			if len(cur) > 0 {
				n = len(cur[0].Low)
				for _, r := range cur[1:] {
					n = min(n, len(r.Low))
				}
			}
			n = min(n, len(s))
			return pack(n), n, false
		}
		cur = next
	}
}

// testInputs returns inputs which, together with their prefixes, exercise
// every distinct behaviour of a codec for csr.  Since sub-trees can be
// shared between different byte positions, the byte values are taken from
// the range bounds at all positions.
func testInputs(csr CodeSpaceRange, rng *rand.Rand, n int) [][]byte {
	seen := map[int]bool{0: true, 255: true}
	for _, r := range csr {
		for i := range r.Low {
			seen[int(r.Low[i])] = true
			seen[int(r.High[i])] = true
			if r.Low[i] > 0 {
				seen[int(r.Low[i])-1] = true
			}
			if r.High[i] < 255 {
				seen[int(r.High[i])+1] = true
			}
		}
	}
	var vals []byte
	for v := range seen {
		vals = append(vals, byte(v))
	}
	slices.Sort(vals)

	var res [][]byte
	if len(vals) <= 12 {
		// all combinations
		for _, a := range vals {
			for _, b := range vals {
				for _, c := range vals {
					for _, d := range vals {
						res = append(res, []byte{a, b, c, d, 0x55})
					}
				}
			}
		}
		return res
	}
	for range n {
		s := make([]byte, 5)
		for i := range s {
			s[i] = vals[rng.IntN(len(vals))]
		}
		res = append(res, s)
	}
	return res
}

// checkDecode compares the codec c for csr with [refDecode].  The input is
// decoded code by code.
func checkDecode(t *testing.T, csr CodeSpaceRange, c *Codec, input []byte) {
	var buf [4]byte
	for len(input) > 0 {
		wantCode, wantN, wantValid := refDecode(csr, input)
		code, n, valid := c.Decode(input)
		if code != wantCode || n != wantN || valid != wantValid {
			t.Fatalf("ranges %v, input % x: got (%x, %d, %t), want (%x, %d, %t)",
				csr, input, code, n, valid, wantCode, wantN, wantValid)
		}
		if n < len(input) { // otherwise the code may be truncated
			if out := c.AppendCode(buf[:0], code); !bytes.Equal(out, input[:n]) {
				t.Fatalf("ranges %v: AppendCode(%x) = % x, want % x",
					csr, code, out, input[:n])
			}
		}
		input = input[wantN:]
	}
}

// checkCodeSpaceRange checks that the codec returns a sorted code space range
// equivalent to csr.
func checkCodeSpaceRange(t *testing.T, csr CodeSpaceRange, c *Codec) {
	t.Helper()
	got := c.CodeSpaceRange()
	if !slices.IsSortedFunc(got, compareRange) {
		t.Fatalf("ranges %v: CodeSpaceRange() %v is not sorted", csr, got)
	}
	if len(csr) <= 64 && !csr.Equivalent(got) {
		t.Fatalf("ranges %v: CodeSpaceRange() gives %v", csr, got)
	}
}

// checkAgainstReference compares the codec for csr with [refDecode].
func checkAgainstReference(t *testing.T, csr CodeSpaceRange, rng *rand.Rand) {
	t.Helper()
	c, err := NewCodec(csr)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range testInputs(csr, rng, 2000) {
		for l := range len(s) + 1 {
			checkDecode(t, csr, c, s[:l])
		}
	}
	checkCodeSpaceRange(t, csr, c)
}

// prefixConflict reports whether some code in one range is a proper prefix
// of a code in another range.
func prefixConflict(csr CodeSpaceRange) bool {
	for _, r := range csr {
		for _, s := range csr {
			if len(r.Low) >= len(s.Low) {
				continue
			}
			overlap := true
			for i := range r.Low {
				if r.High[i] < s.Low[i] || s.High[i] < r.Low[i] {
					overlap = false
					break
				}
			}
			if overlap {
				return true
			}
		}
	}
	return false
}

func randomRanges(rng *rand.Rand) CodeSpaceRange {
	special := []byte{0x00, 0x01, 0x7F, 0x80, 0xFE, 0xFF}
	pick := func() byte {
		if rng.IntN(2) == 0 {
			return special[rng.IntN(len(special))]
		}
		return byte(rng.IntN(256))
	}
	maxLen := 1 + rng.IntN(4)
	csr := make(CodeSpaceRange, 1+rng.IntN(6))
	for k := range csr {
		l := 1 + rng.IntN(maxLen)
		lo, hi := make([]byte, l), make([]byte, l)
		for i := range l {
			a, b := pick(), pick()
			lo[i], hi[i] = min(a, b), max(a, b)
		}
		csr[k] = Range{Low: lo, High: hi}
	}
	return csr
}

func TestCodecReferenceRandom(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	numValid := 0
	for numValid < 2000 {
		csr := randomRanges(rng)
		_, err := NewCodec(csr)
		if (err != nil) != prefixConflict(csr) {
			t.Fatalf("ranges %v: unexpected error status %v", csr, err)
		}
		if err != nil {
			continue
		}
		checkAgainstReference(t, csr, rng)
		numValid++
	}
}

func TestCodecReferenceStructured(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	structured := []CodeSpaceRange{
		Simple, UCS2, UTF8,
		// sub-trees at different depths, which differ only in their
		// invalid entries
		{
			{Low: []byte{0xFE, 0x80}, High: []byte{0xFF, 0xFE}},
			{Low: []byte{0x01, 0x40, 0x44}, High: []byte{0x02, 0xB8, 0xFE}},
		},
		{
			{Low: []byte{0x01, 0xAE}, High: []byte{0x33, 0xFF}},
			{Low: []byte{0x86, 0x7F}, High: []byte{0xE7, 0xFF}},
			{Low: []byte{0x7F, 0xAD}, High: []byte{0xFE, 0xFF}},
		},
		families(4, 3),
		families(3, 5),
		manyGroups()[:64],
	}
	for _, tc := range testCases {
		structured = append(structured, tc.ranges)
	}
	for _, csr := range structured {
		checkAgainstReference(t, csr, rng)
	}
}

// families returns code space ranges of length nBytes, where for every byte
// position d there are k ranges which cover all codes with an even value 2i
// at position d.
func families(nBytes, k int) CodeSpaceRange {
	var csr CodeSpaceRange
	for d := range nBytes {
		for i := range k {
			lo := bytes.Repeat([]byte{0x00}, nBytes)
			hi := bytes.Repeat([]byte{0xFF}, nBytes)
			lo[d], hi[d] = byte(2*i), byte(2*i)
			csr = append(csr, Range{Low: lo, High: hi})
		}
	}
	return csr
}

// TestCodecFamiliesScaling checks that the construction work for the
// families example grows polynomially.  Without sharing of sub-trees during
// construction, the work grows exponentially in the code length: each
// doubling of k multiplied the run time by more than 10.
func TestCodecFamiliesScaling(t *testing.T) {
	steps := func(k int) int {
		b := newBuilder(families(4, k), math.MaxInt)
		if _, err := b.build(); err != nil {
			t.Fatal(err)
		}
		return b.steps
	}
	s1 := steps(32)
	s2 := steps(64)
	ratio := float64(s2) / float64(s1)
	t.Logf("%d steps for k=32, %d steps for k=64", s1, s2)
	// The work is quadratic in the number of ranges.
	if ratio > 5 {
		t.Errorf("work grows too fast: %d steps for k=32, %d steps for k=64", s1, s2)
	}
}

// manyGroups returns a code space range whose lookup tree has more than
// 65535 distinct nodes.
func manyGroups() CodeSpaceRange {
	var csr CodeSpaceRange
	for b := range 256 {
		long := map[int]bool{2 * (b % 128): true}
		if b >= 128 {
			long[254] = true
		}
		for x := 0; x < 256; x += 2 {
			if long[x] {
				csr = append(csr, Range{Low: []byte{byte(b), byte(x), 0}, High: []byte{byte(b), byte(x), byte(b)}})
			} else {
				csr = append(csr, Range{Low: []byte{byte(b), byte(x)}, High: []byte{byte(b), byte(x)}})
			}
		}
	}
	return csr
}

func TestCodecManyNodes(t *testing.T) {
	csr := manyGroups()
	c, err := NewCodec(csr)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.bound) <= 0xFFFF {
		t.Fatalf("test case too small: only %d nodes", len(c.bound))
	}

	checks := []struct {
		in        []byte
		wantCode  Code
		wantN     int
		wantValid bool
	}{
		{[]byte{0xC8, 0x90, 0x05}, 0x0590C8, 3, true},  // long code
		{[]byte{0xC8, 0x90, 0xC9}, 0xC990C8, 3, false}, // long code, last byte out of range
		{[]byte{0xC8, 0x92, 0x05}, 0x92C8, 2, true},    // short code
		{[]byte{0xC8, 0x93, 0x05}, 0x93C8, 2, false},   // odd second byte
		{[]byte{0x05, 0x0A, 0x05}, 0x050A05, 3, true},  // long code
		{[]byte{0xFF, 0xFE, 0xFF}, 0xFFFEFF, 3, true},  // long code
		{[]byte{0x80, 0x00, 0x80}, 0x800080, 3, true},  // long code
		{[]byte{0x80, 0xFE, 0x05}, 0x05FE80, 3, true},  // second long code for b ≥ 128
		{[]byte{0x00, 0x02, 0x00}, 0x0200, 2, true},    // short code
	}
	for _, tc := range checks {
		code, n, valid := c.Decode(tc.in)
		if code != tc.wantCode || n != tc.wantN || valid != tc.wantValid {
			t.Errorf("% x: got (%x, %d, %t), want (%x, %d, %t)",
				tc.in, code, n, valid, tc.wantCode, tc.wantN, tc.wantValid)
		}
	}

	rng := rand.New(rand.NewPCG(5, 6))
	for range 10000 {
		in := []byte{byte(rng.IntN(256)), byte(rng.IntN(256)), byte(rng.IntN(256)), 0x55}
		wantCode, wantN, wantValid := refDecode(csr, in)
		code, n, valid := c.Decode(in)
		if code != wantCode || n != wantN || valid != wantValid {
			t.Fatalf("% x: got (%x, %d, %t), want (%x, %d, %t)", in, code, n, valid, wantCode, wantN, wantValid)
		}
	}
}

// slabs returns code space ranges of length 4 whose lookup tree grows like
// a high power of the number of ranges: for every byte position d < 3
// there are k ranges which fix the byte at position d to 2i and the last
// byte to 3i+d.
func slabs(k int) CodeSpaceRange {
	var csr CodeSpaceRange
	for d := range 3 {
		for i := range k {
			lo := []byte{0x00, 0x00, 0x00, byte(3*i + d)}
			hi := []byte{0xFF, 0xFF, 0xFF, byte(3*i + d)}
			lo[d], hi[d] = byte(2*i), byte(2*i)
			csr = append(csr, Range{Low: lo, High: hi})
		}
	}
	return csr
}

// TestCodecSlabsRejected checks that a code space range whose lookup tree
// grows faster than the number of ranges is rejected, after a number of
// steps which grows linearly with the number of ranges.
func TestCodecSlabsRejected(t *testing.T) {
	// without a limit, the step count grows like the cube of the size
	prev := 0
	for _, k := range []int{4, 8, 16} {
		b := newBuilder(normalize(slabs(k)), math.MaxInt)
		if _, err := b.build(); err != nil {
			t.Fatal(err)
		}
		if prev > 0 && b.steps < 4*prev {
			t.Errorf("%d ranges: %d steps, %d for half as many", 3*k, b.steps, prev)
		}
		prev = b.steps
	}

	for _, k := range []int{32, 64} {
		csr := normalize(slabs(k))
		if _, err := NewCodec(csr); err != ErrTooComplex {
			t.Fatalf("%d ranges: got error %v, want ErrTooComplex", len(csr), err)
		}

		// Memory use is proportional to the steps taken, see builder.
		budget := treeBudget(len(csr))
		b := newBuilder(csr, budget)
		if _, err := b.build(); err != ErrTooComplex {
			t.Fatalf("%d ranges: got error %v, want ErrTooComplex", len(csr), err)
		}
		if b.steps > 2*budget {
			t.Errorf("%d ranges: %d steps taken, budget %d", len(csr), b.steps, budget)
		}
	}
}

// TestCodecBudget checks that the construction of the lookup tree is
// abandoned once the budget is exhausted, and that the data stored until
// then is bounded by the budget.
func TestCodecBudget(t *testing.T) {
	csr := normalize(slabs(64))
	budget := treeBudget(len(csr))
	b := newBuilder(csr, budget)
	if _, err := b.build(); err != ErrTooComplex {
		t.Fatalf("got error %v, want ErrTooComplex", err)
	}

	// Every node and every key is charged before it is created, so the
	// stored data cannot exceed the budget.
	numEntries := 0
	for _, g := range b.groups {
		numEntries += len(g)
	}
	keyBytes := 0
	for k := range b.bySuffix {
		keyBytes += len(k)
	}
	if numEntries > budget || keyBytes > 4*budget {
		t.Errorf("%d nodes and %d key bytes stored, budget %d", numEntries, keyBytes, budget)
	}
	if numEntries == 0 {
		t.Errorf("no nodes constructed before the budget was exhausted")
	}

	// The construction succeeds with exactly the budget it needs, and
	// fails with one step less.
	valid := normalize(families(4, 8))
	full := newBuilder(valid, math.MaxInt)
	if _, err := full.build(); err != nil {
		t.Fatal(err)
	}
	if _, err := newBuilder(valid, full.steps).build(); err != nil {
		t.Errorf("budget %d: %v", full.steps, err)
	}
	if _, err := newBuilder(valid, full.steps-1).build(); err != ErrTooComplex {
		t.Errorf("budget %d: got error %v, want ErrTooComplex", full.steps-1, err)
	}

	// The predefined code space ranges are far below the budget.
	for _, csr := range []CodeSpaceRange{Simple, UCS2, UTF8} {
		b := newBuilder(normalize(csr), math.MaxInt)
		if _, err := b.build(); err != nil {
			t.Fatal(err)
		}
		if b.steps > treeBudget(len(csr))/8 {
			t.Errorf("ranges %v: %d steps", csr, b.steps)
		}
	}
}

func TestCodeSpaceRangeNormalized(t *testing.T) {
	csr := CodeSpaceRange{
		{Low: []byte{0x81, 0x40}, High: []byte{0x9F, 0xFC}},
		{Low: []byte{0x00}, High: []byte{0x80}},
		{Low: []byte{0x81, 0x40}, High: []byte{0x9F, 0xFC}},
		{Low: []byte{0xA0}, High: []byte{0xDF}},
	}
	want := CodeSpaceRange{csr[1], csr[3], csr[0]}
	c, err := NewCodec(csr)
	if err != nil {
		t.Fatal(err)
	}
	got := c.CodeSpaceRange()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("CodeSpaceRange() differs (-want +got):\n%s", diff)
	}
	got[0].Low[0] = 0x55
	if c.CodeSpaceRange()[0].Low[0] != 0x00 {
		t.Errorf("CodeSpaceRange() shares storage")
	}
}

// FuzzCodec checks the codec for arbitrary code space ranges against the
// reference decoder.
func FuzzCodec(f *testing.F) {
	for _, tc := range testCases {
		f.Add(encodeRanges(tc.ranges), []byte{0x00, 0x80, 0xFF, 0x01})
	}
	f.Add(encodeRanges(families(3, 2)), []byte{0x02, 0x03, 0x00})

	f.Fuzz(func(t *testing.T, data []byte, input []byte) {
		csr := decodeRanges(data)
		c, err := NewCodec(csr)
		if err == ErrTooComplex {
			return
		}
		if (err != nil) != prefixConflict(csr) {
			t.Fatalf("ranges %v: unexpected error status %v", csr, err)
		}
		if err != nil {
			return
		}
		checkDecode(t, csr, c, input)
		checkCodeSpaceRange(t, csr, c)
	})
}

// encodeRanges and decodeRanges convert between code space ranges and the
// byte strings used by FuzzCodec: every range is a length byte followed by
// the bytes of Low and High.
func encodeRanges(csr CodeSpaceRange) []byte {
	var res []byte
	for _, r := range csr {
		res = append(res, byte(len(r.Low)))
		res = append(res, r.Low...)
		res = append(res, r.High...)
	}
	return res
}

func decodeRanges(data []byte) CodeSpaceRange {
	var csr CodeSpaceRange
	for len(data) > 0 && len(csr) < 16 {
		l := int(data[0]%4) + 1
		data = data[1:]
		if len(data) < 2*l {
			break
		}
		lo, hi := data[:l], data[l:2*l]
		data = data[2*l:]
		r := Range{Low: make([]byte, l), High: make([]byte, l)}
		for i := range l {
			r.Low[i], r.High[i] = min(lo[i], hi[i]), max(lo[i], hi[i])
		}
		csr = append(csr, r)
	}
	return csr
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
	c, err := NewCodec(Simple)
	if err != nil {
		b.Fatal(err)
	}

	input := []byte("ABCDE")

	for b.Loop() {
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

	for b.Loop() {
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

	for b.Loop() {
		j := 0
		for j < len(input) {
			_, consumed, _ := c.Decode(input[j:])
			j += consumed
		}
	}
}

func TestCodecEqual(t *testing.T) {
	a := must(NewCodec(CodeSpaceRange{
		{Low: []byte{0x00}, High: []byte{0x7F}},
		{Low: []byte{0x80, 0x00}, High: []byte{0xFF, 0xFF}},
	}))
	b := must(NewCodec(CodeSpaceRange{
		{Low: []byte{0x80, 0x00}, High: []byte{0xFF, 0xFF}},
		{Low: []byte{0x00}, High: []byte{0x7F}},
		{Low: []byte{0x00}, High: []byte{0x7F}},
	}))
	c := must(NewCodec(CodeSpaceRange{
		{Low: []byte{0x00}, High: []byte{0x7F}},
		{Low: []byte{0x80, 0x00}, High: []byte{0xBF, 0xFF}},
		{Low: []byte{0xC0, 0x00}, High: []byte{0xFF, 0xFF}},
	}))
	if !a.Equal(b) || !b.Equal(a) {
		t.Error("codecs from reordered ranges differ")
	}
	if a.Equal(c) || c.Equal(a) {
		t.Error("codecs from differently split ranges are equal")
	}
	if a.Equal(SimpleCodec) {
		t.Error("different codecs are equal")
	}
}

// gridRanges returns a code space range of two-byte codes, made of a ranges
// <00 i>-<FF i> for i < a (a <= 128) and b ranges <j 80>-<j FF> for j < b
// (b <= 256).  The ranges are pairwise disjoint.
func gridRanges(a, b int) CodeSpaceRange {
	var csr CodeSpaceRange
	for i := range a {
		csr = append(csr, Range{Low: []byte{0x00, byte(i)}, High: []byte{0xFF, byte(i)}})
	}
	for j := range b {
		csr = append(csr, Range{Low: []byte{byte(j), 0x80}, High: []byte{byte(j), 0xFF}})
	}
	return csr
}

func TestCodecGrid(t *testing.T) {
	for _, size := range [][2]int{{32, 64}, {32, 256}, {128, 256}} {
		a, b := size[0], size[1]
		csr := gridRanges(a, b)
		c, err := NewCodec(csr)
		if err != nil {
			t.Errorf("%dx%d: %v", a, b, err)
			continue
		}
		bld := newBuilder(normalize(csr), math.MaxInt)
		if _, err := bld.build(); err != nil {
			t.Fatal(err)
		}
		t.Logf("%d ranges: %d steps, budget %d", len(csr), bld.steps, treeBudget(len(csr)))

		cases := []struct {
			in    []byte
			valid bool
		}{
			{[]byte{0x00, 0x00}, true},
			{[]byte{0xFF, byte(a - 1)}, true},
			{[]byte{byte(b - 1), 0x90}, true},
			{[]byte{byte(b - 1), 0xFF}, true},
			{[]byte{0x00, 0x80}, true},
			{[]byte{0xFF, 0x80}, b == 256},
			{[]byte{0xFF, 0x7F}, a == 128},
			{[]byte{0xFF, byte(a)}, a == 128},
		}
		for _, tc := range cases {
			_, n, valid := c.Decode(tc.in)
			if n != 2 || valid != tc.valid {
				t.Errorf("%dx%d: Decode(% x) = %d, %v; want 2, %v", a, b, tc.in, n, valid, tc.valid)
			}
		}
	}
}
