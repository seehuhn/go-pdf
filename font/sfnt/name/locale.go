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
// Tables from entries with PlatformID == 3 (Apple) have only language set,
// while tables from entries with PlatformID == 1 (Microsoft) have language
// and country set.
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
	11: {Language: locale.LangJapanese},
	12: {Language: locale.LangArabic},
	14: {Language: locale.LangGreek},
	19: {Language: locale.LangChinese}, // traditional
	21: {Language: locale.LangHindi},
	23: {Language: locale.LangTurkish},
	32: {Language: locale.LangRussian},
	33: {Language: locale.LangChinese}, // simplified
	37: {Language: locale.LangRomanian},
	67: {Language: locale.LangBengali},
}

var appleBCP = map[uint16]string{
	0:   "en",         // English
	1:   "fr",         // French
	2:   "de",         // German
	3:   "it",         // Italian
	4:   "nl",         // Dutch
	5:   "sv",         // Swedish
	6:   "es",         // Spanish
	7:   "da",         // Danish
	8:   "pt",         // Portuguese
	9:   "no",         // Norwegian
	10:  "he",         // Hebrew
	11:  "ja",         // Japanese
	12:  "ar",         // Arabic
	13:  "fi",         // Finnish
	14:  "el",         // Greek
	15:  "is",         // Icelandic
	16:  "mt",         // Maltese
	17:  "tr",         // Turkish
	18:  "hr",         // Croatian
	19:  "zh-Hant",    // Chinese (traditional)
	20:  "ur",         // Urdu
	21:  "hi",         // Hindi
	22:  "th",         // Thai
	23:  "ko",         // Korean
	24:  "lt",         // Lithuanian
	25:  "pl",         // Polish
	26:  "hu",         // Hungarian
	27:  "et",         // Estonian
	28:  "lv",         // Latvian
	29:  "se",         // Sami, TODO(voss): is this correct?
	30:  "fo",         // Faroese
	31:  "fa",         // Farsi/Persian
	32:  "ru",         // Russian
	33:  "zh-Hans",    // Chinese (simplified)
	34:  "nl_BE",      // Flemish
	35:  "ga",         // Irish Gaelic
	36:  "sq",         // Albanian
	37:  "ro",         // Romanian
	38:  "cs",         // Czech
	39:  "sk",         // Slovak
	40:  "sl",         // Slovenian
	41:  "yi",         // Yiddish
	42:  "sr",         // Serbian
	43:  "mk",         // Macedonian
	44:  "bg",         // Bulgarian
	45:  "uk",         // Ukrainian
	46:  "be",         // Byelorussian
	47:  "uz",         // Uzbek
	48:  "kk",         // Kazakh
	49:  "az-Cyrl",    // Azerbaijani (Cyrillic script)
	50:  "az-Arab",    // Azerbaijani (Arabic script)
	51:  "hy",         // Armenian
	52:  "ka",         // Georgian
	53:  "mo",         // Moldavian
	54:  "ky",         // Kirghiz
	55:  "tg",         // Tajiki, TODO(voss): is this correct?
	56:  "tk",         // Turkmen
	57:  "mn-Mong",    // Mongolian (Mongolian script)
	58:  "mn-Cyrl",    // Mongolian (Cyrillic script)
	59:  "ps",         // Pashto
	60:  "ku",         // Kurdish
	61:  "ks",         // Kashmiri
	62:  "sd",         // Sindhi
	63:  "bo",         // Tibetan
	64:  "ne",         // Nepali
	65:  "sa",         // Sanskrit
	66:  "mr",         // Marathi
	67:  "bn",         // Bengali
	68:  "as",         // Assamese
	69:  "gu",         // Gujarati
	70:  "pa",         // Punjabi
	71:  "or",         // Oriya
	72:  "ml",         // Malayalam
	73:  "kn",         // Kannada
	74:  "ta",         // Tamil
	75:  "te",         // Telugu
	76:  "si",         // Sinhalese
	77:  "my",         // Burmese
	78:  "km",         // Khmer
	79:  "lo",         // Lao
	80:  "vi",         // Vietnamese
	81:  "id",         // Indonesian
	82:  "tl",         // Tagalog
	83:  "ms-Latn",    // Malay (Roman script)
	84:  "ms-Arab",    // Malay (Arabic script)
	85:  "am",         // Amharic
	86:  "ti",         // Tigrinya
	87:  "om",         // Galla, TODO(voss): is this correct???
	88:  "so",         // Somali
	89:  "sw",         // Swahili
	90:  "rw",         // Kinyarwanda/Ruanda
	91:  "rn",         // Rundi
	92:  "ny",         // Nyanja/Chewa
	93:  "mg",         // Malagasy
	94:  "eo",         // Esperanto
	128: "cy",         // Welsh
	129: "eu",         // Basque
	130: "ca",         // Catalan
	131: "la",         // Latin
	132: "qu",         // Quechua
	133: "gn",         // Guarani
	134: "ay",         // Aymara
	135: "tt",         // Tatar
	136: "ug",         // Uighur
	137: "dz",         // Dzongkha
	138: "jv-Latn",    // Javanese (Roman script)
	139: "su-Latn",    // Sundanese (Roman script)
	140: "gl",         // Galician
	141: "af",         // Afrikaans
	142: "br",         // Breton
	143: "iu",         // Inuktitut
	144: "gd",         // Scottish Gaelic
	145: "gv",         // Manx Gaelic
	146: "ga",         // Irish Gaelic (with dot above), TODO(voss): what to append for overdot?
	147: "to",         // Tongan
	148: "el-polyton", // Greek (polytonic), TODO(voss): is this correct?
	149: "kl",         // Greenlandic
	150: "az-Latn",    // Azerbaijani (Roman script)
}

// Selected Windows language codes
// https://docs.microsoft.com/en-us/typography/opentype/spec/name#windows-language-ids
var msLang = map[uint16]Loc{
	0x0401: {Language: locale.LangArabic, Country: locale.CountrySAU},
	0x0403: {Language: locale.LangCatalan, Country: locale.CountryESP},
	0x0405: {Language: locale.LangCzech, Country: locale.CountryCZE},
	0x0406: {Language: locale.LangDanish, Country: locale.CountryDNK},
	0x0407: {Language: locale.LangGerman, Country: locale.CountryDEU},
	0x0408: {Language: locale.LangGreek, Country: locale.CountryGRC},
	0x0409: {Language: locale.LangEnglish, Country: locale.CountryUSA},
	0x040A: {Language: locale.LangSpanish, Country: locale.CountryESP}, // traditional sort
	0x040B: {Language: locale.LangFinnish, Country: locale.CountryFIN},
	0x040C: {Language: locale.LangFrench, Country: locale.CountryFRA},
	0x040E: {Language: locale.LangHungarian, Country: locale.CountryHUN},
	0x0410: {Language: locale.LangItalian, Country: locale.CountryITA},
	0x0411: {Language: locale.LangJapanese, Country: locale.CountryJPN},
	0x0412: {Language: locale.LangKorean, Country: locale.CountryKOR},
	0x0413: {Language: locale.LangDutch, Country: locale.CountryNLD},
	0x0414: {Language: locale.LangNorwegianBokmal, Country: locale.CountryNOR},
	0x0415: {Language: locale.LangPolish, Country: locale.CountryPOL},
	0x0416: {Language: locale.LangPortuguese, Country: locale.CountryBRA},
	0x0418: {Language: locale.LangRomanian, Country: locale.CountryROU},
	0x0419: {Language: locale.LangRussian, Country: locale.CountryRUS},
	0x041B: {Language: locale.LangSlovak, Country: locale.CountrySVK},
	0x041D: {Language: locale.LangSwedish, Country: locale.CountrySWE},
	0x041F: {Language: locale.LangTurkish, Country: locale.CountryTUR},
	0x0424: {Language: locale.LangSlovenian, Country: locale.CountrySVN},
	0x042D: {Language: locale.LangBasque, Country: locale.CountryESP},
	0x0439: {Language: locale.LangHindi, Country: locale.CountryIND},
	0x0445: {Language: locale.LangBengali, Country: locale.CountryIND},
	0x0804: {Language: locale.LangChinese, Country: locale.CountryCHN},
	0x0809: {Language: locale.LangEnglish, Country: locale.CountryGBR},
	0x080A: {Language: locale.LangSpanish, Country: locale.CountryMEX},
	0x0816: {Language: locale.LangPortuguese, Country: locale.CountryPRT},
	0x0845: {Language: locale.LangBengali, Country: locale.CountryBGD},
	0x0C0A: {Language: locale.LangSpanish, Country: locale.CountryESP}, // modern sort
	0x0C0C: {Language: locale.LangFrench, Country: locale.CountryCAN},
}

// TODO(voss): fill this in
// https://docs.microsoft.com/en-us/openspecs/office_standards/ms-oe376/6c085406-a698-4e12-9d4d-c3b0ee3dbc4a
var msBCP = map[uint16]string{
	0x0436: "af-ZA",       // Afrikaans, South Africa
	0x041c: "sq-AL",       // Albanian, Albania
	0x0484: "gsw-FR",      // Alsatian, France
	0x045e: "am-ET",       // Amharic, Ethiopia
	0x1401: "ar-DZ",       // Arabic, Algeria
	0x3c01: "ar-BH",       // Arabic, Bahrain
	0x0c01: "ar-EG",       // Arabic, Egypt
	0x0801: "ar-IQ",       // Arabic, Iraq
	0x2c01: "ar-JO",       // Arabic, Jordan
	0x3401: "ar-KW",       // Arabic, Kuwait
	0x3001: "ar-LB",       // Arabic, Lebanon
	0x1001: "ar-LY",       // Arabic, Libya
	0x1801: "ar-MO",       // Arabic, Morocco
	0x2001: "ar-OM",       // Arabic, Oman
	0x4001: "ar-QA",       // Arabic, Qatar
	0x0401: "ar-SA",       // Arabic, Saudi Arabia
	0x2801: "ar-SY",       // Arabic, Syria
	0x1c01: "ar-TN",       // Arabic, Tunisia
	0x3801: "ar-AE",       // Arabic, U.A.E.
	0x2401: "ar-YE",       // Arabic, Yemen
	0x042b: "hy-AM",       // Armenian, Armenia
	0x044d: "as-IN",       // Assamese, India
	0x082c: "az-Cyrl-AZ",  // Azeri (Cyrillic), Azerbaijan
	0x042c: "az-Latn-AZ",  // Azeri (Latin), Azerbaijan
	0x046d: "ba-RU",       // Bashkir, Russia
	0x042d: "eu-ES",       // Basque, Basque
	0x0423: "be-BY",       // Belarusian, Belarus
	0x0845: "bn-BD",       // Bengali, Bangladesh
	0x0445: "bn-IN",       // Bengali, India
	0x201a: "bs-Cyrl-BA",  // Bosnian (Cyrillic), Bosnia and Herzegovina
	0x141a: "bs-Latn-BA",  // Bosnian (Latin), Bosnia and Herzegovina
	0x047e: "br-FR",       // Breton, France
	0x0402: "bg-BG",       // Bulgarian, Bulgaria
	0x0403: "ca-ES",       // Catalan, Catalan
	0x0c04: "zh-HK",       // Chinese, Hong Kong S.A.R.
	0x1404: "zh-MO",       // Chinese, Macao S.A.R.
	0x0804: "zh-CN",       // Chinese, People’s Republic of China
	0x1004: "zh-SG",       // Chinese, Singapore
	0x0404: "zh-TW",       // Chinese, Taiwan
	0x0483: "co-FR",       // Corsican, France
	0x041a: "hr-HR",       // Croatian, Croatia
	0x101a: "hr-BA",       // Croatian (Latin), Bosnia and Herzegovina
	0x0405: "cs-CZ",       // Czech, Czech Republic
	0x0406: "da-DK",       // Danish, Denmark
	0x048c: "fa-AF",       // Dari, Afghanistan
	0x0465: "dv-MV",       // Divehi, Maldives
	0x0813: "nl-BE",       // Dutch, Belgium
	0x0413: "nl-NL",       // Dutch, Netherlands
	0x0c09: "en-AU",       // English, Australia
	0x2809: "en-BZ",       // English, Belize
	0x1009: "en-CA",       // English, Canada
	0x2409: "en-029",      // English, Caribbean
	0x4009: "en-IN",       // English, India
	0x1809: "en-IE",       // English, Ireland
	0x2009: "en-JM",       // English, Jamaica
	0x4409: "en-MY",       // English, Malaysia
	0x1409: "en-NZ",       // English, New Zealand
	0x3409: "en-PH",       // English, Republic of the Philippines
	0x4809: "en-SG",       // English, Singapore
	0x1c09: "en-ZA",       // English, South Africa
	0x2c09: "en-TT",       // English, Trinidad and Tobago
	0x0809: "en-GB",       // English, United Kingdom
	0x0409: "en-US",       // English, United States
	0x3009: "en-ZW",       // English, Zimbabwe
	0x0425: "et-EE",       // Estonian, Estonia
	0x0438: "fo-FO",       // Faroese, Faroe Islands
	0x0464: "fil-PH",      // Filipino, Philippines
	0x040b: "fi-FI",       // Finnish, Finland
	0x080c: "fr-BE",       // French, Belgium
	0x0c0c: "fr-CA",       // French, Canada
	0x040c: "fr-FR",       // French, France
	0x140c: "fr-LU",       // French, Luxembourg
	0x180c: "fr-MC",       // French, Principality of Monaco
	0x100c: "fr-CH",       // French, Switzerland
	0x0462: "fy-NL",       // Frisian, Netherlands
	0x0456: "gl-ES",       // Galician, Galician
	0x0437: "ka-GE",       // Georgian, Georgia
	0x0c07: "de-AT",       // German, Austria
	0x0407: "de-DE",       // German, Germany
	0x1407: "de-LI",       // German, Liechtenstein
	0x1007: "de-LU",       // German, Luxembourg
	0x0807: "de-CH",       // German, Switzerland
	0x0408: "el-GR",       // Greek, Greece
	0x046f: "kl-GL",       // Greenlandic, Greenland
	0x0447: "gu-IN",       // Gujarati, India
	0x0468: "ha-Latn-NG",  // Hausa (Latin), Nigeria
	0x040d: "he-IL",       // Hebrew, Israel
	0x0439: "hi-IN",       // Hindi, India
	0x040e: "hu-HU",       // Hungarian, Hungary
	0x040f: "is-IS",       // Icelandic, Iceland
	0x0470: "ig-NG",       // Igbo, Nigeria
	0x0421: "id-ID",       // Indonesian, Indonesia
	0x045d: "iu-Cans-CA",  // Inuktitut, Canada
	0x085d: "iu-Latn-CA",  // Inuktitut (Latin), Canada
	0x083c: "ga-IE",       // Irish, Ireland
	0x0434: "xh-ZA",       // isiXhosa, South Africa
	0x0435: "zu-ZA",       // isiZulu, South Africa
	0x0410: "it-IT",       // Italian, Italy
	0x0810: "it-CH",       // Italian, Switzerland
	0x0411: "ja-JP",       // Japanese, Japan
	0x044b: "kn-IN",       // Kannada, India
	0x043f: "kk-KZ",       // Kazakh, Kazakhstan
	0x0453: "km-KH",       // Khmer, Cambodia
	0x0486: "quc-GT",      // K’iche, Guatemala
	0x0487: "rw-RW",       // Kinyarwanda, Rwanda
	0x0441: "sw-KE",       // Kiswahili, Kenya
	0x0457: "kok-IN",      // Konkani, India
	0x0412: "ko-KR",       // Korean, Korea
	0x0440: "ky-KG",       // Kyrgyz, Kyrgyzstan
	0x0454: "lo-LA",       // Lao, Lao P.D.R.
	0x0426: "lv-LV",       // Latvian, Latvia
	0x0427: "lt-LT",       // Lithuanian, Lithuania
	0x082e: "dsb-DE",      // Lower Sorbian, Germany
	0x046e: "lb-LU",       // Luxembourgish, Luxembourg
	0x042f: "mk-MK",       // Macedonian, North Macedonia
	0x083e: "ms-BN",       // Malay, Brunei Darussalam
	0x043e: "ms-MY",       // Malay, Malaysia
	0x044c: "ml-IN",       // Malayalam, India
	0x043a: "mt-MT",       // Maltese, Malta
	0x0481: "mi-NZ",       // Maori, New Zealand
	0x047a: "arn-CL",      // Mapudungun, Chile
	0x044e: "mr-IN",       // Marathi, India
	0x047c: "moh",         // Mohawk, Mohawk
	0x0450: "mn-Cyrl-MN",  // Mongolian (Cyrillic), Mongolia
	0x0850: "mn-Mong-CN",  // Mongolian (Traditional), People’s Republic of China
	0x0461: "ne-NP",       // Nepali, Nepal
	0x0414: "nb-NO",       // Norwegian (Bokmal), Norway
	0x0814: "nn-NO",       // Norwegian (Nynorsk), Norway
	0x0482: "oc-FR",       // Occitan, France
	0x0448: "or-IN",       // Odia (formerly Oriya), India
	0x0463: "ps-AF",       // Pashto, Afghanistan
	0x0415: "pl-PL",       // Polish, Poland
	0x0416: "pt-BR",       // Portuguese, Brazil
	0x0816: "pt-PT",       // Portuguese, Portugal
	0x0446: "pa-IN",       // Punjabi, India
	0x046b: "quz-BO",      // Quechua, Bolivia
	0x086b: "quz-EC",      // Quechua, Ecuador
	0x0c6b: "quz-PE",      // Quechua, Peru
	0x0418: "ro-RO",       // Romanian, Romania
	0x0417: "rm-CH",       // Romansh, Switzerland
	0x0419: "ru-RU",       // Russian, Russia
	0x243b: "smn-FI",      // Sami (Inari), Finland
	0x103b: "smj-NO",      // Sami (Lule), Norway
	0x143b: "smj-SE",      // Sami (Lule), Sweden
	0x0c3b: "se-FI",       // Sami (Northern), Finland
	0x043b: "se-NO",       // Sami (Northern), Norway
	0x083b: "se-SE",       // Sami (Northern), Sweden
	0x203b: "sms-FI",      // Sami (Skolt), Finland
	0x183b: "sma-NO",      // Sami (Southern), Norway
	0x1c3b: "sma-SE",      // Sami (Southern), Sweden
	0x044f: "sa-IN",       // Sanskrit, India
	0x1c1a: "sr-Cyrl-BA",  // Serbian (Cyrillic), Bosnia and Herzegovina
	0x0c1a: "sr-Cyrl-CS",  // Serbian (Cyrillic), Serbia
	0x181a: "sr-Latn-BA",  // Serbian (Latin), Bosnia and Herzegovina
	0x081a: "sr-Latn-CS",  // Serbian (Latin), Serbia
	0x046c: "nso-ZA",      // Sesotho sa Leboa, South Africa
	0x0432: "tn-ZA",       // Setswana, South Africa
	0x045b: "si-LK",       // Sinhala, Sri Lanka
	0x041b: "sk-SK",       // Slovak, Slovakia
	0x0424: "sl-SI",       // Slovenian, Slovenia
	0x2c0a: "es-AR",       // Spanish, Argentina
	0x400a: "es-BO",       // Spanish, Bolivia
	0x340a: "es-CL",       // Spanish, Chile
	0x240a: "es-CO",       // Spanish, Colombia
	0x140a: "es-CR",       // Spanish, Costa Rica
	0x1c0a: "es-DO",       // Spanish, Dominican Republic
	0x300a: "es-EC",       // Spanish, Ecuador
	0x440a: "es-SV",       // Spanish, El Salvador
	0x100a: "es-GT",       // Spanish, Guatemala
	0x480a: "es-HN",       // Spanish, Honduras
	0x080a: "es-MX",       // Spanish, Mexico
	0x4c0a: "es-NI",       // Spanish, Nicaragua
	0x180a: "es-PA",       // Spanish, Panama
	0x3c0a: "es-PY",       // Spanish, Paraguay
	0x280a: "es-PE",       // Spanish, Peru
	0x500a: "es-PR",       // Spanish, Puerto Rico
	0x0c0a: "es-ES",       // Spanish (Modern Sort), Spain
	0x040a: "es-ES",       // Spanish (Traditional Sort), Spain
	0x540a: "es-US",       // Spanish, United States
	0x380a: "es-UY",       // Spanish, Uruguay
	0x200a: "es-VE",       // Spanish, Venezuela
	0x081d: "sv-FI",       // Swedish, Finland
	0x041d: "sv-SE",       // Swedish, Sweden
	0x045a: "syr-SY",      // Syriac, Syria
	0x0428: "tg-Cyrl-TJ",  // Tajik (Cyrillic), Tajikistan
	0x085f: "tzm-Latn-DZ", // Tamazight (Latin), Algeria
	0x0449: "ta-IN",       // Tamil, India
	0x0444: "tt-RU",       // Tatar, Russia
	0x044a: "te-IN",       // Telugu, India
	0x041e: "th-TH",       // Thai, Thailand
	0x0451: "bo-CN",       // Tibetan, PRC
	0x041f: "tr-TR",       // Turkish, Turkey
	0x0442: "tk-TM",       // Turkmen, Turkmenistan
	0x0480: "ug-Arab-CN",  // Uighur, PRC
	0x0422: "uk-UA",       // Ukrainian, Ukraine
	0x042e: "hsb-DE",      // Upper Sorbian, Germany
	0x0420: "ur-PK",       // Urdu, Islamic Republic of Pakistan
	0x0843: "uz-Cyrl-UZ",  // Uzbek (Cyrillic), Uzbekistan
	0x0443: "uz-Latn-UZ",  // Uzbek (Latin), Uzbekistan
	0x042a: "vi-VN",       // Vietnamese, Vietnam
	0x0452: "cy-GB",       // Welsh, United Kingdom
	0x0488: "wo-SN",       // Wolof, Senegal
	0x0485: "sah-RU",      // Yakut, Russia
	0x0478: "ii-CN",       // Yi, PRC
	0x046a: "yo-NG",       // Yoruba, Nigeria
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
