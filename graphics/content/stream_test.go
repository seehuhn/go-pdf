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
	"seehuhn.de/go/pdf"
)

func TestStreamRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		stream string
	}{
		{
			name:   "simple operators",
			stream: "q\n1 0 0 1 100 200 cm\nQ\n",
		},
		{
			name:   "path operators",
			stream: "100 100 m\n200 200 l\nS\n",
		},
		{
			name:   "text operators",
			stream: "BT\n/F1 12 Tf\n(Hello) Tj\nET\n",
		},
		{
			name:   "arrays and dicts",
			stream: "[1 2 3] 0 d\n<</Type /XObject>> gs\n",
		},
		{
			name:   "comments",
			stream: "% this is a comment\nq\nQ\n",
		},
		{
			name:   "inline image",
			stream: "BI\n/W 10\n/H 10\nID\nimagedata\nEI\n",
		},
		{
			name:   "mixed content",
			stream: "q\n% save state\n1 0 0 1 0 0 cm\n/F1 12 Tf\n(Text) Tj\nQ\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// parse
			stream1, err := ReadStream(bytes.NewReader([]byte(tt.stream)))
			if err != nil {
				t.Fatalf("first parse failed: %v", err)
			}

			// write
			var buf bytes.Buffer
			if err := stream1.Write(&buf); err != nil {
				t.Fatalf("write failed: %v", err)
			}

			// parse again
			stream2, err := ReadStream(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("second parse failed: %v", err)
			}

			// compare
			if diff := cmp.Diff(stream1, stream2); diff != "" {
				t.Errorf("round trip failed (-first +second):\n%s", diff)
			}
		})
	}
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
