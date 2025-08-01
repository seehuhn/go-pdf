// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package simpleenc

import (
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/postscript/type1/names"
)

func (t *Simple) ToUnicode(fontName string) *cmap.ToUnicodeFile {
	m := make(map[charcode.Code]string)
	for k, c := range t.code {
		glyphName := t.glyphName[k.gid]
		implied := names.ToUnicode(glyphName, fontName)
		if k.text != implied {
			m[charcode.Code(c)] = k.text
		}
	}

	if len(m) == 0 {
		return nil
	}

	tuInfo, _ := cmap.NewToUnicodeFile(charcode.Simple, m)
	return tuInfo
}
