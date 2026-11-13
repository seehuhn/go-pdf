// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package content

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"seehuhn.de/go/pdf"
)

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
	{name: "cm in text", stream: "BT\n1 0 0 1 50 100 cm\n/F1 12 Tf\n1 0 0 1 100 200 Tm\n(Hi) Tj\nET\n"},
	{name: "cm before text end", stream: "BT\n/F1 12 Tf\n1 0 0 1 100 200 Tm\n(Hi) Tj\n1 0 0 1 50 0 cm\nET\n"},
}

// roundTripTest verifies the content-stream round-trip contract:
//   - C1: write must succeed whenever the read succeeded;
//   - C2: re-reading the writer's output yields the same operator sequence.
//
// The caller is responsible for the first read; this helper performs
// write + re-read + compare.
func roundTripTest(t *testing.T, _ pdf.Version, stream1 []Operator) {
	t.Helper()

	rc, _ := (&Operators{Ops: stream1}).RawBytes()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, rc); err != nil {
		t.Fatalf("write: %v", err)
	}
	rc.Close()

	stream2, err := collectStream(NewScanner(bytesOpener(buf.Bytes())))
	if err != nil {
		t.Fatalf("second read: %v", err)
	}

	if diff := cmp.Diff(stream1, stream2, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("round trip failed (-first +second):\n%s", diff)
	}
}

// TestCmInsideText pins that the scanner preserves a cm operator that
// appears inside a BT/ET text object verbatim (no hoist out, no rewrite,
// no drop).  Application of the matrix is the consumer's responsibility:
// [State.applyOperatorToParams] folds it into the CTM (see also
// [TestCmInsideText_AppliesToCTM] in state_test.go).
func TestCmInsideText(t *testing.T) {
	got, err := collectStream(NewScanner(bytesOpener([]byte(
		"BT\n1 0 0 1 50 100 cm\n/F1 12 Tf\n(Hi) Tj\nET\n",
	))))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := []Operator{
		{Name: OpTextBegin},
		{Name: OpTransform, Args: []pdf.Object{
			pdf.Integer(1), pdf.Integer(0), pdf.Integer(0), pdf.Integer(1),
			pdf.Integer(50), pdf.Integer(100),
		}},
		{Name: OpTextSetFont, Args: []pdf.Object{pdf.Name("F1"), pdf.Integer(12)}},
		{Name: OpTextShow, Args: []pdf.Object{pdf.String("Hi")}},
		{Name: OpTextEnd},
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("cm-in-text (-want +got):\n%s", diff)
	}
}

func TestStreamRoundTrip(t *testing.T) {
	for _, tt := range roundTripTestCases {
		t.Run(tt.name, func(t *testing.T) {
			stream1, err := collectStream(NewScanner(bytesOpener([]byte(tt.stream))))
			if err != nil {
				t.Fatalf("first read: %v", err)
			}
			roundTripTest(t, pdf.V2_0, stream1)
		})
	}
}

func FuzzStreamRoundTrip(f *testing.F) {
	for _, tc := range roundTripTestCases {
		f.Add([]byte(tc.stream))
	}
	f.Add([]byte(strings.Repeat("[", 1000)))
	f.Add([]byte(strings.Repeat("<<", 1000)))

	f.Fuzz(func(t *testing.T, data []byte) {
		stream1, err := collectStream(NewScanner(bytesOpener(data)))
		if err != nil {
			return // permissive read; malformed input is not a contract violation
		}
		roundTripTest(t, pdf.V2_0, stream1)
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
			// the entries are checked by DecodeInlineImage
			name:       "width too large",
			stream:     "BI\n/W 100000\n/H 10\nID\ndata\nEI\n",
			wantParsed: true,
		},
		{
			name:       "height too large",
			stream:     "BI\n/W 10\n/H 100000\nID\ndata\nEI\n",
			wantParsed: true,
		},
		{
			name:       "many pixels",
			stream:     "BI\n/W 1000\n/H 1000\nID\ndata\nEI\n",
			wantParsed: true,
		},
		{
			name:       "dimensions at limit",
			stream:     "BI\n/W 65536\n/H 65536\nID\ndata\nEI\n",
			wantParsed: true,
		},
		{
			name:       "more than 4096 bytes",
			stream:     "BI\n/W 10\n/H 10\nID\n" + strings.Repeat("x", 5000) + "\nEI\n",
			wantParsed: true,
		},
		{
			name:       "more than 4096 bytes with L key",
			stream:     "BI\n/W 10\n/H 10\n/L 5000\nID\n" + strings.Repeat("x", 5000) + "\nEI\n",
			wantParsed: true,
		},
		{
			name:       "with L key (PDF 2.0)",
			stream:     "BI\n/W 10\n/H 10\n/L 4\nID\ndataEI\n",
			wantParsed: true,
		},
		{
			name:       "L key beyond end of stream",
			stream:     "BI\n/W 10\n/H 10\n/L 10000\nID\ndata\nEI\n",
			wantParsed: false,
		},
		{
			name:       "huge L key",
			stream:     "BI\n/W 10\n/H 10\n/L 9999999999\nID\ndata\nEI\n",
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
			stream, err := collectStream(NewScanner(bytesOpener([]byte(tt.stream))))
			if err != nil {
				t.Fatalf("collectStream error: %v", err)
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
		// PDF 7.3.5: when "#" is not followed by two hex digits, treat
		// the "#" as a literal character. Mirrors the same edge cases
		// covered by the pdf and content scanners.
		{
			name:  "name with valid hex escape",
			input: "/A#42",
			want:  pdf.Name("AB"),
		},
		{
			name:  "name with invalid first hex digit",
			input: "/A#Z9",
			want:  pdf.Name("A#Z9"),
		},
		{
			name:  "name with invalid second hex digit",
			input: "/A#9Z",
			want:  pdf.Name("A#9Z"),
		},
		{
			name:  "name with trailing hash",
			input: "/A#",
			want:  pdf.Name("A#"),
		},
		{
			name:  "name with double hash",
			input: "/##41",
			want:  pdf.Name("#A"),
		},
		// PDF 7.3.4.2: an unescaped end-of-line marker inside a literal
		// string is normalised to a single LF, regardless of whether it
		// is CR, LF, or CR-LF.
		{
			name:  "string with bare CR",
			input: "(a\rb)",
			want:  pdf.String("a\nb"),
		},
		{
			name:  "string with CR+LF",
			input: "(a\r\nb)",
			want:  pdf.String("a\nb"),
		},
		{
			name:  "string with two bare CRs",
			input: "(a\r\rb)",
			want:  pdf.String("a\n\nb"),
		},
		{
			name:  "string with CR+LF then LF",
			input: "(a\r\n\nb)",
			want:  pdf.String("a\n\nb"),
		},
		{
			name:  "string with two CR+LFs",
			input: "(a\r\n\r\nb)",
			want:  pdf.String("a\n\nb"),
		},
		{
			name:  "string with backslash + CR continuation",
			input: "(a\\\rb)",
			want:  pdf.String("ab"),
		},
		{
			name:  "string with backslash + CR+LF continuation",
			input: "(a\\\r\nb)",
			want:  pdf.String("ab"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &scanner{
				buf: make([]byte, 512),
				src: bytes.NewReader([]byte(tt.input)),
			}
			got, err := s.readValueDepth(0)
			if err != nil {
				t.Fatalf("readValueDepth error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("readValueDepth mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestScanLoopNestingDepthCap(t *testing.T) {
	// Many unmatched opening brackets must not exhaust memory or CPU:
	// scan() rejects each push past the cap with parseError{}, and
	// scanLoop skips it, so the call must terminate cleanly.
	input := strings.Repeat("[", maxContentNestDepth*4)
	if _, err := collectStream(NewScanner(bytesOpener([]byte(input)))); err != nil {
		t.Fatalf("collectStream error: %v", err)
	}
}

// withSizeBound temporarily replaces a package-level size bound for the
// duration of a test, so size-limit checks can be exercised without
// allocating the production limit.
func withSizeBound(t *testing.T, p *int, val int) {
	t.Helper()
	orig := *p
	*p = val
	t.Cleanup(func() { *p = orig })
}

func TestContentReadCommentBomb(t *testing.T) {
	withSizeBound(t, &maxNameBytes, 100)
	// a comment with no end-of-line marker longer than maxNameBytes
	// must be rejected with parseError, and the next operator after a
	// LF must still be emitted (the bad comment's surplus bytes are
	// drained up to the EOL, where Scan resyncs).
	input := "%" + strings.Repeat("a", maxNameBytes+50) + "\n10 j"
	ops, err := collectStream(NewScanner(bytesOpener([]byte(input))))
	if err != nil {
		t.Fatalf("collectStream: %v", err)
	}
	var names []string
	for _, op := range ops {
		names = append(names, string(op.Name))
	}
	want := []string{"j"}
	if !slices.Equal(names, want) {
		t.Errorf("operator names: got %v, want %v", names, want)
	}
}

func TestContentReadStringBomb(t *testing.T) {
	withSizeBound(t, &maxStringBytes, 100)
	// ReadString is called after the opening '('.
	body := strings.Repeat("a", maxStringBytes+1) + ")"
	s := &scanner{buf: make([]byte, 512), src: strings.NewReader(body)}
	if _, err := s.ReadString(); err == nil {
		t.Fatal("expected error, got nil")
	} else if !errors.Is(err, parseError{}) {
		t.Errorf("expected parseError, got %T: %v", err, err)
	}
}

func TestContentReadHexStringBomb(t *testing.T) {
	withSizeBound(t, &maxStringBytes, 100)
	// ReadHexString is called after the opening '<'.
	body := strings.Repeat("00", maxStringBytes+1) + ">"
	s := &scanner{buf: make([]byte, 512), src: strings.NewReader(body)}
	if _, err := s.ReadHexString(); err == nil {
		t.Fatal("expected error, got nil")
	} else if !errors.Is(err, parseError{}) {
		t.Errorf("expected parseError, got %T: %v", err, err)
	}
}

func TestContentReadNameBomb(t *testing.T) {
	withSizeBound(t, &maxNameBytes, 100)
	// ReadName is called after the leading '/'.  Over-long names return
	// parseError, and the surplus regular bytes must have been drained
	// so the scanner resyncs on the next token.
	body := strings.Repeat("a", maxNameBytes+50) + " next"
	s := &scanner{buf: make([]byte, 512), src: strings.NewReader(body)}
	if _, err := s.ReadName(); !errors.Is(err, parseError{}) {
		t.Fatalf("ReadName: got %v, want parseError", err)
	}
	if err := s.SkipWhiteSpace(); err != nil {
		t.Fatalf("SkipWhiteSpace: %v", err)
	}
	next, err := s.ReadName()
	if err != nil {
		t.Fatalf("ReadName (next): %v", err)
	}
	if string(next) != "next" {
		t.Errorf("token after over-long name = %q, want \"next\"", next)
	}
}

func TestContentReadNameBombStreamRecovery(t *testing.T) {
	withSizeBound(t, &maxNameBytes, 100)
	// An over-long name must be dropped by scanLoop's parseError
	// recovery, but the surplus bytes must have been drained so the
	// scanner resyncs on the following operator — otherwise the cs
	// token here would be glued onto the tail of the bad name and
	// lost.
	input := "/" + strings.Repeat("a", maxNameBytes+50) + " cs\n5 j 10 J"
	ops, err := collectStream(NewScanner(bytesOpener([]byte(input))))
	if err != nil {
		t.Fatalf("collectStream: %v", err)
	}
	var names []string
	for _, op := range ops {
		names = append(names, string(op.Name))
	}
	want := []string{"cs", "j", "J"}
	if !slices.Equal(names, want) {
		t.Errorf("operator names: got %v, want %v", names, want)
	}
}

func TestContentScanTokenOperatorBomb(t *testing.T) {
	withSizeBound(t, &maxNameBytes, 100)
	// A run of regular characters at the start of a token longer than
	// maxNameBytes must be rejected with parseError, with the surplus
	// drained so subsequent operators are still emitted.
	input := strings.Repeat("z", maxNameBytes+50) + " j 10 J"
	ops, err := collectStream(NewScanner(bytesOpener([]byte(input))))
	if err != nil {
		t.Fatalf("collectStream: %v", err)
	}
	var names []string
	for _, op := range ops {
		names = append(names, string(op.Name))
	}
	want := []string{"j", "J"}
	if !slices.Equal(names, want) {
		t.Errorf("operator names: got %v, want %v", names, want)
	}
}

func TestContentScanTokenNumberBomb(t *testing.T) {
	withSizeBound(t, &maxNameBytes, 100)
	// A digit run longer than maxNameBytes must be rejected with
	// parseError, surplus drained, and the trailing operator emitted.
	input := strings.Repeat("9", maxNameBytes+50) + " j"
	ops, err := collectStream(NewScanner(bytesOpener([]byte(input))))
	if err != nil {
		t.Fatalf("collectStream: %v", err)
	}
	var names []string
	for _, op := range ops {
		names = append(names, string(op.Name))
	}
	want := []string{"j"}
	if !slices.Equal(names, want) {
		t.Errorf("operator names: got %v, want %v", names, want)
	}
}

func TestContentScanArrayBomb(t *testing.T) {
	withSizeBound(t, &maxArrayLen, 100)
	body := "[" + strings.Repeat("1 ", maxArrayLen+1) + "]"
	s := &scanner{buf: make([]byte, 512), src: strings.NewReader(body)}
	if _, err := s.Scan(); err == nil {
		t.Fatal("expected error, got nil")
	} else if !errors.Is(err, parseError{}) {
		t.Errorf("expected parseError, got %T: %v", err, err)
	}
}

func TestContentScanDictBomb(t *testing.T) {
	withSizeBound(t, &maxDictLen, 100)
	var b strings.Builder
	b.WriteString("<<")
	for i := 0; i < maxDictLen+1; i++ {
		fmt.Fprintf(&b, "/k%d 1 ", i)
	}
	b.WriteString(">>")
	s := &scanner{buf: make([]byte, 512), src: strings.NewReader(b.String())}
	if _, err := s.Scan(); err == nil {
		t.Fatal("expected error, got nil")
	} else if !errors.Is(err, parseError{}) {
		t.Errorf("expected parseError, got %T: %v", err, err)
	}
}

func TestContentReadValueArrayBomb(t *testing.T) {
	withSizeBound(t, &maxArrayLen, 100)
	body := "[" + strings.Repeat("1 ", maxArrayLen+1) + "]"
	s := &scanner{buf: make([]byte, 512), src: strings.NewReader(body)}
	if _, err := s.readValueDepth(0); err == nil {
		t.Fatal("expected error, got nil")
	} else if !errors.Is(err, parseError{}) {
		t.Errorf("expected parseError, got %T: %v", err, err)
	}
}

func TestContentReadValueDictBomb(t *testing.T) {
	withSizeBound(t, &maxDictLen, 100)
	var b strings.Builder
	b.WriteString("<<")
	for i := 0; i < maxDictLen+1; i++ {
		fmt.Fprintf(&b, "/k%d 1 ", i)
	}
	b.WriteString(">>")
	s := &scanner{buf: make([]byte, 512), src: strings.NewReader(b.String())}
	if _, err := s.readValueDepth(0); err == nil {
		t.Fatal("expected error, got nil")
	} else if !errors.Is(err, parseError{}) {
		t.Errorf("expected parseError, got %T: %v", err, err)
	}
}

func TestParseErrorResetsCompositeStack(t *testing.T) {
	// A parseError that escapes Scan() while a <<...>> or [...] frame is
	// open must reset the composite stack, otherwise subsequent tokens are
	// silently appended to the orphan frame instead of being treated as
	// operator arguments.
	//
	// Here <Z> is malformed (Z is not a hex digit), so ReadHexString
	// returns parseError partway through the open <<. Without the reset,
	// the still-open dict frame swallows "5 j 10 J" entirely and no
	// operators are emitted.
	input := "<<<Z 5 j 10 J"
	ops, err := collectStream(NewScanner(bytesOpener([]byte(input))))
	if err != nil {
		t.Fatalf("collectStream error: %v", err)
	}

	var names []string
	for _, op := range ops {
		names = append(names, string(op.Name))
	}
	want := []string{"j", "J"}
	if !slices.Equal(names, want) {
		t.Errorf("operator names: got %v, want %v", names, want)
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
	s := &scanner{
		buf: make([]byte, 512),
		src: bytes.NewReader(buf.Bytes()),
	}
	_, err := s.readValueDepth(0)
	if err == nil {
		t.Error("expected error for deeply nested value")
	}
}

func TestInlineImageWithArrayValue(t *testing.T) {
	// inline image with /D [1 0] — this previously failed because
	// readInlineImage used nextToken which couldn't parse arrays
	stream, err := collectStream(NewScanner(
		bytesOpener([]byte("BI\n/W 10\n/H 10\n/D [1 0]\nID\nimagedata\nEI\n")),
	))
	if err != nil {
		t.Fatalf("collectStream error: %v", err)
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
	stream, err := collectStream(NewScanner(
		bytesOpener([]byte("BI\n/W 10\n/H 10\n/DP <</K -1>>\nID\nimagedata\nEI\n")),
	))
	if err != nil {
		t.Fatalf("collectStream error: %v", err)
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

	stream, err := collectStream(NewScanner(bytesOpener(buf.Bytes())))
	if err != nil {
		t.Fatalf("collectStream error: %v", err)
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
func collectStream(s Stream) ([]Operator, error) {
	it := s.NewIter()
	var ops []Operator
	for name, args := range it.All() {
		ops = append(ops, Operator{Name: name, Args: slices.Clone(args)})
	}
	return ops, it.Err()
}

func TestScannerRewind(t *testing.T) {
	input := []byte("q\n1 0 0 1 100 200 cm\nQ\n")
	s := NewScanner(bytesOpener(input))

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

// TestOperatorsRawBytesStreaming pins the per-byte streaming behaviour
// of [Operators.RawBytes]: each [io.Reader.Read] call returns at most
// the requested byte count, but the final concatenated output must be
// identical to a single-shot read.  This catches regressions that
// re-introduce buffer-the-whole-stream behaviour.
func TestOperatorsRawBytesStreaming(t *testing.T) {
	m := &Operators{Ops: []Operator{
		{Name: OpPushGraphicsState},
		{Name: OpTransform, Args: []pdf.Object{
			pdf.Integer(1), pdf.Integer(0), pdf.Integer(0), pdf.Integer(1),
			pdf.Integer(100), pdf.Integer(200),
		}},
		{Name: OpTextBegin},
		{Name: OpTextSetFont, Args: []pdf.Object{pdf.Name("F1"), pdf.Integer(12)}},
		{Name: OpTextShow, Args: []pdf.Object{pdf.String("hello world")}},
		{Name: OpTextEnd},
		{Name: OpPopGraphicsState},
	}}

	// reference: read everything in one Read call.
	refRC, _ := m.RawBytes()
	var refBuf bytes.Buffer
	if _, err := io.Copy(&refBuf, refRC); err != nil {
		t.Fatalf("reference io.Copy: %v", err)
	}
	refRC.Close()

	// per-byte: force a fresh Read for every byte.  A materialising
	// implementation passes too, but any regression that forgets to
	// honour len(p)=1 will fail.
	rc, _ := m.RawBytes()
	var oneByteBuf bytes.Buffer
	one := make([]byte, 1)
	for {
		n, err := rc.Read(one)
		if n > 0 {
			oneByteBuf.WriteByte(one[0])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("per-byte Read: %v", err)
		}
	}
	rc.Close()

	if !bytes.Equal(refBuf.Bytes(), oneByteBuf.Bytes()) {
		t.Errorf("per-byte streaming mismatch:\nfull:    %q\nperbyte: %q",
			refBuf.String(), oneByteBuf.String())
	}

	// independent readers: a second RawBytes call must yield the same
	// bytes (no shared state with the first reader).
	rc2, _ := m.RawBytes()
	var secondBuf bytes.Buffer
	if _, err := io.Copy(&secondBuf, rc2); err != nil {
		t.Fatalf("second io.Copy: %v", err)
	}
	rc2.Close()
	if !bytes.Equal(refBuf.Bytes(), secondBuf.Bytes()) {
		t.Errorf("second reader mismatch:\nfirst:  %q\nsecond: %q",
			refBuf.String(), secondBuf.String())
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

	ops, err := collectStream(NewScanner(bytesOpener(data)))
	if err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner(bytesOpener(data))

	// Memory(Operators) vs Scanner
	if !StreamsEqual(&Operators{Ops: ops}, scanner) {
		t.Error("expected Operators and Scanner to be equal")
	}

	// Memory vs Memory
	ops2, _ := collectStream(NewScanner(bytesOpener(data)))
	if !StreamsEqual(&Operators{Ops: ops}, &Operators{Ops: ops2}) {
		t.Error("expected two Operators to be equal")
	}

	// Scanner vs Scanner
	s1 := NewScanner(bytesOpener(data))
	s2 := NewScanner(bytesOpener(data))
	if !StreamsEqual(s1, s2) {
		t.Error("expected two Scanners to be equal")
	}

	// nil cases
	if !StreamsEqual(nil, nil) {
		t.Error("expected nil == nil")
	}
	if StreamsEqual(&Operators{Ops: ops}, nil) {
		t.Error("expected non-nil != nil")
	}
	if StreamsEqual(nil, &Operators{Ops: ops}) {
		t.Error("expected nil != non-nil")
	}

	// different streams
	other, _ := collectStream(NewScanner(bytesOpener([]byte("q\nQ\n"))))
	if StreamsEqual(&Operators{Ops: ops}, &Operators{Ops: other}) {
		t.Error("expected different streams to be unequal")
	}
}

// TestScannerNoCloserSynthesis confirms the scanner yields the operator
// sequence verbatim and does not synthesise closers for unbalanced
// contexts.  Closer synthesis is the consumer's job — see
// [State.ClosingOperators] and [seehuhn.de/go/pdf/reader.Reader.ProcessIter].
func TestScannerNoCloserSynthesis(t *testing.T) {
	input := []byte("q\n1 0 0 1 0 0 cm\n")
	ops, err := collectStream(NewScanner(bytesOpener(input)))
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range ops {
		if op.Name == OpPopGraphicsState {
			t.Errorf("scanner should not synthesise Q; got %v", op)
		}
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
		})

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
		})

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
		})

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

	// A regular token whose final byte abuts a sticky malformed error (no
	// trailing delimiter) must still be emitted: its bytes are fully read,
	// so dropping it would lose data that RawBytes preserves, breaking the
	// read-write-read round trip.
	t.Run("malformed abutting trailing token", func(t *testing.T) {
		s := NewScanner(func() (io.ReadCloser, error) {
			return &errReader{
				data: []byte("q\nQ"),
				err:  pdf.Error("corrupt filter body"),
			}, nil
		})

		ops, err := collectStream(s)
		if err != nil {
			t.Errorf("expected nil error (permissive), got %v", err)
		}
		if len(ops) != 2 {
			t.Errorf("expected 2 operators, got %d: %v", len(ops), ops)
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
		})

		it := s.NewIter()
		for range it.All() {
		}
		if err := it.Err(); err != diskErr {
			t.Errorf("expected %v, got %v", diskErr, err)
		}
	})
}

// TestInlineImageWhiteSpaceBeforeEI checks that any white-space byte before
// EI ends the image data, not only a line break (§8.9.7).
func TestInlineImageWhiteSpaceBeforeEI(t *testing.T) {
	for _, sep := range []string{"\n", "\r", " ", "\t", "\f", "\x00"} {
		raw := "BI /W 1 /H 1 /CS /G /BPC 8 ID \x80" + sep + "EI\n2 w\n"
		stream, err := collectStream(NewScanner(bytesOpener([]byte(raw))))
		if err != nil {
			t.Fatalf("%q: collectStream error: %v", sep, err)
		}
		if len(stream) != 2 || stream[0].Name != OpInlineImage || stream[1].Name != OpSetLineWidth {
			t.Errorf("%q: got %v, want inline image followed by w", sep, stream)
			continue
		}
		if diff := cmp.Diff(pdf.String("\x80"), stream[0].Args[1]); diff != "" {
			t.Errorf("%q: wrong image data (-want +got):\n%s", sep, diff)
		}
	}
}

// TestInlineImageInvalidEntries checks that the data of an inline image with
// invalid entries is read as image data, not parsed as operators.
func TestInlineImageInvalidEntries(t *testing.T) {
	for _, tc := range []struct {
		entries string
		valid   bool
	}{
		{"/H 1 /CS /G /BPC 8", false},
		{"/W 0 /H 1 /CS /G /BPC 8", false},
		{"/W 0.4 /H 1 /CS /G /BPC 8", false},
		{"/W 100000 /H 1 /CS /G /BPC 8", false},
		{"/W 1 /H 1 /CS /G /BPC 8 /L 0", true},
	} {
		entries := tc.entries
		raw := "BI " + entries + " ID 0 0 9 9 re f\nEI\n2 w\n"
		stream, err := collectStream(NewScanner(bytesOpener([]byte(raw))))
		if err != nil {
			t.Fatalf("%q: collectStream error: %v", entries, err)
		}
		if len(stream) != 2 || stream[0].Name != OpInlineImage || stream[1].Name != OpSetLineWidth {
			t.Errorf("%q: got %v, want inline image followed by w", entries, stream)
			continue
		}
		if diff := cmp.Diff(pdf.String("0 0 9 9 re f"), stream[0].Args[1]); diff != "" {
			t.Errorf("%q: wrong image data (-want +got):\n%s", entries, diff)
		}
		if _, err := DecodeInlineImage(stream[0], nil); (err == nil) != tc.valid {
			t.Errorf("%q: DecodeInlineImage error %v, want valid=%v", entries, err, tc.valid)
		}
	}
}

// TestInlineImageMalformedDict checks that an inline image with a malformed
// dictionary is dropped, and that its data is not parsed as operators.
func TestInlineImageMalformedDict(t *testing.T) {
	for _, dict := range []string{
		"/W 1 /H 1 5 /CS /G",  // key which is not a name
		"/W ID",               // ID as a value
		"/W 1 /DP << /K ID",   // ID inside a nested dictionary
		"/W 1 /D [0 ID",       // ID inside an array
		"/W 1 /DP << 5 6 >> ", // malformed nested dictionary
	} {
		raw := "BI " + dict + " ID 0 0 9 9 re f\nEI\n2 w\n"
		got, err := collectStream(NewScanner(bytesOpener([]byte(raw))))
		if err != nil {
			t.Fatalf("%q: collectStream error: %v", dict, err)
		}
		want := []Operator{{Name: OpSetLineWidth, Args: []pdf.Object{pdf.Integer(2)}}}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("%q: wrong operators (-want +got):\n%s", dict, diff)
		}
	}

	// EI before ID ends an image without data, and scanning resumes after it
	next := Operator{Name: OpInlineImage, Args: []pdf.Object{
		pdf.Dict{"W": pdf.Integer(1), "H": pdf.Integer(1), "CS": pdf.Name("G"), "BPC": pdf.Integer(8), "L": pdf.Integer(1)},
		pdf.String("x"),
	}}
	for _, dict := range []string{
		"/W 1 5",         // after a malformed entry
		"/W 1 /H 1",      // as a key
		"/W 1 /H",        // as a value
		"/W 1 /D [0",     // inside an array
		"/W 1 /DP << /K", // inside a nested dictionary
	} {
		raw := "BI " + dict + " EI\n2 w\nBI /W 1 /H 1 /CS /G /BPC 8 ID x\nEI\n3 w\n"
		got, err := collectStream(NewScanner(bytesOpener([]byte(raw))))
		if err != nil {
			t.Fatalf("%q: collectStream error: %v", dict, err)
		}
		want := []Operator{
			{Name: OpSetLineWidth, Args: []pdf.Object{pdf.Integer(2)}},
			next,
			{Name: OpSetLineWidth, Args: []pdf.Object{pdf.Integer(3)}},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("%q: wrong operators (-want +got):\n%s", dict, diff)
		}
	}
}

// TestInlineImageWrongLength checks inline images whose Length entry
// disagrees with the data.
func TestInlineImageWrongLength(t *testing.T) {
	setW := Operator{Name: OpSetLineWidth, Args: []pdf.Object{pdf.Integer(3)}}

	t.Run("too short", func(t *testing.T) {
		raw := "BI /W 1 /H 1 /CS /G /BPC 8 /L 3 ID 0 0 9 9 re f\nEI\n3 w\n"
		got, err := collectStream(NewScanner(bytesOpener([]byte(raw))))
		if err != nil {
			t.Fatalf("collectStream error: %v", err)
		}
		want := []Operator{
			{Name: OpInlineImage, Args: []pdf.Object{
				pdf.Dict{
					"W": pdf.Integer(1), "H": pdf.Integer(1), "CS": pdf.Name("G"),
					"BPC": pdf.Integer(8), "L": pdf.Integer(12),
				},
				pdf.String("0 0 9 9 re f"),
			}},
			setW,
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("wrong operators (-want +got):\n%s", diff)
		}
		roundTripTest(t, pdf.V2_0, got)
	})

	// The data ends before the Length says, so the bytes up to the end of
	// the Length are lost.  Scanning resumes after them.
	t.Run("too long", func(t *testing.T) {
		raw := "BI /W 1 /H 1 /CS /G /BPC 8 /L 9 ID abcd\nEI\n2 w\n3 w\n"
		got, err := collectStream(NewScanner(bytesOpener([]byte(raw))))
		if err != nil {
			t.Fatalf("collectStream error: %v", err)
		}
		for _, op := range got {
			if op.Name == OpInlineImage {
				t.Errorf("unexpected inline image %v", op)
			}
		}
		if len(got) == 0 || !cmp.Equal(got[len(got)-1], setW) {
			t.Errorf("got %v, want 3 w at the end", got)
		}
	})

	// The Length ends inside the EI marker, so nothing after the marker has
	// been read and the image is kept.
	t.Run("too long, EI straddles the end", func(t *testing.T) {
		raw := "BI /W 1 /H 1 /CS /G /BPC 8 /L 6 ID abcd\nEI\n2 w\n3 w\n"
		got, err := collectStream(NewScanner(bytesOpener([]byte(raw))))
		if err != nil {
			t.Fatalf("collectStream error: %v", err)
		}
		want := []Operator{
			{Name: OpInlineImage, Args: []pdf.Object{
				pdf.Dict{
					"W": pdf.Integer(1), "H": pdf.Integer(1), "CS": pdf.Name("G"),
					"BPC": pdf.Integer(8), "L": pdf.Integer(4),
				},
				pdf.String("abcd"),
			}},
			{Name: OpSetLineWidth, Args: []pdf.Object{pdf.Integer(2)}},
			setW,
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("wrong operators (-want +got):\n%s", diff)
		}
		roundTripTest(t, pdf.V2_0, got)
	})

	// The Length ends just after the EI marker, so nothing after the marker
	// has been read and the image is kept.
	gray := pdf.Dict{"W": pdf.Integer(1), "H": pdf.Integer(1), "CS": pdf.Name("G"), "BPC": pdf.Integer(8)}
	for _, tc := range []struct {
		name, raw string
		want      []Operator
	}{
		{"too long by the EI marker",
			"BI /W 1 /H 1 /CS /G /BPC 8 /L 7 ID abcd\nEI\n3 w\n",
			[]Operator{{Name: OpInlineImage, Args: []pdf.Object{
				withEntry(gray, "L", pdf.Integer(4)), pdf.String("abcd"),
			}}, setW}},
		{"too long by the EI marker, at the end of the stream",
			"BI /W 1 /H 1 /CS /G /BPC 8 /L 7 ID abcd\nEI",
			[]Operator{{Name: OpInlineImage, Args: []pdf.Object{
				withEntry(gray, "L", pdf.Integer(4)), pdf.String("abcd"),
			}}}},
		{"too long by the EI marker, empty data",
			"BI /W 1 /H 1 /CS /G /BPC 8 /L 2 ID EI\n3 w\n",
			[]Operator{{Name: OpInlineImage, Args: []pdf.Object{
				withEntry(gray, "L", pdf.Integer(0)), pdf.String(""),
			}}, setW}},
		{"EI at the end of data, not a marker",
			"BI /W 1 /H 1 /CS /G /BPC 8 /L 7 ID abcd\nEIx\nEI\n3 w\n",
			[]Operator{{Name: OpInlineImage, Args: []pdf.Object{
				withEntry(gray, "L", pdf.Integer(8)), pdf.String("abcd\nEIx"),
			}}, setW}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := collectStream(NewScanner(bytesOpener([]byte(tc.raw))))
			if err != nil {
				t.Fatalf("collectStream error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("wrong operators (-want +got):\n%s", diff)
			}
			roundTripTest(t, pdf.V2_0, got)
		})
	}
}

// withEntry returns a copy of dict with key set to val.
func withEntry(dict pdf.Dict, key pdf.Name, val pdf.Object) pdf.Dict {
	res := maps.Clone(dict)
	res[key] = val
	return res
}

// TestInlineImageRealLength checks that a Length entry given as a real
// number is ignored, and replaced by the integer length of the data.
func TestInlineImageRealLength(t *testing.T) {
	for _, l := range []string{"4.0", "4.5"} {
		raw := "BI /W 1 /H 1 /CS /G /BPC 8 /L " + l + " ID abcd\nEI\n3 w\n"
		got, err := collectStream(NewScanner(bytesOpener([]byte(raw))))
		if err != nil {
			t.Fatalf("/L %s: collectStream error: %v", l, err)
		}
		want := []Operator{
			{Name: OpInlineImage, Args: []pdf.Object{
				pdf.Dict{
					"W": pdf.Integer(1), "H": pdf.Integer(1), "CS": pdf.Name("G"),
					"BPC": pdf.Integer(8), "L": pdf.Integer(4),
				},
				pdf.String("abcd"),
			}},
			{Name: OpSetLineWidth, Args: []pdf.Object{pdf.Integer(3)}},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("/L %s: wrong operators (-want +got):\n%s", l, diff)
		}
	}
}

// TestInlineImageDataLimit checks that inline image data longer than
// maxInlineImageBytes is dropped, and that scanning resumes after it.
func TestInlineImageDataLimit(t *testing.T) {
	withSizeBound(t, &maxInlineImageBytes, 16)
	atLimit := strings.Repeat("x", 16)
	beyond := strings.Repeat("x", 17)

	for _, tc := range []struct {
		name, raw string
		wantImage bool
	}{
		{"at limit", "BI /W 1 ID " + atLimit + "\nEI\n3 w\n", true},
		{"beyond limit", "BI /W 1 ID " + beyond + "\nEI\n3 w\n", false},
		{"Length at limit", "BI /W 1 /L 16 ID " + atLimit + "\nEI\n3 w\n", true},
		{"Length beyond limit", "BI /W 1 /L 17 ID " + beyond + "\nEI\n3 w\n", false},
		{"Length beyond limit and too small",
			"BI /W 1 /L 17 ID " + beyond + "yy\nEI\n3 w\n", false},
		{"Length too small, data beyond limit",
			"BI /W 1 /L 4 ID " + beyond + "\nEI\n3 w\n", false},
		{"white space after data", "BI /W 1 /L 4 ID abcd" +
			strings.Repeat(" ", 40) + "EI\n3 w\n", true},
		{"white space beyond limit", "BI /W 1 /L 4 ID abcd" +
			strings.Repeat(" ", 40) + "x\nEI\n3 w\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := collectStream(NewScanner(bytesOpener([]byte(tc.raw))))
			if err != nil {
				t.Fatalf("collectStream error: %v", err)
			}
			setW := Operator{Name: OpSetLineWidth, Args: []pdf.Object{pdf.Integer(3)}}
			if len(got) == 0 || !cmp.Equal(got[len(got)-1], setW) {
				t.Fatalf("got %v, want 3 w at the end", got)
			}
			hasImage := len(got) == 2 && got[0].Name == OpInlineImage
			if hasImage != tc.wantImage || len(got) > 2 {
				t.Errorf("got %v, want image %t", got, tc.wantImage)
			}
		})
	}
}

// TestReadNameNullEscape checks that "#00" is kept literally, since a name
// cannot contain a null byte.
func TestReadNameNullEscape(t *testing.T) {
	got, err := collectStream(NewScanner(bytesOpener([]byte("/A#00B 12 Tf\n"))))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := []Operator{
		{Name: OpTextSetFont, Args: []pdf.Object{pdf.Name("A#00B"), pdf.Integer(12)}},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// TestReadNameBraces checks that { and } are regular characters in content
// streams, so that they do not split names or form operators.
func TestReadNameBraces(t *testing.T) {
	got, err := collectStream(NewScanner(bytesOpener([]byte("/F{1} 12 Tf\n"))))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := []Operator{
		{Name: OpTextSetFont, Args: []pdf.Object{pdf.Name("F{1}"), pdf.Integer(12)}},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("read failed (-want +got):\n%s", diff)
	}
}

func TestInlineImageLengthEntry(t *testing.T) {
	for _, tc := range []struct {
		dict pdf.Dict
		want int
	}{
		{pdf.Dict{"L": pdf.Integer(7)}, 7},
		{pdf.Dict{"Length": pdf.Integer(7)}, 7},
		{pdf.Dict{"L": pdf.Integer(7), "Length": pdf.Integer(9)}, 7},
		{pdf.Dict{"L": pdf.Integer(0)}, 0},
		{pdf.Dict{"L": pdf.Integer(-3)}, -1},
		{pdf.Dict{"L": pdf.Integer(1) << 40}, math.MaxInt32},
		{pdf.Dict{"L": pdf.Real(7)}, -1},
		{pdf.Dict{"L": pdf.Name("7")}, -1},
		{pdf.Dict{}, -1},
		{nil, -1},
	} {
		if got := inlineImageLength(tc.dict); got != tc.want {
			t.Errorf("%v: got %d, want %d", tc.dict, got, tc.want)
		}
	}
}

// TestInlineImageEmptyData checks that an inline image without data ends at
// the EI marker directly after ID, so that the operators after it are kept.
func TestInlineImageEmptyData(t *testing.T) {
	setW := Operator{Name: OpSetLineWidth, Args: []pdf.Object{pdf.Integer(3)}}
	for _, tc := range []struct {
		name, header string
		want         pdf.Dict
	}{
		{"no Length", "/W 1 /H 1 /CS /G /BPC 8", pdf.Dict{
			"W": pdf.Integer(1), "H": pdf.Integer(1), "CS": pdf.Name("G"),
			"BPC": pdf.Integer(8), "L": pdf.Integer(0),
		}},
		{"zero Length", "/W 1 /H 1 /CS /G /BPC 8 /L 0", pdf.Dict{
			"W": pdf.Integer(1), "H": pdf.Integer(1), "CS": pdf.Name("G"),
			"BPC": pdf.Integer(8), "L": pdf.Integer(0),
		}},
		{"ASCII filter", "/W 1 /H 1 /CS /G /BPC 8 /F /AHx", pdf.Dict{
			"W": pdf.Integer(1), "H": pdf.Integer(1), "CS": pdf.Name("G"),
			"BPC": pdf.Integer(8), "F": pdf.Name("AHx"), "L": pdf.Integer(0),
		}},
		{"L and Length", "/W 1 /H 1 /CS /G /BPC 8 /L 0 /Length 7", pdf.Dict{
			"W": pdf.Integer(1), "H": pdf.Integer(1), "CS": pdf.Name("G"),
			"BPC": pdf.Integer(8), "L": pdf.Integer(0),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := "BI " + tc.header + " ID EI 3 w\n"
			got, err := collectStream(NewScanner(bytesOpener([]byte(raw))))
			if err != nil {
				t.Fatalf("collectStream error: %v", err)
			}
			want := []Operator{
				{Name: OpInlineImage, Args: []pdf.Object{tc.want, pdf.String("")}},
				setW,
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("wrong operators (-want +got):\n%s", diff)
			}
		})
	}
}

func TestContainsInlineImageEnd(t *testing.T) {
	for _, tc := range []struct {
		data string
		want bool
	}{
		{"", false},
		{"E", false},
		{"EI", true},
		{"EI ", true},
		{"EIb", false},
		{"a EI", true},
		{"a EI b", true},
		{"a\nEI/", true},
		{"a EIb", false},
		{"aEI b", false},
		{"a E I", false},
	} {
		if got := ContainsInlineImageEnd([]byte(tc.data)); got != tc.want {
			t.Errorf("%q: got %t, want %t", tc.data, got, tc.want)
		}
	}
}

// TestInlineImageLeadingWhiteSpace checks that white space at the start of
// inline image data is kept as data, so that a Length entry which counts it
// ends the data at the EI marker.
func TestInlineImageLeadingWhiteSpace(t *testing.T) {
	setW := Operator{Name: OpSetLineWidth, Args: []pdf.Object{pdf.Integer(3)}}
	for _, tc := range []struct {
		name, raw string
		want      pdf.Dict
		data      string
	}{
		{"exact Length",
			"BI /W 1 /H 1 /CS /G /BPC 8 /F /AHx /L 5 ID \n\n41>\nEI 3 w\n",
			pdf.Dict{
				"W": pdf.Integer(1), "H": pdf.Integer(1), "CS": pdf.Name("G"),
				"BPC": pdf.Integer(8), "F": pdf.Name("AHx"), "L": pdf.Integer(5),
			},
			"\n\n41>"},
		{"Length without the white space",
			"BI /W 1 /H 1 /CS /G /BPC 8 /F /AHx /L 3 ID\r\n41>\nEI 3 w\n",
			pdf.Dict{
				"W": pdf.Integer(1), "H": pdf.Integer(1), "CS": pdf.Name("G"),
				"BPC": pdf.Integer(8), "F": pdf.Name("AHx"), "L": pdf.Integer(4),
			},
			"\n41>"},
		{"no Length",
			"BI /W 1 /H 1 /CS /G /BPC 8 /F /AHx ID \n\n41>\nEI 3 w\n",
			pdf.Dict{
				"W": pdf.Integer(1), "H": pdf.Integer(1), "CS": pdf.Name("G"),
				"BPC": pdf.Integer(8), "F": pdf.Name("AHx"), "L": pdf.Integer(5),
			},
			"\n\n41>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := collectStream(NewScanner(bytesOpener([]byte(tc.raw))))
			if err != nil {
				t.Fatalf("collectStream error: %v", err)
			}
			want := []Operator{
				{Name: OpInlineImage, Args: []pdf.Object{tc.want, pdf.String(tc.data)}},
				setW,
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("wrong operators (-want +got):\n%s", diff)
			}
			decoded, err := DecodeInlineImage(got[0], nil)
			if err != nil {
				t.Fatalf("DecodeInlineImage error: %v", err)
			}
			if diff := cmp.Diff([]byte{0x41}, decoded); diff != "" {
				t.Errorf("wrong decoded data (-want +got):\n%s", diff)
			}
			roundTripTest(t, pdf.V2_0, got)
		})
	}
}

// TestFormatInlineImageLength checks that an inline image is written with a
// Length entry, so that data which contains an EI marker reads back
// unchanged, and that a Length entry which disagrees with the data is
// rejected.
func TestFormatInlineImageLength(t *testing.T) {
	data := pdf.String("ab\nEI cd")
	dict := pdf.Dict{"W": pdf.Integer(8), "H": pdf.Integer(1), "CS": pdf.Name("G"), "BPC": pdf.Integer(8)}
	op := Operator{Name: OpInlineImage, Args: []pdf.Object{dict, data}}

	buf := &bytes.Buffer{}
	if err := op.Format(buf, 0); err != nil {
		t.Fatal(err)
	}
	if _, ok := dict["L"]; ok {
		t.Error("Format modified the dictionary of the operator")
	}
	got, err := collectStream(NewScanner(bytesOpener(buf.Bytes())))
	if err != nil {
		t.Fatal(err)
	}
	wantDict := maps.Clone(dict)
	wantDict["L"] = pdf.Integer(len(data))
	want := []Operator{{Name: OpInlineImage, Args: []pdf.Object{wantDict, data}}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}

	for _, key := range []pdf.Name{"L", "Length"} {
		for _, val := range []pdf.Object{pdf.Integer(3), pdf.Real(8), pdf.Name("x")} {
			bad := maps.Clone(dict)
			bad[key] = val
			op := Operator{Name: OpInlineImage, Args: []pdf.Object{bad, data}}
			if err := op.Format(io.Discard, 0); err == nil {
				t.Errorf("%s %v: no error for wrong Length", key, val)
			}
		}
	}
}
