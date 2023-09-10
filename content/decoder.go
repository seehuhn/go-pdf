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

	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/tounicode"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/font/type3"
)

func makeTextDecoder(r pdf.Getter, ref pdf.Object) (func(pdf.String) string, error) {
	dicts, err := font.ExtractDicts(r, ref)
	if err != nil {
		return nil, err
	}

	var toUnicode *tounicode.Info
	// TODO(voss): make this less repetitive
	switch dicts.Type {
	case font.Type1, font.Builtin:
		info, err := type1.Extract(r, dicts)
		if err != nil {
			return nil, err
		}
		if info.ToUnicode != nil {
			toUnicode = info.ToUnicode
			break
		}

		// construct a ToUnicode map from the Encoding
		m := make(map[charcode.CharCode][]rune)
		for i := 0; i < 256; i++ {
			name := info.Encoding[i]
			m[charcode.CharCode(i)] = names.ToUnicode(name, false)
		}
		toUnicode = tounicode.New(charcode.Simple, m)

	case font.CFFSimple:
		info, err := cff.ExtractSimple(r, dicts)
		if err != nil {
			return nil, err
		}
		if info.ToUnicode != nil {
			toUnicode = info.ToUnicode
			break
		}

		// construct a ToUnicode map from the Encoding
		m := make(map[charcode.CharCode][]rune)
		for i := 0; i < 256; i++ {
			gid := info.Encoding[i]
			name := info.Font.Glyphs[gid].Name
			m[charcode.CharCode(i)] = names.ToUnicode(name, false)
		}
		toUnicode = tounicode.New(charcode.Simple, m)

	case font.OpenTypeCFFSimple:
		info, err := opentype.ExtractCFFSimple(r, dicts)
		if err != nil {
			return nil, err
		}
		if info.ToUnicode != nil {
			toUnicode = info.ToUnicode
			break
		}

		// construct a ToUnicode map from the Encoding
		m := make(map[charcode.CharCode][]rune)
		for i := 0; i < 256; i++ {
			gid := info.Encoding[i]
			name := info.Font.GlyphName(gid)
			m[charcode.CharCode(i)] = names.ToUnicode(name, false)
		}
		toUnicode = tounicode.New(charcode.Simple, m)

	case font.TrueTypeSimple:
		info, err := truetype.ExtractSimple(r, dicts)
		if err != nil {
			return nil, err
		}
		if info.ToUnicode != nil {
			toUnicode = info.ToUnicode
			break
		}

		// construct a ToUnicode map from the Encoding
		// TODO(voss): revisit this, once
		// https://github.com/pdf-association/pdf-issues/issues/316 is resolved.
		if encodingEntry, _ := pdf.Resolve(r, dicts.FontDict["Encoding"]); encodingEntry != nil {
			encodingNames, _ := font.UndescribeEncodingType1(r, encodingEntry, pdfenc.StandardEncoding[:])
			for i, name := range encodingNames {
				if name == ".notdef" {
					encodingNames[i] = pdfenc.StandardEncoding[i]
				}
			}

			m := make(map[charcode.CharCode][]rune)
			for i, name := range encodingNames {
				m[charcode.CharCode(i)] = names.ToUnicode(name, false)
			}
			toUnicode = tounicode.New(charcode.Simple, m)
			break
		}

		// TODO(voss): use info.Encoding together with the TrueType "cmap" table

	case font.OpenTypeGlyfSimple:
		info, err := opentype.ExtractGlyfSimple(r, dicts)
		if err != nil {
			return nil, err
		}
		if info.ToUnicode != nil {
			toUnicode = info.ToUnicode
			break
		}
		// TODO(voss): other methods???

	case font.Type3:
		info, err := type3.Extract(r, dicts)
		if err != nil {
			return nil, err
		}
		if info.ToUnicode != nil {
			toUnicode = info.ToUnicode
			break
		}

		// construct a ToUnicode map from the Encoding
		m := make(map[charcode.CharCode][]rune)
		for i := 0; i < 256; i++ {
			name := info.Encoding[i]
			m[charcode.CharCode(i)] = names.ToUnicode(name, false)
		}
		toUnicode = tounicode.New(charcode.Simple, m)

	case font.CFFComposite:
		info, err := cff.ExtractComposite(r, dicts)
		if err != nil {
			return nil, err
		}

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
			rr, k := toUnicode.Decode(s)
			s = s[k:]
			res = append(res, rr...)
		}
		return string(res)
	}
	return fn, nil
}
