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

package fonttypes

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/internal/makefont"
)

var Type1 = &type1embedder{}

type type1embedder struct{}

func (_ *type1embedder) Embed(w pdf.Putter, opt *font.Options) (font.Layouter, error) {
	info, err := makefont.Type1()
	if err != nil {
		return nil, err
	}
	afm, err := makefont.AFM()
	if err != nil {
		return nil, err
	}

	F, err := type1.New(info, afm)
	if err != nil {
		return nil, err
	}

	return F.Embed(w, opt)
}