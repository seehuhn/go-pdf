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

package text

import (
	"math"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
)

// TestShowLeadingAcrossCalls checks that NL in a call without F or Leading
// uses the leading left in the graphics state by an earlier call.
func TestShowLeadingAcrossCalls(t *testing.T) {
	fnt := font.Must(standard.Helvetica.New())

	for _, explicit := range []bool{false, true} {
		b := builder.New(content.Page, nil, pdf.V2_0)

		var y0, y1 float64
		args := []any{M{X: 100, Y: 500}, F{Font: fnt, Size: 10}}
		if explicit {
			args = append(args, Leading(20))
		}
		args = append(args, RecordPos{UserY: &y0}, "a", NL, RecordPos{UserY: &y1})
		Show(b, args...)
		want := y0 - y1
		if explicit && want != 20 {
			t.Errorf("explicit leading: line distance %g, want 20", want)
		}

		var y2, y3 float64
		Show(b,
			M{X: 100, Y: 300},
			RecordPos{UserY: &y2},
			"b", NL,
			RecordPos{UserY: &y3},
		)
		if b.Err != nil {
			t.Fatal(b.Err)
		}
		if y2 != 300 {
			t.Errorf("explicit=%t: first M gave y=%g, want 300", explicit, y2)
		}
		if got := y2 - y3; math.Abs(got-want) > 1e-9 {
			t.Errorf("explicit=%t: second call line distance %g, want %g",
				explicit, got, want)
		}
	}
}
