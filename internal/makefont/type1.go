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

package makefont

import (
	"seehuhn.de/go/postscript/afm"
	pst1 "seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf/internal/convert"
)

// Type1 returns a Type1 font.
func Type1() (*pst1.Font, error) {
	info := TrueType()
	return convert.ToType1(info)
}

// AFM returns the font metrics for the font returned by [Type1].
func AFM() (*afm.Info, error) {
	info := TrueType()
	return convert.ToAFM(info)
}
