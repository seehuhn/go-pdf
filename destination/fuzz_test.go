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

package destination

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func FuzzRoundTrip(f *testing.F) {
	// seed corpus with test cases
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}

	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(pdf.V1_7, opt)
		rm := pdf.NewResourceManager(w)

		obj, err := tc.Dest.Encode(rm)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:Dest"] = obj
		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:Dest"]
		if obj == nil {
			t.Skip("missing destination object")
		}

		x := pdf.NewExtractor(r)
		dest, err := Decode(x, obj)
		if err != nil {
			t.Skip("malformed destination")
		}

		// round-trip test
		testRoundTrip(t, dest)
	})
}
