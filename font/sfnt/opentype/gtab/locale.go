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
