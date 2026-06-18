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

package reference

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var testCases = []struct {
	name string
	dict *Dict
}{
	{
		name: "page index",
		dict: &Dict{
			F:         &file.Specification{FileName: "target.pdf", AFRelationship: file.RelationshipUnspecified},
			PageIndex: 3,
		},
	},
	{
		name: "first page, single use",
		dict: &Dict{
			F:         &file.Specification{FileName: "other.pdf", AFRelationship: file.RelationshipUnspecified},
			PageIndex: 0,
			SingleUse: true,
		},
	},
	{
		name: "page label",
		dict: &Dict{
			F:         &file.Specification{FileName: "target.pdf", AFRelationship: file.RelationshipUnspecified},
			PageLabel: "iv",
		},
	},
	{
		name: "with file identifier",
		dict: &Dict{
			F:         &file.Specification{FileName: "target.pdf", AFRelationship: file.RelationshipUnspecified},
			PageIndex: 7,
			ID:        []pdf.String{pdf.String("0123456789abcdef"), pdf.String("fedcba9876543210")},
		},
	},
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
			t.Run(tc.name+"/"+v.String(), func(t *testing.T) {
				roundTripTest(t, v, tc.dict)
			})
		}
	}
}

func roundTripTest(t *testing.T, version pdf.Version, d1 *Dict) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	obj, err := rm.Embed(d1)
	if pdf.IsWrongVersion(err) {
		t.Skip("version not supported")
	} else if err != nil {
		t.Fatalf("embed failed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm.Close failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("w.Close failed: %v", err)
	}

	x := pdf.NewExtractor(w)
	d2, err := pdf.ExtractorGet(x, nil, obj, ExtractDict)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if !d1.Equal(d2) {
		t.Errorf("round trip failed:\n got %+v\nwant %+v", d2, d1)
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}

	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(pdf.V1_7, opt)
		if err := memfile.AddBlankPage(w); err != nil {
			continue
		}
		rm := pdf.NewResourceManager(w)
		obj, err := rm.Embed(tc.dict)
		if err != nil {
			continue
		}
		if err := rm.Close(); err != nil {
			continue
		}
		w.GetMeta().Trailer["Quir:E"] = obj
		if err := w.Close(); err != nil {
			continue
		}
		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing object")
		}
		x := pdf.NewExtractor(r)
		d, err := ExtractDict(x, nil, obj, false)
		if err != nil {
			t.Skip("malformed reference dictionary")
		}
		roundTripTest(t, pdf.GetVersion(r), d)
	})
}
