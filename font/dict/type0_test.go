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

package dict

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestType0RoundTrip(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	ref := w.Alloc()

	fd := &font.Descriptor{}
	ros := &cmap.CIDSystemInfo{
		Registry:   "Quire",
		Ordering:   "Test",
		Supplement: 2,
	}
	d1 := &CIDFontType0{
		Ref:            ref,
		PostScriptName: "Test",
		Descriptor:     fd,
		ROS:            ros,
		Encoding: &cmap.File{
			Name:           "Test-cmap",
			ROS:            ros,
			WMode:          font.Vertical,
			CodeSpaceRange: charcode.Simple,
			CIDSingles: []cmap.Single{
				{
					Code:  []byte{' '},
					Value: 1,
				},
			},
		},
		Width: map[cmap.CID]float64{
			0: 1000,
			1: 500,
		},
		DefaultWidth: 750,
		Text: &cmap.ToUnicodeFile{
			CodeSpaceRange: charcode.Simple,
			Singles: []cmap.ToUnicodeSingle{
				{
					Code:  []byte{' '},
					Value: " ",
				},
			},
		},
		FontType: glyphdata.None,
	}
	err := d1.WriteToPDF(rm)
	if err != nil {
		t.Fatal(err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	d2, err := ExtractCIDFontType0(w, ref)
	if err != nil {
		t.Fatal(err)
	}

	if d := cmp.Diff(d1, d2); d != "" {
		t.Errorf("diff: %s", d)
	}
}
