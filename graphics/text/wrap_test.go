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

package text

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestWrap(t *testing.T) {
	type testCase struct {
		name string
		in   []string
		out  [][]string
	}
	cases := []testCase{
		{
			name: "single word",
			in:   []string{"word"},
			out:  [][]string{{"word"}},
		},
		{
			name: "single input",
			in:   []string{"a b c"},
			out:  [][]string{{"a", "b", "c"}},
		},
		{
			name: "two paragraphs",
			in:   []string{"a b\nc d"},
			out:  [][]string{{"a", "b"}, {"c", "d"}},
		},
		{
			name: "trailing newline",
			in:   []string{"a b\nc d\n"},
			out:  [][]string{{"a", "b"}, {"c", "d"}},
		},
		{
			name: "empty paragraph",
			in:   []string{"a b\n\nc d"},
			out:  [][]string{{"a", "b"}, {}, {"c", "d"}},
		},
		{
			name: "several inputs",
			in:   []string{"a b", "c d"},
			out:  [][]string{{"a", "b", "c", "d"}},
		},
		{
			name: "many inputs",
			in:   []string{"a", "b", "c", "d"},
			out:  [][]string{{"a", "b", "c", "d"}},
		},
		{
			name: "complex",
			in:   []string{"a b", "c\nd e"},
			out:  [][]string{{"a", "b", "c"}, {"d", "e"}},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			w := Wrap(100, testCase.in...)
			if d := cmp.Diff(testCase.out, w.words); d != "" {
				t.Errorf("Wrap() mismatch (-want +got):\n%s", d)
			}
		})
	}
}
