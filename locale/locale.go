// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

// TODO(voss): read https://go.dev/blog/matchlang

// Locale information is used in the following places:
//
// * The /Lang entry in the catalog dictionary:
//   (Optional; PDF 1.4) A language identifier that shall specify the natural
//   language for all text in the document except where overridden by language
//   specifications for structure elements or marked content (see 14.9.2,
//   "Natural Language Specification"). If this entry is absent, the language
//   shall be considered unknown.
//
// * Text strings:
//   An escape sequence may appear anywhere in a Unicode text string to
//   indicate the language in which subsequent text shall be written.
//   The escape sequence shall consist of the following elements, in order:
//     a) The Unicode value U+001B (that is, the byte sequence 0 followed by 27).
//     b) A 2-byte ISO 639 language code.
//     c) (Optional) A 2-byte ISO 3166 country code.
//     d) The Unicode value U+001B.
//
// * The /Language entry in Optional Content Usage Dictionaries
//   uses 14.9.2, "Natural Language Specification"
//
// * The /Lang entry in CIDFonts dictionaries:
//   (Optional; PDF 1.5) A name specifying the language of the font, which may
//   be used for encodings where the language is not implied by the encoding
//   itself. The value shall be one of the codes defined by Internet RFC 3066,
//   Tags for the Identification of Languages, or (PDF 1.0) 2-character language
//   codes defined by ISO 639. If this entry is absent,
//   the language shall be considered to be unknown.

// Script indicates writing systems, for use with OpenType fonts
type Script uint16

func (s Script) String() string {
	switch s {
	case ScriptArabic:
		return "Arabic"
	case ScriptCJKIdeographic:
		return "CJKIdeographic"
	case ScriptCyrillic:
		return "Cyrillic"
	case ScriptGreek:
		return "Greek"
	case ScriptHebrew:
		return "Hebrew"
	case ScriptHiragana:
		return "Hiragana"
	case ScriptLatin:
		return "Latin"
	case ScriptThai:
		return "Thai"
	default:
		return fmt.Sprintf("Script(%d)", s)
	}
}

// Selected writing systems.
const (
	ScriptUndefined Script = iota
	ScriptArabic
	ScriptCJKIdeographic
	ScriptCyrillic
	ScriptGreek
	ScriptHebrew
	ScriptHiragana
	ScriptLatin
	ScriptThai
)

// Locale defines language properties of a text.
type Locale struct {
	Language Language
	Country  Country
	Script   Script
}

func (loc *Locale) String() string {
	if loc == nil || loc.Language == 0 && loc.Country == 0 {
		return ""
	}
	if loc.Country == 0 {
		return loc.Language.String()
	}
	return loc.Language.String() + "-" + loc.Country.String()
}

// TODO: are these needed/useful?
var (
	// DeDE denotes German as written in Germany.
	DeDE = &Locale{
		Language: LangGerman,
		Country:  CountryDEU,
		Script:   ScriptLatin,
	}

	// EnGB denotes English as written in the UK.
	EnGB = &Locale{
		Language: LangEnglish,
		Country:  CountryGBR,
		Script:   ScriptLatin,
	}

	// EnUS denotes English as written in the USA.
	EnUS = &Locale{
		Language: LangEnglish,
		Country:  CountryUSA,
		Script:   ScriptLatin,
	}
)
