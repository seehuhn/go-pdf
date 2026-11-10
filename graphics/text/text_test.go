// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
