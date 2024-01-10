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

package main

import (
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/font/type3"
)

var (
	_ = &type1.EmbedInfo{}
	_ = &type3.EmbedInfo{}
	_ = &cff.EmbedInfoCFFSimple{}
	_ = &cff.EmbedInfoComposite{}
	_ = &truetype.EmbedInfoSimple{}
	_ = &truetype.EmbedInfoComposite{}
	_ = &opentype.EmbedInfoCFFSimple{}
	_ = &opentype.EmbedInfoCFFComposite{}
	_ = &opentype.EmbedInfoGlyfSimple{}
	_ = &opentype.EmbedInfoGlyfComposite{}
)
