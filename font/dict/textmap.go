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

package dict

import (
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/postscript/type1/names"
)

func simpleTextMap(postScriptName string, encoding encoding.Simple, toUnicode *cmap.ToUnicodeFile) map[byte]string {
	textMap := make(map[byte]string)
	for code := range 256 {
		glyphName := encoding(byte(code))
		text := names.ToUnicode(glyphName, postScriptName)
		if text != "" {
			textMap[byte(code)] = text
		}
	}
	if toUnicode != nil {
		codec, _ := charcode.NewCodec(charcode.Simple)
		for code, text := range toUnicode.All(codec) {
			if text != "" {
				textMap[byte(code)] = text
			}
		}
	}
	return textMap
}
