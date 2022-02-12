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
