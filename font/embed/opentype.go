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

package embed

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/sfnt"
)

// OpenTypeFile loads and embeds an OpenType/TrueType font.
func OpenTypeFile(fname string, opt *font.Options) (font.Layouter, error) {
	info, err := sfnt.ReadFile(fname)
	if err != nil {
		return nil, err
	}

	return OpenTypeFont(info, opt)
}

// OpenTypeFont embeds an OpenType/TrueType font.
func OpenTypeFont(info *sfnt.Font, opt *font.Options) (font.Layouter, error) {
	var F font.Layouter
	var err error
	if info.IsCFF() {
		F, err = cff.New(info, opt)
	} else {
		F, err = truetype.New(info, opt)
	}
	if err != nil {
		return nil, err
	}
	return F, nil
}
