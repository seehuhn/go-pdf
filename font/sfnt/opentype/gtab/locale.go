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

package gtab

import "seehuhn.de/go/pdf/locale"

// https://docs.microsoft.com/en-us/typography/opentype/spec/scripttags
var otfScript = map[string]locale.Script{
	"DFLT": locale.ScriptUndefined,

	"cyrl": locale.ScriptCyrillic,
	"grek": locale.ScriptGreek,
	"latn": locale.ScriptLatin,
	"kana": locale.ScriptHiragana,
	"hani": locale.ScriptCJKIdeographic,
	"thai": locale.ScriptThai,
	"arab": locale.ScriptArabic,
	"hebr": locale.ScriptHebrew,
}

// https://docs.microsoft.com/en-us/typography/opentype/spec/languagetags
var otfLanguage = map[string]locale.Language{
	"dftl": locale.LangUndefined,

	"ARA ": locale.LangArabic,
	"AZE ": locale.LangAzerbaijani,
	"BEN ": locale.LangBengali,
	"BGR ": locale.LangBulgarian,
	"CAT ": locale.LangCatalan,
	"CSY ": locale.LangCzech,
	"DAN ": locale.LangDanish,
	"DEU ": locale.LangGerman,
	"ELL ": locale.LangGreek,
	"ENG ": locale.LangEnglish,
	"ESP ": locale.LangSpanish,
	"EUQ ": locale.LangBasque,
	"FIN ": locale.LangFinnish,
	"FRA ": locale.LangFrench,
	"HIN ": locale.LangHindi,
	"HUN ": locale.LangHungarian,
	"ITA ": locale.LangItalian,
	"JAN ": locale.LangJapanese,
	"KOR ": locale.LangKorean,
	"NLD ": locale.LangDutch,
	"NOR ": locale.LangNorwegianBokmal,
	"PLK ": locale.LangPolish,
	"PTG ": locale.LangPortuguese,
	"ROM ": locale.LangRomanian,
	"RUS ": locale.LangRussian,
	"SKY ": locale.LangSlovak,
	"SLV ": locale.LangSlovenian,
	"SVE ": locale.LangSwedish,
	"TRK ": locale.LangTurkish,
	"ZHS ": locale.LangChinese, // Chinese, Simplified
}
