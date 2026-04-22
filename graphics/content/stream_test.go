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
	"errors"
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

// bytesOpener returns a reader factory that opens the given data as a stream.
func bytesOpener(data []byte) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
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
			stream1, err := ReadStream(bytesOpener([]byte(tt.stream)), pdf.V2_0, Page, testResources)
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
			stream2, err := ReadStream(bytesOpener(buf.Bytes()), pdf.V2_0, Page, testResources)
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
		stream1, err := ReadStream(bytesOpener(data), pdf.V2_0, Page, testResources)
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
		stream2, err := ReadStream(bytesOpener(buf.Bytes()), pdf.V2_0, Page, testResources)
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
			stream, err := ReadStream(bytesOpener([]byte(tt.stream)), pdf.V2_0, Page, &Resources{})
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

func TestReadValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  pdf.Object
	}{
		{
			name:  "integer",
			input: "42",
			want:  pdf.Integer(42),
		},
		{
			name:  "name",
			input: "/DeviceRGB",
			want:  pdf.Name("DeviceRGB"),
		},
		{
			name:  "simple array",
			input: "[1 0]",
			want:  pdf.Array{pdf.Integer(1), pdf.Integer(0)},
		},
		{
			name:  "nested array",
			input: "[[1 2] [3 4]]",
			want: pdf.Array{
				pdf.Array{pdf.Integer(1), pdf.Integer(2)},
				pdf.Array{pdf.Integer(3), pdf.Integer(4)},
			},
		},
		{
			name:  "simple dict",
			input: "<</Type /XObject>>",
			want:  pdf.Dict{"Type": pdf.Name("XObject")},
		},
		{
			name:  "dict with array value",
			input: "<</D [1 0]>>",
			want:  pdf.Dict{"D": pdf.Array{pdf.Integer(1), pdf.Integer(0)}},
		},
		{
			name:  "array with names",
			input: "[/DeviceRGB]",
			want:  pdf.Array{pdf.Name("DeviceRGB")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &streamScanner{
				buf: make([]byte, 512),
				src: bytes.NewReader([]byte(tt.input)),
			}
			got, err := s.readValue()
			if err != nil {
				t.Fatalf("readValue() error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("readValue() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestReadValueDepthLimit(t *testing.T) {
	// nesting deeper than maxValueDepth should fail
	var buf bytes.Buffer
	for range maxValueDepth + 1 {
		buf.WriteByte('[')
	}
	buf.WriteString("1")
	for range maxValueDepth + 1 {
		buf.WriteByte(']')
	}
	s := &streamScanner{
		buf: make([]byte, 512),
		src: bytes.NewReader(buf.Bytes()),
	}
	_, err := s.readValue()
	if err == nil {
		t.Error("expected error for deeply nested value")
	}
}

func TestInlineImageWithArrayValue(t *testing.T) {
	// inline image with /D [1 0] — this previously failed because
	// readInlineImage used nextToken which couldn't parse arrays
	stream, err := ReadStream(
		bytesOpener([]byte("BI\n/W 10\n/H 10\n/D [1 0]\nID\nimagedata\nEI\n")),
		pdf.V2_0, Page, &Resources{},
	)
	if err != nil {
		t.Fatalf("ReadStream error: %v", err)
	}

	hasImage := false
	for _, op := range stream {
		if op.Name == OpInlineImage {
			hasImage = true
			dict, ok := op.Args[0].(pdf.Dict)
			if !ok {
				t.Fatal("expected dict as first arg")
			}
			arr, ok := dict["D"].(pdf.Array)
			if !ok {
				t.Fatalf("expected array for /D, got %T", dict["D"])
			}
			if len(arr) != 2 {
				t.Errorf("expected 2-element array, got %d", len(arr))
			}
		}
	}
	if !hasImage {
		t.Error("inline image was not parsed")
	}
}

func TestInlineImageWithDictValue(t *testing.T) {
	// inline image with a dict value in the header
	stream, err := ReadStream(
		bytesOpener([]byte("BI\n/W 10\n/H 10\n/DP <</K -1>>\nID\nimagedata\nEI\n")),
		pdf.V2_0, Page, &Resources{},
	)
	if err != nil {
		t.Fatalf("ReadStream error: %v", err)
	}

	hasImage := false
	for _, op := range stream {
		if op.Name == OpInlineImage {
			hasImage = true
			dict, ok := op.Args[0].(pdf.Dict)
			if !ok {
				t.Fatal("expected dict as first arg")
			}
			dp, ok := dict["DP"].(pdf.Dict)
			if !ok {
				t.Fatalf("expected dict for /DP, got %T", dict["DP"])
			}
			if k, ok := dp["K"].(pdf.Integer); !ok || k != -1 {
				t.Errorf("expected /K -1, got %v", dp["K"])
			}
		}
	}
	if !hasImage {
		t.Error("inline image was not parsed")
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

	stream, err := ReadStream(bytesOpener(buf.Bytes()), pdf.V2_0, Page, &Resources{})
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

// appendClosingOps tracks state on ops and appends any closing operators.
func appendClosingOps(ops Operators, ct Type, res *Resources) Operators {
	state := NewState(ct, res)
	for _, op := range ops {
		state.ApplyStateChanges(op.Name, op.Args)
	}
	for _, name := range state.ClosingOperators() {
		ops = append(ops, Operator{Name: name})
	}
	return ops
}

// collectStream collects all operators from a Stream into an Operators slice.
func collectStream(s Stream) (Operators, error) {
	it := s.NewIter()
	var ops Operators
	for name, args := range it.All() {
		ops = append(ops, Operator{Name: name, Args: slices.Clone(args)})
	}
	return ops, it.Err()
}

func TestNewScannerMatchesReadStream(t *testing.T) {
	for _, tt := range roundTripTestCases {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(tt.stream)

			// ReadStream result (includes closing ops)
			want, err := ReadStream(bytesOpener(data), pdf.V2_0, Page, testResources)
			if err != nil {
				t.Fatalf("ReadStream: %v", err)
			}

			// NewScanner result (no closing ops); append them manually
			s := NewScanner(bytesOpener(data), pdf.V2_0, Page, testResources)
			got, err := collectStream(s)
			if err != nil {
				t.Fatalf("NewScanner: %v", err)
			}
			got = appendClosingOps(got, Page, testResources)

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
		want, err1 := ReadStream(bytesOpener(data), pdf.V2_0, Page, testResources)

		s := NewScanner(bytesOpener(data), pdf.V2_0, Page, testResources)
		got, err2 := collectStream(s)

		// both should either succeed or fail
		if (err1 != nil) != (err2 != nil) {
			t.Fatalf("error mismatch: ReadStream=%v, NewScanner=%v", err1, err2)
		}
		if err1 != nil {
			return
		}

		got = appendClosingOps(got, Page, testResources)

		if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("mismatch (-ReadStream +NewScanner):\n%s", diff)
		}
	})
}

func TestScannerRewind(t *testing.T) {
	input := []byte("q\n1 0 0 1 100 200 cm\nQ\n")
	s := NewScanner(bytesOpener(input), pdf.V2_0, Page, &Resources{})

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

func TestStreamsEqual(t *testing.T) {
	data := []byte("q\n1 0 0 1 100 200 cm\nQ\n")

	ops, err := ReadStream(bytesOpener(data), pdf.V2_0, Page, &Resources{})
	if err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner(bytesOpener(data), pdf.V2_0, Page, &Resources{})

	// Operators vs Scanner
	if !StreamsEqual(ops, scanner) {
		t.Error("expected Operators and Scanner to be equal")
	}

	// Operators vs Operators
	ops2, _ := ReadStream(bytesOpener(data), pdf.V2_0, Page, &Resources{})
	if !StreamsEqual(ops, ops2) {
		t.Error("expected two Operators to be equal")
	}

	// Scanner vs Scanner
	s1 := NewScanner(bytesOpener(data), pdf.V2_0, Page, &Resources{})
	s2 := NewScanner(bytesOpener(data), pdf.V2_0, Page, &Resources{})
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
	other, _ := ReadStream(bytesOpener([]byte("q\nQ\n")), pdf.V2_0, Page, &Resources{})
	if StreamsEqual(ops, other) {
		t.Error("expected different streams to be unequal")
	}
}

func TestReadStreamUnbalancedQ(t *testing.T) {
	// a single unbalanced q should produce exactly one closing Q
	input := []byte("q\n1 0 0 1 0 0 cm\n")
	ops, err := ReadStream(bytesOpener(input), pdf.V2_0, Page, &Resources{})
	if err != nil {
		t.Fatal(err)
	}
	var qCount int
	for _, op := range ops {
		if op.Name == OpPopGraphicsState {
			qCount++
		}
	}
	if qCount != 1 {
		t.Errorf("expected 1 closing Q, got %d", qCount)
	}
}

func TestScannerOpenError(t *testing.T) {
	// Malformed-PDF errors surface as open failures too (for example an
	// unknown /Filter detected by pdf.DecodeStream).  Per the permissive-
	// reader policy the scanner yields an empty iteration and reports no
	// error.
	t.Run("malformed", func(t *testing.T) {
		s := NewScanner(func() (io.ReadCloser, error) {
			return nil, pdf.Error("malformed open failure")
		}, pdf.V2_0, Page, &Resources{})

		it := s.NewIter()
		for range it.All() {
			t.Fatal("expected empty iterator when open fails")
		}
		if err := it.Err(); err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	// A real IO error at open time means the underlying byte source or
	// the library itself failed (e.g. disk, context cancellation).  It
	// must reach the caller unchanged.
	t.Run("real error", func(t *testing.T) {
		diskErr := errors.New("disk read failed")
		s := NewScanner(func() (io.ReadCloser, error) {
			return nil, diskErr
		}, pdf.V2_0, Page, &Resources{})

		it := s.NewIter()
		for range it.All() {
			t.Fatal("expected empty iterator when open fails")
		}
		if err := it.Err(); err != diskErr {
			t.Errorf("expected %v, got %v", diskErr, err)
		}
	})
}

// errReader is an io.ReadCloser that first yields some bytes and then
// returns the supplied error on every subsequent Read.  Used to exercise
// the scanner's handling of read-time errors.
type errReader struct {
	data []byte
	pos  int
	err  error
}

func (r *errReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *errReader) Close() error { return nil }

func TestScannerReadError(t *testing.T) {
	// A malformed-PDF error raised during Read (for example, a corrupt
	// flate body) is sticky: the reader keeps returning it.  The scanner
	// must stop gracefully, yielding any operators already parsed and
	// reporting no error.
	t.Run("malformed mid-stream", func(t *testing.T) {
		s := NewScanner(func() (io.ReadCloser, error) {
			return &errReader{
				data: []byte("q\n1 0 0 1 0 0 cm\n"),
				err:  pdf.Error("corrupt filter body"),
			}, nil
		}, pdf.V2_0, Page, &Resources{})

		it := s.NewIter()
		ops := 0
		for range it.All() {
			ops++
		}
		if ops == 0 {
			t.Errorf("expected at least one operator before the error")
		}
		if err := it.Err(); err != nil {
			t.Errorf("expected nil error (permissive), got %v", err)
		}
	})

	// A real read error mid-stream must reach the caller.
	t.Run("real mid-stream error", func(t *testing.T) {
		diskErr := errors.New("disk read failed")
		s := NewScanner(func() (io.ReadCloser, error) {
			return &errReader{
				data: []byte("q\n1 0 0 1 0 0 cm\n"),
				err:  diskErr,
			}, nil
		}, pdf.V2_0, Page, &Resources{})

		it := s.NewIter()
		for range it.All() {
		}
		if err := it.Err(); err != diskErr {
			t.Errorf("expected %v, got %v", diskErr, err)
		}
	})
}
