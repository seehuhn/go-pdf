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
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
)

// [Write] is a thin serialiser — the validation-bearing API lives in
// [Builder].  These tests pin Write's actual behaviour: each operator is
// emitted via [Operator.Format] and no rewriting or validation happens.
func TestWrite_Simple(t *testing.T) {
	var buf bytes.Buffer

	stream := []Operator{
		{Name: OpSetLineWidth, Args: []pdf.Object{pdf.Number(2)}},
		{Name: OpMoveTo, Args: []pdf.Object{pdf.Number(100), pdf.Number(100)}},
		{Name: OpLineTo, Args: []pdf.Object{pdf.Number(200), pdf.Number(200)}},
		{Name: OpStroke},
	}

	if err := Write(&buf, &Operators{Ops: stream}); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got := buf.String()
	for _, want := range []string{"w", "m", "l", "S"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q: %s", want, got)
		}
	}
}

// TestWrite_NoValidation confirms Write accepts inputs that Builder
// would reject: a stray Tj outside BT/ET, a missing-resource Tf, etc.
// The construction-time gate is Builder's job.
func TestWrite_NoValidation(t *testing.T) {
	cases := [][]Operator{
		// stray Tj at page level — invalid PDF, Write accepts.
		{{Name: OpTextShow, Args: []pdf.Object{pdf.String("oops")}}},
		// missing-resource Tf — Write doesn't look it up.
		{{Name: OpTextSetFont, Args: []pdf.Object{pdf.Name("F1"), pdf.Number(12)}}},
		// unbalanced q — Write doesn't track.
		{{Name: OpPushGraphicsState}},
		// deprecated F — Write emits it verbatim.
		{{Name: OpFillCompat}},
	}
	for i, ops := range cases {
		var buf bytes.Buffer
		if err := Write(&buf, &Operators{Ops: ops}); err != nil {
			t.Errorf("case %d: unexpected Write error: %v", i, err)
		}
	}
}
