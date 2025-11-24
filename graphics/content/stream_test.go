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
	"testing"

	"github.com/google/go-cmp/cmp"
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
