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

package pdf_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestArrayRoundTrip(t *testing.T) {
	var cases = []pdf.Array{
		nil,
		{},
		{pdf.Integer(1), pdf.Integer(2), pdf.Integer(3)},
		{pdf.Integer(1), nil, pdf.Integer(3)},
		{pdf.Array{}},
	}
	for i, a := range cases {
		t.Run(fmt.Sprintf("case%d", i), func(t *testing.T) {
			w, m := memfile.NewPDFWriter(pdf.V2_0, nil)
			ref := w.Alloc()
			w.Put(ref, a)
			b, err := pdf.GetArray(w, ref)
			if err != nil {
				t.Fatal(err)
			}
			if d := cmp.Diff(a, b); d != "" {
				fmt.Println(string(m.Data))
				t.Error(d)
			}
		})
	}
}
