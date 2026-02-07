// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package color

import (
	"testing"

	"seehuhn.de/go/icc"
)

func TestICCBased(t *testing.T) {
	for _, profile := range [][]byte{icc.SRGBv2Profile, icc.SRGBv4Profile} {
		icc, err := ICCBased(profile, nil)
		if err != nil {
			t.Errorf("ICCBased: %v", err)
			continue
		}

		if icc.N != 3 {
			t.Errorf("expected 3 components, got %d", icc.N)
		}
		if !isValues(icc.Ranges, 0, 1, 0, 1, 0, 1) {
			t.Errorf("invalid ranges: %v", icc.Ranges)
		}
	}
}
