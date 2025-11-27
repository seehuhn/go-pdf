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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"seehuhn.de/go/pdf"
)

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
	{name: "mixed content", stream: "q\n% save state\n1 0 0 1 0 0 cm\n/F1 12 Tf\n(Text) Tj\nQ\n"},
}

func TestStreamRoundTrip(t *testing.T) {
	for _, tt := range roundTripTestCases {
		t.Run(tt.name, func(t *testing.T) {
			// first read
			stream1, err := ReadStream(bytes.NewReader([]byte(tt.stream)))
			if err != nil {
				t.Fatalf("first read: %v", err)
			}

			// write
			var buf bytes.Buffer
			if err := stream1.Write(&buf); err != nil {
				t.Fatalf("write: %v", err)
			}

			// second read
			stream2, err := ReadStream(bytes.NewReader(buf.Bytes()))
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
		stream1, err := ReadStream(bytes.NewReader(data))
		if err != nil {
			return
		}

		// write
		var buf bytes.Buffer
		if err := stream1.Write(&buf); err != nil {
			t.Fatalf("write failed after successful read: %v", err)
		}

		// second read
		stream2, err := ReadStream(bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatalf("second read failed: %v", err)
		}

		// compare
		if diff := cmp.Diff(stream1, stream2, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("round trip failed (-first +second):\n%s", diff)
		}
	})
}

func TestStreamValidate(t *testing.T) {
	tests := []struct {
		name    string
		stream  Stream
		version pdf.Version
		wantErr error
	}{
		{
			name: "valid PDF 1.0",
			stream: Stream{
				{Name: OpPushGraphicsState},
				{Name: OpPopGraphicsState},
			},
			version: pdf.V1_0,
			wantErr: nil,
		},
		{
			name: "operator not available in version",
			stream: Stream{
				{Name: OpSetExtGState}, // introduced in PDF 1.2
			},
			version: pdf.V1_0,
			wantErr: ErrVersion,
		},
		{
			name: "deprecated operator",
			stream: Stream{
				{Name: OpFillCompat}, // deprecated in PDF 2.0
			},
			version: pdf.V2_0,
			wantErr: ErrDeprecated,
		},
		{
			name: "unknown operator outside compatibility",
			stream: Stream{
				{Name: "XX"}, // unknown
			},
			version: pdf.V1_7,
			wantErr: ErrUnknown,
		},
		{
			name: "unknown operator inside BX/EX",
			stream: Stream{
				{Name: OpBeginCompatibility},
				{Name: "XX"}, // unknown but allowed
				{Name: OpEndCompatibility},
			},
			version: pdf.V1_7,
			wantErr: nil,
		},
		{
			name: "nested BX/EX",
			stream: Stream{
				{Name: OpBeginCompatibility},
				{Name: "XX"},
				{Name: OpBeginCompatibility},
				{Name: "YY"},
				{Name: OpEndCompatibility},
				{Name: "ZZ"},
				{Name: OpEndCompatibility},
			},
			version: pdf.V1_7,
			wantErr: nil,
		},
		{
			name: "error after EX",
			stream: Stream{
				{Name: OpBeginCompatibility},
				{Name: "XX"}, // ok
				{Name: OpEndCompatibility},
				{Name: "YY"}, // not ok
			},
			version: pdf.V1_7,
			wantErr: ErrUnknown,
		},
		{
			name: "version error inside BX/EX still reported",
			stream: Stream{
				{Name: OpBeginCompatibility},
				{Name: OpSetExtGState}, // known but not available in 1.0
				{Name: OpEndCompatibility},
			},
			version: pdf.V1_0,
			wantErr: ErrVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.stream.Validate(tt.version)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
			}
		})
	}
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
			stream, err := ReadStream(bytes.NewReader([]byte(tt.stream)))
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
	for i := 0; i < 100; i++ {
		buf.WriteString("1 ")
	}
	buf.WriteString("m\n") // moveto with 100 args (should be skipped)
	buf.WriteString("q\n") // save graphics state (should be parsed)

	stream, err := ReadStream(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ReadStream error: %v", err)
	}

	// the operator with too many args should be skipped
	if len(stream) != 1 {
		t.Errorf("got %d operators, want 1", len(stream))
	}
	if len(stream) > 0 && stream[0].Name != OpPushGraphicsState {
		t.Errorf("got operator %q, want %q", stream[0].Name, OpPushGraphicsState)
	}
}
