// seehuhn.de/go/pdf - support for reading and writing PDF files
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

// Language represents a RFC 3066 language subtag, as required by section
// 14.9.2.2 of PDF 32000-1:2008 and described in
// https://datatracker.ietf.org/doc/html/rfc3066
type Language string

// Selected language tags, see
// https://en.wikipedia.org/wiki/List_of_ISO_639-1_codes
const (
	LangEnglish  = "en"
	LangGerman   = "de"
	LangRomanian = "ro"
)

// Country represents a RFC 3066 country subtag,
// denoting the area to which a language variant relates.
type Country string

// Selected country tags, see
// https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2
const (
	CountryGermany = "DE"
	CountryRomania = "RO"
	CountryUSA     = "US"
	CountryUK      = "GB"
)

// Script indicates writing systems, for use with OpenType fonts
type Script int

// Selected writing systems.
const (
	ScriptCyrillic Script = iota
	ScriptGreek
	ScriptLatin
)

// Locale defines language properties of a text.
type Locale struct {
	Language Language
	Country  Country
	Script   Script
}

func (loc *Locale) String() string {
	if loc == nil || loc.Language == "" && loc.Country == "" {
		return ""
	}
	if loc.Country == "" {
		return string(loc.Language)
	}
	return string(loc.Language) + "-" + string(loc.Country)
}

var (
	// DeDE denotes German as written in Germany.
	DeDE = &Locale{
		Language: LangGerman,
		Country:  CountryGermany,
		Script:   ScriptLatin,
	}

	// EnGB denotes English as written in the UK.
	EnGB = &Locale{
		Language: LangEnglish,
		Country:  CountryUK,
		Script:   ScriptLatin,
	}

	// EnUS denotes English as written in the USA.
	EnUS = &Locale{
		Language: LangEnglish,
		Country:  CountryUSA,
		Script:   ScriptLatin,
	}
)
