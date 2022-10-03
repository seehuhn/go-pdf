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

import (
	"errors"

	"golang.org/x/text/language"
)

func getOtfScript(tag string) (string, error) {
	t, err := language.Parse(tag)
	if err != nil {
		return "", err
	}
	script, _ := t.Script()
	scriptName := script.String()
	for key, val := range scriptBcp47 {
		if val == scriptName {
			return key, nil
		}
	}

	// TODO(voss): how to choose non-unique values

	return "", errors.New("no matching OpenType script")
}

// https://docs.microsoft.com/en-us/typography/opentype/spec/scripttags
var scriptBcp47 = map[string]string{
	"DFLT": "Zyyy", // Default

	"adlm": "Adlm", // Adlam
	"ahom": "Ahom", // Ahom
	"hluw": "Hluw", // Anatolian Hieroglyphs
	"arab": "Arab", // Arabic
	"armn": "Armn", // Armenian
	"avst": "Avst", // Avestan
	"bali": "Bali", // Balinese
	"bamu": "Bamu", // Bamum
	"bass": "Bass", // Bassa Vah
	"batk": "Batk", // Batak
	"beng": "Beng", // Bengali
	"bng2": "Beng", // Bengali v.2
	"bhks": "Bhks", // Bhaiksuki
	"bopo": "Bopo", // Bopomofo
	"brah": "Brah", // Brahmi
	"brai": "Brai", // Braille
	"bugi": "Bugi", // Buginese
	"buhd": "Buhd", // Buhid
	"byzm": "Zzzz", // Byzantine Music
	"cans": "Cans", // Unified Canadian Aboriginal Syllabics
	"cari": "Cari", // Carian
	"aghb": "Aghb", // Caucasian Albanian
	"cakm": "Cakm", // Chakma
	"cham": "Cham", // Cham
	"cher": "Cher", // Cherokee
	"chrs": "Chrs", // Chorasmian
	"hani": "Hani", // Han
	"copt": "Copt", // Coptic
	"cprt": "Cprt", // Cypriot syllabary
	"cpmn": "Cpmn", // Cypro-Minoan
	"cyrl": "Cyrl", // Cyrillic
	"dsrt": "Dsrt", // Deseret
	"deva": "Deva", // Devanagari
	"dev2": "Deva", // Devanagari v.2
	"diak": "Diak", // Dives Akuru
	"dogr": "Dogr", // Dogra
	"dupl": "Dupl", // Duployan shorthand
	"egyp": "Egyp", // Egyptian hieroglyphs
	"elba": "Elba", // Elbasan
	"elym": "Elym", // Elymaic
	"ethi": "Ethi", // Ethiopic
	"geor": "Geor", // Georgian (Mkhedruli and Mtavruli)
	"glag": "Glag", // Glagolitic
	"goth": "Goth", // Gothic
	"gran": "Gran", // Grantha
	"grek": "Grek", // Greek
	"gujr": "Gujr", // Gujarati
	"gjr2": "Gujr", // Gujarati v.2
	"gong": "Gong", // Gunjala Gondi
	"guru": "Guru", // Gurmukhi
	"gur2": "Guru", // Gurmukhi v.2
	"hang": "Hang", // Hangul
	"jamo": "Jamo", // Jamo (alias for Jamo subset of Hangul)
	"rohg": "Rohg", // Hanifi Rohingya
	"hano": "Hano", // Hanunoo
	"hatr": "Hatr", // Hatran
	"hebr": "Hebr", // Hebrew
	"armi": "Armi", // Imperial Aramaic
	"phli": "Phli", // Inscriptional Pahlavi
	"prti": "Prti", // Inscriptional Parthian
	"kana": "Hrkt", // Japanese syllabaries (Hiragana + Katakana), see https://github.com/MicrosoftDocs/typography-issues/issues/976
	"java": "Java", // Javanese
	"kthi": "Kthi", // Kaithi
	"knda": "Knda", // Kannada
	"knd2": "Knda", // Kannada v.2
	"kali": "Kali", // Kayah Li
	"khar": "Khar", // Kharoshthi
	"kits": "Kits", // Khitan small script
	"khmr": "Khmr", // Khmer
	"khoj": "Khoj", // Khojki
	"sind": "Sind", // Khudawadi
	"lao ": "Laoo", // Lao
	"latn": "Latn", // Latin
	"lepc": "Lepc", // Lepcha
	"limb": "Limb", // Limbu
	"lina": "Lina", // Linear A
	"linb": "Linb", // Linear B
	"lisu": "Lisu", // Lisu
	"lyci": "Lyci", // Lycian
	"lydi": "Lydi", // Lydian
	"mahj": "Mahj", // Mahajani
	"maka": "Maka", // Makasar
	"mlym": "Mlym", // Malayalam
	"mlm2": "Mlym", // Malayalam v.2
	"mand": "Mand", // Mandaic
	"mani": "Mani", // Manichaean
	"marc": "Marc", // Marchen
	"gonm": "Gonm", // Masaram Gondi
	"math": "Zzzz", // Mathematical Alphanumeric Symbols
	"medf": "Medf", // Medefaidrin
	"mtei": "Mtei", // Meitei Mayek
	"mend": "Mend", // Mende Kikakui
	"merc": "Merc", // Meroitic Cursive
	"mero": "Mero", // Meroitic Hieroglyphs
	"plrd": "Plrd", // Miao
	"modi": "Modi", // Modi
	"mong": "Mong", // Mongolian
	"mroo": "Mroo", // Mro
	"mult": "Mult", // Multani
	"musc": "Zzzz", // Musical Symbols
	"mymr": "Mymr", // Myanmar
	"mym2": "Mymr", // Myanmar v.2
	"nbat": "Nbat", // Nabataean
	"nand": "Nand", // Nandinagari
	"newa": "Newa", // Newa
	"talu": "Talu", // New Tai Lue
	"nko ": "Nkoo", // N'Ko
	"nshu": "Nshu", // Nüshu
	"hmnp": "Hmnp", // Nyiakeng Puachue Hmong
	"orya": "Orya", // Oriya
	"ory2": "Orya", // Oriya v.2
	"ogam": "Ogam", // Ogham
	"olck": "Olck", // Ol Chiki
	"ital": "Ital", // Old Italic (Etruscan, Oscan, etc.)
	"hung": "Hung", // Old Hungarian
	"narb": "Narb", // Old North Arabian
	"perm": "Perm", // Old Permic
	"xpeo": "Xpeo", // Old Persian
	"sogo": "Sogo", // Old Sogdian
	"sarb": "Sarb", // Old South Arabian
	"orkh": "Orkh", // Old Turkic
	// "ougr": "Ougr", // Old Uyghur, TODO(voss): not supported by golang.org/x/text/language
	"osge": "Osge", // Osage
	"osma": "Osma", // Osmanya
	"hmng": "Hmng", // Pahawh Hmong
	"palm": "Palm", // Palmyrene
	"pauc": "Pauc", // Pau Cin Hau
	"phag": "Phag", // Phags-pa
	"phnx": "Phnx", // Phoenician
	"phlp": "Phlp", // Psalter Pahlavi
	"rjng": "Rjng", // Rejang
	"runr": "Runr", // Runic
	"samr": "Samr", // Samaritan
	"saur": "Saur", // Saurashtra
	"shrd": "Shrd", // Sharada
	"shaw": "Shaw", // Shavian
	"sidd": "Sidd", // Siddham
	"sgnw": "Sgnw", // SignWriting
	"sinh": "Sinh", // Sinhala
	"sogd": "Sogd", // Sogdian
	"sora": "Sora", // Sora Sompeng
	"soyo": "Soyo", // Soyombo
	"xsux": "Xsux", // Sumero-Akkadian cuneiform
	"sund": "Sund", // Sundanese
	"sylo": "Sylo", // Syloti Nagri
	"syrc": "Syrc", // Syriac
	"tglg": "Tglg", // Tagalog
	"tagb": "Tagb", // Tagbanwa
	"tale": "Tale", // Tai Le
	"lana": "Lana", // Tai Tham
	"tavt": "Tavt", // Tai Viet
	"takr": "Takr", // Takri
	"taml": "Taml", // Tamil
	"tml2": "Taml", // Tamil v.2
	// "tnsa": "Tnsa", // Tangsa, TODO(voss): not supported by golang.org/x/text/language
	"tang": "Tang", // Tangut
	"telu": "Telu", // Telugu
	"tel2": "Telu", // Telugu v.2
	"thaa": "Thaa", // Thaana
	"thai": "Thai", // Thai
	"tibt": "Tibt", // Tibetan
	"tfng": "Tfng", // Tifinagh
	"tirh": "Tirh", // Tirhuta
	"toto": "Toto", // Toto
	"ugar": "Ugar", // Ugaritic
	"vai ": "Vaii", // Vai
	// "vith": "Vith", // Vithkuqi, TODO(voss): not supported by golang.org/x/text/language
	"wcho": "Wcho", // Wancho
	"wara": "Wara", // Warang Citi
	"yezi": "Yezi", // Yezidi
	"yi  ": "Yiii", // Yi
	"zanb": "Zanb", // Zanabazar Square
}

// https://docs.microsoft.com/en-us/typography/opentype/spec/languagetags
var langBcp47 = map[string]string{
	"ABA ": "abq",        // Abaza
	"ABK ": "ab",         // Abkhazian
	"ACH ": "ach",        // Acoli
	"ACR ": "acr",        // Achi
	"ADY ": "ady",        // Adyghe
	"AFK ": "af",         // Afrikaans
	"AFR ": "aa",         // Afar
	"AGW ": "ahg",        // Qimant
	"AIO ": "aio",        // Aiton
	"AKA ": "ak",         // Akan
	"AKB ": "akb",        // Batak Angkola
	"ALS ": "gsw",        // Alsatian
	"ALT ": "tut",        // Altaic languages
	"AMH ": "am",         // Amharic
	"ANG ": "ang",        // Old English (ca. 450-1100)
	"ARA ": "ar",         // Arabic
	"ARG ": "an",         // Aragonese
	"ARI ": "aiw",        // Aari
	"ARK ": "rki",        // Rakhine
	"ASM ": "as",         // Assamese
	"AST ": "ast",        // Asturian
	"ATH ": "ath",        // Athapascan languages
	"AVN ": "avn",        // Avatime
	"AVR ": "av",         // Avaric
	"AWA ": "awa",        // Awadhi
	"AYM ": "ay",         // Aymara
	"AZB ": "azb",        // South Azerbaijani
	"AZE ": "az",         // Azerbaijani
	"BAD ": "bfq",        // Badaga
	"BAD0": "bad",        // Banda languages
	"BAG ": "bfy",        // Bagheli
	"BAL ": "krc",        // Karachay-Balkar
	"BAN ": "ban",        // Balinese
	"BAR ": "bar",        // Bavarian
	"BAU ": "bci",        // Baoulé
	"BBC ": "bbc",        // Batak Toba
	"BBR ": "ber",        // Berber languages
	"BCH ": "bcq",        // Bench
	"BDY ": "bdy",        // Bandjalang
	"BEL ": "be",         // Belarusian
	"BEM ": "bem",        // Bemba (Zambia)
	"BEN ": "bn",         // Bengali
	"BGC ": "bgc",        // Haryanvi
	"BGQ ": "bgq",        // Bagri
	"BGR ": "bg",         // Bulgarian
	"BHI ": "bhb",        // Bhili
	"BHO ": "bho",        // Bhojpuri
	"BIK ": "bik",        // Bikol
	"BIL ": "byn",        // Bilin
	"BIS ": "bi",         // Bislama
	"BJJ ": "bjj",        // Kanauji
	"BKF ": "bla",        // Siksika
	"BLI ": "bal",        // Baluchi
	"BLK ": "blk",        // Pa'o Karen
	"BLN ": "ble",        // Balanta-Kentohe
	"BLT ": "bft",        // Balti
	"BMB ": "bm",         // Bambara
	"BML ": "bai",        // Bamileke languages
	"BOS ": "bs",         // Bosnian
	"BPY ": "bpy",        // Bishnupriya
	"BRE ": "br",         // Breton
	"BRH ": "brh",        // Brahui
	"BRI ": "bra",        // Braj
	"BRM ": "my",         // Burmese
	"BRX ": "brx",        // Bodo (India)
	"BSH ": "ba",         // Bashkir
	"BSK ": "bsk",        // Burushaski
	"BTD ": "btd",        // Batak Dairi
	"BTI ": "btb",        // Beti (Cameroon)
	"BTK ": "btk",        // Batak languages
	"BTM ": "btm",        // Batak Mandailing
	"BTS ": "bts",        // Batak Simalungun
	"BTX ": "btx",        // Batak Karo
	"BTZ ": "btz",        // Batak Alas-Kluet
	"BUG ": "bug",        // Buginese
	"BYV ": "byv",        // Medumba
	"CAK ": "cak",        // Kaqchikel
	"CAT ": "ca",         // Catalan
	"CBK ": "cbk",        // Chavacano
	"CEB ": "ceb",        // Cebuano
	"CGG ": "cgg",        // Chiga
	"CHA ": "ch",         // Chamorro
	"CHE ": "ce",         // Chechen
	"CHG ": "sgw",        // Sebat Bet Gurage
	"CHH ": "hne",        // Chhattisgarhi
	"CHI ": "ny",         // Chichewa
	"CHK ": "ckt",        // Chukot
	"CHK0": "chk",        // Chuukese
	"CHO ": "cho",        // Choctaw
	"CHP ": "chp",        // Chipewyan
	"CHR ": "chr",        // Cherokee
	"CHU ": "cv",         // Chuvash
	"CHY ": "chy",        // Cheyenne
	"CJA ": "cja",        // Western Cham
	"CJM ": "cjm",        // Eastern Cham
	"COP ": "cop",        // Coptic
	"COR ": "kw",         // Cornish
	"COS ": "co",         // Corsican
	"CPP ": "crp",        // Creoles and pidgins
	"CRE ": "cr",         // Cree
	"CRR ": "crx",        // Carrier
	"CRT ": "crh",        // Crimean Tatar
	"CSB ": "csb",        // Kashubian
	"CSL ": "cu",         // Church Slavonic
	"CSY ": "cs",         // Czech
	"CTG ": "ctg",        // Chittagonian
	"CTT ": "ctt",        // Wayanad Chetti
	"CUK ": "cuk",        // San Blas Kuna
	"DAG ": "dag",        // Dagbani
	"DAN ": "da",         // Danish
	"DAR ": "dar",        // Dargwa
	"DAX ": "dax",        // Dayi
	"DCR ": "cwd",        // Woods Cree
	"DEU ": "de",         // German
	"DGO ": "dgo",        // Dogri (individual language)
	"DGR ": "doi",        // Dogri (macrolanguage)
	"DHG ": "dhg",        // Dhangu
	"DHV ": "dv",         // Dhivehi
	"DIQ ": "diq",        // Dimli (individual language)
	"DIV ": "dv",         // Dhivehi
	"DJR ": "dje",        // Zarma
	"DJR0": "djr",        // Djambarrpuyngu
	"DNG ": "ada",        // Adangme
	"DNJ ": "dnj",        // Dan
	"DNK ": "din",        // Dinka
	"DRI ": "prs",        // Dari
	"DUJ ": "dwu",        // Dhuwal
	"DUN ": "dng",        // Dungan
	"DZN ": "dz",         // Dzongkha
	"EBI ": "igb",        // Ebira
	"EDO ": "bin",        // Edo
	"EFI ": "efi",        // Efik
	"ELL ": "el",         // Modern Greek (1453-)
	"EMK ": "emk",        // Eastern Maninkakan
	"ENG ": "en",         // English
	"ERZ ": "myv",        // Erzya
	"ESP ": "es",         // Spanish
	"ESU ": "esu",        // Central Yupik
	"ETI ": "et",         // Estonian
	"EUQ ": "eu",         // Basque
	"EVK ": "evn",        // Evenki
	"EVN ": "eve",        // Even
	"EWE ": "ee",         // Ewe
	"FAN ": "acf",        // Saint Lucian Creole French
	"FAN0": "fan",        // Fang (Equatorial Guinea)
	"FAR ": "fa",         // Persian
	"FAT ": "fat",        // Fanti
	"FIN ": "fi",         // Finnish
	"FJI ": "fj",         // Fijian
	"FLE ": "nl",         // Dutch
	"FMP ": "fmp",        // Fe'fe'
	"FNE ": "enf",        // Forest Enets
	"FON ": "fon",        // Fon
	"FOS ": "fo",         // Faroese
	"FRA ": "fr",         // French
	"FRC ": "frc",        // Cajun French
	"FRI ": "fy",         // Western Frisian
	"FRL ": "fur",        // Friulian
	"FRP ": "frp",        // Arpitan
	"FTA ": "fuf",        // Pular
	"FUL ": "ff",         // Fulah
	"FUV ": "fuv",        // Nigerian Fulfulde
	"GAD ": "gaa",        // Ga
	"GAE ": "gd",         // Scottish Gaelic
	"GAG ": "gag",        // Gagauz
	"GAL ": "gl",         // Galician
	"GAW ": "gbm",        // Garhwali
	"GEZ ": "gez",        // Geez
	"GIH ": "gih",        // Githabul
	"GIL ": "niv",        // Gilyak
	"GIL0": "gil",        // Gilbertese
	"GKP ": "gkp",        // Guinea Kpelle
	"GLK ": "glk",        // Gilaki
	"GMZ ": "guk",        // Gumuz
	"GNN ": "gnn",        // Gumatj
	"GOG ": "gog",        // Gogo
	"GON ": "gon",        // Gondi
	"GRN ": "kl",         // Greenlandic
	"GRO ": "grt",        // Garo
	"GUA ": "gn",         // Guarani
	"GUC ": "guc",        // Wayuu
	"GUF ": "guf",        // Gupapuyngu
	"GUJ ": "gu",         // Gujarati
	"GUZ ": "guz",        // Gusii
	"HAI ": "ht",         // Haitian Creole
	"HAI0": "hai",        // Haida
	"HAL ": "cfm",        // Falam Chin
	"HAR ": "hoj",        // Hadothi
	"HAU ": "ha",         // Hausa
	"HAW ": "haw",        // Hawaiian
	"HAY ": "hay",        // Haya
	"HAZ ": "haz",        // Hazaragi
	"HBN ": "amf",        // Hamer-Banna
	"HEI ": "hei",        // Heiltsuk
	"HER ": "hz",         // Herero
	"HIL ": "hil",        // Hiligaynon
	"HIN ": "hi",         // Hindi
	"HMA ": "mrj",        // Western Mari
	"HMD ": "hmd",        // Large Flowery Miao
	"HMN ": "hmn",        // Hmong
	"HMO ": "ho",         // Hiri Motu
	"HMZ ": "hmz",        // Hmong Shua
	"HND ": "lah",        // Lahnda
	"HO  ": "hoc",        // Ho
	"HRI ": "har",        // Harari
	"HRV ": "hr",         // Croatian
	"HUN ": "hu",         // Hungarian
	"HYE ": "hy",         // Armenian
	"HYE0": "hy",         // Armenian
	"IBA ": "iba",        // Iban
	"IBB ": "ibb",        // Ibibio
	"IBO ": "ig",         // Igbo
	"IDO ": "io",         // Ido
	"IJO ": "ijo",        // Ijo languages
	"ILE ": "ie",         // Interlingue
	"ILO ": "ilo",        // Iloko
	"INA ": "ia",         // Interlingua (International Auxiliary Language Association)
	"IND ": "id",         // Indonesian
	"ING ": "inh",        // Ingush
	"INU ": "iu",         // Inuktitut
	"INUK": "iu",         // Inuktitut
	"IPK ": "ik",         // Inupiaq
	"IRI ": "ga",         // Irish
	"IRT ": "ga",         // Irish
	"IRU ": "iru",        // Irula
	"ISL ": "is",         // Icelandic
	"ISM ": "smn",        // Inari Sami
	"ITA ": "it",         // Italian
	"IWR ": "he",         // Hebrew
	"JAM ": "jam",        // Jamaican Creole English
	"JAN ": "ja",         // Japanese
	"JAV ": "jv",         // Javanese
	"JBO ": "jbo",        // Lojban
	"JCT ": "jct",        // Krymchak
	"JII ": "yi",         // Yiddish
	"JUD ": "lad",        // Ladino
	"JUL ": "dyu",        // Dyula
	"KAB ": "kbd",        // Kabardian
	"KAB0": "kab",        // Kabyle
	"KAC ": "kfr",        // Kachhi
	"KAL ": "kln",        // Kalenjin
	"KAN ": "kn",         // Kannada
	"KAR ": "krc",        // Karachay-Balkar
	"KAT ": "ka",         // Georgian
	"KAW ": "kaw",        // Kawi
	"KAZ ": "kk",         // Kazakh
	"KDE ": "kde",        // Makonde
	"KEA ": "kea",        // Kabuverdianu
	"KEB ": "ktb",        // Kambaata
	"KEK ": "kek",        // Kekchí
	"KGE ": "ka",         // Georgian
	"KHA ": "kjh",        // Khakas
	"KHK ": "kca",        // Khanty
	"KHM ": "km",         // Khmer
	"KHS ": "kca",        // Khanty
	"KHT ": "kht",        // Khamti
	"KHV ": "kca",        // Khanty
	"KHW ": "khw",        // Khowar
	"KIK ": "ki",         // Kikuyu
	"KIR ": "ky",         // Kirghiz
	"KIU ": "kiu",        // Kirmanjki (individual language)
	"KJD ": "kjd",        // Southern Kiwai
	"KJP ": "kjp",        // Pwo Eastern Karen
	"KJZ ": "kjz",        // Bumthangkha
	"KKN ": "kex",        // Kukna
	"KLM ": "xal",        // Kalmyk
	"KMB ": "kam",        // Kamba (Kenya)
	"KMN ": "kfy",        // Kumaoni
	"KMO ": "kmw",        // Komo (Democratic Republic of Congo)
	"KMS ": "kxc",        // Konso
	"KMZ ": "kmz",        // Khorasani Turkish
	"KNR ": "kr",         // Kanuri
	"KOD ": "kfa",        // Kodava
	"KOH ": "okm",        // Middle Korean (10th-16th cent.)
	"KOK ": "kok",        // Konkani (macrolanguage)
	"KOM ": "kv",         // Komi
	"KON ": "ktu",        // Kituba (Democratic Republic of Congo)
	"KON0": "kg",         // Kongo
	"KOP ": "koi",        // Komi-Permyak
	"KOR ": "ko",         // Korean
	"KOS ": "kos",        // Kosraean
	"KOZ ": "kpv",        // Komi-Zyrian
	"KPL ": "kpe",        // Kpelle
	"KRI ": "kri",        // Krio
	"KRK ": "kaa",        // Karakalpak
	"KRL ": "krl",        // Karelian
	"KRM ": "kdr",        // Karaim
	"KRN ": "kar",        // Karen languages
	"KRT ": "kqy",        // Koorete
	"KSH ": "ks",         // Kashmiri
	"KSH0": "ksh",        // Kölsch
	"KSI ": "kha",        // Khasi
	"KSM ": "sjd",        // Kildin Sami
	"KSW ": "ksw",        // S'gaw Karen
	"KUA ": "kj",         // Kuanyama
	"KUI ": "uki",        // Kui (India)
	"KUL ": "kfx",        // Kullu Pahari
	"KUM ": "kum",        // Kumyk
	"KUR ": "ku",         // Kurdish
	"KUU ": "kru",        // Kurukh
	"KUY ": "kdt",        // Kuy
	"KWK ": "kwk",        // Kwakiutl
	"KYK ": "kpy",        // Koryak
	"KYU ": "kyu",        // Western Kayah
	"LAD ": "lld",        // Ladin
	"LAH ": "bfu",        // Gahri
	"LAK ": "lbe",        // Lak
	"LAM ": "lmn",        // Lambadi
	"LAO ": "lo",         // Lao
	"LAT ": "la",         // Latin
	"LAZ ": "lzz",        // Laz
	"LCR ": "crm",        // Moose Cree
	"LDK ": "lbj",        // Ladakhi
	"LEF ": "lef",        // Lelemi
	"LEZ ": "lez",        // Lezghian
	"LIJ ": "lij",        // Ligurian
	"LIM ": "li",         // Limburgish
	"LIN ": "ln",         // Lingala
	"LIS ": "lis",        // Lisu
	"LJP ": "ljp",        // Lampung Api
	"LKI ": "lki",        // Laki
	"LMA ": "mhr",        // Eastern Mari
	"LMB ": "lif",        // Limbu
	"LMO ": "lmo",        // Lombard
	"LMW ": "ngl",        // Lomwe
	"LOM ": "lom",        // Loma (Liberia)
	"LPO ": "lpo",        // Lipo
	"LRC ": "ldd",        // Luri
	"LSB ": "dsb",        // Lower Sorbian
	"LSM ": "smj",        // Lule Sami
	"LTH ": "lt",         // Lithuanian
	"LTZ ": "lb",         // Luxembourgish
	"LUA ": "lua",        // Luba-Lulua
	"LUB ": "lu",         // Luba-Katanga
	"LUG ": "lg",         // Ganda
	"LUH ": "luy",        // Luyia
	"LUO ": "luo",        // Luo (Kenya and Tanzania)
	"LVI ": "lv",         // Latvian
	"MAD ": "mad",        // Madurese
	"MAG ": "mag",        // Magahi
	"MAH ": "mh",         // Marshallese
	"MAJ ": "mpe",        // Majang
	"MAK ": "vmw",        // Makhuwa
	"MAL ": "ml",         // Malayalam
	"MAM ": "mam",        // Mam
	"MAN ": "mns",        // Mansi
	"MAP ": "arn",        // Mapudungun
	"MAR ": "mr",         // Marathi
	"MAW ": "mwr",        // Marwari
	"MBN ": "kmb",        // Kimbundu
	"MBO ": "mbo",        // Mbo (Cameroon)
	"MCH ": "mnc",        // Manchu
	"MCR ": "crm",        // Moose Cree
	"MDE ": "men",        // Mende (Sierra Leone)
	"MDR ": "mdr",        // Mandar
	"MEN ": "mym",        // Me'en
	"MER ": "mer",        // Meru
	"MFA ": "mfa",        // Pattani Malay
	"MFE ": "mfe",        // Morisyen
	"MIN ": "min",        // Minangkabau
	"MIZ ": "lus",        // Lushai
	"MKD ": "mk",         // Macedonian
	"MKR ": "mak",        // Makasar
	"MKW ": "mkw",        // Kituba (Congo)
	"MLE ": "mdy",        // Male (Ethiopia)
	"MLG ": "mg",         // Malagasy
	"MLN ": "mlq",        // Western Maninkakan
	"MLR ": "ml",         // Malayalam
	"MLY ": "ms",         // Malay (macrolanguage)
	"MND ": "mnk",        // Mandinka
	"MNG ": "mn",         // Mongolian
	"MNI ": "mni",        // Manipuri
	"MNK ": "man",        // Mandingo
	"MNX ": "gv",         // Manx
	"MOH ": "moh",        // Mohawk
	"MOK ": "mdf",        // Moksha
	"MOL ": "ro",         // Moldavian
	"MON ": "mnw",        // Mon
	"MONT": "mnw",        // Mon
	"MOS ": "mos",        // Mossi
	"MRI ": "mi",         // Maori
	"MTH ": "mai",        // Maithili
	"MTS ": "mt",         // Maltese
	"MUN ": "unr",        // Mundari
	"MUS ": "mus",        // Creek
	"MWL ": "mwl",        // Mirandese
	"MWW ": "mww",        // Hmong Daw
	"MYN ": "myn",        // Mayan languages
	"MZN ": "mzn",        // Mazanderani
	"NAG ": "nag",        // Naga Pidgin
	"NAH ": "nah",        // Nahuatl languages
	"NAN ": "gld",        // Nanai
	"NAP ": "nap",        // Neapolitan
	"NAS ": "nsk",        // Naskapi
	"NAU ": "na",         // Nauru
	"NAV ": "nv",         // Navajo
	"NCR ": "csw",        // Swampy Cree
	"NDC ": "ndc",        // Ndau
	"NDG ": "ng",         // Ndonga
	"NDS ": "nds",        // Low Saxon
	"NEP ": "ne",         // Nepali (macrolanguage)
	"NEW ": "new",        // Newari
	"NGA ": "nga",        // Ngbaka
	"NHC ": "csw",        // Swampy Cree
	"NIS ": "dap",        // Nisi (India)
	"NIU ": "niu",        // Niuean
	"NKL ": "nyn",        // Nyankole
	"NKO ": "nqo",        // N'Ko
	"NLD ": "nl",         // Dutch
	"NOE ": "noe",        // Nimadi
	"NOG ": "nog",        // Nogai
	"NOR ": "no",         // Norwegian
	"NOV ": "nov",        // Novial
	"NSM ": "se",         // Northern Sami
	"NSO ": "nso",        // Northern Sotho
	"NTA ": "nod",        // Northern Thai
	"NTO ": "eo",         // Esperanto
	"NYM ": "nym",        // Nyamwezi
	"NYN ": "nn",         // Norwegian Nynorsk
	"NZA ": "nza",        // Tigon Mbembe
	"OCI ": "oc",         // Occitan (post 1500)
	"OCR ": "ojs",        // Severn Ojibwa
	"OJB ": "oj",         // Ojibwa
	"ORI ": "or",         // Oriya (macrolanguage)
	"ORO ": "om",         // Oromo
	"OSS ": "os",         // Ossetian
	"PAA ": "sam",        // Samaritan Aramaic
	"PAG ": "pag",        // Pangasinan
	"PAL ": "pi",         // Pali
	"PAM ": "pam",        // Pampanga
	"PAN ": "pa",         // Punjabi
	"PAP ": "plp",        // Palpa
	"PAP0": "pap",        // Papiamento
	"PAS ": "ps",         // Pashto
	"PAU ": "pau",        // Palauan
	"PCC ": "pcc",        // Bouyei
	"PCD ": "pcd",        // Picard
	"PDC ": "pdc",        // Pennsylvania German
	"PGR ": "el-polyton", // Polytonic Greek
	"PHK ": "phk",        // Phake
	"PIH ": "pih",        // Pitcairn-Norfolk
	"PIL ": "fil",        // Filipino
	"PLK ": "pl",         // Polish
	"PMS ": "pms",        // Piemontese
	"PNB ": "pnb",        // Western Panjabi
	"POH ": "poh",        // Poqomchi'
	"PON ": "pon",        // Pohnpeian
	"PRO ": "pro",        // Old Provençal (to 1500)
	"PTG ": "pt",         // Portuguese
	"PWO ": "pwo",        // Pwo Western Karen
	"QUC ": "quc",        // K'iche'
	"QUH ": "quh",        // South Bolivian Quechua
	"QUZ ": "qu",         // Quechua
	"QVI ": "qvi",        // Imbabura Highland Quichua
	"QWH ": "qwh",        // Huaylas Ancash Quechua
	"RAJ ": "raj",        // Rajasthani
	"RAR ": "rar",        // Rarotongan
	"RBU ": "bxr",        // Russia Buriat
	"RCR ": "atj",        // Atikamekw
	"REJ ": "rej",        // Rejang
	"RIA ": "ria",        // Riang (India)
	"RHG ": "rhg",        // Rohingya
	"RIF ": "rif",        // Tarifit
	"RIT ": "rit",        // Ritharrngu
	"RKW ": "rkw",        // Arakwal
	"RMS ": "rm",         // Romansh
	"RMY ": "rmy",        // Vlax Romani
	"ROM ": "ro",         // Romanian
	"ROY ": "rom",        // Romany
	"RSY ": "rue",        // Rusyn
	"RTM ": "rtm",        // Rotuman
	"RUA ": "rw",         // Kinyarwanda
	"RUN ": "rn",         // Rundi
	"RUP ": "rup",        // Aromanian
	"RUS ": "ru",         // Russian
	"SAD ": "sck",        // Sadri
	"SAN ": "sa",         // Sanskrit
	"SAS ": "sas",        // Sasak
	"SAT ": "sat",        // Santali
	"SAY ": "chp",        // Chipewyan
	"SCN ": "scn",        // Sicilian
	"SCO ": "sco",        // Scots
	"SCS ": "scs",        // North Slavey
	"SEK ": "xan",        // Xamtanga
	"SEL ": "sel",        // Selkup
	"SFM ": "sfm",        // Small Flowery Miao
	"SGA ": "sga",        // Old Irish (to 900)
	"SGO ": "sg",         // Sango
	"SGS ": "sgs",        // Samogitian
	"SHI ": "shi",        // Tachelhit
	"SHN ": "shn",        // Shan
	"SIB ": "nco",        // Sibe
	"SID ": "sid",        // Sidamo
	"SIG ": "stv",        // Silt'e
	"SKS ": "sms",        // Skolt Sami
	"SKY ": "sk",         // Slovak
	"SLA ": "den",        // Slave (Athapascan)
	"SLV ": "sl",         // Slovenian
	"SML ": "so",         // Somali
	"SMO ": "sm",         // Samoan
	"SNA ": "seh",        // Sena
	"SNA0": "sn",         // Shona
	"SND ": "sd",         // Sindhi
	"SNH ": "si",         // Sinhala
	"SNK ": "snk",        // Soninke
	"SOG ": "gru",        // Kistane
	"SOP ": "sop",        // Songe
	"SOT ": "st",         // Southern Sotho
	"SQI ": "sq",         // Albanian
	"SRB ": "sr",         // Serbian
	"SRD ": "sc",         // Sardinian
	"SRK ": "skr",        // Saraiki
	"SRR ": "srr",        // Serer
	"SSL ": "xsl",        // South Slavey
	"SSM ": "sma",        // Southern Sami
	"STQ ": "stq",        // Saterfriesisch
	"SUK ": "suk",        // Sukuma
	"SUN ": "su",         // Sundanese
	"SUR ": "suq",        // Suri
	"SVA ": "sva",        // Svan
	"SVE ": "sv",         // Swedish
	"SWA ": "aii",        // Assyrian Neo-Aramaic
	"SWK ": "sw",         // Swahili (macrolanguage)
	"SWZ ": "ss",         // Swati
	"SXT ": "ngo",        // Ngoni
	"SXU ": "sxu",        // Upper Saxon
	"SYL ": "syl",        // Sylheti
	"SYR ": "syr",        // Syriac
	"SYRE": "syr-Syre",   // Syriac, Estrangela script-variant
	"SYRJ": "syr-Syrj",   // Syriac, Western script-variant
	"SYRN": "syr-Syrn",   // Syriac, Eastern script-variant
	"SZL ": "szl",        // Silesian
	"TAB ": "tab",        // Tabassaran
	"TAJ ": "tg",         // Tajik
	"TAM ": "ta",         // Tamil
	"TAT ": "tt",         // Tatar
	"TCR ": "cwd",        // Woods Cree
	"TDD ": "tdd",        // Tai Nüa
	"TEL ": "te",         // Telugu
	"TET ": "tet",        // Tetum
	"TGL ": "fil",        // Tagalog
	"TGN ": "to",         // Tonga (Tonga Islands)
	"TGR ": "tig",        // Tigre
	"TGY ": "ti",         // Tigrinya
	"THA ": "th",         // Thai
	"THT ": "ty",         // Tahitian
	"TIB ": "bo",         // Tibetan
	"TIV ": "tiv",        // Tiv
	"TJL ": "tjl",        // Tai Laing
	"TKM ": "tk",         // Turkmen
	"TLI ": "tli",        // Tlingit
	"TMH ": "tmh",        // Tamashek
	"TMN ": "tem",        // Timne
	"TNA ": "tn",         // Tswana
	"TNE ": "enh",        // Tundra Enets
	"TNG ": "toi",        // Tonga (Zambia)
	"TOD ": "xal",        // Kalmyk
	"TOD0": "tod",        // Toma
	"TPI ": "tpi",        // Tok Pisin
	"TRK ": "tr",         // Turkish
	"TSG ": "ts",         // Tsonga
	"TSJ ": "tsj",        // Tshangla
	"TUA ": "tru",        // Turoyo
	"TUL ": "tum",        // Tumbuka
	"TUM ": "tcy",        // Tulu
	"TUV ": "tyv",        // Tuvinian
	"TVL ": "tvl",        // Tuvalu
	"TWI ": "tw",         // Twi
	"TYZ ": "tyz",        // Tày
	"TZM ": "tzm",        // Central Atlas Tamazight
	"TZO ": "tzo",        // Tzotzil
	"UDM ": "udm",        // Udmurt
	"UKR ": "uk",         // Ukrainian
	"UMB ": "umb",        // Umbundu
	"URD ": "ur",         // Urdu
	"USB ": "hsb",        // Upper Sorbian
	"UYG ": "ug",         // Uyghur
	"UZB ": "uz",         // Uzbek
	"VEC ": "vec",        // Venetian
	"VEN ": "ve",         // Venda
	"VIT ": "vi",         // Vietnamese
	"VOL ": "vo",         // Volapük
	"VRO ": "vro",        // Võro
	"WA  ": "wbm",        // Wa
	"WAG ": "wbr",        // Wagdi
	"WAR ": "war",        // Waray (Philippines)
	"WCI ": "wci",        // Waci Gbe
	"WCR ": "crk",        // Plains Cree
	"WEL ": "cy",         // Welsh
	"WLF ": "wo",         // Wolof
	"WLN ": "wa",         // Walloon
	"WTM ": "wtm",        // Mewati
	"XBD ": "khb",        // Lü
	"XHS ": "xh",         // Xhosa
	"XJB ": "xjb",        // Minjungbal
	"XKF ": "xkf",        // Khengkha
	"XOG ": "xog",        // Soga
	"XPE ": "xpe",        // Liberia Kpelle
	"XUB ": "xub",        // Betta Kurumba
	"XUJ ": "xuj",        // Jennu Kurumba
	"YAK ": "sah",        // Yakut
	"YAO ": "yao",        // Yao
	"YAP ": "yap",        // Yapese
	"YBA ": "yo",         // Yoruba
	"YCR ": "cr",         // Cree
	"YGP ": "ygp",        // Gepo
	"YIM ": "ii",         // Sichuan Yi
	"YNA ": "yna",        // Aluo
	"YWQ ": "ywq",        // Wuding-Luquan Yi
	"ZEA ": "zea",        // Zeeuws
	"ZGH ": "zgh",        // Standard Moroccan Tamazight
	"ZHA ": "za",         // Zhuang
	"ZHH ": "zh-Hant-HK", // Hong Kong Chinese in traditional script
	"ZHP ": "zh",         // Chinese
	"ZHS ": "zh-Hans",    // simplified Chinese
	"ZHT ": "zh-Hant",    // traditional Chinese
	"ZHTM": "zh-Hant-MO", // Macao Chinese in traditional script
	"ZND ": "znd",        // Zande languages
	"ZUL ": "zu",         // Zulu
	"ZZA ": "zza",        // Zazaki
}
