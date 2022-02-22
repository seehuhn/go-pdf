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

package locale

import "fmt"

// Language represents a spoken language.
type Language uint16

// Language represents a RFC 3066 language subtag, as required by section
// 14.9.2.2 of PDF 32000-1:2008 and described in
// https://datatracker.ietf.org/doc/html/rfc3066

// String returns the two-letter ISO 639-1 language code.
func (l Language) String() string {
	language, ok := languages[l]
	if ok {
		return language.A2
	}
	return fmt.Sprintf("Language(%d)", l)
}

// Alpha3 returns the three-letter ISO 639-3 language code.
func (l Language) Alpha3() string {
	return languages[l].A3
}

// Name returns the ISO 639-1 language name.
func (l Language) Name() string {
	return languages[l].N
}

// Selected language tags, see
// https://en.wikipedia.org/wiki/List_of_ISO_639-1_codes
const (
	LangUndefined Language = iota
	LangArabic
	LangBengali
	LangChinese
	LangDutch
	LangEnglish
	LangFrench
	LangGerman
	LangGreek
	LangHindi
	LangItalian
	LangRomanian
	LangRussian
	LangSpanish
)

type languageCodes struct {
	A2 string // ISO 639-1 two-letter code
	A3 string // ISO 639-3 three-letter code
	N  string // ISO Language name
}

// selected languages from
// https://en.wikipedia.org/wiki/List_of_ISO_639-1_codes
var languages = map[Language]languageCodes{
	LangArabic:   {"ar", "ara", "Arabic"},
	LangBengali:  {"bn", "ben", "Bengali"},
	LangChinese:  {"zh", "zho", "Chinese"},
	LangDutch:    {"nl", "nld", "Dutch"},
	LangEnglish:  {"en", "eng", "English"},
	LangFrench:   {"fr", "fra", "French"},
	LangGerman:   {"de", "deu", "German"},
	LangGreek:    {"el", "ell", "Greek"},
	LangItalian:  {"it", "ita", "Italian"},
	LangHindi:    {"hi", "hin", "Hindi"},
	LangRomanian: {"ro", "ron", "Romanian"},
	LangRussian:  {"ru", "rus", "Russian"},
	LangSpanish:  {"es", "spa", "Spanish"},
}
