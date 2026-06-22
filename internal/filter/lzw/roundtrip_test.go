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

package lzw

import (
	"bytes"
	"io"
	"math/rand"
	"os"
	"testing"
)

// lzwSourceText returns this package's own source, repeated past the
// dictionary-reset threshold, as a sample of realistic compressible text with
// many distinct substrings.
func lzwSourceText(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile("reader.go")
	if err != nil {
		t.Fatalf("read source input: %v", err)
	}
	return bytes.Repeat(body, 1+100_000/len(body))
}

// lzwTestInputs returns a set of plaintext inputs that exercise the codec:
// the empty and single-byte edge cases, the spec example, long runs (which
// drive the code==hi special expansion), an input cycling through all 256
// byte values, realistic source text, and a large pseudo-random buffer (which
// fills the dictionary to the maximum 12-bit width and forces the output
// buffer to flush).
func lzwTestInputs(t *testing.T) []struct {
	name string
	in   []byte
} {
	t.Helper()

	byteCycle := make([]byte, 256*64)
	for i := range byteCycle {
		byteCycle[i] = byte(i)
	}

	random := make([]byte, 200_000)
	rand.New(rand.NewSource(1)).Read(random)

	return []struct {
		name string
		in   []byte
	}{
		{"empty", nil},
		{"single", []byte{0x42}},
		{"spec-example", []byte{45, 45, 45, 45, 45, 65, 45, 45, 45, 66}},
		{"long-run", bytes.Repeat([]byte{0xAB}, 5000)},
		{"byte-cycle", byteCycle},
		{"source-text", lzwSourceText(t)},
		{"pseudo-random", random},
	}
}

// lzwRoundTrip encodes in and decodes the result, both with the given
// earlyChange setting, and checks that the decoded bytes match in.
func lzwRoundTrip(t *testing.T, in []byte, earlyChange bool) {
	t.Helper()

	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, earlyChange)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if _, err := w.Write(in); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("writer Close: %v", err)
	}

	r := NewReader(buf, earlyChange)
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("reader Close: %v", err)
	}

	if !bytes.Equal(in, out) {
		t.Errorf("round trip mismatch: %d bytes in, %d bytes out", len(in), len(out))
	}
}

func TestRoundTrip(t *testing.T) {
	// matched setting: encoding then decoding with the same earlyChange
	// reproduces the input.
	for _, tc := range lzwTestInputs(t) {
		for _, earlyChange := range []bool{false, true} {
			name := tc.name
			if earlyChange {
				name += "-earlychange"
			}
			t.Run(name, func(t *testing.T) {
				lzwRoundTrip(t, tc.in, earlyChange)
			})
		}
	}

	// The earlyChange setting is part of the wire format: a reader must match
	// the writer.  Decoding a substantial input with the opposite setting
	// desynchronises the code width, so it must not reproduce the input (it
	// may also fail outright).
	t.Run("earlychange-mismatch", func(t *testing.T) {
		in := lzwSourceText(t)
		for _, ecw := range []bool{false, true} {
			ecr := !ecw

			buf := &bytes.Buffer{}
			w, err := NewWriter(buf, ecw)
			if err != nil {
				t.Fatalf("NewWriter: %v", err)
			}
			if _, err := w.Write(in); err != nil {
				t.Fatalf("Write: %v", err)
			}
			if err := w.Close(); err != nil {
				t.Fatalf("writer Close: %v", err)
			}

			out, err := io.ReadAll(NewReader(buf, ecr))
			if err == nil && bytes.Equal(in, out) {
				t.Errorf("writer earlyChange=%v reader earlyChange=%v: round trip unexpectedly succeeded", ecw, ecr)
			}
		}
	})
}

// FuzzRoundTrip checks two properties on mutated inputs:
//
//   - Decoding arbitrary bytes must not panic or hang.  Malformed compressed
//     data is untrusted input, so the decoder must fail gracefully (an error
//     is fine) rather than crash or loop.
//
//   - Encoding then decoding any input reproduces it exactly.
//
// The seeds are deliberately small so the fuzzer can explore many mutations
// per second; the large inputs that fill the dictionary live in TestRoundTrip.
func FuzzRoundTrip(f *testing.F) {
	seeds := [][]byte{
		nil,
		{0x42},
		{45, 45, 45, 45, 45, 65, 45, 45, 45, 66}, // spec example
		bytes.Repeat([]byte{0xAB}, 64),           // run -> code==hi case
		{0x80, 0x0B, 0x60, 0x50, 0x22, 0x0C, 0x0C, 0x85, 0x01}, // a valid stream
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		for _, earlyChange := range []bool{false, true} {
			// malformed-input safety: treat the fuzz input as a compressed
			// stream.  A hostile stream can decode to much more than its own
			// size, so cap how much output we materialise; the point is only
			// that the decoder must not panic or loop, not how much it yields.
			r := NewReader(bytes.NewReader(data), earlyChange)
			_, _ = io.Copy(io.Discard, io.LimitReader(r, 1<<20))
			_ = r.Close()

			// round trip: encoding then decoding the input reproduces it.
			buf := &bytes.Buffer{}
			w, err := NewWriter(buf, earlyChange)
			if err != nil {
				t.Fatalf("NewWriter: %v", err)
			}
			if _, err := w.Write(data); err != nil {
				t.Fatalf("Write: %v", err)
			}
			if err := w.Close(); err != nil {
				t.Fatalf("writer Close: %v", err)
			}

			out, err := io.ReadAll(NewReader(buf, earlyChange))
			if err != nil {
				t.Fatalf("decode of encoded data failed: %v", err)
			}
			if !bytes.Equal(data, out) {
				t.Errorf("round trip mismatch: %d bytes in, %d bytes out", len(data), len(out))
			}
		}
	})
}
