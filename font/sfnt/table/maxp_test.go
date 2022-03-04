// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package table

import (
	"bytes"
	"testing"
)

func TestMaxp(t *testing.T) {
	for _, numGlyphs := range []int{1, 2, 3, 255, 256, 1000, 65535} {
		info := &MaxpInfo{NumGlyphs: numGlyphs}
		maxp, err := info.Encode()
		if err != nil {
			t.Errorf("EncodeMaxp(%d): %v", numGlyphs, err)
			continue
		}
		maxpInfo, err := ReadMaxp(bytes.NewReader(maxp))
		if err != nil {
			t.Errorf("ReadMaxp(%d): %v", numGlyphs, err)
			continue
		}
		gotNumGlyphs := maxpInfo.NumGlyphs
		if gotNumGlyphs != numGlyphs {
			t.Errorf("ReadMaxp(%d): got %d glyphs, want %d", numGlyphs, gotNumGlyphs, numGlyphs)
		}
	}
}

func FuzzMaxp(f *testing.F) {
	f.Add([]byte{0x00, 0x00, 0x50, 0x00, 0x12, 0x34})
	f.Fuzz(func(t *testing.T, data []byte) {
		maxpInfo, err := ReadMaxp(bytes.NewReader(data))
		if err != nil {
			return
		}
		data2, err := maxpInfo.Encode()
		if err != nil {
			t.Fatal(err)
		}
		maxpInfo2, err := ReadMaxp(bytes.NewReader(data2))
		if err != nil {
			t.Fatal(err)
		}
		if maxpInfo2.NumGlyphs != maxpInfo.NumGlyphs {
			t.Errorf("numGlyphs: %d != %d", maxpInfo2.NumGlyphs, maxpInfo.NumGlyphs)
		}
	})
}
