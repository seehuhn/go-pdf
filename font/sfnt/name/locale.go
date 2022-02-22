// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package name

import (
	"unicode/utf16"

	"seehuhn.de/go/pdf/locale"
)

// Loc contains locale information for name table entries.
type Loc struct {
	Language locale.Language
	Country  locale.Country
}

func (loc Loc) String() string {
	if loc.Country == 0 {
		return loc.Language.String()
	}
	return loc.Language.String() + "-" + loc.Country.String()
}

// Selected Macintosh language codes
// https://docs.microsoft.com/en-us/typography/opentype/spec/name#macintosh-language-ids
var appleCodes = map[uint16]Loc{
	0:  {Language: locale.LangEnglish},
	1:  {Language: locale.LangFrench},
	2:  {Language: locale.LangGerman},
	3:  {Language: locale.LangItalian},
	4:  {Language: locale.LangDutch},
	6:  {Language: locale.LangSpanish},
	12: {Language: locale.LangArabic},
	14: {Language: locale.LangGreek},
	21: {Language: locale.LangHindi},
	32: {Language: locale.LangRussian},
	37: {Language: locale.LangRomanian},
	67: {Language: locale.LangBengali},
}

// Selected Windows language codes
// https://docs.microsoft.com/en-us/typography/opentype/spec/name#windows-language-ids
var microsoftCodes = map[uint16]Loc{
	0x0401: {Language: locale.LangArabic, Country: locale.CountrySAU},
	0x0407: {Language: locale.LangGerman, Country: locale.CountryDEU},
	0x0408: {Language: locale.LangGreek, Country: locale.CountryGRC},
	0x0409: {Language: locale.LangEnglish, Country: locale.CountryUSA},
	0x040A: {Language: locale.LangSpanish, Country: locale.CountryESP}, // traditional sort
	0x040C: {Language: locale.LangFrench, Country: locale.CountryFRA},
	0x0410: {Language: locale.LangItalian, Country: locale.CountryITA},
	0x0413: {Language: locale.LangDutch, Country: locale.CountryNLD},
	0x0418: {Language: locale.LangRomanian, Country: locale.CountryROU},
	0x0419: {Language: locale.LangRussian, Country: locale.CountryRUS},
	0x0439: {Language: locale.LangHindi, Country: locale.CountryIND},
	0x0445: {Language: locale.LangBengali, Country: locale.CountryIND},
	0x0804: {Language: locale.LangChinese, Country: locale.CountryCHN},
	0x0809: {Language: locale.LangEnglish, Country: locale.CountryGBR},
	0x0845: {Language: locale.LangBengali, Country: locale.CountryBGD},
	0x0C0A: {Language: locale.LangSpanish, Country: locale.CountryESP}, // modern sort
}

func utf16Encode(s string) []byte {
	rr := utf16.Encode([]rune(s))
	res := make([]byte, len(rr)*2)
	for i, r := range rr {
		res[i*2] = byte(r >> 8)
		res[i*2+1] = byte(r)
	}
	return res
}

func utf16Decode(buf []byte) string {
	var nameWords []uint16
	for i := 0; i+1 < len(buf); i += 2 {
		nameWords = append(nameWords, uint16(buf[i])<<8|uint16(buf[i+1]))
	}
	return string(utf16.Decode(nameWords))
}
