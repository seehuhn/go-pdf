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
	"seehuhn.de/go/pdf/internal/debug/mock"
)

// TestMovieInvalidTimestamp checks that malformed Start and Duration values
// in a movie action are read like absent entries.
func TestMovieInvalidTimestamp(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	base := pdf.Dict{"S": pdf.Name("Movie"), "T": pdf.String("movie")}
	want, err := Decode(c, base, true)
	if err != nil {
		t.Fatal(err)
	}

	invalid := []pdf.Object{
		pdf.Integer(-5),
		pdf.String("short"),
		pdf.Array{pdf.Integer(-1), pdf.Integer(600)},
		pdf.Array{pdf.Integer(10), pdf.Integer(0)},
		pdf.Array{pdf.Integer(10)},
	}
	for _, key := range []pdf.Name{"Start", "Duration"} {
		for _, value := range invalid {
			dict := maps.Clone(base)
			dict[key] = value
			got, err := Decode(c, dict, true)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("%s %v read differently from absent entry (-absent +got):\n%s", key, value, diff)
			}
		}
	}
}

// TestMovieInvalidFWEntries checks that malformed FWScale and FWPosition
// values read like absent entries.
func TestMovieInvalidFWEntries(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	base := pdf.Dict{"S": pdf.Name("Movie"), "T": pdf.String("movie")}
	want, err := Decode(c, base, true)
	if err != nil {
		t.Fatal(err)
	}

	dangling := pdf.NewReference(99, 0)
	invalid := map[pdf.Name][]pdf.Object{
		"FWScale": {
			pdf.Array{pdf.Integer(0), pdf.Integer(2)},
			pdf.Array{pdf.Integer(1), pdf.Integer(-2)},
			pdf.Array{pdf.Integer(1)},
			pdf.Array{pdf.Name("x"), pdf.Integer(2)},
			pdf.Array{pdf.Integer(1), dangling},
			pdf.Array{},
		},
		"FWPosition": {
			pdf.Array{pdf.Real(-0.5), pdf.Real(0.5)},
			pdf.Array{pdf.Real(0.5), pdf.Real(1.5)},
			pdf.Array{pdf.Real(0.5)},
			pdf.Array{pdf.Name("x"), pdf.Real(0.5)},
			pdf.Array{pdf.Real(0.5), dangling},
			pdf.Array{},
		},
	}
	for key, values := range invalid {
		for _, value := range values {
			dict := maps.Clone(base)
			dict[key] = value
			got, err := Decode(c, dict, true)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("%s %v read differently from absent entry (-absent +got):\n%s", key, value, diff)
			}
		}
	}
}
