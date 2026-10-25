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

package annotation

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

// TestBorderStyleInvalidWidth checks that a reference to a nonexistent
// object, and a value of the wrong type, are read like an absent /W entry.
func TestBorderStyleInvalidWidth(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)

	want, err := ExtractBorderStyle(c, pdf.Dict{}, true)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]pdf.Object{
		"dangling": pdf.NewReference(99, 0),
		"name":     pdf.Name("x"),
	} {
		got, err := ExtractBorderStyle(c, pdf.Dict{"W": value}, true)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("%s value read differently from absent entry (-absent +got):\n%s", name, diff)
		}
	}
}
