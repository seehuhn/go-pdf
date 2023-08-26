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

package content

import (
	"fmt"
	"unicode"

	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/font/type3"
)

func MakeTextDecoder(r pdf.Getter, ref pdf.Reference) (func(pdf.String) string, error) {
	dicts, err := font.ExtractDicts(r, ref)
	if err != nil {
		return nil, err
	}

	var CS charcode.CodeSpaceRange
	var toUnicode map[charcode.CharCode][]rune
	// TODO(voss): make this less repetitive
	switch dicts.Type {
	case font.Type1, font.Builtin:
		CS = charcode.Simple
		info, err := type1.Extract(r, dicts)
		if err != nil {
			return nil, err
		}
		if info.ToUnicode != nil {
			toUnicode = info.ToUnicode
			break
		}

		toUnicode = make(map[charcode.CharCode][]rune)
		for i := 0; i < 256; i++ {
			name := info.Encoding[i]
			toUnicode[charcode.CharCode(i)] = names.ToUnicode(name, false)
		}
	case font.CFFSimple:
		CS = charcode.Simple
		info, err := cff.ExtractSimple(r, dicts)
		if err != nil {
			return nil, err
		}
		if info.ToUnicode != nil {
			toUnicode = info.ToUnicode
			break
		}

		toUnicode = make(map[charcode.CharCode][]rune)
		for i := 0; i < 256; i++ {
			gid := info.Encoding[i]
			name := info.Font.Glyphs[gid].Name
			toUnicode[charcode.CharCode(i)] = names.ToUnicode(name, false)
		}
	case font.OpenTypeCFFSimple:
		CS = charcode.Simple
		info, err := opentype.ExtractCFFSimple(r, dicts)
		if err != nil {
			return nil, err
		}
		if info.ToUnicode != nil {
			toUnicode = info.ToUnicode
			break
		}

		toUnicode = make(map[charcode.CharCode][]rune)
		for i := 0; i < 256; i++ {
			gid := info.Encoding[i]
			name := info.Font.GlyphName(gid)
			toUnicode[charcode.CharCode(i)] = names.ToUnicode(name, false)
		}
	case font.TrueTypeSimple:
		CS = charcode.Simple
		info, err := truetype.ExtractSimple(r, dicts)
		if err != nil {
			return nil, err
		}
		if info.ToUnicode != nil {
			toUnicode = info.ToUnicode
			break
		}
	case font.OpenTypeGlyfSimple:
		CS = charcode.Simple
		info, err := opentype.ExtractGlyfSimple(r, dicts)
		if err != nil {
			return nil, err
		}
		if info.ToUnicode != nil {
			toUnicode = info.ToUnicode
			break
		}
	case font.Type3:
		CS = charcode.Simple
		info, err := type3.Extract(r, dicts)
		if err != nil {
			return nil, err
		}
		if info.ToUnicode != nil {
			toUnicode = info.ToUnicode
			break
		}

		toUnicode = make(map[charcode.CharCode][]rune)
		for i := 0; i < 256; i++ {
			name := info.Encoding[i]
			toUnicode[charcode.CharCode(i)] = names.ToUnicode(name, false)
		}

	case font.CFFComposite:
		info, err := cff.ExtractComposite(r, dicts)
		if err != nil {
			return nil, err
		}
		CS = info.CS

		if info.ToUnicode != nil {
			toUnicode = info.ToUnicode
			break
		}
		// TODO(voss): other methods ...

	case font.OpenTypeCFFComposite:
		info, err := opentype.ExtractCFFComposite(r, dicts)
		if err != nil {
			return nil, err
		}
		CS = info.CS

		if info.ToUnicode != nil {
			toUnicode = info.ToUnicode
			break
		}
		// TODO(voss): other methods ...

	case font.TrueTypeComposite:
		info, err := truetype.ExtractComposite(r, dicts)
		if err != nil {
			return nil, err
		}
		CS = info.CS

		if info.ToUnicode != nil {
			toUnicode = info.ToUnicode
			break
		}
		// TODO(voss): other methods ...

	case font.OpenTypeGlyfComposite:
		info, err := opentype.ExtractGlyfComposite(r, dicts)
		if err != nil {
			return nil, err
		}
		CS = info.CS

		if info.ToUnicode != nil {
			toUnicode = info.ToUnicode
			break
		}
		// TODO(voss): other methods ...

	}

	if toUnicode == nil {
		return nil, fmt.Errorf("unsupported font type: %s", dicts.Type)
	}

	fn := func(s pdf.String) string {
		var res []rune
		for len(s) > 0 {
			code, k := CS.Decode(s)
			s = s[k:]

			if code < 0 {
				res = append(res, unicode.ReplacementChar)
			} else {
				res = append(res, toUnicode[charcode.CharCode(code)]...)
			}
		}
		return string(res)
	}
	return fn, nil
}