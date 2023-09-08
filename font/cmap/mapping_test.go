// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package cmap

import (
	"testing"

	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/type1"
)

func TestMapping(t *testing.T) {
	cs := charcode.CodeSpaceRange{
		{Low: []byte{0x00}, High: []byte{0x0F}},
		{Low: []byte{0x10, 0x12}, High: []byte{0x10, 0x7F}},
		{Low: []byte{0x11, 0x80, 0x80}, High: []byte{0x11, 0xFF, 0xFF}},
	}
	info := &Info{
		Name:   "Test",
		CS:     cs,
		CSFile: cs,
	}
	m1 := map[charcode.CharCode]type1.CID{
		0:  0, // single 1
		2:  1, // range ...
		3:  2,
		5:  3, // range ...
		7:  5,
		9:  7,
		10: 9, // single 2

		14: 10, // range ...
		15: 11,
		16: 12, // range ...
		17: 13,

		1000: 14, // single 3
	}
	info.SetMapping(m1)

	if len(info.Singles) != 3 {
		t.Errorf("expected 3 singles, got %d", len(info.Singles))
	}

	m2 := info.GetMapping()

	for code, cid := range m1 {
		if cid2, ok := m2[code]; !ok || cid2 != cid {
			t.Errorf("mismatch for code %d: %d != %d", code, cid2, cid)
		}
	}
}

func TestMappingPredefined(t *testing.T) {
	for _, name := range allPredefined {
		name := name
		t.Run(name, func(t *testing.T) {
			r, err := openPredefined(name)
			if err != nil {
				t.Fatal(err)
			}
			info, err := Read(r, nil)
			if err != nil {
				t.Fatal(err)
			}
			err = r.Close()
			if err != nil {
				t.Fatal(err)
			}

			m1 := info.GetMapping()
			info.SetMapping(m1)
			m2 := info.GetMapping()

			for code, cid := range m1 {
				if cid2, ok := m2[code]; !ok || cid2 != cid {
					t.Errorf("%s: mismatch for code %d: %d != %d", name, code, cid2, cid)
				}
			}
		})
	}
}
