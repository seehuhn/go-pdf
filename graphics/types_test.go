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

package graphics

import "testing"

// TestLineCapValues checks the numeric values of the line cap styles.
// These are written to PDF files as the operand of the "J" operator and as
// the /LC entry of an ExtGState dictionary, so they must be the values
// section 8.4.3.3 of ISO 32000-2:2020 gives.  The constants alias
// seehuhn.de/go/geom/path, which is not bound by the PDF specification.
func TestLineCapValues(t *testing.T) {
	cases := []struct {
		style LineCapStyle
		want  uint8
	}{
		{LineCapButt, 0},
		{LineCapRound, 1},
		{LineCapSquare, 2},
	}
	for _, tc := range cases {
		if uint8(tc.style) != tc.want {
			t.Errorf("%s has value %d, want %d", tc.style, uint8(tc.style), tc.want)
		}
	}
}

// TestLineJoinValues checks the numeric values of the line join styles.
// These are written to PDF files as the operand of the "j" operator and as
// the /LJ entry of an ExtGState dictionary, so they must be the values
// section 8.4.3.4 of ISO 32000-2:2020 gives.  The constants alias
// seehuhn.de/go/geom/path, which is not bound by the PDF specification.
func TestLineJoinValues(t *testing.T) {
	cases := []struct {
		style LineJoinStyle
		want  uint8
	}{
		{LineJoinMiter, 0},
		{LineJoinRound, 1},
		{LineJoinBevel, 2},
	}
	for _, tc := range cases {
		if uint8(tc.style) != tc.want {
			t.Errorf("%s has value %d, want %d", tc.style, uint8(tc.style), tc.want)
		}
	}
}
