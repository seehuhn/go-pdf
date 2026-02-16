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

package content

import (
	"bytes"
	"io"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

// testResources provides resources for round-trip tests.
// Font F1 is referenced by "text operators" and "mixed content" test cases.
var testResources = &Resources{
	Font: map[pdf.Name]font.Instance{"F1": nil},
}

var roundTripTestCases = []struct {
	name   string
	stream string
}{
	{name: "simple operators", stream: "q\n1 0 0 1 100 200 cm\nQ\n"},
	{name: "path operators", stream: "100 100 m\n200 200 l\nS\n"},
	{name: "text operators", stream: "BT\n/F1 12 Tf\n(Hello) Tj\nET\n"},
	{name: "arrays and dicts", stream: "[1 2 3] 0 d\n<</Type /XObject>> gs\n"},
	{name: "comments", stream: "% this is a comment\nq\nQ\n"},
	{name: "inline image", stream: "BI\n/W 10\n/H 10\nID\nimagedata\nEI\n"},
	{name: "mixed content", stream: "q\n% save state\n1 0 0 1 0 0 cm\nBT\n/F1 12 Tf\n(Text) Tj\nET\nQ\n"},
}

func TestStreamRoundTrip(t *testing.T) {
	for _, tt := range roundTripTestCases {
		t.Run(tt.name, func(t *testing.T) {
			// first read
			stream1, err := ReadStream(bytes.NewReader([]byte(tt.stream)), pdf.V2_0, Page, testResources)
			if err != nil {
				t.Fatalf("first read: %v", err)
			}

			// write using Writer
			var buf bytes.Buffer
			w := NewWriter(pdf.V2_0, Page, testResources)
			if err := w.Write(&buf, stream1); err != nil {
				t.Fatalf("write: %v", err)
			}
			if err := w.Close(); err != nil {
				t.Fatalf("writer close: %v", err)
			}

			// second read
			stream2, err := ReadStream(bytes.NewReader(buf.Bytes()), pdf.V2_0, Page, testResources)
			if err != nil {
				t.Fatalf("second read: %v", err)
			}

			// compare
			if diff := cmp.Diff(stream1, stream2, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("round trip failed (-first +second):\n%s", diff)
			}
		})
	}
}

func FuzzStreamRoundTrip(f *testing.F) {
	for _, tc := range roundTripTestCases {
		f.Add([]byte(tc.stream))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// first read - permissive, may skip malformed content
		stream1, err := ReadStream(bytes.NewReader(data), pdf.V2_0, Page, testResources)
		if err != nil {
			return
		}

		// write - must succeed if read succeeded
		var buf bytes.Buffer
		w := NewWriter(pdf.V2_0, Page, testResources)
		if err := w.Write(&buf, stream1); err != nil {
			// Fuzzed input may reference resources we don't have - skip these
			return
		}
		if err := w.Close(); err != nil {
			t.Fatalf("writer close: %v", err)
		}

		// second read
		stream2, err := ReadStream(bytes.NewReader(buf.Bytes()), pdf.V2_0, Page, testResources)
		if err != nil {
			t.Fatalf("second read failed: %v", err)
		}

		// compare
		if diff := cmp.Diff(stream1, stream2, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("round trip failed (-first +second):\n%s", diff)
		}
	})
}

func TestInlineImageLimits(t *testing.T) {
	tests := []struct {
		name       string
		stream     string
		wantParsed bool // whether we expect the image to be parsed
	}{
		{
			name:       "valid small image",
			stream:     "BI\n/W 10\n/H 10\nID\ndata\nEI\n",
			wantParsed: true,
		},
		{
			name:       "width too large",
			stream:     "BI\n/W 100000\n/H 10\nID\ndata\nEI\n",
			wantParsed: false,
		},
		{
			name:       "height too large",
			stream:     "BI\n/W 10\n/H 100000\nID\ndata\nEI\n",
			wantParsed: false,
		},
		{
			name:       "pixel count too large",
			stream:     "BI\n/W 1000\n/H 1000\nID\ndata\nEI\n", // 1M pixels > 256K limit
			wantParsed: false,
		},
		{
			name:       "pixel count at limit",
			stream:     "BI\n/W 512\n/H 512\nID\ndata\nEI\n", // 262144 pixels < 256K limit
			wantParsed: true,
		},
		{
			name:       "with L key (PDF 2.0)",
			stream:     "BI\n/W 10\n/H 10\n/L 4\nID\ndataEI\n",
			wantParsed: true,
		},
		{
			name:       "L key too large",
			stream:     "BI\n/W 10\n/H 10\n/L 10000\nID\ndata\nEI\n",
			wantParsed: false,
		},
		{
			name:       "ASCII85 filter with extra whitespace",
			stream:     "BI\n/W 10\n/H 10\n/F /A85\nID\n   data~>\nEI\n",
			wantParsed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream, err := ReadStream(bytes.NewReader([]byte(tt.stream)), pdf.V2_0, Page, &Resources{})
			if err != nil {
				t.Fatalf("ReadStream error: %v", err)
			}

			hasImage := false
			for _, op := range stream {
				if op.Name == OpInlineImage {
					hasImage = true
					break
				}
			}

			if hasImage != tt.wantParsed {
				t.Errorf("image parsed = %v, want %v", hasImage, tt.wantParsed)
			}
		})
	}
}

func TestOperatorArgLimit(t *testing.T) {
	// Build a stream with 100 numbers followed by an operator, then a valid operator
	var buf bytes.Buffer
	for range 100 {
		buf.WriteString("1 ")
	}
	buf.WriteString("m\n") // moveto with 100 args (should be skipped)
	buf.WriteString("q\n") // save graphics state (should be parsed)
	buf.WriteString("Q\n") // restore graphics state

	stream, err := ReadStream(bytes.NewReader(buf.Bytes()), pdf.V2_0, Page, &Resources{})
	if err != nil {
		t.Fatalf("ReadStream error: %v", err)
	}

	// the operator with too many args should be skipped
	if len(stream) != 2 {
		t.Errorf("got %d operators, want 2", len(stream))
	}
	if len(stream) > 0 && stream[0].Name != OpPushGraphicsState {
		t.Errorf("got operator %q, want %q", stream[0].Name, OpPushGraphicsState)
	}
}

// collectStream collects all operators from a Stream into an Operators slice.
func collectStream(s Stream) (Operators, error) {
	var ops Operators
	for name, args := range s.All() {
		ops = append(ops, Operator{Name: name, Args: slices.Clone(args)})
	}
	return ops, s.Err()
}

func TestNewScannerMatchesReadStream(t *testing.T) {
	for _, tt := range roundTripTestCases {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(tt.stream)

			// ReadStream result
			want, err := ReadStream(bytes.NewReader(data), pdf.V2_0, Page, testResources)
			if err != nil {
				t.Fatalf("ReadStream: %v", err)
			}

			// NewScanner result
			s := NewScanner(bytes.NewReader(data), pdf.V2_0, Page, testResources)
			got, err := collectStream(s)
			if err != nil {
				t.Fatalf("NewScanner: %v", err)
			}

			if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-ReadStream +NewScanner):\n%s", diff)
			}
		})
	}
}

func FuzzNewScannerMatchesReadStream(f *testing.F) {
	for _, tc := range roundTripTestCases {
		f.Add([]byte(tc.stream))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		want, err1 := ReadStream(bytes.NewReader(data), pdf.V2_0, Page, testResources)

		s := NewScanner(bytes.NewReader(data), pdf.V2_0, Page, testResources)
		got, err2 := collectStream(s)

		// both should either succeed or fail
		if (err1 != nil) != (err2 != nil) {
			t.Fatalf("error mismatch: ReadStream=%v, NewScanner=%v", err1, err2)
		}
		if err1 != nil {
			return
		}

		if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("mismatch (-ReadStream +NewScanner):\n%s", diff)
		}
	})
}

func TestScannerRewind(t *testing.T) {
	input := []byte("q\n1 0 0 1 100 200 cm\nQ\n")
	r := bytes.NewReader(input)
	s := NewScanner(r, pdf.V2_0, Page, &Resources{})

	first, err := collectStream(s)
	if err != nil {
		t.Fatalf("first iteration: %v", err)
	}

	second, err := collectStream(s)
	if err != nil {
		t.Fatalf("second iteration: %v", err)
	}

	if diff := cmp.Diff(first, second, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("rewind mismatch (-first +second):\n%s", diff)
	}
}

func TestParseNumber(t *testing.T) {
	tests := []struct {
		input string
		want  pdf.Native
	}{
		// integers
		{"0", pdf.Integer(0)},
		{"1", pdf.Integer(1)},
		{"42", pdf.Integer(42)},
		{"-1", pdf.Integer(-1)},
		{"+1", pdf.Integer(1)},
		{"-0", pdf.Integer(0)},
		{"+0", pdf.Integer(0)},
		{"123456789", pdf.Integer(123456789)},

		// reals
		{"0.0", pdf.Real(0)},
		{"1.0", pdf.Real(1)},
		{".5", pdf.Real(0.5)},
		{"-.5", pdf.Real(-0.5)},
		{"+.5", pdf.Real(0.5)},
		{"3.14", pdf.Real(3.14)},
		{"-3.14", pdf.Real(-3.14)},
		{"100.", pdf.Real(100)},

		// invalid
		{"", nil},
		{"abc", nil},
		{"+", nil},
		{"-", nil},
		{".", nil},
		{"1.2.3", nil},
		{"12abc", nil},
		{"--1", nil},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseNumber([]byte(tt.input))
			if got != tt.want {
				t.Errorf("parseNumber(%q) = %v (%T), want %v (%T)",
					tt.input, got, got, tt.want, tt.want)
			}
		})
	}
}

// nonSeekReader wraps a reader to hide any Seek method.
type nonSeekReader struct {
	io.Reader
}

func TestStreamsEqual(t *testing.T) {
	data := []byte("q\n1 0 0 1 100 200 cm\nQ\n")

	ops, err := ReadStream(bytes.NewReader(data), pdf.V2_0, Page, &Resources{})
	if err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner(bytes.NewReader(data), pdf.V2_0, Page, &Resources{})

	// Operators vs Scanner
	if !StreamsEqual(ops, scanner) {
		t.Error("expected Operators and Scanner to be equal")
	}

	// Operators vs Operators
	ops2, _ := ReadStream(bytes.NewReader(data), pdf.V2_0, Page, &Resources{})
	if !StreamsEqual(ops, ops2) {
		t.Error("expected two Operators to be equal")
	}

	// Scanner vs Scanner
	s1 := NewScanner(bytes.NewReader(data), pdf.V2_0, Page, &Resources{})
	s2 := NewScanner(bytes.NewReader(data), pdf.V2_0, Page, &Resources{})
	if !StreamsEqual(s1, s2) {
		t.Error("expected two Scanners to be equal")
	}

	// nil cases
	if !StreamsEqual(nil, nil) {
		t.Error("expected nil == nil")
	}
	if StreamsEqual(ops, nil) {
		t.Error("expected non-nil != nil")
	}
	if StreamsEqual(nil, ops) {
		t.Error("expected nil != non-nil")
	}

	// different streams
	other, _ := ReadStream(bytes.NewReader([]byte("q\nQ\n")), pdf.V2_0, Page, &Resources{})
	if StreamsEqual(ops, other) {
		t.Error("expected different streams to be unequal")
	}
}

func TestScannerNoRewind(t *testing.T) {
	input := []byte("q\nQ\n")
	r := nonSeekReader{bytes.NewReader(input)}
	s := NewScanner(r, pdf.V2_0, Page, &Resources{})

	// first call should succeed
	_, err := collectStream(s)
	if err != nil {
		t.Fatalf("first iteration: %v", err)
	}

	// second call on non-seekable should yield empty iterator
	for range s.All() {
		t.Fatal("expected empty iterator on second call")
	}
	if s.Err() == nil {
		t.Error("expected non-nil error after second All() on non-seekable reader")
	}
}
