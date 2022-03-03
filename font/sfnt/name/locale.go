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
	return loc.Language.String() + "_" + loc.Country.String()
}

// Selected Macintosh language codes
// https://docs.microsoft.com/en-us/typography/opentype/spec/name#macintosh-language-ids
var appleLang = map[uint16]Loc{
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
var msLang = map[uint16]Loc{
	0x0401: {Language: locale.LangArabic, Country: locale.CountrySAU},
	0x0406: {Language: locale.LangDanish, Country: locale.CountryDNK},
	0x0407: {Language: locale.LangGerman, Country: locale.CountryDEU},
	0x0408: {Language: locale.LangGreek, Country: locale.CountryGRC},
	0x0409: {Language: locale.LangEnglish, Country: locale.CountryUSA},
	0x040A: {Language: locale.LangSpanish, Country: locale.CountryESP}, // traditional sort
	0x040B: {Language: locale.LangFinnish, Country: locale.CountryFIN},
	0x040C: {Language: locale.LangFrench, Country: locale.CountryFRA},
	0x0410: {Language: locale.LangItalian, Country: locale.CountryITA},
	0x0413: {Language: locale.LangDutch, Country: locale.CountryNLD},
	0x0414: {Language: locale.LangNorwegianBokmal, Country: locale.CountryNOR},
	0x0415: {Language: locale.LangPolish, Country: locale.CountryPOL},
	0x0416: {Language: locale.LangPortuguese, Country: locale.CountryBRA},
	0x0418: {Language: locale.LangRomanian, Country: locale.CountryROU},
	0x0419: {Language: locale.LangRussian, Country: locale.CountryRUS},
	0x041D: {Language: locale.LangSwedish, Country: locale.CountrySWE},
	0x0439: {Language: locale.LangHindi, Country: locale.CountryIND},
	0x0445: {Language: locale.LangBengali, Country: locale.CountryIND},
	0x0804: {Language: locale.LangChinese, Country: locale.CountryCHN},
	0x0809: {Language: locale.LangEnglish, Country: locale.CountryGBR},
	0x0816: {Language: locale.LangPortuguese, Country: locale.CountryPRT},
	0x0845: {Language: locale.LangBengali, Country: locale.CountryBGD},
	0x0C0A: {Language: locale.LangSpanish, Country: locale.CountryESP}, // modern sort
	0x0C0C: {Language: locale.LangFrench, Country: locale.CountryCAN},
}

func appleCode(lang locale.Language) (uint16, bool) {
	for k, v := range appleLang {
		if v.Language == lang {
			return k, true
		}
	}
	return 0, false
}

func msCode(loc Loc) uint16 {
	for k, v := range msLang {
		if v == loc {
			return k
		}
	}
	return 0xFFFF
}
