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

package file

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/mock"
	"seehuhn.de/go/pdf/internal/limits"
)

// TestExtractAssociatedFiles checks that unusable AF entries are dropped,
// and that a list with no usable entry reads as nil rather than empty.
func TestExtractAssociatedFiles(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	good := pdf.Dict{"Type": pdf.Name("Filespec"), "F": pdf.String("a.pdf")}
	unnamed := pdf.Dict{"Type": pdf.Name("Filespec")}

	tooMany := make(pdf.Array, limits.MaxAssociatedFiles+1)
	for i := range tooMany {
		tooMany[i] = good
	}

	cases := map[string]struct {
		in   pdf.Object
		want int
	}{
		"absent":       {nil, 0},
		"not an array": {pdf.Integer(1), 0},
		"empty":        {pdf.Array{}, 0},
		"all unusable": {pdf.Array{pdf.Integer(1), unnamed, pdf.NewReference(99, 0)}, 0},
		"mixed":        {pdf.Array{pdf.Integer(1), good, unnamed, good}, 2},
		"too many":     {tooMany, 0},
	}
	for name, tc := range cases {
		got, err := ExtractAssociatedFiles(c, tc.in)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if tc.want == 0 && got != nil {
			t.Errorf("%s: got %v, want nil", name, got)
		} else if len(got) != tc.want {
			t.Errorf("%s: got %d entries, want %d", name, len(got), tc.want)
		}
	}
}
