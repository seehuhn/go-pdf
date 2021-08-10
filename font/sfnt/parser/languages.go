package parser

import "seehuhn.de/go/pdf/locale"

// https://docs.microsoft.com/en-us/typography/opentype/spec/scripttags
var otfScript = map[locale.Script]string{
	locale.ScriptCyrillic: "cyrl",
	locale.ScriptGreek:    "grek",
	locale.ScriptLatin:    "latn",
}

// https://docs.microsoft.com/en-us/typography/opentype/spec/languagetags
var otfLanguage = map[locale.Language]string{
	locale.LangGerman:   "DEU ",
	locale.LangEnglish:  "ENG ",
	locale.LangRomanian: "ROM ",
}
