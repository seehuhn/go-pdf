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

package graphics_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/debug/tempfile"
)

func TestMarkedContentInline(t *testing.T) {
	type TestCase struct {
		Prop pdf.Dict
		out  string
		err  error
	}
	var testCases = []TestCase{
		{nil, "/test MP\n", nil},
		{pdf.Dict{"X": pdf.Integer(1)}, "/test <</X 1>> DP\n", nil},
		{pdf.Dict{"X": pdf.NewReference(1, 0)}, "", graphics.ErrNotDirect},
	}
	data, _ := tempfile.NewTempWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(data)
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test%02d", i), func(t *testing.T) {
			buf := &bytes.Buffer{}
			w := graphics.NewWriter(buf, rm)
			mc := &graphics.MarkedContent{
				Tag:        "test",
				Properties: tc.Prop,
				Inline:     true,
			}

			w.MarkedContentPoint(mc)
			if w.Err != tc.err {
				t.Fatalf("expected %v, got %v", tc.err, w.Err)
			}

			if tc.out != "" && !strings.Contains(buf.String(), tc.out) {
				t.Errorf("expected %q, got %q", tc.out, buf.String())
			}
		})
	}
}
