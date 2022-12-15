// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package color

import (
	"bytes"
	"testing"
)

func TestColor(t *testing.T) {
	type testCase struct {
		color      Color
		wantFill   string
		wantStroke string
	}
	testCases := []testCase{
		{RGB(0, 0, 0), "0 0 0 rg\n", "0 0 0 RG\n"},
		{RGB(1, 0, 0), "1 0 0 rg\n", "1 0 0 RG\n"},
		{RGB(0, .5, 0), "0 .5 0 rg\n", "0 .5 0 RG\n"},
		{RGB(0, 0, 0.12345), "0 0 .123 rg\n", "0 0 .123 RG\n"},
		{RGB(0, 0, 0.98765), "0 0 .988 rg\n", "0 0 .988 RG\n"},
		{Gray(0), "0 g\n", "0 G\n"},
		{Gray(1), "1 g\n", "1 G\n"},
		{Gray(.25), ".25 g\n", ".25 G\n"},
		{Gray(1. / 3), ".333 g\n", ".333 G\n"},
		{Gray(.5), ".5 g\n", ".5 G\n"},
		{Gray(.12345), ".123 g\n", ".123 G\n"},
	}
	for _, tc := range testCases {
		buf := &bytes.Buffer{}
		err := tc.color.SetFill(buf)
		if err != nil {
			t.Fatalf("Fill(%v): %v", tc.color, err)
		}
		if got := buf.String(); got != tc.wantFill {
			t.Errorf("Fill(%v): got %q, want %q", tc.color, got, tc.wantFill)
		}

		buf.Reset()
		err = tc.color.SetStroke(buf)
		if err != nil {
			t.Fatalf("Stroke(%v): %v", tc.color, err)
		}
		if got := buf.String(); got != tc.wantStroke {
			t.Errorf("Stroke(%v): got %q, want %q", tc.color, got, tc.wantStroke)
		}
	}
}
