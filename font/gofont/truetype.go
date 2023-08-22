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

package gofont

import (
	"bytes"

	"seehuhn.de/go/sfnt"
)

// TrueType returns `font` as a TrueType font.
func TrueType(font FontID) (*sfnt.Font, error) {
	raw, ok := ttf[font]
	if !ok {
		return nil, ErrInvalidFontID
	}
	r := bytes.NewReader(raw)

	info, err := sfnt.Read(r)
	if err != nil {
		return nil, err
	}
	if !info.IsGlyf() {
		panic("not a TrueType font")
	}
	return info, nil
}
