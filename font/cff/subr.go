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

func (cff *Font) getSubr(biased int) []byte {
	idx := biased + bias(len(cff.subrs))
	return cff.subrs[idx]
}

func (cff *Font) getGSubr(biased int) []byte {
	idx := biased + bias(len(cff.gsubrs))
	return cff.gsubrs[idx]
}
