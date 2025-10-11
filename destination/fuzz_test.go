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

	pageRef := pdf.Reference(10)

	testCases := []Destination{
		&XYZ{Page: Target(pageRef), Left: 100, Top: 200, Zoom: 1.5},
		&XYZ{Page: Target(pageRef), Left: Unset, Top: Unset, Zoom: Unset},
		&Fit{Page: Target(pageRef)},
		&FitH{Page: Target(pageRef), Top: 500},
		&FitV{Page: Target(pageRef), Left: 100},
		&FitR{Page: Target(pageRef), Left: 100, Bottom: 200, Right: 400, Top: 500},
		&FitB{Page: Target(pageRef)},
		&FitBH{Page: Target(pageRef), Top: 600},
		&FitBV{Page: Target(pageRef), Left: 50},
		&Named{Name: "Chapter6"},
	}

	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(pdf.V1_7, opt)
		rm := pdf.NewResourceManager(w)

		obj, err := tc.Encode(rm)
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
		w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
		rm := pdf.NewResourceManager(w)

		obj2, err := dest.Encode(rm)
		if err != nil {
			t.Fatalf("encode failed after decode: %v", err)
		}

		x2 := pdf.NewExtractor(w)
		dest2, err := Decode(x2, obj2)
		if err != nil {
			t.Fatalf("second decode failed: %v", err)
		}

		obj3, err := dest2.Encode(rm)
		if err != nil {
			t.Fatalf("second encode failed: %v", err)
		}

		if !equalObjects(obj2, obj3) {
			t.Errorf("round trip failed:\nfirst:  %v\nsecond: %v", obj2, obj3)
		}
	})
}
