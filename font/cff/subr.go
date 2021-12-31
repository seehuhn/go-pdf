// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package cff

func bias(nSubrs int) int {
	if nSubrs < 1240 {
		return 107
	} else if nSubrs < 33900 {
		return 1131
	} else {
		return 32768
	}
}

func (cff *Font) getSubr(biased int) ([]byte, error) {
	idx := biased + bias(len(cff.subrs))
	if idx < 0 || idx >= len(cff.subrs) {
		return nil, errInvalidSubroutine
	}
	return cff.subrs[idx], nil
}

func (cff *Font) getGSubr(biased int) ([]byte, error) {
	idx := biased + bias(len(cff.gsubrs))
	if idx < 0 || idx >= len(cff.gsubrs) {
		return nil, errInvalidSubroutine
	}
	return cff.gsubrs[idx], nil
}

// size used for a subroutine:
//   - an entry in the subrs and gsubrs INDEX takes
//     up to 4 bytes, plus the size of the subroutine
//   - the subrouting must finish with t2return
//     or t2endchar (1 byte)
//   - calling the subroutine uses k+1 bytes, where
//     k=1 for the first 215 subroutines of each type, and
//     k=2 for the next 2048 subroutines of each type.
// An approximation could be the following:
//   - if n bytes occur k times, this uses n*k bytes
//   - if the n bytes are turned into a subroutine, this uses
//     approximately k*2 + n + 3 or k*3 + n + 4 bytes.
//   - the savings are n*k - k*2 - n - 3 = (n-2)*(k-1)-5
//     or n*k - k*3 - n - 4 = (n-3)*(k-1)-7 bytes.
