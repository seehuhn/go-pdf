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

package action

import (
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
	"seehuhn.de/go/pdf/optional"
)

// TestRenditionUndecodableJS checks that a JS stream which cannot be decoded
// is read like an absent JS entry.
func TestRenditionUndecodableJS(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	jsRef := w.Alloc()
	stm, err := w.OpenStream(jsRef, pdf.Dict{"Filter": pdf.Name("FlateDecode")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stm.Write([]byte("not deflate data")); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}

	c := pdf.NewCursor(w)
	for _, base := range []pdf.Dict{
		{"S": pdf.Name("Rendition"), "AN": pdf.NewReference(5, 0), "OP": pdf.Integer(1)},
	} {
		want, err := Decode(c, base, true)
		if err != nil {
			t.Fatal(err)
		}
		withJS := maps.Clone(base)
		withJS["JS"] = jsRef
		got, err := Decode(c, withJS, true)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("undecodable JS read differently from absent entry (-absent +got):\n%s", diff)
		}
	}
}

// TestRenditionOP checks how the OP entry is read.  OP is required unless JS
// is present, and an operation which cannot be honoured is never replaced by
// a different one.
func TestRenditionOP(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	an := pdf.NewReference(5, 0)
	badR := pdf.Name("x")
	js := pdf.String("app.alert(1);")

	type opCase struct {
		name string
		dict pdf.Dict
		op   optional.UInt // expected OP, if the read succeeds
		fail bool
	}
	var stop, none optional.UInt
	stop.Set(1)

	cases := []opCase{
		{"stop", pdf.Dict{"AN": an, "OP": pdf.Integer(1)}, stop, false},
		{"play with malformed R", pdf.Dict{"AN": an, "OP": pdf.Integer(0), "R": badR}, none, true},
		{"play with dangling R", pdf.Dict{"AN": an, "OP": pdf.Integer(0), "R": pdf.NewReference(99, 0)}, none, true},
		{"play without R", pdf.Dict{"AN": an, "OP": pdf.Integer(0)}, none, true},
		{"play-or-resume without R", pdf.Dict{"AN": an, "OP": pdf.Integer(4)}, none, true},
		{"absent OP", pdf.Dict{"AN": an}, none, true},
		{"absent OP with malformed R", pdf.Dict{"AN": an, "R": badR}, none, true},
		{"invalid OP", pdf.Dict{"AN": an, "OP": pdf.Integer(7)}, none, true},
		{"negative OP", pdf.Dict{"AN": an, "OP": pdf.Integer(-1)}, none, true},
		{"OP of wrong type", pdf.Dict{"AN": an, "OP": pdf.Name("Play")}, none, true},
		{"dangling OP", pdf.Dict{"AN": an, "OP": pdf.NewReference(99, 0)}, none, true},
		{"OP without AN", pdf.Dict{"OP": pdf.Integer(1)}, none, true},
		{"stop with malformed R", pdf.Dict{"AN": an, "OP": pdf.Integer(1), "R": badR}, stop, false},
		{"JS only", pdf.Dict{"AN": an, "JS": js}, none, false},
		{"JS and stop", pdf.Dict{"AN": an, "OP": pdf.Integer(1), "JS": js}, stop, false},
		{"JS and invalid OP", pdf.Dict{"AN": an, "OP": pdf.Integer(7), "JS": js}, none, false},
		{"JS and play without R", pdf.Dict{"AN": an, "OP": pdf.Integer(0), "JS": js}, none, false},
		{"JS and play with malformed R", pdf.Dict{"AN": an, "OP": pdf.Integer(0), "R": badR, "JS": js}, none, false},
		{"JS and OP without AN", pdf.Dict{"OP": pdf.Integer(1), "JS": js}, none, false},
		{"JS without AN", pdf.Dict{"JS": js}, none, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.dict["S"] = pdf.Name("Rendition")
			a, err := Decode(c, tc.dict, true)
			if tc.fail {
				if !pdf.IsMalformed(err) {
					t.Fatalf("got %v, want malformed error", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := a.(*Rendition).OP; got != tc.op {
				t.Errorf("OP = %v, want %v", got, tc.op)
			}
			testActionRoundTrip(t, pdf.V1_7, a)
		})
	}
}
